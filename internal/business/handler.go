package business

import (
	"errors"
	"io"
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

// Handler exposes corporate account administration without adding a second
// identity system: members are existing rider users.
type Handler struct{ q *db.Queries }

func NewHandler(q *db.Queries) *Handler { return &Handler{q: q} }

type accountRequest struct {
	Name                string `json:"name" binding:"required,min=1,max=120"`
	Currency            string `json:"currency"`
	MonthlyLimitCents   *int64 `json:"monthlyLimitCents"`
	RequireRideApproval bool   `json:"requireRideApproval"`
}

type memberRequest struct {
	UserID             string `json:"userId" binding:"required"`
	Role               string `json:"role" binding:"omitempty,oneof=admin member"`
	SpendingLimitCents *int64 `json:"spendingLimitCents"`
}

type memberUpdateRequest struct {
	SpendingLimitCents *int64 `json:"spendingLimitCents"`
	Status             string `json:"status" binding:"omitempty,oneof=active suspended"`
}

type limitsRequest struct {
	MonthlyLimitCents   *int64 `json:"monthlyLimitCents"`
	RequireRideApproval bool   `json:"requireRideApproval"`
}

type decisionRequest struct {
	Note string `json:"note" binding:"max=240"`
}

func (h *Handler) CreateAccount(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req accountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid business account body"})
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "AOA"
	}
	if currency != "AOA" || req.MonthlyLimitCents != nil && *req.MonthlyLimitCents < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid currency or monthly limit"})
		return
	}
	account, err := h.q.CreateBusinessAccount(c.Request.Context(), db.CreateBusinessAccountParams{
		OwnerID: uid, Name: strings.TrimSpace(req.Name), Currency: currency,
		MonthlyLimitCents:   optionalCents(req.MonthlyLimitCents),
		RequireRideApproval: req.RequireRideApproval,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create business account"})
		return
	}
	if _, err := h.q.CreateBusinessMember(c.Request.Context(), db.CreateBusinessMemberParams{
		AccountID: account.ID, UserID: uid, Role: "owner",
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account created but owner membership failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"account": account})
}

func (h *Handler) ListAccounts(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	page, size, offset := pageParams(c)
	accounts, err := h.q.ListBusinessAccountsForUser(c.Request.Context(), db.ListBusinessAccountsForUserParams{
		UserID: uid, Limit: int32(size + 1), Offset: int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list business accounts"})
		return
	}
	hasMore := len(accounts) > size
	if hasMore {
		accounts = accounts[:size]
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts, "pagination": gin.H{
		"page": page, "pageSize": size, "hasMore": hasMore,
	}})
}

