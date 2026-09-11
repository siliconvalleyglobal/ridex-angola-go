package config

import "testing"

func TestValidateRejectsDevelopmentSecretsOutsideDevelopment(t *testing.T) {
	c := &Config{
		Environment:         "production",
		JWTAccessSecret:     "dev-access-change-me",
		JWTRefreshSecret:    "dev-refresh-change-me",
		DBPassword:          "ridex_secret",
		MaxRequestBodyBytes: 1024,
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected unsafe production secrets to be rejected")
	}
}

func TestValidateAcceptsDistinctStrongSecretsOutsideDevelopment(t *testing.T) {
	c := &Config{
		Environment:         "production",
		JWTAccessSecret:     "access-secret-with-at-least-32-random-bytes",
		JWTRefreshSecret:    "refresh-secret-with-at-least-32-random-bytes",
		DBPassword:          "another-strong-database-password",
		MaxRequestBodyBytes: 1024,
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
