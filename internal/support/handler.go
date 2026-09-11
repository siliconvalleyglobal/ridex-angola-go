// Package support contains participant feedback and operational support
// workflows. All records are tied to a ride so that access and audit history
// can be evaluated against the ride participants.
package support

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

type Handler struct{ q *db.Queries }

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

type reviewRequest struct {
	Rating  int16  `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment" binding:"max=1000"`
	Review  string `json:"review" binding:"max=1000"`
}

type supportTicketRequest struct {
	Category    string `json:"category" binding:"required,oneof=general payment safety driver rider app_issue other"`
	Subject     string `json:"subject" binding:"required,min=1,max=200"`
	Description string `json:"description" binding:"required,min=1,max=4000"`
}

type ticketStatusRequest struct {
	Status         string `json:"status" binding:"required,oneof=open in_progress resolved closed"`
	ResolutionNote string `json:"resolutionNote" binding:"max=2000"`
}

type disputeRequest struct {
	Category    string `json:"category" binding:"required,oneof=fare payment service safety other"`
	Description string `json:"description" binding:"required,min=1,max=4000"`
}

type disputeStatusRequest struct {
	Status         string `json:"status" binding:"required,oneof=open in_review resolved rejected"`
	ResolutionNote string `json:"resolutionNote" binding:"max=2000"`
}

// CreateReview allows each ride participant to review the other participant
// once after a completed ride. The SQL query repeats the participant and
// completion checks so this invariant also holds outside the HTTP API.
func (h *Handler) CreateReview(c *gin.Context) {
	uid, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	if ride.Status != "completed" || !ride.DriverID.Valid {
		c.JSON(http.StatusConflict, gin.H{"error": "reviews are available after a completed ride"})
		return
	}
	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating must be between 1 and 5 and comment must be at most 1000 characters"})
		return
	}
	if !validRating(req.Rating) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating must be between 1 and 5"})
		return
	}
	comment := req.Comment
	if comment == "" {
		comment = req.Review
	}
	reviewee := ride.RiderID
	if uid == ride.RiderID {
		reviewee = fromPgUUID(ride.DriverID)
	}
	review, err := h.q.CreateRideReview(c.Request.Context(), db.CreateRideReviewParams{
		RideID: ride.ID, ReviewerID: uid, RevieweeID: reviewee,
		Rating: req.Rating, Comment: comment,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusConflict, gin.H{"error": "you have already reviewed this ride or the ride is not reviewable"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create review"})
		return
	}
	if _, err := h.q.RefreshUserRating(c.Request.Context(), reviewee); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "review created but rating could not be refreshed"})
		return
	}
	if err := h.audit(c, ride.ID, "review_created", uid, gin.H{
		"reviewId": review.ID, "revieweeId": reviewee, "rating": review.Rating,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "review created but audit event failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"review": reviewJSON(review)})
}

// ListReviews returns reviews for a ride only to its rider or assigned driver.
func (h *Handler) ListReviews(c *gin.Context) {
	_, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	reviews, err := h.q.GetRideReviews(c.Request.Context(), ride.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reviews"})
		return
	}
	out := make([]gin.H, 0, len(reviews))
	for _, review := range reviews {
		out = append(out, reviewJSON(review))
	}
	c.JSON(http.StatusOK, gin.H{"reviews": out})
}

func (h *Handler) CreateSupportTicket(c *gin.Context) {
	uid, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	if !isParticipantRole(c.GetString(auth.ContextRole)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only riders and drivers can create support tickets"})
		return
	}
	var req supportTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid support ticket body: " + err.Error()})
		return
	}
	ticket, err := h.q.CreateSupportTicket(c.Request.Context(), db.CreateSupportTicketParams{
		RideID: ride.ID, CreatedBy: uid, Category: req.Category,
		Subject: req.Subject, Description: req.Description,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you are not a participant in this ride"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create support ticket"})
		return
	}
	if err := h.audit(c, ride.ID, "support_ticket_created", uid, gin.H{"ticketId": ticket.ID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "support ticket created but audit event failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ticket": ticketJSON(ticket)})
}

