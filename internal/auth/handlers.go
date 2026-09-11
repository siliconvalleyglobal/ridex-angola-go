package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ridex/ridex-angola/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// Handler implements the auth HTTP endpoints.
type Handler struct {
	q              *db.Queries
	tokens         *TokenManager
	refresh        time.Duration // refresh token lifetime, for session expiry
	otpDelivery    OTPDelivery
	otpExpiry      time.Duration
	otpLength      int
	resendThrottle time.Duration
}

// NewHandler creates an auth handler wired to the DB queries and token manager.
func NewHandler(q *db.Queries, tokens *TokenManager, refreshExpiry time.Duration) *Handler {
	return NewHandlerWithOTP(q, tokens, refreshExpiry, NoopOTPDelivery{}, 5*time.Minute, 6, time.Minute)
}

// NewHandlerWithOTP wires an OTP delivery implementation into the auth handler.
// The default application wiring deliberately uses NoopOTPDelivery until a
// documented provider is selected and implemented.
func NewHandlerWithOTP(q *db.Queries, tokens *TokenManager, refreshExpiry time.Duration, delivery OTPDelivery, otpExpiry time.Duration, otpLength int, resendThrottle time.Duration) *Handler {
	if delivery == nil {
		delivery = NoopOTPDelivery{}
	}
	if otpExpiry <= 0 {
		otpExpiry = 5 * time.Minute
	}
	if otpLength < 4 || otpLength > 10 {
		otpLength = 6
	}
	if resendThrottle <= 0 {
		resendThrottle = time.Minute
	}
	return &Handler{
		q:              q,
		tokens:         tokens,
		refresh:        refreshExpiry,
		otpDelivery:    delivery,
		otpExpiry:      otpExpiry,
		otpLength:      otpLength,
		resendThrottle: resendThrottle,
	}
}

// ── Requests ──────────────────────────────────────────────────────────

