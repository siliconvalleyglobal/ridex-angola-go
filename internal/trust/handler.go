package trust

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
	"github.com/ridex/ridex-angola/internal/notifications"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	q        *db.Queries
	notifier notifications.Service
}

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

func NewHandlerWithNotifications(q *db.Queries, notifier notifications.Service) *Handler {
	return &Handler{q: q, notifier: notifier}
}

func (h *Handler) SetPaymentMethod(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	var body struct {
		Method string `json:"method" binding:"required,oneof=cash multicaixa card"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "method must be cash, multicaixa, or card"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	ride, err := h.q.SetRidePaymentMethod(c.Request.Context(), db.SetRidePaymentMethodParams{ID: id, PaymentMethod: body.Method, RiderID: uid})
	if err != nil {
		c.JSON(409, gin.H{"error": "payment method cannot be changed for this ride"})
		return
	}
	c.JSON(200, gin.H{"paymentMethod": ride.PaymentMethod})
}

func (h *Handler) CreatePIN(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	code, err := pin()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate trip PIN"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to secure trip PIN"})
		return
	}
	if _, err = h.q.SetRideTripPIN(c.Request.Context(), db.SetRideTripPINParams{ID: id, TripPinHash: pgtype.Text{String: string(hash), Valid: true}, RiderID: uid}); err != nil {
		c.JSON(409, gin.H{"error": "trip PIN cannot be created for this ride"})
		return
	}
	c.JSON(201, gin.H{"tripPin": code})
}

func (h *Handler) VerifyPIN(c *gin.Context) {
	driverID, ok := userID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	var body struct {
		PIN string `json:"pin" binding:"required,len=4"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "pin must be 4 digits"})
		return
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), id)
	if err != nil || !ride.DriverID.Valid || ride.DriverID.Bytes != driverID || !ride.TripPinHash.Valid {
		c.JSON(403, gin.H{"error": "invalid trip PIN request"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(ride.TripPinHash.String), []byte(body.PIN)); err != nil {
		c.JSON(401, gin.H{"error": "invalid trip PIN"})
		return
	}
	if _, err = h.q.VerifyRideTripPIN(c.Request.Context(), db.VerifyRideTripPINParams{ID: id, DriverID: pgtype.UUID{Bytes: driverID, Valid: true}}); err != nil {
		c.JSON(409, gin.H{"error": "trip PIN already verified or ride unavailable"})
		return
	}
	c.JSON(200, gin.H{"verified": true})
}

func (h *Handler) SubmitKYC(c *gin.Context) {
	driverID, ok := userID(c)
	if !ok {
		return
	}
	var body struct {
		IDDocument    string `json:"idDocument" binding:"required,min=3,max=120"`
		LicenseNumber string `json:"licenseNumber" binding:"required,min=3,max=60"`
		VehiclePlate  string `json:"vehiclePlate" binding:"required,min=3,max=30"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "idDocument, licenseNumber, and vehiclePlate are required"})
		return
	}
	kyc, err := h.q.UpsertDriverKYC(c.Request.Context(), db.UpsertDriverKYCParams{DriverID: driverID, IDDocument: body.IDDocument, LicenseNumber: body.LicenseNumber, VehiclePlate: body.VehiclePlate})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to submit driver verification"})
		return
	}
	h.notify(c, driverID, "kyc_update", "Verification submitted", "Your driver verification is now under review.")
	c.JSON(201, gin.H{"kyc": kyc})
}

func (h *Handler) GetKYC(c *gin.Context) {
	driverID, ok := userID(c)
	if !ok {
		return
	}
	kyc, err := h.q.GetDriverKYC(c.Request.Context(), driverID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(404, gin.H{"error": "verification not submitted"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load verification"})
		return
	}
	c.JSON(200, gin.H{"kyc": kyc})
}

func (h *Handler) ReviewKYC(c *gin.Context) {
	driverID, err := uuid.Parse(c.Param("driverId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid driver id"})
		return
	}
	var body struct {
		Status string `json:"status" binding:"required,oneof=under_review approved rejected"`
		Note   string `json:"note" binding:"max=500"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "status must be under_review, approved, or rejected"})
		return
	}
	kyc, err := h.q.ReviewDriverKYC(c.Request.Context(), db.ReviewDriverKYCParams{
		DriverID: driverID, Status: body.Status,
		ReviewNote: pgtype.Text{String: body.Note, Valid: body.Note != ""},
	})
	if err != nil {
		c.JSON(404, gin.H{"error": "driver verification not found"})
		return
	}
	h.notify(c, driverID, "kyc_status", "Verification status updated", "Your driver verification status is now "+kyc.Status+".")
	c.JSON(200, gin.H{"kyc": kyc})
}

func (h *Handler) ListKYC(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	items, err := h.q.ListDriverKYCPage(c.Request.Context(), db.ListDriverKYCPageParams{
		Status: status, Limit: int32(pageSize + 1), Offset: int32((page - 1) * pageSize),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list driver verification"})
		return
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	c.JSON(200, gin.H{
		"kyc": items,
		"pagination": gin.H{
			"page": page, "pageSize": pageSize, "hasMore": hasMore,
		},
	})
}

func userID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}

	return id, true
}

func (h *Handler) notify(c *gin.Context, userID uuid.UUID, kind, title, body string) {
	if h.notifier == nil {
		return
	}
	// Notification persistence is best-effort so a notification outage never
	// changes the result of an otherwise successful KYC mutation.
	_, _ = h.notifier.Notify(c.Request.Context(), notifications.Input{
		UserID: userID, Type: kind, Title: title, Body: body,
	})
}

// pin returns a uniformly distributed 4-digit PIN in [1000, 9999].
// Rejection sampling avoids modulo bias that a simple `1000 + b%9000`
// approach has when only a single random byte is available.
func pin() (string, error) {
	var b [2]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		n := int(b[0])<<8 | int(b[1]) // uniform 0..65535
		if n < 62000 {                // 62000 % 9000 == 0 → unbiased
			return strconv.Itoa(1000 + n%9000), nil
		}
	}
}
