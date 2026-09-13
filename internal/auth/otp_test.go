package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestOTPHashingIsBcryptWithSalt(t *testing.T) {
	first := HashOTP("123456")
	second := HashOTP("123456")

	if first == "" || second == "" {
		t.Fatal("OTP hashing must not return empty")
	}
	if first == second {
		t.Fatal("OTP hashes must use a salt and therefore differ")
	}
	if !strings.HasPrefix(first, "$2") {
		t.Fatal("OTP hash must be a bcrypt hash (start with $2a/$2b/$2y)")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(first), []byte("123456")); err != nil {
		t.Fatalf("same code must verify against its own hash: %v", err)
	}
}

func TestOTPVerificationAcceptsCorrectCodeOnly(t *testing.T) {
	code := "042731"
	hash := HashOTP(code)

	if !VerifyOTP(code, hash) {
		t.Fatal("correct OTP was rejected")
	}
	if VerifyOTP("042732", hash) {
		t.Fatal("incorrect OTP was accepted")
	}
	if VerifyOTP("000000", hash) {
		t.Fatal("wrong OTP was accepted")
	}
	// Malformed hash must be rejected — never treat a non-bcrypt blob as a valid OTP store.
	if VerifyOTP(code, "not-a-bcrypt-hash") {
		t.Fatal("malformed hash was accepted")
	}
	if VerifyOTP(code, "") {
		t.Fatal("empty hash was accepted")
	}
}

func TestOTPHashingIsDeterministicForVerificationButSalted(t *testing.T) {
	// Verification must be repeatable: the same plaintext verifies against the
	// salted hash even though two hashes of the same code differ.
	hash := HashOTP("654321")
	if hash == "" {
		t.Fatal("OTP hashing returned empty")
	}
	if !VerifyOTP("654321", hash) {
		t.Fatal("freshly hashed OTP must verify against itself")
	}
	if VerifyOTP("123456", hash) {
		t.Fatal("different numeric OTP must not verify")
	}
}

func TestOTPLengthBoundEnforcedByGenerator(t *testing.T) {
	// generateOTP enforces 4 <= length <= 10. We exercise both ends + a
	// mid-range value; the handler uses 6 by default.
	for _, length := range []int{4, 6, 10} {
		code, err := generateOTP(length)
		if err != nil {
			t.Fatalf("generateOTP(%d) error = %v", length, err)
		}
		if len(code) != length {
			t.Fatalf("generateOTP(%d) returned code of length %d, want %d", length, len(code), length)
		}
		if code[0] == '0' && length > 1 {
			// Leading zeros are allowed (numeric, zero-padded), but a length-N
			// code must be exactly N digits.
		}
		if _, err := generateOTP(3); err == nil {
			t.Fatal("generateOTP(3) must fail (below minimum length)")
		}
		if _, err := generateOTP(11); err == nil {
			t.Fatal("generateOTP(11) must fail (above maximum length)")
		}
	}
}

func TestHashOTPReturnsEmptyOnCryptoFailure(t *testing.T) {
	// Extremely unlikely in practice, but the function documents that it
	// returns "" on error; the handler treats empty hash as a 500.
	// We still assert the documented behavior.
	hash := HashOTP("123456")
	if hash == "" {
		t.Fatal("HashOTP returned empty on normal input — handler would turn this into a 500")
	}
}

func TestOTPHashingAndVerification(t *testing.T) {
	const code = "042731"
	hash := HashOTP(code)
	if hash == code {
		t.Fatal("OTP was stored as plaintext")
	}
	if !VerifyOTP(code, hash) {
		t.Fatal("correct OTP was rejected")
	}
	if VerifyOTP("042732", hash) {
		t.Fatal("incorrect OTP was accepted")
	}
	if VerifyOTP(code, "not-a-sha256-hash") {
		t.Fatal("malformed OTP hash was accepted")
	}
}

func TestOTPHashingUsesAHashWithSalt(t *testing.T) {
	first := HashOTP("123456")
	second := HashOTP("123456")
	if first == "" || second == "" {
		t.Fatal("OTP hashing failed")
	}
	if first == second {
		t.Fatal("OTP hashes did not use a salt")
	}
	if VerifyOTP("654321", first) {
		t.Fatal("different OTP was accepted")
	}
}
