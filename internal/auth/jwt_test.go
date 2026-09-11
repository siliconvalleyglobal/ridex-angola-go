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

func TestTokenManagerRejectsWrongSecretAndMalformedToken(t *testing.T) {
	manager := NewTokenManager(
		"access-secret",
		"refresh-secret",
		time.Minute,
		time.Hour,
	)
	pair, err := manager.Issue(uuid.New(), "driver", uuid.New())
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	if _, err := manager.VerifyRefresh(pair.AccessToken); err == nil {
		t.Fatal("access token was accepted as refresh token")
	}
	if _, err := manager.VerifyAccess(pair.RefreshToken); err == nil {
		t.Fatal("refresh token was accepted as access token")
	}
	if _, err := manager.VerifyRefresh("not-a-jwt"); err == nil {
		t.Fatal("malformed token was accepted")
	}
}
