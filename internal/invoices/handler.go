package invoices

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

type Handler struct{ q *db.Queries }

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

func (h *Handler) Create(c *gin.Context) {
	riderID, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ride id"})
		return
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		return
	}
	if ride.RiderID != riderID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your ride"})
		return
	}
	if ride.Status != "completed" {
		c.JSON(http.StatusConflict, gin.H{"error": "invoice requires a completed ride"})
		return
	}
	existing, err := h.q.GetSAFTInvoicesByRide(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check existing invoices"})
		return
	}
	if len(existing) > 0 {
		c.JSON(http.StatusOK, gin.H{"invoice": existing[0]})
		return
	}

	user, err := h.q.GetUserByID(c.Request.Context(), riderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load customer"})
		return
	}
	now := time.Now().UTC()
	invoice, err := h.q.CreateSAFTInvoice(c.Request.Context(), db.CreateSAFTInvoiceParams{
		RideID:        rideID,
		InvoiceNumber: fmt.Sprintf("RIDEX-%s-%s", now.Format("20060102"), rideID.String()[:8]),
		IssueDate:     pgtype.Date{Time: now, Valid: true},
		DueDate:       pgtype.Date{Time: now, Valid: true},
		CustomerName:  user.Name,
		Items:         mustJSON([]map[string]interface{}{{"description": "Ride", "quantity": 1, "amountCents": numericValue(ride.SuggestedFareCents)}}),
		TotalCents:    ride.SuggestedFareCents,
		Currency:      ride.Currency,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invoice"})
		return
	}
	invoice, err = h.q.IssueSAFTInvoice(c.Request.Context(), invoice.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue invoice"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"invoice": invoice})
}

func (h *Handler) Get(c *gin.Context) {
	invoice, ok := h.loadOwned(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"invoice": invoice})
}

func (h *Handler) ExportXML(c *gin.Context) {
	invoice, ok := h.loadOwned(c)
	if !ok {
		return
	}
	content, err := renderXML(invoice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render invoice"})
		return
	}
	c.Data(http.StatusOK, "application/xml; charset=utf-8", content)
}

func (h *Handler) loadOwned(c *gin.Context) (db.SaftInvoice, bool) {
	uid, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return db.SaftInvoice{}, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invoice id"})
		return db.SaftInvoice{}, false
	}
	invoice, err := h.q.GetSAFTInvoiceByID(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "invoice not found"})
		return db.SaftInvoice{}, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load invoice"})
		return db.SaftInvoice{}, false
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), invoice.RideID)
	if err != nil || ride.RiderID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your invoice"})
		return db.SaftInvoice{}, false
	}
	return invoice, true
}

type xmlInvoice struct {
	XMLName   xml.Name  `xml:"Invoice"`
	Number    string    `xml:"InvoiceNo"`
	IssueDate string    `xml:"InvoiceDate"`
	Customer  string    `xml:"CustomerName"`
	Currency  string    `xml:"Currency"`
	Total     int64     `xml:"TotalCents"`
	Items     []xmlItem `xml:"LineItems>LineItem"`
}

type xmlItem struct {
	Description string `xml:"Description"`
	Quantity    int    `xml:"Quantity"`
	AmountCents int64  `xml:"AmountCents"`
}

func renderXML(invoice db.SaftInvoice) ([]byte, error) {
	var items []struct {
		Description string `json:"description"`
		Quantity    int    `json:"quantity"`
		AmountCents int64  `json:"amountCents"`
	}
	if err := json.Unmarshal(invoice.Items, &items); err != nil {
		return nil, err
	}
	out := xmlInvoice{
		Number:    invoice.InvoiceNumber,
		IssueDate: invoice.IssueDate.Time.Format("2006-01-02"),
		Customer:  invoice.CustomerName,
		Currency:  invoice.Currency,
		Total:     numericValue(invoice.TotalCents),
	}
	for _, item := range items {
		out.Items = append(out.Items, xmlItem(item))
	}
	return xml.MarshalIndent(out, "", "  ")
}

func numericValue(n pgtype.Numeric) int64 {
	if !n.Valid || n.Int == nil {
		return 0
	}
	if n.Exp >= 0 {
		return n.Int.Int64() * pow10(int(n.Exp))
	}
	return n.Int.Int64() / pow10(int(-n.Exp))
}

func pow10(exp int) int64 {
	value := int64(1)
	for i := 0; i < exp; i++ {
		value *= 10
	}
	return value
}

func mustJSON(value interface{}) []byte {
	data, _ := json.Marshal(value)
	return data
}
