package rides

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/db"
)

// Handler implements the ride lifecycle HTTP endpoints.
type Handler struct {
	q     *db.Queries
	begin func(context.Context) (pgx.Tx, error)
}

// NewHandler creates a ride handler wired to the DB queries.
func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

// NewHandlerWithTx enables atomic ride state and audit-event mutations. The
// legacy constructor remains useful for read-only/unit-test wiring.
func NewHandlerWithTx(q *db.Queries, begin func(context.Context) (pgx.Tx, error)) *Handler {
	return &Handler{q: q, begin: begin}
}

var errIdempotencyConflict = errors.New("idempotency key was already used with different ride details")

func (h *Handler) withTx(ctx context.Context, fn func(*db.Queries) error) error {
	if h.begin == nil {
		return fn(h.q)
	}
	tx, err := h.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(h.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── Requests ──────────────────────────────────────────────────────────

type createRideRequest struct {
	PickupLat          float64 `json:"pickupLat" binding:"required"`
	PickupLng          float64 `json:"pickupLng" binding:"required"`
	PickupAddress      string  `json:"pickupAddress" binding:"required"`
	DestinationLat     float64 `json:"destinationLat" binding:"required"`
	DestinationLng     float64 `json:"destinationLng" binding:"required"`
	DestinationAddress string  `json:"destinationAddress" binding:"required"`
	SuggestedFareCents int64   `json:"suggestedFareCents" binding:"required,min=100"`
	PromoCode          string  `json:"promoCode"`
	RequestedPickupAt  string  `json:"requestedPickupAt"`
	BusinessAccountID  string  `json:"businessAccountId"`
	IdempotencyKey     string  `json:"idempotencyKey"`
}

// ── Endpoints ─────────────────────────────────────────────────────────

// Create registers a new ride request by the authenticated rider.
func (h *Handler) Create(c *gin.Context) {
	uid, ok := parseUser(c)
	if !ok {
		return
	}
	// Ride requests are intentionally bounded so a retry on a constrained
	// connection never has to upload an unbounded body.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req createRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large", "code": "request_too_large"})
			return
		}
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		key = strings.TrimSpace(req.IdempotencyKey)
	}
	if len(key) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "idempotency key is too long"})
		return
	}
	hashInput := req
	hashInput.IdempotencyKey = ""
	hash, _ := json.Marshal(hashInput)
	payloadHash := sha256.Sum256(hash)
	payloadHashText := fmt.Sprintf("%x", payloadHash[:])

	// A read before the insert lets retries return the exact original ride and
	// avoids appending another requested event. The unique index remains the
	// race-safe backstop for concurrent requests.
	var existing db.Ride
	var err error
	if key != "" {
		existing, err = h.q.GetRideByIdempotencyKey(c.Request.Context(), db.GetRideByIdempotencyKeyParams{
			RiderID: uid, IdempotencyKey: pgtype.Text{String: key, Valid: true},
		})
		if err == nil {
			if existing.IdempotencyPayloadHash.Valid && existing.IdempotencyPayloadHash.String != payloadHashText {
				c.JSON(http.StatusConflict, gin.H{"error": "idempotency key was already used with different ride details"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ride": rideJSON(existing), "idempotentReplay": true})
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check idempotency key"})
			return
		}
	}

	var promo pgtype.Text
	if req.PromoCode != "" {
		promo = pgtype.Text{String: req.PromoCode, Valid: true}
	}

	var requestedAt pgtype.Timestamptz
	if req.RequestedPickupAt != "" {
		parsed, err := parseRequestedPickupAt(req.RequestedPickupAt, time.Now())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requestedPickupAt must be a valid future RFC3339 timestamp"})
			return
		}
		requestedAt = pgtype.Timestamptz{Time: parsed.UTC(), Valid: true}
	}

	var businessAccountID, businessMemberID pgtype.UUID
	authorizationStatus := ""
	if strings.TrimSpace(req.BusinessAccountID) != "" {
		accountID, parseErr := uuid.Parse(strings.TrimSpace(req.BusinessAccountID))
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid business account id"})
			return
		}
		account, accountErr := h.q.GetBusinessAccount(c.Request.Context(), accountID)
		if errors.Is(accountErr, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "business account not found"})
			return
		}
		if accountErr != nil || !account.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "business account is unavailable"})
			return
		}
		member, memberErr := h.q.GetBusinessMember(c.Request.Context(), db.GetBusinessMemberParams{
			AccountID: accountID, UserID: uid,
		})
		if errors.Is(memberErr, pgx.ErrNoRows) || memberErr == nil && (member.Status != "active" || member.Role == "") {
			c.JSON(http.StatusForbidden, gin.H{"error": "you are not an active business member"})
			return
		}
		if memberErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate business membership"})
			return
		}
		usage, usageErr := h.q.GetBusinessMonthlyUsage(c.Request.Context(), db.GetBusinessMonthlyUsageParams{
			BusinessAccountID: pgUUID(accountID),
			Column2:           pgtype.Date{Time: time.Now().UTC(), Valid: true},
		})
		if usageErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate business spending"})
			return
		}
		estimated := req.SuggestedFareCents
		if exceedsLimit(usage.TotalCents, estimated, account.MonthlyLimitCents) ||
			exceedsLimit(usage.TotalCents, estimated, member.SpendingLimitCents) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "business spending limit exceeded"})
			return
		}
		businessAccountID, businessMemberID = pgUUID(accountID), pgUUID(uid)
		if account.RequireRideApproval {
			authorizationStatus = "pending"
		} else {
			authorizationStatus = "approved"
		}
	}

	var zoneID pgtype.UUID
	activeZones, err := h.q.CountActiveServiceZones(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate service zones"})
		return
	}
	if activeZones > 0 {
		zone, zoneErr := h.q.FindServiceZoneForPoint(c.Request.Context(), db.FindServiceZoneForPointParams{
			Column1: decimal(req.PickupLat), Column2: decimal(req.PickupLng),
		})
		if errors.Is(zoneErr, pgx.ErrNoRows) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "pickup is outside active service zones"})
			return
		}
		if zoneErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate pickup zone"})
			return
		}
		// A local ride must have a covered destination as well. The pickup
		// zone is retained for pricing and operational assignment.
		if _, zoneErr = h.q.FindServiceZoneForPoint(c.Request.Context(), db.FindServiceZoneForPointParams{
			Column1: decimal(req.DestinationLat), Column2: decimal(req.DestinationLng),
		}); errors.Is(zoneErr, pgx.ErrNoRows) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "destination is outside active service zones"})
			return
		} else if zoneErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate destination zone"})
			return
		}
		zoneID = pgUUID(zone.ID)
	}

	var ride db.Ride
	idempotentReplay := false
	err = h.withTx(c.Request.Context(), func(q *db.Queries) error {
		created, createErr := q.CreateRideIdempotent(c.Request.Context(), db.CreateRideIdempotentParams{
			RiderID:                uid,
			Column2:                pgtype.Point{P: pgtype.Vec2{X: req.PickupLng, Y: req.PickupLat}, Valid: true},
			Column3:                pgtype.Point{P: pgtype.Vec2{X: req.DestinationLng, Y: req.DestinationLat}, Valid: true},
			PickupAddress:          req.PickupAddress,
			DestinationAddress:     req.DestinationAddress,
			SuggestedFareCents:     numeric(req.SuggestedFareCents),
			Currency:               "AOA",
			PromoCode:              promo,
			ServiceZoneID:          zoneID,
			RequestedPickupAt:      requestedAt,
			BusinessAccountID:      businessAccountID,
			BusinessMemberID:       businessMemberID,
			IdempotencyKey:         pgtype.Text{String: key, Valid: key != ""},
			IdempotencyPayloadHash: pgtype.Text{String: payloadHashText, Valid: true},
		})
		if errors.Is(createErr, pgx.ErrNoRows) && key != "" {
			ride, createErr = q.GetRideByIdempotencyKey(c.Request.Context(), db.GetRideByIdempotencyKeyParams{
				RiderID: uid, IdempotencyKey: pgtype.Text{String: key, Valid: true},
			})
			if createErr == nil {
				idempotentReplay = true
			}
			return createErr
		}
		if createErr != nil {
			return createErr
		}
		ride = created
		if ride.IdempotencyPayloadHash.Valid && ride.IdempotencyPayloadHash.String != payloadHashText {
			return errIdempotencyConflict
		}
		if authorizationStatus != "" {
			if _, createErr = q.CreateRideAuthorization(c.Request.Context(), db.CreateRideAuthorizationParams{
				RideID: ride.ID, AccountID: fromPgUUID(businessAccountID),
				MemberID: fromPgUUID(businessMemberID), Status: authorizationStatus,
				RequestedAmountCents: numeric(req.SuggestedFareCents),
			}); createErr != nil {
				return createErr
			}
		}
		return recordRideEvent(q, c.Request.Context(), ride.ID, "requested", uid, nil)
	})
	if errors.Is(err, errIdempotencyConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": errIdempotencyConflict.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ride"})
		return
	}
	if idempotentReplay {
		c.JSON(http.StatusOK, gin.H{"ride": rideJSON(ride), "idempotentReplay": true})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ride": rideJSON(ride), "idempotentReplay": false})
}

