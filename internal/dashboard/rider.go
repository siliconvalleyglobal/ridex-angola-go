package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

// RiderRides returns paginated ride history for the rider.
func (h *Handler) RiderRides(c *gin.Context) {
	uid, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	page, pageSize := parsePagination(c)
	rides, err := h.q.GetRidesByRiderPage(c.Request.Context(), db.GetRidesByRiderPageParams{
		RiderID: uid,
		Limit:   int32(pageSize + 1),
		Offset:  int32((page - 1) * pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load rides"})
		return
	}

	hasMore := len(rides) > pageSize
	if hasMore {
		rides = rides[:pageSize]
	}

	c.JSON(http.StatusOK, gin.H{
		"rides":      ridesJSON(rides),
		"pagination": gin.H{"page": page, "pageSize": pageSize, "hasMore": hasMore},
	})
}

// RiderInvoices returns paginated invoices for the rider.
func (h *Handler) RiderInvoices(c *gin.Context) {
	uid, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	page, pageSize := parsePagination(c)
	// Get all invoices and paginate in memory since the query doesn't support offset
	invoices, err := h.q.GetSAFTInvoicesByRider(c.Request.Context(), db.GetSAFTInvoicesByRiderParams{
		RiderID: uid,
		Limit:   int32(pageSize * page), // Get enough to fill current page
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load invoices"})
		return
	}

	// Manual pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(invoices) {
		invoices = []db.SaftInvoice{}
	} else {
		if end > len(invoices) {
			end = len(invoices)
		}
		invoices = invoices[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"invoices":   saftInvoicesJSON(invoices),
		"pagination": gin.H{"page": page, "pageSize": pageSize, "hasMore": end < len(invoices)},
	})
}

func ridesJSON(rides []db.Ride) []gin.H {
	out := make([]gin.H, 0, len(rides))
	for _, r := range rides {
		out = append(out, gin.H{
			"id":                 r.ID,
			"status":             r.Status,
			"pickup":             r.PickupAddress,
			"destination":        r.DestinationAddress,
			"suggestedFareCents": r.SuggestedFareCents,
			"acceptedFareCents":  r.AcceptedFareCents,
			"paymentMethod":      r.PaymentMethod,
			"createdAt":          r.CreatedAt,
			"completedAt":        r.CompletedAt,
			"cancelledAt":        r.CancelledAt,
			"cancellationReason": r.CancellationReason,
		})
	}
	return out
}

func saftInvoicesJSON(invoices []db.SaftInvoice) []gin.H {
	out := make([]gin.H, 0, len(invoices))
	for _, inv := range invoices {
		out = append(out, gin.H{
			"id":            inv.ID,
			"rideId":        inv.RideID,
			"invoiceNumber": inv.InvoiceNumber,
			"status":        inv.Status,
			"createdAt":     inv.CreatedAt,
		})
	}
	return out
}

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if v := parseInt(p); v > 0 {
			page = v
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if v := parseInt(ps); v > 0 && v <= 100 {
			pageSize = v
		}
	}
	return page, pageSize
}

func parseInt(s string) int {
	var v int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		}
	}
	return v
}
