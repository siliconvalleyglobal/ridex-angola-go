package safety

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

// Handler implements emergency-contact and ride-safety endpoints.
type Handler struct {
	q *db.Queries
}

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

type emergencyContactRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=120"`
	Phone        string `json:"phone" binding:"required,min=7,max=30"`
	Relationship string `json:"relationship" binding:"required,min=2,max=60"`
}

type sosRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Note      string   `json:"note" binding:"max=500"`
}

type incidentRequest struct {
	Category    string `json:"category" binding:"required,oneof=accident harassment unsafe_driving vehicle_issue payment_dispute other"`
	Description string `json:"description" binding:"required,min=1,max=4000"`
}

// ListContacts lists only the authenticated user's emergency contacts.
func (h *Handler) ListContacts(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	contacts, err := h.q.ListEmergencyContacts(c.Request.Context(), uid)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list emergency contacts"})
		return
	}
	out := make([]gin.H, 0, len(contacts))
	for _, contact := range contacts {
		out = append(out, contactJSON(contact))
	}
	c.JSON(200, gin.H{"contacts": out})
}

// GetContact returns one contact only when it belongs to the authenticated
// user.
func (h *Handler) GetContact(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	contact, err := h.q.GetEmergencyContact(c.Request.Context(), db.GetEmergencyContactParams{
		ID: id, UserID: uid,
	})
	if err != nil {
		c.JSON(404, gin.H{"error": "emergency contact not found"})
		return
	}
	c.JSON(200, gin.H{"contact": contactJSON(contact)})
}

