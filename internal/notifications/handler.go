package notifications

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

type Handler struct {
	service Service
	tokens  DeviceTokenStore
}

// DeviceTokenStore persists push-notification device registrations.
type DeviceTokenStore interface {
	UpsertDeviceToken(context.Context, db.UpsertDeviceTokenParams) (db.DeviceToken, error)
	ListDeviceTokensByUser(context.Context, uuid.UUID) ([]db.DeviceToken, error)
	DeleteDeviceToken(context.Context, db.DeleteDeviceTokenParams) error
}

// NewHandler wires the notification handler. tokens may be nil (device
// registration endpoints will fail with a clear error) so existing callers
// keep working.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// NewHandlerWithTokens wires the notification handler with device-token
// persistence for push registration.
func NewHandlerWithTokens(service Service, tokens DeviceTokenStore) *Handler {
	return &Handler{service: service, tokens: tokens}
}

// ── Device token registration ─────────────────────────────────────────

type registerTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform" binding:"required"`
}

// RegisterDevice records or refreshes a push device token for the caller.
func (h *Handler) RegisterDevice(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req registerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token and platform are required"})
		return
	}
	if h.tokens == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device registration is not available"})
		return
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform != "ios" && platform != "android" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform must be ios or android"})
		return
	}
	token, err := h.tokens.UpsertDeviceToken(c.Request.Context(), db.UpsertDeviceTokenParams{
		UserID: userID, Token: req.Token, Platform: platform,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"device": token})
}

// UnregisterDevice removes the caller's push registration for a token.
func (h *Handler) UnregisterDevice(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	if h.tokens == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "device registration is not available"})
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}
	if err := h.tokens.DeleteDeviceToken(c.Request.Context(), db.DeleteDeviceTokenParams{
		Token: token, UserID: userID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unregister device"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListUnread(c *gin.Context) {
	h.list(c, true)
}

func (h *Handler) ListRead(c *gin.Context) {
	h.list(c, false)
}

func (h *Handler) list(c *gin.Context, unread bool) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	page, pageSize := pagination(c)
	items, err := h.service.List(c.Request.Context(), userID, unread, int32(pageSize+1), int32((page-1)*pageSize))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notifications"})
		return
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	response := make([]NotificationJSON, 0, len(items))
	for _, item := range items {
		response = append(response, ToJSON(item))
	}
	c.JSON(http.StatusOK, gin.H{
		"notifications": response,
		"pagination":    gin.H{"page": page, "pageSize": pageSize, "hasMore": hasMore},
	})
}

func (h *Handler) MarkRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}
	item, err := h.service.MarkRead(c.Request.Context(), userID, notificationID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark notification read"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"notification": ToJSON(item)})
}

func (h *Handler) GetPreferences(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	prefs, err := h.service.Preferences(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load notification preferences"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"preferences": prefs})
}

type preferencesRequest struct {
	RideUpdates    *bool `json:"rideUpdates"`
	PaymentUpdates *bool `json:"paymentUpdates"`
	KycUpdates     *bool `json:"kycUpdates"`
	Marketing      *bool `json:"marketing"`
}

func (h *Handler) SavePreferences(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var request preferencesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification preferences"})
		return
	}
	current, err := h.service.Preferences(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load notification preferences"})
		return
	}
	if request.RideUpdates != nil {
		current.RideUpdates = *request.RideUpdates
	}
	if request.PaymentUpdates != nil {
		current.PaymentUpdates = *request.PaymentUpdates
	}
	if request.KycUpdates != nil {
		current.KycUpdates = *request.KycUpdates
	}
	if request.Marketing != nil {
		current.Marketing = *request.Marketing
	}
	prefs, err := h.service.SavePreferences(c.Request.Context(), PreferencesInput{
		UserID: userID, RideUpdates: current.RideUpdates,
		PaymentUpdates: current.PaymentUpdates, KycUpdates: current.KycUpdates,
		Marketing: current.Marketing,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save notification preferences"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"preferences": prefs})
}

func (h *Handler) DeletePreferences(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	if err := h.service.DeletePreferences(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete notification preferences"})
		return
	}
	c.Status(http.StatusNoContent)
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return userID, true
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	return page, pageSize
}