func (h *Handler) ListSupportTickets(c *gin.Context) {
	_, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	tickets, err := h.q.ListSupportTicketsForRide(c.Request.Context(), ride.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list support tickets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tickets": ticketJSONList(tickets)})
}

func (h *Handler) ListMySupportTickets(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	page := parsePage(c)
	tickets, err := h.q.ListSupportTicketsByUser(c.Request.Context(), db.ListSupportTicketsByUserParams{
		CreatedBy: uid, Limit: int32(page.size + 1), Offset: int32(page.offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list support tickets"})
		return
	}
	hasMore := len(tickets) > page.size
	if hasMore {
		tickets = tickets[:page.size]
	}
	c.JSON(http.StatusOK, gin.H{"tickets": ticketJSONList(tickets), "pagination": page.response(hasMore)})
}

func (h *Handler) AdminSupportTickets(c *gin.Context) {
	page := parsePage(c)
	tickets, err := h.q.ListSupportTicketsAdmin(c.Request.Context(), db.ListSupportTicketsAdminParams{
		Column1: c.Query("status"), Limit: int32(page.size + 1), Offset: int32(page.offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list support tickets"})
		return
	}
	hasMore := len(tickets) > page.size
	if hasMore {
		tickets = tickets[:page.size]
	}
	c.JSON(http.StatusOK, gin.H{"tickets": ticketJSONList(tickets), "pagination": page.response(hasMore)})
}

func (h *Handler) AdminUpdateSupportTicket(c *gin.Context) {
	adminID, ok := currentUser(c)
	if !ok {
		return
	}
	ticketID, err := uuid.Parse(c.Param("ticketId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	var req ticketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid support ticket status: " + err.Error()})
		return
	}
	ticket, err := h.q.GetSupportTicket(c.Request.Context(), ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "support ticket not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load support ticket"})
		return
	}
	updated, err := h.q.UpdateSupportTicketStatus(c.Request.Context(), db.UpdateSupportTicketStatusParams{
		ID: ticketID, Status: req.Status,
		ResolutionNote: optionalText(req.ResolutionNote), UpdatedBy: pgtype.UUID{Bytes: adminID, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update support ticket"})
		return
	}
	if err := h.audit(c, ticket.RideID, "support_ticket_status_updated", adminID, gin.H{
		"ticketId": ticket.ID, "status": updated.Status,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "support ticket updated but audit event failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticketJSON(updated)})
}

func (h *Handler) CreateDispute(c *gin.Context) {
	uid, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	var req disputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dispute body: " + err.Error()})
		return
	}
	dispute, err := h.q.CreateRideDispute(c.Request.Context(), db.CreateRideDisputeParams{
		RideID: ride.ID, FiledBy: uid, Category: req.Category, Description: req.Description,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you are not a participant in this ride"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create dispute"})
		return
	}
	if err := h.audit(c, ride.ID, "dispute_created", uid, gin.H{"disputeId": dispute.ID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dispute created but audit event failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"dispute": disputeJSON(dispute)})
}

func (h *Handler) ListDisputes(c *gin.Context) {
	_, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	disputes, err := h.q.ListRideDisputes(c.Request.Context(), ride.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list disputes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"disputes": disputeJSONList(disputes)})
}

func (h *Handler) AdminDisputes(c *gin.Context) {
	page := parsePage(c)
	disputes, err := h.q.ListRideDisputesAdmin(c.Request.Context(), db.ListRideDisputesAdminParams{
		Column1: c.Query("status"), Limit: int32(page.size + 1), Offset: int32(page.offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list disputes"})
		return
	}
	hasMore := len(disputes) > page.size
	if hasMore {
		disputes = disputes[:page.size]
	}
	c.JSON(http.StatusOK, gin.H{"disputes": disputeJSONList(disputes), "pagination": page.response(hasMore)})
}

func (h *Handler) AdminUpdateDispute(c *gin.Context) {
	adminID, ok := currentUser(c)
	if !ok {
		return
	}
	disputeID, err := uuid.Parse(c.Param("disputeId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dispute id"})
		return
	}
	var req disputeStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dispute status: " + err.Error()})
		return
	}
	dispute, err := h.q.GetRideDispute(c.Request.Context(), disputeID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "dispute not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load dispute"})
		return
	}
	updated, err := h.q.UpdateRideDisputeStatus(c.Request.Context(), db.UpdateRideDisputeStatusParams{
		ID: disputeID, Status: req.Status,
		ResolutionNote: optionalText(req.ResolutionNote),
		ResolvedBy:     pgtype.UUID{Bytes: adminID, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update dispute"})
		return
	}
	if err := h.audit(c, dispute.RideID, "dispute_status_updated", adminID, gin.H{
		"disputeId": dispute.ID, "status": updated.Status,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dispute updated but audit event failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"dispute": disputeJSON(updated)})
}

func (h *Handler) authorizedRide(c *gin.Context) (uuid.UUID, db.Ride, bool) {
	uid, ok := currentUser(c)
	if !ok {
		return uuid.Nil, db.Ride{}, false
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ride id"})
		return uuid.Nil, db.Ride{}, false
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), rideID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		return uuid.Nil, db.Ride{}, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load ride"})
		return uuid.Nil, db.Ride{}, false
	}
	if !CanAccessRide(uid, c.GetString(auth.ContextRole), ride) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a participant in this ride"})
		return uuid.Nil, db.Ride{}, false
	}
	return uid, ride, true
}

// CanAccessRide is kept pure so authorization remains easy to test and reuse.
func CanAccessRide(userID uuid.UUID, role string, ride db.Ride) bool {
	switch role {
	case "rider":
		return ride.RiderID == userID
	case "driver":
		return ride.DriverID.Valid && fromPgUUID(ride.DriverID) == userID
	default:
		return false
	}
}

func isParticipantRole(role string) bool { return role == "rider" || role == "driver" }

func validRating(rating int16) bool { return rating >= 1 && rating <= 5 }

func (h *Handler) audit(c *gin.Context, rideID uuid.UUID, eventType string, by uuid.UUID, payload gin.H) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = h.q.CreateRideEvent(c.Request.Context(), db.CreateRideEventParams{
		RideID: rideID, EventType: eventType, Payload: data,
		CreatedBy: pgtype.UUID{Bytes: by, Valid: true},
	})
	return err
}

