package airport

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/auth"
	"go.uber.org/zap"
)

// Handler handles HTTP requests for airport operations.
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a new airport handler backed by the airport service.
func NewHandler(store Store, logger *zap.Logger) *Handler {
	return &Handler{
		service: NewService(store, logger),
		logger:  logger,
	}
}

// Service exposes the underlying service (used for flight-provider wiring).
func (h *Handler) Service() *Service { return h.service }

// currentUser resolves the authenticated user from the request context.
func currentUser(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return id, true
}

// ListAirports handles GET /airports.
func (h *Handler) ListAirports(c *gin.Context) {
	airports, err := h.service.ListAirports(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list airports", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list airports"})
		return
	}
	if airports == nil {
		airports = []Airport{}
	}
	c.JSON(http.StatusOK, gin.H{"airports": airports})
}

// GetAirport handles GET /airports/:id.
func (h *Handler) GetAirport(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid airport ID"})
		return
	}
	airport, err := h.service.GetAirport(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrAirportNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airport not found"})
			return
		}
		h.logger.Error("failed to get airport", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get airport"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"airport": airport})
}

// GetAirportByIATA handles GET /airports/iata/:code.
func (h *Handler) GetAirportByIATA(c *gin.Context) {
	iataCode := c.Param("code")
	if iataCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IATA code is required"})
		return
	}
	airport, err := h.service.GetAirportByIATA(c.Request.Context(), iataCode)
	if err != nil {
		if errors.Is(err, ErrAirportNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airport not found"})
			return
		}
		h.logger.Error("failed to get airport by IATA", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get airport"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"airport": airport})
}

// CreateTransfer handles POST /airport/transfers. The transfer is created
// only for a ride owned by the authenticated rider.
func (h *Handler) CreateTransfer(c *gin.Context) {
	callerID, ok := currentUser(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var input AirportTransferCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if input.RideID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ride_id is required"})
		return
	}
	if input.AirportID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "airport_id is required"})
		return
	}

	transfer, err := h.service.CreateTransfer(c.Request.Context(), callerID, input)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not have access to this ride"})
			return
		}
		if errors.Is(err, ErrRideNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
			return
		}
		if errors.Is(err, ErrInvalidFlightNumber) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flight number format"})
			return
		}
		if errors.Is(err, ErrAirportNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "airport not found"})
			return
		}
		h.logger.Error("failed to create transfer", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transfer"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"transfer": transfer})
}

// ListTransfers handles GET /airport-transfers.
func (h *Handler) ListTransfers(c *gin.Context) {
	transfers, err := h.service.ListTransfers(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list transfers", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list transfers"})
		return
	}
	if transfers == nil {
		transfers = []AirportTransfer{}
	}
	c.JSON(http.StatusOK, gin.H{"transfers": transfers})
}

// GetTransfer handles GET /airport-transfers/:id.
func (h *Handler) GetTransfer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transfer ID"})
		return
	}
	transfer, err := h.service.GetTransfer(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrTransferNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "transfer not found"})
			return
		}
		h.logger.Error("failed to get transfer", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get transfer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"transfer": transfer})
}

// UpdateTransfer handles PATCH /airport/transfers/:id. At least one updatable
// field is required; status transitions and participant access are validated
// server-side.
func (h *Handler) UpdateTransfer(c *gin.Context) {
	callerID, ok := currentUser(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transfer ID"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var input AirportTransferUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if input.Status == nil && input.ActualArrival == nil && input.WaitingMinutes == nil && input.Notes == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one updatable field is required"})
		return
	}
	transfer, err := h.service.UpdateTransfer(c.Request.Context(), callerID, id, input)
	if err != nil {
		if errors.Is(err, ErrTransferNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "transfer not found"})
			return
		}
		if errors.Is(err, ErrRideNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
			return
		}
		if errors.Is(err, ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not have access to this transfer"})
			return
		}
		if errors.Is(err, ErrInvalidStatusTransition) {
			c.JSON(http.StatusConflict, gin.H{"error": "invalid transfer status transition"})
			return
		}
		h.logger.Error("failed to update transfer", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update transfer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"transfer": transfer})
}

// CancelTransfer handles POST /airport/transfers/:id/cancel.
func (h *Handler) CancelTransfer(c *gin.Context) {
	callerID, ok := currentUser(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transfer ID"})
		return
	}
	var input struct {
		Reason string `json:"reason" binding:"max=240"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	transfer, err := h.service.CancelTransfer(c.Request.Context(), callerID, id, input.Reason)
	if err != nil {
		if errors.Is(err, ErrAlreadyCompleted) {
			c.JSON(http.StatusConflict, gin.H{"error": "transfer already completed or cancelled"})
			return
		}
		if errors.Is(err, ErrRideNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
			return
		}
		if errors.Is(err, ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not have access to this transfer"})
			return
		}
		h.logger.Error("failed to cancel transfer", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel transfer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"transfer": transfer})
}
