package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// WebhookVerifier verifies the raw request body before it is written to the
// payment ledger. Provider adapters can implement this interface when their
// signature scheme is selected and documented.
type WebhookVerifier interface {
	Verify(payload []byte, signature string) error
}

var (
	ErrInvalidSignature     = errors.New("invalid webhook signature")
	ErrVerifierUnavailable  = errors.New("webhook verification is not configured")
	ErrUnsupportedSignature = errors.New("unsupported webhook signature encoding")
)

// HMACSHA256Verifier is a provider-neutral HMAC-SHA256 verifier. It accepts a
// raw hexadecimal or base64 digest, with an optional "sha256=" prefix.
type HMACSHA256Verifier struct {
	secret []byte
}

func NewHMACSHA256Verifier(secret string) *HMACSHA256Verifier {
	return &HMACSHA256Verifier{secret: []byte(secret)}
}

// VerifyHMACSHA256 is a convenience wrapper for callers that do not need to
// retain a verifier instance.
func VerifyHMACSHA256(secret string, payload []byte, signature string) error {
	return NewHMACSHA256Verifier(secret).Verify(payload, signature)
}

// Sign returns the canonical hexadecimal digest used by tests and internal
// callers. Provider adapters should format signatures according to their
// documented protocol before calling Verify.
func (v *HMACSHA256Verifier) Sign(payload []byte) string {
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (v *HMACSHA256Verifier) Verify(payload []byte, signature string) error {
	if len(v.secret) == 0 {
		return ErrVerifierUnavailable
	}
	signature = strings.TrimSpace(signature)
	if strings.HasPrefix(signature, "sha256=") {
		signature = strings.TrimPrefix(signature, "sha256=")
	}

	var provided []byte
	if decoded, err := hex.DecodeString(signature); err == nil {
		provided = decoded
	} else if decoded, err := base64.StdEncoding.DecodeString(signature); err == nil {
		provided = decoded
	} else {
		return ErrUnsupportedSignature
	}

	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
		return ErrInvalidSignature
	}
	return nil
}

// RejectingVerifier is the safe default when no webhook secret has been
// configured. It prevents an endpoint from accepting unauthenticated events.
type RejectingVerifier struct{}

func (RejectingVerifier) Verify([]byte, string) error { return ErrVerifierUnavailable }
