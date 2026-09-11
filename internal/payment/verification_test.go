package payment

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestHMACSHA256VerifierAcceptsCanonicalAndBase64Signatures(t *testing.T) {
	verifier := NewHMACSHA256Verifier("test-secret")
	payload := []byte(`{"event":"payment.completed"}`)
	signature := verifier.Sign(payload)

	if err := verifier.Verify(payload, signature); err != nil {
		t.Fatalf("verify canonical signature: %v", err)
	}
	if err := verifier.Verify(payload, "sha256="+signature); err != nil {
		t.Fatalf("verify prefixed signature: %v", err)
	}

	raw, err := hex.DecodeString(signature)
	if err != nil {
		t.Fatalf("decode generated signature: %v", err)
	}
	if err := verifier.Verify(payload, base64.StdEncoding.EncodeToString(raw)); err != nil {
		t.Fatalf("verify base64 signature: %v", err)
	}
}

func TestHMACSHA256VerifierRejectsTamperingAndMissingSecret(t *testing.T) {
	payload := []byte("payload")
	verifier := NewHMACSHA256Verifier("test-secret")
	if err := verifier.Verify([]byte("tampered"), verifier.Sign(payload)); err == nil {
		t.Fatal("tampered payload was accepted")
	}
	if err := verifier.Verify(payload, "not-a-signature"); err == nil {
		t.Fatal("malformed signature was accepted")
	}
	if err := NewHMACSHA256Verifier("").Verify(payload, ""); err != ErrVerifierUnavailable {
		t.Fatalf("empty secret error = %v, want %v", err, ErrVerifierUnavailable)
	}
}
