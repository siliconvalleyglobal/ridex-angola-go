// Package auth provides test helpers for auth handler and token-manager tests.
package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ridex/ridex-angola/internal/db"
	"golang.org/x/crypto/bcrypt"
	"time"
)

// testRouter returns a Gin test-mode router mounted with the given handler at
// /auth. Callers must call t.Cleanup(router.Close) when done.
func testRouter(t *testing.T, h *Handler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.Refresh)
	r.POST("/auth/logout", h.Logout)
	r.POST("/auth/otp/request", h.RequestOTP)
	r.POST("/auth/otp/verify", h.VerifyOTP)
	r.POST("/auth/password-reset/request", h.RequestPasswordReset)
	r.POST("/auth/password-reset/reset", h.CompletePasswordReset)
	return r
}

// registerUser registers a new user via the /auth/register endpoint.
// It returns the created user's phone, name, role, and password for test use.
func registerUser(t *testing.T, router *gin.Engine, phone, name, role, password string) {
	t.Helper()
	w := httptest.NewRecorder()
	body := map[string]string{
		"phone":    phone,
		"name":     name,
		"role":     role,
		"password": password,
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode register body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, "/auth/register", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("register user: want 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// loginAndGetPair logs in a user and returns the token pair.
func loginAndGetPair(t *testing.T, router *gin.Engine, phone, password string) (accessToken, refreshToken string, expiresAt time.Time) {
	t.Helper()
	w := httptest.NewRecorder()
	body := map[string]string{
		"phone":    phone,
		"password": password,
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode login body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, "/auth/login", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var pair struct {
		AccessToken  string    `json:"accessToken"`
		RefreshToken string    `json:"refreshToken"`
		ExpiresAt    time.Time `json:"expiresAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pair); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	return pair.AccessToken, pair.RefreshToken, pair.ExpiresAt
}

// refreshWithToken exchanges a refresh token for a new token pair.
func refreshWithToken(t *testing.T, router *gin.Engine, refreshToken string) (accessToken, newRefreshToken string, expiresAt time.Time) {
	t.Helper()
	w := httptest.NewRecorder()
	body := map[string]string{
		"refreshToken": refreshToken,
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode refresh body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, "/auth/refresh", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: want 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var pair struct {
		AccessToken  string    `json:"accessToken"`
		RefreshToken string    `json:"refreshToken"`
		ExpiresAt    time.Time `json:"expiresAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pair); err != nil {
		t.Fatalf("unmarshal refresh response: %v", err)
	}
	return pair.AccessToken, pair.RefreshToken, pair.ExpiresAt
}

// hashPassword hashes a password using bcrypt with the test cost.
func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(hash)
}

// checkPassword checks a password against a bcrypt hash.
func checkPassword(t *testing.T, password, hash string) bool {
	t.Helper()
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// createUserInDB creates a user directly in the database for tests that bypass
// the HTTP layer.
func createUserInDB(t *testing.T, q *db.Queries, phone, name, role, password string) db.User {
	t.Helper()
	hash := hashPassword(t, password)
	user, err := q.CreateUser(t.Context(), db.CreateUserParams{
		Phone:        phone,
		Name:         name,
		Role:         role,
		PasswordHash: hash,
	})
	if err != nil {
		t.Fatalf("create user in db: %v", err)
	}
	return user
}

// parseJSONStatus parses the JSON response body and returns the status code
// and a map of the response.
func parseJSONStatus(t *testing.T, r *httptest.ResponseRecorder) (int, map[string]interface{}) {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(r.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return r.Code, resp
}

// newOTP returns an OTP delivery suitable for tests (defaults to no-op).
func newOTP(t *testing.T) OTPDelivery {
	t.Helper()
	return NoopOTPDelivery{}
}

// newLocalizedOTP returns a LocalizedOTPDelivery wrapping a no-op delivery.
func newLocalizedOTP(t *testing.T) *LocalizedOTPDelivery {
	t.Helper()
	return NewLocalizedOTPDelivery(NoopOTPDelivery{})
}

// bcryptCost is the bcrypt cost used by test password helpers.
const bcryptCost = 4

// adminAuthHeader returns a Bearer token for a fresh admin login.
func adminAuthHeader(t *testing.T, router *gin.Engine, phone, role, password string) string {
	t.Helper()
	accessToken, _, _ := loginAndGetPair(t, router, phone, password)
	return "Bearer " + accessToken
}

// testOTP wraps the no-op OTP delivery for tests.
func testOTP(t *testing.T) OTPDelivery {
	t.Helper()
	return NoopOTPDelivery{}
}

// testLocalizedOTP wraps a no-op delivery with test locale overrides.
func testLocalizedOTP(t *testing.T) *LocalizedOTPDelivery {
	t.Helper()
	return NewLocalizedOTPDelivery(NoopOTPDelivery{})
}
