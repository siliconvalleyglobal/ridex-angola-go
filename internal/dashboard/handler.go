package dashboard

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
	"github.com/ridex/ridex-angola/internal/drivers"
	"github.com/ridex/ridex-angola/internal/payment"
)

type Handler struct{ q *db.Queries }

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

func (h *Handler) AdminSummary(c *gin.Context) {
	counts, err := h.q.CountUsersByRole(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load dashboard summary"})
		return
	}
	open, err := h.q.GetOpenRides(c.Request.Context(), 100)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load open rides"})
		return
	}
	pending, err := h.q.ListDriverKYC(c.Request.Context(), db.ListDriverKYCParams{Status: "pending", Limit: 100})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load KYC queue"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"users":      counts,
		"openRides":  len(open),
		"pendingKYC": len(pending),
		"service":    "ridex-angola",
	})
}

func (h *Handler) DriverSummary(c *gin.Context) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return
	}
	availability, err := h.q.GetDriverAvailability(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load driver status"})
		return
	}
	offers, err := h.q.GetDriverOffers(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load driver offers"})
		return
	}
	kyc, err := h.q.GetDriverKYC(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load driver verification"})
		return
	}
	var vehicleProfile any
	var documentExpiry any
	profile, profileErr := h.q.GetDriverVehicleProfile(c.Request.Context(), id)
	if profileErr == nil {
		vehicleProfile = drivers.VehicleProfileJSON(profile)
		documentExpiry = drivers.DocumentExpiryJSON(profile)
	} else if !errors.Is(profileErr, pgx.ErrNoRows) {
		c.JSON(500, gin.H{"error": "failed to load vehicle profile"})
		return
	}
	earnings, err := h.q.GetDriverEarningsSummary(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load driver earnings"})
		return
	}

	// Aggregation: active ride + recent rides (same source as the rider dashboard).
	var activeRide any
	recentRides := []gin.H{}
	rides, err := h.q.GetRidesByDriver(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			c.JSON(500, gin.H{"error": "failed to load driver rides"})
			return
		}
	} else {
		recentRides = ridesJSON(rides)
		if len(rides) > 20 {
			recentRides = recentRides[:20]
		}
		for i := range rides {
			switch rides[i].Status {
			case "requested", "matched", "driver_arriving", "in_progress":
				activeRide = ridesJSON(rides[i : i+1])[0]
				break
			}
			if activeRide != nil {
				break
			}
		}
	}

	// Wallet snapshot — absent wallets serialize to null rather than erroring.
	var wallet any
	w, walletErr := h.q.GetDriverWallet(c.Request.Context(), id)
	if walletErr == nil {
		wallet = gin.H{
			"balanceCents":     w.BalanceCents,
			"pendingCents":     w.PendingCents,
			"totalEarnedCents": w.TotalEarnedCents,
			"totalPaidCents":   w.TotalPaidCents,
			"currency":         w.Currency,
			"lastPayoutAt":     w.LastPayoutAt,
		}
	} else if !errors.Is(walletErr, pgx.ErrNoRows) {
		c.JSON(500, gin.H{"error": "failed to load wallet"})
		return
	}

	// Active payout queue — same source as /drivers/payouts.
	queue, err := h.q.GetActivePayoutRequestsByDriver(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to load payout queue"})
		return
	}
	queueJSON := make([]gin.H, 0, len(queue))
	for _, p := range queue {
		queueJSON = append(queueJSON, gin.H{
			"id": p.ID, "amountCents": p.AmountCents, "method": p.Method,
			"status": p.Status, "requestedAt": p.RequestedAt,
		})
	}

	// Loyalty summary — absent balances serialize to null.
	var loyalty any
	lp, loyaltyErr := h.q.GetLoyaltyBalance(c.Request.Context(), id)
	if loyaltyErr == nil {
		loyalty = gin.H{
			"balance": lp.Balance, "totalEarned": lp.TotalEarned,
			"totalRedeemed": lp.TotalRedeemed, "tier": lp.Tier,
		}
	} else if !errors.Is(loyaltyErr, pgx.ErrNoRows) {
		c.JSON(500, gin.H{"error": "failed to load loyalty"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"online":                availability.IsOnline,
		"availabilityUpdatedAt": availability.UpdatedAt,
		"kycStatus":             kyc.Status,
		"offers":                offers,
		"earnings":              earningsJSON(earnings),
		"vehicleProfile":        vehicleProfile,
		"documentExpiry":        documentExpiry,
		"activeRide":            activeRide,
		"recentRides":           recentRides,
		"wallet":                wallet,
		"payoutQueue":           queueJSON,
		"loyalty":               loyalty,
	})
}

