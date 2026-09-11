// Identity verification HTTP handlers: submit (rider/driver), check own
// status, and the admin review queue.
package identity

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type submitRequest struct {
	IDDocumentType string `json:"idDocumentType" binding:"required"`
	IDDocumentURL  string `json:"idDocumentUrl" binding:"required,max=1024"`
	SelfieURL      string `json:"selfieUrl" binding:"required,max=1024"`
}

type reviewRequest struct {
	Approve    bool    `json:"approve"`
	Reason     string  `json:"reason" binding:"max=500"`
	Confidence float64 `json:"confidence"`
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return id, true
}

// Submit records the caller's ID document + selfie for review.
func (h *Handler) Submit(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "idDocumentType, idDocumentUrl and selfieUrl are required"})
		return
	}
	verification, err := h.service.Submit(c.Request.Context(), userID, req.IDDocumentType, req.IDDocumentURL, req.SelfieURL)
	if errors.Is(err, ErrInvalidDocument) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit verification"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"verification": verificationJSON(verification)})
}

// GetMine returns the caller's latest verification status.
func (h *Handler) GetMine(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	verification, err := h.service.Mine(c.Request.Context(), userID)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no identity verification submitted"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load verification"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"verification": verificationJSON(verification)})
}

// AdminPending lists verifications awaiting review.
func (h *Handler) AdminPending(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	verifications, err := h.service.Pending(c.Request.Context(), int32(limit), int32(offset))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pending verifications"})
		return
	}
	out := make([]gin.H, 0, len(verifications))
	for _, v := range verifications {
		out = append(out, verificationJSON(v))
	}
	c.JSON(http.StatusOK, gin.H{"verifications": out})
}

// AdminReview approves or rejects one verification.
func (h *Handler) AdminReview(c *gin.Context) {
	reviewerID, ok := currentUserID(c)
	if !ok {
		return
	}
	verificationID, err := uuid.Parse(c.Param("verificationId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid verification id"})
		return
	}
	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	verification, err := h.service.Review(c.Request.Context(), verificationID, reviewerID, req.Approve, req.Reason, req.Confidence)
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "verification not found"})
	case errors.Is(err, ErrNotPending):
		c.JSON(http.StatusConflict, gin.H{"error": "verification is not pending"})
	case errors.Is(err, ErrInvalidDocument):
		c.JSON(http.StatusBadRequest, gin.H{"error": "a rejection reason is required"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to review verification"})
	default:
		c.JSON(http.StatusOK, gin.H{"verification": verificationJSON(verification)})
	}
}

func verificationJSON(v db.IdentityVerification) gin.H {
	return gin.H{
		"id": v.ID, "userId": v.UserID, "status": v.Status,
		"idDocumentType": v.IDDocumentType,
		"verifiedAt":     v.VerifiedAt, "expiresAt": v.ExpiresAt,
		"rejectionReason": v.RejectionReason, "createdAt": v.CreatedAt,
	}
}