type registerRequest struct {
	Phone    string `json:"phone" binding:"required,min=9,max=20"`
	Name     string `json:"name" binding:"required,min=2,max=80"`
	Role     string `json:"role" binding:"required,oneof=rider driver"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type loginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type otpRequest struct {
	Phone   string `json:"phone" binding:"required,min=7,max=30"`
	Purpose string `json:"purpose" binding:"omitempty,oneof=login registration phone_change password_reset"`
}

type otpVerifyRequest struct {
	RequestID string `json:"requestId" binding:"required"`
	Code      string `json:"code" binding:"required"`
}

type passwordResetRequest struct {
	RequestID string `json:"requestId" binding:"required"`
	Password  string `json:"password" binding:"required,min=8,max=72"`
	Code      string `json:"code,omitempty"`
}

// ── Endpoints ─────────────────────────────────────────────────────────

// Register creates a new user and issues a token pair.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to hash password"})
		return
	}

	user, err := h.q.CreateUser(c.Request.Context(), db.CreateUserParams{
		Phone:        req.Phone,
		Name:         req.Name,
		Role:         req.Role,
		PasswordHash: hash,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(409, gin.H{"error": "phone number already registered"})
			return
		}
		c.JSON(500, gin.H{"error": "failed to create user"})
		return
	}

	if err := h.issueAndTrack(c, user.ID, user.Role); err != nil {
		c.JSON(500, gin.H{"error": "failed to issue tokens"})
	}
}

// Login authenticates with phone + password and issues a token pair.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}

	user, err := h.q.GetUserByPhone(c.Request.Context(), req.Phone)
	if err != nil {
		// Same message whether phone exists or password is wrong (no user enumeration).
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	if !CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	if err := h.issueAndTrack(c, user.ID, user.Role); err != nil {
		c.JSON(500, gin.H{"error": "failed to issue tokens"})
	}
}

// Refresh exchanges a valid refresh token for a new token pair.
func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}

	claims, err := h.tokens.VerifyRefresh(req.RefreshToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid refresh token"})
		return
	}

	sessionID, err := uuid.Parse(claims.ID)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid refresh token"})
		return
	}
	if _, err := h.q.RotateSession(c.Request.Context(), db.RotateSessionParams{
		ID:               sessionID,
		RefreshTokenHash: pgtype.Text{String: hashRefreshToken(req.RefreshToken), Valid: true},
	}); err != nil {
		c.JSON(401, gin.H{"error": "refresh token has been revoked or expired"})
		return
	}

	user, err := h.q.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(401, gin.H{"error": "user no longer active"})
		return
	}
	if err := h.issueAndTrack(c, user.ID, user.Role); err != nil {
		c.JSON(500, gin.H{"error": "failed to issue tokens"})
	}
}

// RequestOTP creates a short-lived, single-use OTP challenge. It intentionally
// returns the same response when the phone is unknown and never returns the
// code to the caller.
func (h *Handler) RequestOTP(c *gin.Context) {
	var req otpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if req.Purpose == "" {
		req.Purpose = "login"
	}
	h.requestOTP(c, req.Phone, req.Purpose)
}

// RequestPasswordReset starts a password-reset OTP challenge.
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required,min=7,max=30"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	h.requestOTP(c, req.Phone, "password_reset")
}

// VerifyOTP checks and consumes an OTP challenge. Every submitted code counts
// as an attempt, including an incorrect code.
func (h *Handler) VerifyOTP(c *gin.Context) {
	var req otpVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request id"})
		return
	}
	code, err := h.verifyOTP(c.Request.Context(), requestID, req.Code)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid, expired, or locked verification code"})
		return
	}
	c.JSON(200, gin.H{
		"verified":  true,
		"requestId": requestID,
		"userId":    code.UserID,
		"purpose":   code.Purpose,
	})
}

// VerifyPasswordResetOTP is the password-reset-specific OTP verification
// endpoint. The reset is completed separately with the same opaque request ID.
func (h *Handler) VerifyPasswordResetOTP(c *gin.Context) {
	var req otpVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request id"})
		return
	}
	code, err := h.verifyOTP(c.Request.Context(), requestID, req.Code)
	if err != nil || code.Purpose != "password_reset" {
		c.JSON(401, gin.H{"error": "invalid, expired, or locked verification code"})
		return
	}
	c.JSON(200, gin.H{"verified": true, "requestId": requestID})
}

// CompletePasswordReset updates the password after a verified reset
// challenge. Supplying code is supported for clients that use a single-step
// reset; otherwise VerifyPasswordResetOTP must have run first.
func (h *Handler) CompletePasswordReset(c *gin.Context) {
	var req passwordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request id"})
		return
	}
	ctx := c.Request.Context()
	var code db.VerificationCode
	if req.Code != "" {
		code, err = h.verifyOTP(ctx, requestID, req.Code)
	} else {
		code, err = h.q.GetVerifiedVerificationCode(ctx, requestID)
		if err == nil && code.Purpose != "password_reset" {
			err = errors.New("wrong OTP purpose")
		}
	}
	if err != nil || code.Purpose != "password_reset" {
		c.JSON(401, gin.H{"error": "password reset verification is invalid or expired"})
		return
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to hash password"})
		return
	}
	if _, err := h.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: code.UserID, PasswordHash: hash}); err != nil {
		c.JSON(401, gin.H{"error": "password reset is invalid or expired"})
		return
	}
	_ = h.q.ConsumeVerifiedVerificationCode(ctx, code.ID)
	_ = h.q.DeleteSessionsForUser(ctx, code.UserID)
	c.JSON(200, gin.H{"status": "password reset"})
}

// Logout revokes all sessions for the authenticated user.
func (h *Handler) Logout(c *gin.Context) {
	id, err := uuid.Parse(c.GetString(ContextUserID))
	if err == nil {
		_ = h.q.DeleteSessionsForUser(c.Request.Context(), id)
	}
	c.JSON(200, gin.H{"status": "logged out"})
}

// Me returns the authenticated user's profile.
func (h *Handler) Me(c *gin.Context) {
	id, err := uuid.Parse(c.GetString(ContextUserID))
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return
	}
	user, err := h.q.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.JSON(200, gin.H{
		"id":     user.ID,
		"phone":  user.Phone,
		"name":   user.Name,
		"role":   user.Role,
		"rating": user.Rating,
	})
}

// issueAndTrack mints a token pair and records the session audit row.
func (h *Handler) issueAndTrack(c *gin.Context, userID uuid.UUID, role string) error {
	sessionID := uuid.New()
	pair, err := h.tokens.Issue(userID, role, sessionID)
	if err != nil {
		return err
	}

	_, err = h.q.CreateSession(c.Request.Context(), db.CreateSessionParams{
		ID:               sessionID,
		UserID:           userID,
		DeviceID:         c.GetHeader("X-Device-Id"),
		IpAddress:        c.ClientIP(),
		UserAgent:        c.Request.UserAgent(),
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(h.refresh), Valid: true},
		RefreshTokenHash: pgtype.Text{String: hashRefreshToken(pair.RefreshToken), Valid: true},
	})
	if err != nil {
		return err
	}

	c.JSON(200, gin.H{
		"accessToken":  pair.AccessToken,
		"refreshToken": pair.RefreshToken,
		"expiresAt":    pair.ExpiresAt.UTC().Format(time.RFC3339),
		"user":         gin.H{"id": userID, "role": role},
	})
	return nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (h *Handler) requestOTP(c *gin.Context, phone, purpose string) {
	ctx := c.Request.Context()
	user, err := h.q.GetUserByPhone(ctx, phone)
	if err != nil {
		// Do not disclose whether a phone is registered.
		c.JSON(202, gin.H{
			"status":    "accepted",
			"requestId": uuid.New(),
			"expiresAt": time.Now().Add(h.otpExpiry).UTC().Format(time.RFC3339),
			"delivery":  "not_configured",
		})
		return
	}
	latest, latestErr := h.q.GetLatestVerificationCode(ctx, db.GetLatestVerificationCodeParams{
		UserID: user.ID, Purpose: purpose,
	})
	if latestErr == nil && latest.CreatedAt.Valid && time.Since(latest.CreatedAt.Time) < h.resendThrottle {
		c.Header("Retry-After", strconv.Itoa(int(time.Until(latest.CreatedAt.Time.Add(h.resendThrottle)).Seconds())+1))
		c.JSON(429, gin.H{"error": "verification code recently requested"})
		return
	}
	if err := h.q.InvalidateVerificationCodes(ctx, db.InvalidateVerificationCodesParams{
		UserID: user.ID, Purpose: purpose,
	}); err != nil {
		c.JSON(500, gin.H{"error": "failed to create verification challenge"})
		return
	}
	code, err := generateOTP(h.otpLength)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to create verification challenge"})
		return
	}
	hash := HashOTP(code)
	if hash == "" {
		c.JSON(500, gin.H{"error": "failed to create verification challenge"})
		return
	}
	challenge, err := h.q.CreateVerificationCode(ctx, db.CreateVerificationCodeParams{
		UserID: user.ID, CodeHash: pgtype.Text{String: hash, Valid: true},
		Purpose: purpose, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(h.otpExpiry), Valid: true},
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to create verification challenge"})
		return
	}
	deliveryStatus := "not_configured"
	if err := h.otpDelivery.DeliverOTP(ctx, phone, purpose, code); err == nil {
		// "accepted" describes the interface call only; this endpoint does
		// not claim that an SMS was delivered.
		deliveryStatus = "accepted"
	} else if !errors.Is(err, ErrOTPDeliveryNotConfigured) {
		c.JSON(500, gin.H{"error": "failed to submit verification challenge"})
		return
	}
	c.JSON(202, gin.H{
		"status":    "accepted",
		"requestId": challenge.ID,
		"expiresAt": challenge.ExpiresAt.Time.UTC().Format(time.RFC3339),
		"delivery":  deliveryStatus,
	})
}

func (h *Handler) verifyOTP(ctx context.Context, id uuid.UUID, plaintext string) (db.VerificationCode, error) {
	row, err := h.q.RecordVerificationAttempt(ctx, id)
	if err != nil || !row.CodeHash.Valid {
		return db.VerificationCode{}, errors.New("challenge unavailable")
	}
	if !VerifyOTP(plaintext, row.CodeHash.String) {
		return db.VerificationCode{}, errors.New("invalid code")
	}
	return h.q.MarkVerificationCodeVerified(ctx, id)
}

// HashOTP hashes an OTP before it is persisted. Bcrypt's salt and work factor
// protect the low-entropy code if the verification table is exposed.
func HashOTP(code string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

// VerifyOTP compares a submitted OTP against its persisted bcrypt hash.
func VerifyOTP(code, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil
}

func generateOTP(length int) (string, error) {
	if length < 4 || length > 10 {
		return "", errors.New("invalid OTP length")
	}
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", length, n.Int64()), nil
}

// ── Middleware ────────────────────────────────────────────────────────

const (
	ContextUserID = "authUserID"
	ContextRole   = "authRole"
)

// Middleware validates the Bearer access token and stores claims in the context.
func (tm *TokenManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			abortWith(c, 401, "missing or malformed Authorization header")
			return
		}
		claims, err := tm.VerifyAccess(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			abortWith(c, 401, "invalid or expired access token")
			return
		}
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextRole, claims.Role)
		c.Next()
	}
}

// RequireRole restricts the route to the given roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(ContextRole)
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		abortWith(c, 403, "insufficient permissions")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword compares plaintext against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func abort(c *gin.Context, msg string) {
	abortWith(c, 401, msg)
}

func abortWith(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}
