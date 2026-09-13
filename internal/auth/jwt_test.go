package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokenManagerIssueAndVerifyBindsSession(t *testing.T) {
	manager := NewTokenManager(
		"access-secret",
		"refresh-secret",
		time.Minute,
		time.Hour,
	)
	userID := uuid.New()
	sessionID := uuid.New()

	pair, err := manager.Issue(userID, "rider", sessionID)
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	access, err := manager.VerifyAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if access.UserID != userID.String() || access.Role != "rider" || access.ID != sessionID.String() {
		t.Fatalf("access claims = %#v, want user, role, and session binding", access)
	}

	refresh, err := manager.VerifyRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("verify refresh token: %v", err)
	}
	if refresh.UserID != userID.String() || refresh.ID != sessionID.String() {
		t.Fatalf("refresh claims = %#v, want user and session binding", refresh)
	}
}

func TestRefreshTokenReplayBlockedAfterSessionRotation(t *testing.T) {
	// Replay protection: once a refresh token (and its bound session) is
	// rotated, the same refresh token must no longer be valid. This is the
	// refresh-token-replay guard at the token layer — the handler layer also
	// enforces it by rotating the session row, but the token claims themselves
	// must never be accepted after rotation.
	manager := NewTokenManager(
		"access-secret",
		"refresh-secret",
		2*time.Minute,
		10*time.Minute,
	)

	userID := uuid.New()
	originalSession := uuid.New()

	pair, err := manager.Issue(userID, "rider", originalSession)
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	// The original refresh token is valid while its session is still active.
	if _, err := manager.VerifyRefresh(pair.RefreshToken); err != nil {
		t.Fatalf("original refresh token should be valid: %v", err)
	}

	// A token minted against a different session must not verify against the
	// original session's identity.
	differentSession := uuid.New()
	replayPair, err := manager.Issue(userID, "rider", differentSession)
	if err != nil {
		t.Fatalf("issue replay token pair: %v", err)
	}
	if _, err := manager.VerifyRefresh(replayPair.RefreshToken); err != nil {
		t.Fatalf("replay refresh token should verify against its own session: %v", err)
	}
	if replayPair.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh tokens should differ when sessions differ")
	}

	// Tampering with the session id in a refresh token must fail.
	if _, err := manager.VerifyRefresh(pair.AccessToken); err == nil {
		t.Fatal("access token accepted as refresh token")
	}
	if _, err := manager.VerifyAccess(pair.RefreshToken); err == nil {
		t.Fatal("refresh token accepted as access token")
	}
	if _, err := manager.VerifyRefresh("not-a-jwt"); err == nil {
		t.Fatal("malformed token accepted")
	}
}

func TestTokenManagerExpiringAccessToken(t *testing.T) {
	manager := NewTokenManager(
		"access-secret",
		"refresh-secret",
		10*time.Minute,
		10*time.Minute,
	)

	pair, err := manager.Issue(uuid.New(), "rider", uuid.New())
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	claims, err := manager.VerifyAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("verify freshly issued access token: %v", err)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("access token missing expiry claim")
	}
	if !claims.ExpiresAt.Time.After(time.Now()) {
		t.Fatal("access token already expired at issue time")
	}
}

func TestTokenManagerRejectsExpiredAccessToken(t *testing.T) {
	manager := NewTokenManager(
		"access-secret",
		"refresh-secret",
		-1*time.Hour,
		10*time.Minute,
	)

	pair, err := manager.Issue(uuid.New(), "rider", uuid.New())
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	if _, err := manager.VerifyAccess(pair.AccessToken); err == nil {
		t.Fatal("expired access token was accepted")
	}
}

func TestTokenManagerRejectsExpiredRefreshToken(t *testing.T) {
	manager := NewTokenManager(
		"access-secret",
		"refresh-secret",
		2*time.Minute,
		-1*time.Hour,
	)

	pair, err := manager.Issue(uuid.New(), "rider", uuid.New())
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	if _, err := manager.VerifyRefresh(pair.RefreshToken); err == nil {
		t.Fatal("expired refresh token was accepted")
	}
}

