package payouts

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

// Handler exposes wallet and payout endpoints. Mutations are delegated to the
// ledger, which owns its transactions.
type Handler struct {
	ledger *Ledger
}

// NewHandler wires payout routes to a transaction-aware ledger.
func NewHandler(ledger *Ledger) *Handler { return &Handler{ledger: ledger} }

func parseUser(c *gin.Context) (uuid.UUID, bool) {
	uid, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return uid, true
}

func paginateFromQuery(c *gin.Context) (int32, int32) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	return int32(limit), int32(offset)
}

// ── Driver wallet ──────────────────────────────────────────────────────

// Wallet returns the driver's balance and rolling earnings summary.
func (h *Handler) Wallet(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	overview, err := h.ledger.WalletOverview(c.Request.Context(), driverID)
	switch {
	case errors.Is(err, ErrWalletNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "wallet not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load wallet"})
	default:
		c.JSON(http.StatusOK, gin.H{
			"wallet":          WalletJSON(overview.Wallet),
			"earningsSummary": overview.EarningsSummary,
		})
	}
}

// WalletTransactions lists the driver's wallet ledger entries.
func (h *Handler) WalletTransactions(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	limit, offset := paginateFromQuery(c)
	transactions, err := h.ledger.WalletTransactions(c.Request.Context(), driverID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list wallet transactions"})
		return
	}
	jsonList := make([]gin.H, 0, len(transactions))
	for _, tx := range transactions {
		jsonList = append(jsonList, WalletTransactionJSON(tx))
	}
	c.JSON(http.StatusOK, gin.H{"transactions": jsonList})
}

// ── Driver payouts ────────────────────────────────────────────────────

type requestWithdrawalRequest struct {
	AmountCents int64  `json:"amountCents" binding:"required,gt=0"`
	Method      string `json:"method" binding:"required"`
	Notes       string `json:"notes"`
}

// RequestWithdrawal opens a withdrawal for the authenticated driver.
func (h *Handler) RequestWithdrawal(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req requestWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amountCents and method are required"})
		return
	}
	request, _, err := h.ledger.RequestWithdrawal(c.Request.Context(), WithdrawalInput{
		DriverID: driverID, AmountCents: req.AmountCents, Method: req.Method, Description: req.Notes,
	})
	switch {
	case errors.Is(err, ErrPayoutAlreadyPending):
		c.JSON(http.StatusConflict, gin.H{"error": "a payout is already pending or processing"})
	case errors.Is(err, ErrInsufficientBalance):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "insufficient wallet balance"})
	case errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payout request"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to request payout"})
	default:
		c.JSON(http.StatusCreated, gin.H{"payout": PayoutRequestJSON(request)})
	}
}

// DriverPayouts lists the authenticated driver's payout requests.
func (h *Handler) DriverPayouts(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	limit, offset := paginateFromQuery(c)
	requests, err := h.ledger.DriverPayouts(c.Request.Context(), driverID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payouts"})
		return
	}
	jsonList := make([]gin.H, 0, len(requests))
	for _, req := range requests {
		jsonList = append(jsonList, PayoutRequestJSON(req))
	}
	c.JSON(http.StatusOK, gin.H{"payouts": jsonList})
}

// ── Admin payout queue ────────────────────────────────────────────────

// AdminPayoutQueue lists payout requests for the admin console, optionally
// filtered by status (?status=processing).
func (h *Handler) AdminPayoutQueue(c *gin.Context) {
	limit, offset := paginateFromQuery(c)
	requests, err := h.ledger.AdminPayoutQueue(c.Request.Context(),
		strings.ToLower(strings.TrimSpace(c.Query("status"))), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payout queue"})
		return
	}
	jsonList := make([]gin.H, 0, len(requests))
	for _, req := range requests {
		jsonList = append(jsonList, PayoutRequestJSON(req))
	}
	c.JSON(http.StatusOK, gin.H{"payouts": jsonList})
}

type payoutActionRequest struct {
	Reason string `json:"reason"`
}

func parsePayoutID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("payoutId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payout id"})
		return uuid.Nil, false
	}
	return id, true
}

func bindReason(c *gin.Context) string {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	var req payoutActionRequest
	_ = c.ShouldBindJSON(&req) // reason is optional
	return strings.TrimSpace(req.Reason)
}

// ApprovePayout promotes a pending withdrawal to processing.
func (h *Handler) ApprovePayout(c *gin.Context) {
	id, ok := parsePayoutID(c)
	if !ok {
		return
	}
	request, err := h.ledger.ApprovePayout(c.Request.Context(), id)
	switch {
	case errors.Is(err, ErrPayoutNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "payout not found"})
	case errors.Is(err, ErrPayoutTransition):
		c.JSON(http.StatusConflict, gin.H{"error": "payout can only be approved when pending"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve payout"})
	default:
		c.JSON(http.StatusOK, gin.H{"payout": PayoutRequestJSON(request)})
	}
}

// CompletePayout settles an approved withdrawal.
func (h *Handler) CompletePayout(c *gin.Context) {
	id, ok := parsePayoutID(c)
	if !ok {
		return
	}
	request, err := h.ledger.CompletePayout(c.Request.Context(), id)
	switch {
	case errors.Is(err, ErrPayoutNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "payout not found"})
	case errors.Is(err, ErrPayoutTransition):
		c.JSON(http.StatusConflict, gin.H{"error": "payout must be approved before completion"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete payout"})
	default:
		c.JSON(http.StatusOK, gin.H{"payout": PayoutRequestJSON(request)})
	}
}

// FailPayout rejects a withdrawal and refunds the reserved balance.
func (h *Handler) FailPayout(c *gin.Context) {
	id, ok := parsePayoutID(c)
	if !ok {
		return
	}
	reason := bindReason(c)
	request, err := h.ledger.FailPayout(c.Request.Context(), id, reason)
	switch {
	case errors.Is(err, ErrPayoutNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "payout not found"})
	case errors.Is(err, ErrPayoutTransition):
		c.JSON(http.StatusConflict, gin.H{"error": "completed payouts cannot be failed"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fail payout"})
	default:
		c.JSON(http.StatusOK, gin.H{"payout": PayoutRequestJSON(request), "refunded": true})
	}
}

// PayoutRequestJSON serializes a payout request for HTTP responses.
func PayoutRequestJSON(req db.PayoutRequest) gin.H {
	return gin.H{
		"id":            req.ID,
		"driverId":      req.DriverID,
		"amountCents":   req.AmountCents,
		"method":        req.Method,
		"status":        req.Status,
		"failureReason": req.FailureReason,
		"referenceId":   req.ReferenceID,
		"requestedAt":   req.RequestedAt,
		"processedAt":   req.ProcessedAt,
	}
}

