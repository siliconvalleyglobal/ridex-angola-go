package auth

import "testing"

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