func currentUser(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return id, true
}

func fromPgUUID(value pgtype.UUID) uuid.UUID {
	id, _ := uuid.FromBytes(value.Bytes[:])
	return id
}

func optionalText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

type page struct {
	number int
	size   int
	offset int
}

func parsePage(c *gin.Context) page {
	number, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if number < 1 {
		number = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page{number: number, size: size, offset: (number - 1) * size}
}

func (p page) response(hasMore bool) gin.H {
	return gin.H{"page": p.number, "pageSize": p.size, "hasMore": hasMore}
}

func reviewJSON(review db.RideReview) gin.H {
	return gin.H{
		"id": review.ID, "rideId": review.RideID, "reviewerId": review.ReviewerID,
		"revieweeId": review.RevieweeID, "rating": review.Rating, "comment": review.Comment,
		"createdAt": review.CreatedAt,
	}
}

func ticketJSON(ticket db.SupportTicket) gin.H {
	return gin.H{
		"id": ticket.ID, "rideId": ticket.RideID, "createdBy": ticket.CreatedBy,
		"category": ticket.Category, "subject": ticket.Subject, "description": ticket.Description,
		"status": ticket.Status, "resolutionNote": ticket.ResolutionNote, "updatedBy": ticket.UpdatedBy,
		"createdAt": ticket.CreatedAt, "updatedAt": ticket.UpdatedAt, "resolvedAt": ticket.ResolvedAt,
	}
}

func ticketJSONList(tickets []db.SupportTicket) []gin.H {
	out := make([]gin.H, 0, len(tickets))
	for _, ticket := range tickets {
		out = append(out, ticketJSON(ticket))
	}
	return out
}

func disputeJSON(dispute db.RideDispute) gin.H {
	return gin.H{
		"id": dispute.ID, "rideId": dispute.RideID, "filedBy": dispute.FiledBy,
		"category": dispute.Category, "description": dispute.Description, "status": dispute.Status,
		"resolutionNote": dispute.ResolutionNote, "resolvedBy": dispute.ResolvedBy,
		"createdAt": dispute.CreatedAt, "updatedAt": dispute.UpdatedAt, "resolvedAt": dispute.ResolvedAt,
	}
}

func disputeJSONList(disputes []db.RideDispute) []gin.H {
	out := make([]gin.H, 0, len(disputes))
	for _, dispute := range disputes {
		out = append(out, disputeJSON(dispute))
	}
	return out
}