// DriverEarnings exposes the same payment-backed earnings calculation as the
// driver dashboard for clients that only need the financial summary.
func (h *Handler) DriverEarnings(c *gin.Context) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	earnings, err := h.q.GetDriverEarningsSummary(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load driver earnings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"earnings": earningsJSON(earnings)})
}

func (h *Handler) RiderSummary(c *gin.Context) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	rides, err := h.q.GetRidesByRider(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load rider rides"})
		return
	}
	invoices, err := h.q.GetSAFTInvoicesByRider(c.Request.Context(), db.GetSAFTInvoicesByRiderParams{
		RiderID: id,
		Limit:   20,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load rider invoices"})
		return
	}

	var active *db.Ride
	for i := range rides {
		switch rides[i].Status {
		case "requested", "matched", "driver_arriving", "in_progress":
			active = &rides[i]
			break
		}
		if active != nil {
			break
		}
	}

	recent := rides
	if len(recent) > 20 {
		recent = recent[:20]
	}

	c.JSON(http.StatusOK, gin.H{
		"activeRide":  active,
		"recentRides": recent,
		"invoices":    invoices,
	})
}

// DriverTransactions returns paginated ride-side transaction history for the
// driver, mirroring the rider dashboard. Each entry includes the ride summary
// and any payment charges recorded against it.
func (h *Handler) DriverTransactions(c *gin.Context) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	page, pageSize := parsePagination(c)
	rides, err := h.q.GetRidesByDriverPage(c.Request.Context(), db.GetRidesByDriverPageParams{
		DriverID: pgtype.UUID{Bytes: id, Valid: true},
		Limit:    int32(pageSize + 1),
		Offset:   int32((page - 1) * pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load rides"})
		return
	}

	hasMore := len(rides) > pageSize
	if hasMore {
		rides = rides[:pageSize]
	}

	transactions := make([]gin.H, 0, len(rides))
	for _, ride := range rides {
		charges, err := h.q.GetPaymentChargesByRide(c.Request.Context(), ride.ID)
		var payments []gin.H
		if err == nil {
			payments = paymentJSONList(charges)
		}
		transactions = append(transactions, gin.H{
			"rideId":      ride.ID,
			"status":      ride.Status,
			"pickup":      ride.PickupAddress,
			"destination": ride.DestinationAddress,
			"fareCents":   ride.AcceptedFareCents,
			"createdAt":   ride.CreatedAt,
			"completedAt": ride.CompletedAt,
			"cancelledAt": ride.CancelledAt,
			"payments":    payments,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"pagination":   gin.H{"page": page, "pageSize": pageSize, "hasMore": hasMore},
	})
}
func (h *Handler) AdminUsers(c *gin.Context) {
	p := adminPagination(c)
	users, err := h.q.ListUsers(c.Request.Context(), db.ListUsersParams{
		Column1: c.Query("search"), Limit: int32(p.PageSize + 1), Offset: int32(p.Offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}
	hasMore := len(users) > p.PageSize
	if hasMore {
		users = users[:p.PageSize]
	}
	out := make([]gin.H, 0, len(users))
	for _, user := range users {
		out = append(out, adminUserJSON(user))
	}
	c.JSON(http.StatusOK, gin.H{"users": out, "pagination": p.response(hasMore)})
}

func (h *Handler) AdminSetUserStatus(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	requesterID, _ := uuid.Parse(c.GetString(auth.ContextUserID))
	var body struct {
		Active *bool `json:"active" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Active == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "active is required"})
		return
	}
	if userID == requesterID && !*body.Active {
		c.JSON(http.StatusBadRequest, gin.H{"error": "an admin cannot deactivate their own account"})
		return
	}
	user, err := h.q.SetUserActive(c.Request.Context(), db.SetUserActiveParams{
		ID: userID, IsActive: *body.Active,
	})
	if errorsIsNoRows(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user status"})
		return
	}
	if !*body.Active {
		if err := h.q.DeleteSessionsForUser(c.Request.Context(), userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user disabled but active sessions could not be revoked"})
			return
		}
		if user.Role == "driver" {
			if _, err := h.q.SetDriverAvailability(c.Request.Context(), db.SetDriverAvailabilityParams{
				DriverID: userID, IsOnline: false,
			}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "driver disabled but availability could not be updated"})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"user": adminUserJSON(user)})
}

// AdminRides provides a bounded operational view of rides. Detailed audit
// history is available from AdminRideAudit.
func (h *Handler) AdminRides(c *gin.Context) {
	p := adminPagination(c)
	rides, err := h.q.ListRidesForAdmin(c.Request.Context(), db.ListRidesForAdminParams{
		Column1: c.Query("status"), Limit: int32(p.PageSize + 1), Offset: int32(p.Offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rides"})
		return
	}
	hasMore := len(rides) > p.PageSize
	if hasMore {
		rides = rides[:p.PageSize]
	}
	out := make([]gin.H, 0, len(rides))
	for _, ride := range rides {
		out = append(out, adminRideJSON(ride))
	}
	c.JSON(http.StatusOK, gin.H{"rides": out, "pagination": p.response(hasMore)})
}

func (h *Handler) AdminRideAudit(c *gin.Context) {
	rideID, err := uuid.Parse(c.Param("rideId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ride id"})
		return
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), rideID)
	if errorsIsNoRows(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load ride"})
		return
	}
	events, err := h.q.GetRideEvents(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load ride audit"})
		return
	}
	charges, err := h.q.GetPaymentChargesByRide(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load payment oversight"})
		return
	}
	reviews, err := h.q.GetRideReviews(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load review oversight"})
		return
	}
	tickets, err := h.q.ListSupportTicketsForRide(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load support oversight"})
		return
	}
	disputes, err := h.q.ListRideDisputes(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load dispute oversight"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ride": adminRideJSON(ride), "events": events, "payments": paymentJSONList(charges),
		"reviews": reviews, "supportTickets": tickets, "disputes": disputes,
	})
}

func (h *Handler) AdminPayments(c *gin.Context) {
	p := adminPagination(c)
	charges, err := h.q.ListPaymentCharges(c.Request.Context(), db.ListPaymentChargesParams{
		Column1: c.Query("status"), Column2: c.Query("provider"),
		Limit: int32(p.PageSize + 1), Offset: int32(p.Offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payments"})
		return
	}
	hasMore := len(charges) > p.PageSize
	if hasMore {
		charges = charges[:p.PageSize]
	}
	c.JSON(http.StatusOK, gin.H{"payments": paymentJSONList(charges), "pagination": p.response(hasMore)})
}

func (h *Handler) AdminPaymentAudit(c *gin.Context) {
	chargeID, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	charge, err := h.q.GetPaymentChargeByID(c.Request.Context(), chargeID)
	if errorsIsNoRows(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load payment"})
		return
	}
	events, err := h.q.GetPaymentEvents(c.Request.Context(), chargeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load payment audit"})
		return
	}
	eventJSON := make([]gin.H, 0, len(events))
	for _, event := range events {
		eventJSON = append(eventJSON, payment.EventJSON(event))
	}
	c.JSON(http.StatusOK, gin.H{
		"payment": payment.PaymentJSON(charge), "rawResponse": charge.RawResponse, "events": eventJSON,
	})
}

type adminPage struct {
	Page     int
	PageSize int
	Offset   int
}

func adminPagination(c *gin.Context) adminPage {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	return adminPage{Page: page, PageSize: pageSize, Offset: (page - 1) * pageSize}
}

func (p adminPage) response(hasMore bool) gin.H {
	return gin.H{"page": p.Page, "pageSize": p.PageSize, "hasMore": hasMore}
}

func adminUserJSON(user db.User) gin.H {
	return gin.H{
		"id": user.ID, "phone": user.Phone, "name": user.Name, "role": user.Role,
		"rating": user.Rating, "acceptanceRate": user.AcceptanceRate,
		"isActive": user.IsActive, "createdAt": user.CreatedAt, "updatedAt": user.UpdatedAt,
	}
}

func adminRideJSON(ride db.Ride) gin.H {
	return gin.H{
		"id": ride.ID, "riderId": ride.RiderID, "driverId": ride.DriverID,
		"status": ride.Status, "pickupAddress": ride.PickupAddress,
		"destinationAddress": ride.DestinationAddress, "suggestedFareCents": ride.SuggestedFareCents,
		"acceptedFareCents": ride.AcceptedFareCents, "currency": ride.Currency,
		"paymentMethod": ride.PaymentMethod, "createdAt": ride.CreatedAt,
		"updatedAt": ride.UpdatedAt, "completedAt": ride.CompletedAt, "cancelledAt": ride.CancelledAt,
		"cancellationReason": ride.CancellationReason, "cancelledBy": ride.CancelledBy,
	}
}

func paymentJSONList(charges []db.PaymentCharge) []gin.H {
	out := make([]gin.H, 0, len(charges))
	for _, charge := range charges {
		out = append(out, payment.PaymentJSON(charge))
	}
	return out
}

func earningsJSON(earnings db.GetDriverEarningsSummaryRow) gin.H {
	return gin.H{
		"completedRides":  earnings.CompletedRides,
		"grossFares":      earnings.GrossFares,
		"cashEarnings":    earnings.CashEarnings,
		"digitalEarnings": earnings.DigitalEarnings,
		"totalEarnings":   earnings.TotalEarnings,
		"unpaidFares":     earnings.UnpaidFares,
		"currency":        "AOA",
	}
}

func errorsIsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