func (h *Handler) GetAccount(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	accountID, ok := pathUUID(c, "id")
	if !ok || !h.canAccess(c, accountID, uid) {
		return
	}
	account, err := h.q.GetBusinessAccount(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "business account not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"account": account})
}

func (h *Handler) AddMember(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	accountID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	if !h.canManage(c, accountID, uid) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req memberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member body"})
		return
	}
	memberID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}
	if req.SpendingLimitCents != nil && *req.SpendingLimitCents < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "spending limit cannot be negative"})
		return
	}
	user, err := h.q.GetUserByID(c.Request.Context(), memberID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err != nil || user.Role != "rider" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "business members must be rider users"})
		return
	}
	member, err := h.q.CreateBusinessMember(c.Request.Context(), db.CreateBusinessMemberParams{
		AccountID: accountID, UserID: memberID, Role: req.Role,
		SpendingLimitCents: optionalCents(req.SpendingLimitCents),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add business member"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"member": member})
}

func (h *Handler) ListMembers(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	accountID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	if !h.canAccess(c, accountID, uid) {
		return
	}
	page, size, offset := pageParams(c)
	members, err := h.q.ListBusinessMembers(c.Request.Context(), db.ListBusinessMembersParams{
		AccountID: accountID, Limit: int32(size + 1), Offset: int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list business members"})
		return
	}
	hasMore := len(members) > size
	if hasMore {
		members = members[:size]
	}
	c.JSON(http.StatusOK, gin.H{"members": members, "pagination": gin.H{
		"page": page, "pageSize": size, "hasMore": hasMore,
	}})
}

func (h *Handler) UpdateMember(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	accountID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	memberID, ok := pathUUID(c, "userId")
	if !ok {
		return
	}
	if !h.canManage(c, accountID, uid) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req memberUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil ||
		req.SpendingLimitCents != nil && *req.SpendingLimitCents < 0 ||
		req.Status != "" && req.Status != "active" && req.Status != "suspended" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member limits body"})
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	member, err := h.q.UpdateBusinessMemberLimit(c.Request.Context(), db.UpdateBusinessMemberLimitParams{
		AccountID: accountID, UserID: memberID,
		SpendingLimitCents: optionalCents(req.SpendingLimitCents), Status: status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "business member not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update business member"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"member": member})
}

func (h *Handler) UpdateLimits(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	accountID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	if !h.canManage(c, accountID, uid) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req limitsRequest
	if err := c.ShouldBindJSON(&req); err != nil ||
		req.MonthlyLimitCents != nil && *req.MonthlyLimitCents < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spending limits body"})
		return
	}
	account, err := h.q.UpdateBusinessAccountLimits(c.Request.Context(), db.UpdateBusinessAccountLimitsParams{
		ID: accountID, MonthlyLimitCents: optionalCents(req.MonthlyLimitCents),
		RequireRideApproval: req.RequireRideApproval,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update business limits"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"account": account})
}

func (h *Handler) Usage(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	accountID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	if !h.canAccess(c, accountID, uid) {
		return
	}
	month := time.Now().UTC()
	if value := strings.TrimSpace(c.Query("month")); value != "" {
		parsed, err := time.Parse("2006-01", value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "month must use YYYY-MM"})
			return
		}
		month = parsed.UTC()
	}
	usage, err := h.q.GetBusinessMonthlyUsage(c.Request.Context(), db.GetBusinessMonthlyUsageParams{
		BusinessAccountID: pgUUID(accountID),
		Column2:           pgtype.Date{Time: month, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load monthly usage"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"month": month.Format("2006-01"), "usage": usage})
}

func (h *Handler) PendingAuthorizations(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	accountID, ok := pathUUID(c, "id")
	if !ok || !h.canManage(c, accountID, uid) {
		return
	}
	page, size, offset := pageParams(c)
	items, err := h.q.ListPendingRideAuthorizations(c.Request.Context(), db.ListPendingRideAuthorizationsParams{
		AccountID: accountID, Limit: int32(size + 1), Offset: int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ride authorizations"})
		return
	}
	hasMore := len(items) > size
	if hasMore {
		items = items[:size]
	}
	c.JSON(http.StatusOK, gin.H{"authorizations": items, "pagination": gin.H{
		"page": page, "pageSize": size, "hasMore": hasMore,
	}})
}

func (h *Handler) DecideAuthorization(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	rideID, ok := pathUUID(c, "rideId")
	if !ok {
		return
	}
	authz, err := h.q.GetRideAuthorization(c.Request.Context(), rideID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "ride authorization not found"})
		return
	}
	if err != nil || !h.canManage(c, authz.AccountID, uid) {
		return
	}
	decision := strings.ToLower(strings.TrimSpace(c.Param("decision")))
	if decision != "approve" && decision != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "decision must be approve or reject"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req decisionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid authorization decision"})
		return
	}
	if decision != "reject" {
		account, accountErr := h.q.GetBusinessAccount(c.Request.Context(), authz.AccountID)
		member, memberErr := h.q.GetBusinessMember(c.Request.Context(), db.GetBusinessMemberParams{
			AccountID: authz.AccountID, UserID: authz.MemberID,
		})
		usage, usageErr := h.q.GetBusinessMonthlyUsage(c.Request.Context(), db.GetBusinessMonthlyUsageParams{
			BusinessAccountID: pgUUID(authz.AccountID),
			Column2:           pgtype.Date{Time: time.Now().UTC(), Valid: true},
		})
		if accountErr != nil || memberErr != nil || usageErr != nil ||
			exceedsLimit(usage.TotalCents, 0, account.MonthlyLimitCents) ||
			exceedsLimit(usage.TotalCents, 0, member.SpendingLimitCents) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "approval would exceed business spending limit"})
			return
		}
	}
	var updated db.RideAuthorization
	params := db.ApproveRideAuthorizationParams{
		RideID: rideID, AuthorizedBy: pgUUID(uid),
		DecisionNote: pgtype.Text{String: strings.TrimSpace(req.Note), Valid: req.Note != ""},
	}
	if decision == "reject" {
		updated, err = h.q.RejectRideAuthorization(c.Request.Context(), db.RejectRideAuthorizationParams(params))
	} else {
		updated, err = h.q.ApproveRideAuthorization(c.Request.Context(), params)
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "ride authorization is no longer pending"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"authorization": updated})
}

func (h *Handler) canAccess(c *gin.Context, accountID, uid uuid.UUID) bool {
	account, err := h.q.GetBusinessAccount(c.Request.Context(), accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "business account not found"})
		return false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load business account"})
		return false
	}
	if !account.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "business account is inactive"})
		return false
	}
	member, err := h.q.GetBusinessMember(c.Request.Context(), db.GetBusinessMemberParams{
		AccountID: accountID, UserID: uid,
	})
	if err != nil || member.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a business member"})
		return false
	}
	return true
}

func (h *Handler) canManage(c *gin.Context, accountID, uid uuid.UUID) bool {
	if !h.canAccess(c, accountID, uid) {
		return false
	}
	account, _ := h.q.GetBusinessAccount(c.Request.Context(), accountID)
	member, _ := h.q.GetBusinessMember(c.Request.Context(), db.GetBusinessMemberParams{
		AccountID: accountID, UserID: uid,
	})
	if account.OwnerID != uid && member.Role != "owner" && member.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "business admin access required"})
		return false
	}
	return true
}

func userID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.GetString(auth.ContextUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return uuid.Nil, false
	}
	return id, true
}

func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func pageParams(c *gin.Context) (page, size, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size, (page - 1) * size
}

func optionalCents(value *int64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	return pgtype.Numeric{Int: big.NewInt(*value), Valid: true}
}

func exceedsLimit(used pgtype.Numeric, requested int64, limit pgtype.Numeric) bool {
	if !limit.Valid || limit.Int == nil || limit.Exp != 0 ||
		!used.Valid || used.Int == nil || used.Exp != 0 {
		return false
	}
	return used.Int.Int64()+requested > limit.Int.Int64()
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
