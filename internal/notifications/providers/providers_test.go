package providers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ridex/ridex-angola/internal/notifications"
)

// rsaServiceAccountPEM builds a minimal service-account JSON for tests.
func rsaServiceAccountPEM(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	sa, _ := json.Marshal(serviceAccount{
		ClientEmail: "push@test.iam.gserviceaccount.com",
		PrivateKey:  string(pemKey),
		ProjectID:   "ridex-test",
	})
	return string(sa), "ridex-test"
}

func TestFCMDisabledWithoutCredentials(t *testing.T) {
	d := NewFCMDelivery("", "")
	err := d.SendPush(context.Background(), "tok", notifications.PushNotification{Title: "t", Body: "b"})
	if !errors.Is(err, notifications.ErrDeliveryNotConfigured{Channel: "push"}) {
		t.Fatalf("err = %v, want ErrDeliveryNotConfigured", err)
	}
	if d.IsEnabled() {
		t.Fatal("provider should be disabled")
	}
}

func TestFCMDisabledWithMalformedCredentials(t *testing.T) {
	d := NewFCMDelivery("proj", "{not json")
	if d.IsEnabled() {
		t.Fatal("provider should be disabled for malformed credentials")
	}
}

func TestFCMSendPushPostsV1Message(t *testing.T) {
	sa, projectID := rsaServiceAccountPEM(t)

	var gotAuth string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "ya29.test", "expires_in": 3600})
			return
		}
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Route both token exchange and send to the test server.
	oldTokenURL, oldEndpoint := fcmTokenURL, fcmEndpoint
	fcmTokenURL = server.URL + "/token"
	fcmEndpoint = server.URL + "/v1/projects/%s/messages:send"
	defer func() { fcmTokenURL, fcmEndpoint = oldTokenURL, oldEndpoint }()

	d := NewFCMDelivery(projectID, sa)
	if !d.IsEnabled() {
		t.Fatal("provider should be enabled with valid credentials")
	}
	err := d.SendPush(context.Background(), "devtok", notifications.PushNotification{
		Title: "Ride accepted", Body: "On the way", Data: map[string]string{"rideId": "r1"},
	})
	if err != nil {
		t.Fatalf("SendPush: %v", err)
	}
	if gotAuth != "Bearer ya29.test" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotPath != "/v1/projects/ridex-test/messages:send" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestAPNSTokenIncludesKidAndCache(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})

	d := NewAPNSDelivery("TEAM123", "KEY456", "com.ridex.app", string(pemKey), false)
	if !d.IsEnabled() {
		t.Fatal("provider should be enabled")
	}
	tok1, err := d.providerToken()
	if err != nil {
		t.Fatalf("providerToken: %v", err)
	}
	tok2, _ := d.providerToken()
	if tok1 != tok2 {
		t.Fatal("token should be cached")
	}

	parsed, err := jwt.Parse(tok1, func(tok *jwt.Token) (any, error) {
		return key.Public(), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Header["kid"] != "KEY456" {
		t.Fatalf("kid = %v, want KEY456", parsed.Header["kid"])
	}
}

func TestAPNSDisabledWithoutCredentials(t *testing.T) {
	d := NewAPNSDelivery("", "", "", "", false)
	err := d.SendPush(context.Background(), "tok", notifications.PushNotification{Title: "t", Body: "b"})
	if !errors.Is(err, notifications.ErrDeliveryNotConfigured{Channel: "push"}) {
		t.Fatalf("err = %v, want ErrDeliveryNotConfigured", err)
	}
}
