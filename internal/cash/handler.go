package cash

import (
	"math/big"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

type Handler struct{ q *db.Queries }

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

func (h *Handler) Confirm(c *gin.Context) {
	driverID, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(401, gin.H{"error": "não autenticado"})
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "ID de viagem inválido"})
		return
	}
	var body struct {
		AmountPaid int64 `json:"amountPaid" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "amountPaid é obrigatório"})
		return
	}
	payment, err := h.q.ConfirmCashPayment(c.Request.Context(), rideID, driverID, pgtype.Numeric{Int: big.NewInt(body.AmountPaid), Valid: true})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "pagamento em dinheiro não pode ser confirmado para esta viagem"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"status": "paid", "amountDue": payment.AmountDue, "amountPaid": payment.AmountPaid,
		"changeCents": payment.ChangeCents, "currency": "AOA", "confirmedAt": payment.ConfirmedAt,
	})
}
