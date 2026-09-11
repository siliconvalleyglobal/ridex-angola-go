package dashboard

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

// riderUserID resolves the authenticated rider's UUID.
func riderUserID(c *gin.Context) (uuid.UUID, bool) {
	uid, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return uid, true
}

// RiderSavedPlaces lists the rider's saved places.
func (h *Handler) RiderSavedPlaces(c *gin.Context) {
	uid, ok := riderUserID(c)
	if !ok {
		return
	}
	places, err := h.q.ListSavedPlaces(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load saved places"})
		return
	}
	out := make([]gin.H, 0, len(places))
	for _, p := range places {
		out = append(out, gin.H{
			"id": p.ID, "name": p.Name, "address": p.Address,
			"latitude": p.Latitude, "longitude": p.Longitude,
			"icon": p.Icon, "createdAt": p.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"savedPlaces": out})
}

// RiderPromos lists currently active promo codes the rider can use.
func (h *Handler) RiderPromos(c *gin.Context) {
	uid, ok := riderUserID(c)
	if !ok {
		return
	}
	promos, err := h.q.ListActivePromoCodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promotions"})
		return
	}
	out := make([]gin.H, 0, len(promos))
	for _, p := range promos {
		used, _ := h.q.CheckUserPromoUsage(c.Request.Context(), db.CheckUserPromoUsageParams{
			PromoID: p.ID, UserID: uid,
		})
		out = append(out, gin.H{
			"code":             p.Code,
			"discountType":     p.DiscountType,
			"discountCents":    p.DiscountCents,
			"minRideCents":     p.MinRideCents,
			"maxDiscountCents": p.MaxDiscountCents,
			"validUntil":       p.ValidUntil,
			"alreadyUsed":      used,
		})
	}
	c.JSON(http.StatusOK, gin.H{"promos": out})
}

// RiderLoyalty returns the rider's loyalty balance, tier and recent transactions.
func (h *Handler) RiderLoyalty(c *gin.Context) {
	uid, ok := riderUserID(c)
	if !ok {
		return
	}
	balance, err := h.q.GetLoyaltyBalance(c.Request.Context(), uid)
	if errors.Is(err, pgx.ErrNoRows) {
		// Rider has not earned points yet.
		c.JSON(http.StatusOK, gin.H{
			"loyalty": gin.H{
				"balance": 0, "totalEarned": 0, "totalRedeemed": 0,
				"tier": "bronze", "transactions": []gin.H{},
			},
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load loyalty"})
		return
	}
	txRows, err := h.q.ListLoyaltyTransactions(c.Request.Context(), db.ListLoyaltyTransactionsParams{
		UserID: uid, Limit: 10, Offset: 0,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load loyalty history"})
		return
	}
	txs := make([]gin.H, 0, len(txRows))
	for _, t := range txRows {
		reason := any(nil)
		if t.Reason.Valid {
			reason = t.Reason.String
		}
		txs = append(txs, gin.H{
			"points": t.Points, "type": t.Type, "reason": reason, "createdAt": t.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"loyalty": gin.H{
		"balance": balance.Balance, "totalEarned": balance.TotalEarned,
		"totalRedeemed": balance.TotalRedeemed, "tier": balance.Tier,
		"transactions": txs,
	}})
}

// RiderSupportTickets lists support tickets the rider opened.
func (h *Handler) RiderSupportTickets(c *gin.Context) {
	uid, ok := riderUserID(c)
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)
	tickets, err := h.q.ListSupportTicketsByUser(c.Request.Context(), db.ListSupportTicketsByUserParams{
		CreatedBy: uid,
		Limit:     int32(pageSize + 1),
		Offset:    int32((page - 1) * pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load support tickets"})
		return
	}
	hasMore := len(tickets) > pageSize
	if hasMore {
		tickets = tickets[:pageSize]
	}
	out := make([]gin.H, 0, len(tickets))
	for _, t := range tickets {
		resolution := any(nil)
		if t.ResolutionNote.Valid {
			resolution = t.ResolutionNote.String
		}
		out = append(out, gin.H{
			"id": t.ID, "rideId": t.RideID, "category": t.Category,
			"subject": t.Subject, "status": t.Status,
			"resolutionNote": resolution,
			"createdAt":      t.CreatedAt, "resolvedAt": t.ResolvedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"supportTickets": out,
		"pagination":     gin.H{"page": page, "pageSize": pageSize, "hasMore": hasMore},
	})
}