// Get returns a ride by ID; riders see their own rides, drivers any assigned one.
func (h *Handler) Get(c *gin.Context) {
	uid, ok := parseUser(c)
	if !ok {
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(404, gin.H{"error": "ride not found"})
		return
	}
	if ride.RiderID != uid && (!ride.DriverID.Valid || fromPgUUID(ride.DriverID) != uid) {
		c.JSON(403, gin.H{"error": "not your ride"})
		return
	}
	eventPage := parsePagination(c)
	events, err := h.q.GetRideEventsPage(c.Request.Context(), db.GetRideEventsPageParams{
		RideID: rideID, Limit: int32(eventPage.PageSize + 1), Offset: int32(eventPage.Offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load ride events"})
		return
	}
	hasMore := len(events) > eventPage.PageSize
	if hasMore {
		events = events[:eventPage.PageSize]
	}
	out := gin.H{"events": eventsJSON(events), "eventsPagination": eventPage.response(hasMore)}
	if ride.DriverID.Valid {
		if vehicle, verr := h.q.GetDriverVehicleProfile(c.Request.Context(), fromPgUUID(ride.DriverID)); verr == nil {
			out["ride"] = rideJSON(ride, &vehicle)
		} else {
			out["ride"] = rideJSON(ride)
		}
	} else {
		out["ride"] = rideJSON(ride)
	}
	c.JSON(200, out)
}

// List returns the caller's rides (rider or driver).
func (h *Handler) List(c *gin.Context) {
	uid, ok := parseUser(c)
	if !ok {
		return
	}
	role := c.GetString(auth.ContextRole)
	var (
		rides []db.Ride
		err   error
	)
	p := parsePagination(c)
	if role == "driver" {
		rides, err = h.q.GetRidesByDriverPage(c.Request.Context(), db.GetRidesByDriverPageParams{
			DriverID: pgtype.UUID{Bytes: uid, Valid: true}, Limit: int32(p.PageSize + 1), Offset: int32(p.Offset),
		})
	} else {
		rides, err = h.q.GetRidesByRiderPage(c.Request.Context(), db.GetRidesByRiderPageParams{
			RiderID: uid, Limit: int32(p.PageSize + 1), Offset: int32(p.Offset),
		})
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list rides"})
		return
	}
	hasMore := len(rides) > p.PageSize
	if hasMore {
		rides = rides[:p.PageSize]
	}
	out := make([]gin.H, 0, len(rides))
	for _, r := range rides {
		out = append(out, rideJSON(r))
	}
	c.JSON(200, gin.H{"rides": out, "pagination": p.response(hasMore)})
}

// Open lists requested rides for drivers to accept.
func (h *Handler) Open(c *gin.Context) {
	p := parsePagination(c)
	rides, err := h.q.GetOpenRidesPage(c.Request.Context(), db.GetOpenRidesPageParams{
		Limit: int32(p.PageSize + 1), Offset: int32(p.Offset),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list open rides"})
		return
	}

	hasMore := len(rides) > p.PageSize
	if hasMore {
		rides = rides[:p.PageSize]
	}
	out := make([]gin.H, 0, len(rides))
	for _, r := range rides {
		out = append(out, rideJSON(r))
	}
	c.JSON(200, gin.H{"rides": out, "pagination": p.response(hasMore)})
}

func (h *Handler) Offers(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	p := parsePagination(c)
	offers, err := h.q.GetDriverOffersPage(c.Request.Context(), db.GetDriverOffersPageParams{
		DriverID: driverID, Limit: int32(p.PageSize + 1), Offset: int32(p.Offset),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list ride offers"})
		return
	}
	hasMore := len(offers) > p.PageSize
	if hasMore {
		offers = offers[:p.PageSize]
	}
	c.JSON(200, gin.H{"offers": offers, "pagination": p.response(hasMore)})
}

// Accept lets a driver claim a requested ride.
func (h *Handler) Accept(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	if c.GetString(auth.ContextRole) != "driver" {
		c.JSON(403, gin.H{"error": "only drivers can accept rides"})
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	var ride db.Ride
	err = h.withTx(c.Request.Context(), func(q *db.Queries) error {
		var mutationErr error
		ride, mutationErr = q.AcceptRideOffer(c.Request.Context(), db.AcceptRideOfferParams{RideID: rideID, DriverID: driverID})
		if mutationErr != nil {
			return mutationErr
		}
		return recordRideEvent(q, c.Request.Context(), rideID, "matched", driverID, nil)
	})
	if err != nil {
		c.JSON(409, gin.H{"error": "ride offer is unavailable or expired"})
		return
	}
	c.JSON(200, gin.H{"ride": rideJSON(ride)})
}

func (h *Handler) Decline(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	if err := h.q.DeclineRideOffer(c.Request.Context(), db.DeclineRideOfferParams{RideID: rideID, DriverID: driverID}); err != nil {
		c.JSON(500, gin.H{"error": "failed to decline ride offer"})
		return
	}
	if _, err := h.q.SetDriverAvailability(c.Request.Context(), db.SetDriverAvailabilityParams{
		DriverID: driverID,
		IsOnline: true,
	}); err != nil {
		c.JSON(500, gin.H{"error": "offer declined but failed to restore availability"})
		return
	}
	c.JSON(200, gin.H{"status": "declined"})
}

// Arrive marks the ride as driver_arriving (driver only, must own ride).
func (h *Handler) Arrive(c *gin.Context) {
	h.transition(c, "driver_arriving", func(q *db.Queries, ctx context.Context, id uuid.UUID) (db.Ride, error) {
		return q.MarkDriverArriving(ctx, id)
	})
}

// Start marks the ride in_progress (driver only, must own ride).
func (h *Handler) Start(c *gin.Context) {
	h.transition(c, "in_progress", func(q *db.Queries, ctx context.Context, id uuid.UUID) (db.Ride, error) {
		return q.MarkRideInProgress(ctx, id)
	})
}

// Complete completes the ride (driver only, must own ride).
func (h *Handler) Complete(c *gin.Context) {
	h.transition(c, "completed", func(q *db.Queries, ctx context.Context, id uuid.UUID) (db.Ride, error) {
		return q.CompleteRide(ctx, id)
	})
}

// Cancel cancels the ride; riders cancel their own, drivers cancel assigned.
func (h *Handler) Cancel(c *gin.Context) {
	uid, ok := parseUser(c)
	if !ok {
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	ride, err := h.q.GetRideByID(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(404, gin.H{"error": "ride not found"})
		return
	}
	role := c.GetString(auth.ContextRole)
	allowed := (role == "rider" && ride.RiderID == uid) ||
		(role == "driver" && ride.DriverID.Valid && fromPgUUID(ride.DriverID) == uid)
	if !allowed {
		c.JSON(403, gin.H{"error": "not your ride"})
		return
	}
	reason := "rider_cancelled"
	if role == "driver" {
		reason = "driver_cancelled"
	}
	var body struct {
		Reason string `json:"reason" binding:"max=120"`
	}
	if err := c.ShouldBindJSON(&body); err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cancellation body"})
		return
	}
	if body.Reason != "" {
		reason = body.Reason
	}
	var updated db.Ride
	err = h.withTx(c.Request.Context(), func(q *db.Queries) error {
		var mutationErr error
		if role == "driver" && ride.DriverID.Valid && fromPgUUID(ride.DriverID) == uid &&
			(ride.Status == "matched" || ride.Status == "driver_arriving") {
			// Automatic reassignment: the assigned driver bailed before the
			// trip started, so the ride returns to the open pool for other
			// drivers to bid on instead of dying outright.
			updated, mutationErr = q.ReassignRideToRequested(c.Request.Context(), db.ReassignRideToRequestedParams{
				ID: rideID, DriverID: pgUUID(uid),
			})
			if mutationErr == nil {
				return recordRideEvent(q, c.Request.Context(), rideID, "reassigned", uid, gin.H{
					"reason": reason, "previousDriverId": uid.String(),
				})
			}
			// Fall through to a normal cancel when the ride already left the
			// reassignment window (concurrent state change).
		}
		if role == "rider" {
			updated, mutationErr = q.CancelRideByRider(c.Request.Context(), db.CancelRideByRiderParams{
				ID: rideID, RiderID: uid,
				CancellationReason: pgtype.Text{String: reason, Valid: true}, CancelledBy: pgUUID(uid),
			})
		} else {
			updated, mutationErr = q.CancelRide(c.Request.Context(), db.CancelRideParams{
				ID: rideID, CancellationReason: pgtype.Text{String: reason, Valid: true}, CancelledBy: pgUUID(uid),
			})
		}
		if mutationErr != nil {
			return mutationErr
		}
		if mutationErr = recordRideEvent(q, c.Request.Context(), rideID, "cancelled", uid, gin.H{"reason": reason}); mutationErr != nil {
			return mutationErr
		}
		if ride.DriverID.Valid {
			_, mutationErr = q.SetDriverAvailability(c.Request.Context(), db.SetDriverAvailabilityParams{
				DriverID: fromPgUUID(ride.DriverID), IsOnline: true,
			})
		}
		return mutationErr
	})
	if err != nil {
		c.JSON(409, gin.H{"error": "ride cannot be cancelled"})
		return
	}
	c.JSON(200, gin.H{"ride": rideJSON(updated)})
}

// NoShow records a rider no-show after the assigned driver has arrived.
func (h *Handler) NoShow(c *gin.Context) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	if c.GetString(auth.ContextRole) != "driver" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only drivers can report a no-show"})
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ride id"})
		return
	}
	var updated db.Ride
	err = h.withTx(c.Request.Context(), func(q *db.Queries) error {
		var mutationErr error
		updated, mutationErr = q.MarkRideNoShow(c.Request.Context(), db.MarkRideNoShowParams{
			ID: rideID, CancelledBy: pgUUID(driverID),
		})
		if mutationErr != nil {
			return mutationErr
		}
		if mutationErr = recordRideEvent(q, c.Request.Context(), rideID, "no_show", driverID, gin.H{"reason": "rider_no_show"}); mutationErr != nil {
			return mutationErr
		}
		_, mutationErr = q.SetDriverAvailability(c.Request.Context(), db.SetDriverAvailabilityParams{
			DriverID: driverID, IsOnline: true,
		})
		return mutationErr
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "ride is not eligible for a no-show"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ride": rideJSON(updated)})
}

// ── Internals ─────────────────────────────────────────────────────────

func (h *Handler) transition(c *gin.Context, eventType string, fn func(*db.Queries, context.Context, uuid.UUID) (db.Ride, error)) {
	driverID, ok := parseUser(c)
	if !ok {
		return
	}
	if c.GetString(auth.ContextRole) != "driver" {
		c.JSON(403, gin.H{"error": "only drivers can perform this action"})
		return
	}
	rideID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ride id"})
		return
	}
	// Ownership check.
	ride, err := h.q.GetRideByID(c.Request.Context(), rideID)
	if err != nil {
		c.JSON(404, gin.H{"error": "ride not found"})
		return
	}
	if !ride.DriverID.Valid || fromPgUUID(ride.DriverID) != driverID {
		c.JSON(403, gin.H{"error": "not your ride"})
		return
	}
	if eventType == "in_progress" && ride.TripPinHash.Valid && !ride.TripPinVerified {
		c.JSON(409, gin.H{"error": "trip PIN must be verified before starting the ride"})
		return
	}
	var updated db.Ride
	err = h.withTx(c.Request.Context(), func(q *db.Queries) error {
		var mutationErr error
		updated, mutationErr = fn(q, c.Request.Context(), rideID)
		if mutationErr != nil {
			return mutationErr
		}
		if mutationErr = recordRideEvent(q, c.Request.Context(), rideID, eventType, driverID, nil); mutationErr != nil {
			return mutationErr
		}
		if eventType == "completed" {
			_, mutationErr = q.SetDriverAvailability(c.Request.Context(), db.SetDriverAvailabilityParams{
				DriverID: driverID, IsOnline: true,
			})
		}
		return mutationErr
	})
	if err != nil {
		c.JSON(409, gin.H{"error": "invalid transition from current status"})
		return
	}
	c.JSON(200, gin.H{"ride": rideJSON(updated)})
}

func recordRideEvent(q *db.Queries, ctx context.Context, rideID uuid.UUID, eventType string, by uuid.UUID, payload gin.H) error {
	var data []byte
	var err error
	if payload == nil {
		data = []byte(`{}`)
	} else {
		data, err = json.Marshal(payload)
	}
	if err != nil {
		return err
	}
	_, err = q.CreateRideEvent(ctx, db.CreateRideEventParams{
		RideID:    rideID,
		EventType: string(eventType),
		Payload:   data,
		CreatedBy: pgUUID(by),
	})
	return err
}

// ── Helpers ───────────────────────────────────────────────────────────

func parseUser(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return id, true
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func fromPgUUID(p pgtype.UUID) uuid.UUID { id, _ := uuid.FromBytes(p.Bytes[:]); return id }

func numeric(v int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(v), Exp: 0, Valid: true}
}

func exceedsLimit(used pgtype.Numeric, requested int64, limit pgtype.Numeric) bool {
	if !limit.Valid || limit.Int == nil {
		return false
	}
	if !used.Valid || used.Int == nil || used.Exp != 0 || limit.Exp != 0 {
		return false
	}
	return used.Int.Int64()+requested > limit.Int.Int64()
}

func decimal(v float64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(int64(math.Round(v * 1_000_000))), Exp: -6, Valid: true}
}

func parseRequestedPickupAt(value string, now time.Time) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || !parsed.After(now) {
		return time.Time{}, errors.New("pickup time must be in the future")
	}
	return parsed.UTC(), nil
}

type pagination struct {
	Page     int
	PageSize int
	Offset   int
}

func parsePagination(c *gin.Context) pagination {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return pagination{Page: page, PageSize: pageSize, Offset: (page - 1) * pageSize}
}

func (p pagination) response(hasMore bool) gin.H {
	return gin.H{"page": p.Page, "pageSize": p.PageSize, "hasMore": hasMore}
}

// rideJSON serializes a ride. An optional driver vehicle profile, when
// present, is embedded under "vehicle" so riders see make/model/plate of the
// car that accepted their request.
func rideJSON(r db.Ride, vehicle ...*db.DriverVehicleProfile) gin.H {
	out := rideJSONBase(r)
	for _, v := range vehicle {
		if v != nil {
			out["vehicle"] = vehicleJSON(*v)
		}
	}
	return out
}

func rideJSONBase(r db.Ride) gin.H {
	return gin.H{
		"id":                 r.ID,
		"riderId":            r.RiderID,
		"driverId":           r.DriverID,
		"pickup":             gin.H{"address": r.PickupAddress, "point": r.PickupPoint},
		"destination":        gin.H{"address": r.DestinationAddress, "point": r.DestinationPoint},
		"status":             r.Status,
		"suggestedFareCents": r.SuggestedFareCents,
		"acceptedFareCents":  r.AcceptedFareCents,
		"currency":           r.Currency,
		"promoCode":          r.PromoCode,
		"createdAt":          r.CreatedAt,
		"completedAt":        r.CompletedAt,
		"cancelledAt":        r.CancelledAt,
		"cancellationReason": r.CancellationReason,
		"cancelledBy":        r.CancelledBy,
		"serviceZoneId":      r.ServiceZoneID,
		"requestedPickupAt":  r.RequestedPickupAt,
		"businessAccountId":  r.BusinessAccountID,
		"businessMemberId":   r.BusinessMemberID,
	}
}

func vehicleJSON(v db.DriverVehicleProfile) gin.H {
	return gin.H{
		"licensePlate": v.LicensePlate,
		"make":         v.Make,
		"model":        v.Model,
		"year":         v.Year,
		"color":        v.Color,
	}
}

func eventsJSON(events []db.RideEvent) []gin.H {
	out := make([]gin.H, 0, len(events))
	for _, e := range events {
		out = append(out, gin.H{"type": e.EventType, "at": e.CreatedAt})
	}
	return out
}