// CreateContact adds an emergency contact owned by the authenticated user.
func (h *Handler) CreateContact(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	var req emergencyContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	contact, err := h.q.CreateEmergencyContact(c.Request.Context(), db.CreateEmergencyContactParams{
		UserID: uid, Name: req.Name, Phone: req.Phone, Relationship: req.Relationship,
	})
	if err != nil {
		c.JSON(409, gin.H{"error": "emergency contact already exists or is invalid"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"contact": contactJSON(contact)})
}

// UpdateContact updates only a contact belonging to the authenticated user.
func (h *Handler) UpdateContact(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req emergencyContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	contact, err := h.q.UpdateEmergencyContact(c.Request.Context(), db.UpdateEmergencyContactParams{
		ID: id, UserID: uid, Name: req.Name, Phone: req.Phone, Relationship: req.Relationship,
	})
	if err != nil {
		c.JSON(404, gin.H{"error": "emergency contact not found"})
		return
	}
	c.JSON(200, gin.H{"contact": contactJSON(contact)})
}

// DeleteContact deletes only a contact belonging to the authenticated user.
func (h *Handler) DeleteContact(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, err := h.q.DeleteEmergencyContact(c.Request.Context(), db.DeleteEmergencyContactParams{ID: id, UserID: uid}); err != nil {
		c.JSON(404, gin.H{"error": "emergency contact not found"})
		return
	}
	c.Status(204)
}

// TriggerSOS records an active SOS for a participant in an active ride.
func (h *Handler) TriggerSOS(c *gin.Context) {
	uid, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	if ride.Status == "completed" || ride.Status == "cancelled" {
		c.JSON(409, gin.H{"error": "SOS is only available during an active ride"})
		return
	}
	var req sosRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	location, err := optionalPoint(req.Latitude, req.Longitude)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	event, err := h.q.CreateRideSOSEvent(c.Request.Context(), db.CreateRideSOSEventParams{
		RideID: ride.ID, TriggeredBy: uid, Location: location,
		Note: pgtype.Text{String: req.Note, Valid: req.Note != ""},
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to record SOS"})
		return
	}
	if err := h.audit(c, ride.ID, "sos_triggered", uid, gin.H{"sosId": event.ID}); err != nil {
		c.JSON(500, gin.H{"error": "SOS recorded but audit event failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"sos": sosJSON(event)})
}

// ReportIncident records a participant's incident report and appends a ride
// audit event. It does not contact emergency services or an external provider.
func (h *Handler) ReportIncident(c *gin.Context) {
	uid, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	var req incidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	report, err := h.q.CreateRideIncidentReport(c.Request.Context(), db.CreateRideIncidentReportParams{
		RideID: ride.ID, ReportedBy: uid, Category: req.Category, Description: req.Description,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to record incident report"})
		return
	}
	if err := h.audit(c, ride.ID, "incident_reported", uid, gin.H{
		"incidentId": report.ID, "category": report.Category,
	}); err != nil {
		c.JSON(500, gin.H{"error": "incident recorded but audit event failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"incident": incidentJSON(report)})
}

// ListSafetyEvents returns the participant-visible safety records for a ride.
func (h *Handler) ListSafetyEvents(c *gin.Context) {
	_, ride, ok := h.authorizedRide(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	sos, err := h.q.ListRideSOSEvents(ctx, ride.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list safety events"})
		return
	}
	incidents, err := h.q.ListRideIncidentReports(ctx, ride.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list safety events"})
		return
	}
	outSOS := make([]gin.H, 0, len(sos))
	for _, event := range sos {
		outSOS = append(outSOS, sosJSON(event))
	}
	outIncidents := make([]gin.H, 0, len(incidents))
	for _, report := range incidents {
		outIncidents = append(outIncidents, incidentJSON(report))
	}
	c.JSON(200, gin.H{"sos": outSOS, "incidents": outIncidents})
}

// CanAccessRide centralizes participant authorization for safety operations.
// Administrators can inspect ride audit data through existing admin routes but
// cannot create SOS events or alter rider/driver safety records.
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

func (h *Handler) authorizedRide(c *gin.Context) (uuid.UUID, db.Ride, bool) {
	uid, ok := currentUser(c)
	if !ok {
		return uuid.Nil, db.Ride{}, false
	}
	rideID, ok := pathID(c)
	if !ok {
		return uuid.Nil, db.Ride{}, false
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(404, gin.H{"error": "ride not found"})
		return uuid.Nil, db.Ride{}, false
	}
	if !CanAccessRide(uid, c.GetString(auth.ContextRole), ride) {
		c.JSON(403, gin.H{"error": "not a participant in this ride"})
		return uuid.Nil, db.Ride{}, false
	}
	return uid, ride, true
}

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
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return id, true
}

func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func fromPgUUID(value pgtype.UUID) uuid.UUID {
	id, _ := uuid.FromBytes(value.Bytes[:])
	return id
}

func optionalPoint(latitude, longitude *float64) (pgtype.Point, error) {
	if latitude == nil && longitude == nil {
		return pgtype.Point{}, nil
	}
	if latitude == nil || longitude == nil {
		return pgtype.Point{}, errors.New("latitude and longitude must be provided together")
	}
	if *latitude < -90 || *latitude > 90 || *longitude < -180 || *longitude > 180 {
		return pgtype.Point{}, errors.New("invalid location")
	}
	return pgtype.Point{P: pgtype.Vec2{X: *longitude, Y: *latitude}, Valid: true}, nil
}

func contactJSON(contact db.EmergencyContact) gin.H {
	return gin.H{
		"id": contact.ID, "name": contact.Name, "phone": contact.Phone,
		"relationship": contact.Relationship, "createdAt": contact.CreatedAt,
		"updatedAt": contact.UpdatedAt,
	}
}

func sosJSON(event db.RideSosEvent) gin.H {
	return gin.H{
		"id": event.ID, "rideId": event.RideID, "triggeredBy": event.TriggeredBy,
		"status": event.Status, "location": event.Location, "note": event.Note,
		"createdAt": event.CreatedAt,
	}
}

func incidentJSON(report db.RideIncidentReport) gin.H {
	return gin.H{
		"id": report.ID, "rideId": report.RideID, "reportedBy": report.ReportedBy,
		"category": report.Category, "description": report.Description,
		"createdAt": report.CreatedAt,
	}
}
