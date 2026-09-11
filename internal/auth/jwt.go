package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the custom JWT claim set used by RideX.
type Claims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// TokenPair holds a freshly minted access and refresh token.
type TokenPair struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"` // access token expiry
}

// TokenManager signs and verifies access/refresh JWTs.
type TokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewTokenManager builds a TokenManager from secrets and expiry durations.
func NewTokenManager(accessSecret, refreshSecret string, accessExpiry, refreshExpiry time.Duration) *TokenManager {
	return &TokenManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// Issue creates a signed access + refresh token pair for the user.
func (tm *TokenManager) Issue(userID uuid.UUID, role string, sessionID uuid.UUID) (*TokenPair, error) {
	now := time.Now()

	access, err := tm.sign(userID, role, sessionID, now, tm.accessExpiry, tm.accessSecret)
	if err != nil {
		return nil, err
	}
	refresh, err := tm.sign(userID, role, sessionID, now, tm.refreshExpiry, tm.refreshSecret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    now.Add(tm.accessExpiry),
	}, nil
}

func (tm *TokenManager) sign(userID uuid.UUID, role string, sessionID uuid.UUID, now time.Time, ttl time.Duration, secret []byte) (string, error) {
	claims := Claims{
		UserID: userID.String(),
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        sessionID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "ridex-angola",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// VerifyAccess validates an access token and returns its claims.
func (tm *TokenManager) VerifyAccess(token string) (*Claims, error) {
	return tm.verify(token, tm.accessSecret)
}

// VerifyRefresh validates a refresh token and returns its claims.
func (tm *TokenManager) VerifyRefresh(token string) (*Claims, error) {
	return tm.verify(token, tm.refreshSecret)
}

var ErrInvalidToken = errors.New("invalid or expired token")

func (tm *TokenManager) verify(token string, secret []byte) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