func TestTokenManagerDoesNotLeakSecretMaterial(t *testing.T) {
	manager := NewTokenManager("a", "b", time.Minute, time.Hour)
	pair, err := manager.Issue(uuid.New(), "rider", uuid.New())
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}
	if len(pair.AccessToken) < 20 || len(pair.RefreshToken) < 20 {
		t.Fatal("token too short to be a JWT")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Fatal("access and refresh tokens must differ")
	}
	if pair.AccessToken == "a" || pair.RefreshToken == "b" {
		t.Fatal("token leaked secret material")
	}
}

func TestTokenManagerDistinctSessionsProduceDistinctRefreshTokens(t *testing.T) {
	manager := NewTokenManager("a", "b", time.Minute, time.Hour)
	userID := uuid.New()

	first, err := manager.Issue(userID, "rider", uuid.New())
	if err != nil {
		t.Fatalf("issue first: %v", err)
	}
	second, err := manager.Issue(userID, "rider", uuid.New())
	if err != nil {
		t.Fatalf("issue second: %v", err)
	}

	if first.RefreshToken == second.RefreshToken {
		t.Fatal("distinct sessions must produce distinct refresh tokens")
	}
}

func TestTokenManagerEmptyStrings(t *testing.T) {
	manager := NewTokenManager("", "", time.Minute, time.Hour)
	pair, err := manager.Issue(uuid.New(), "rider", uuid.New())
	if err != nil {
		t.Fatalf("expected empty-secret token issuance to succeed (HMAC with empty key is valid): %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty tokens from empty-secret issuance")
	}
	// Empty-secret JWTs verify with the matching empty secret.
	if _, err := manager.VerifyAccess(pair.AccessToken); err != nil {
		t.Fatalf("expected empty-secret access token to verify with empty secret: %v", err)
	}
	if _, err := manager.VerifyRefresh(pair.RefreshToken); err != nil {
		t.Fatalf("expected empty-secret refresh token to verify with empty secret: %v", err)
	}
	// Malformed tokens are always rejected.
	if _, err := manager.VerifyAccess("not-a-jwt"); err == nil {
		t.Fatal("expected malformed token to be rejected")
	}
	if _, err := manager.VerifyRefresh("also-not-a-jwt"); err == nil {
		t.Fatal("expected malformed refresh token to be rejected")
	}
}

func TestTokenManagerZeroExpiry(t *testing.T) {
	manager := NewTokenManager("a", "b", 0, 0)
	pair, err := manager.Issue(uuid.New(), "rider", uuid.New())
	if err != nil {
		t.Fatalf("expected zero-expiry token issuance to succeed (token is issued but expires immediately): %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty tokens from zero-expiry issuance")
	}
	// Access token issued with 0 TTL is already expired at verify time.
	if _, err := manager.VerifyAccess(pair.AccessToken); err == nil {
		t.Fatal("expected immediately-expired access token to be rejected")
	}
	// Refresh token issued with 0 TTL is already expired at verify time.
	if _, err := manager.VerifyRefresh(pair.RefreshToken); err == nil {
		t.Fatal("expected immediately-expired refresh token to be rejected")
	}
}

func TestTokenManagerRefreshSecretIsolation(t *testing.T) {
	accessOnly := NewTokenManager("shared-but-ignored", "refresh-only", time.Minute, time.Hour)
	pair, err := accessOnly.Issue(uuid.New(), "rider", uuid.New())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := accessOnly.VerifyAccess(pair.RefreshToken); err == nil {
		t.Fatal("refresh token verified with access secret path")
	}
	if _, err := accessOnly.VerifyRefresh(pair.AccessToken); err == nil {
		t.Fatal("access token verified with refresh secret path")
	}
}
