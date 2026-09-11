package providers

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ridex/ridex-angola/internal/notifications"
)

// APNSDelivery implements DeliveryProvider using Apple Push Notification
// service over HTTP/2 with token-based (JWT) authentication. Provider tokens
// are valid for at most one hour and are cached until shortly before expiry.
type APNSDelivery struct {
	teamID     string
	keyID      string
	bundleID   string
	privateKey *ecdsa.PrivateKey
	host       string
	enabled    bool

	mu          sync.Mutex
	providerJWT string
	tokenExp    time.Time
	client      *http.Client
}

// NewAPNSDelivery creates a new APNs delivery provider. privateKey may be a
// path to a .p8 key file or the PEM itself. It returns a disabled provider if
// credentials are not configured.
func NewAPNSDelivery(teamID, keyID, bundleID, privateKey string, production bool) *APNSDelivery {
	if teamID == "" || keyID == "" || bundleID == "" || privateKey == "" {
		return &APNSDelivery{enabled: false}
	}
	raw := privateKey
	if data, err := os.ReadFile(privateKey); err == nil {
		raw = string(data)
	}
	key, err := jwt.ParseECPrivateKeyFromPEM([]byte(raw))
	if err != nil {
		return &APNSDelivery{enabled: false}
	}
	host := "https://api.push.apple.com"
	if !production {
		host = "https://api.development.push.apple.com"
	}
	return &APNSDelivery{
		teamID:     teamID,
		keyID:      keyID,
		bundleID:   bundleID,
		privateKey: key,
		host:       host,
		enabled:    true,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// SendPush sends a push notification via APNs HTTP/2.
func (d *APNSDelivery) SendPush(ctx context.Context, deviceToken string, notification notifications.PushNotification) error {
	if !d.enabled {
		return notifications.ErrDeliveryNotConfigured{Channel: "push"}
	}
	if deviceToken == "" {
		return fmt.Errorf("device token is required")
	}

	authJWT, err := d.providerToken()
	if err != nil {
		return fmt.Errorf("apns auth: %w", err)
	}

	payload := map[string]any{
		"aps": map[string]any{
			"alert": map[string]string{
				"title": notification.Title,
				"body":  notification.Body,
			},
			"sound": "default",
		},
	}
	for k, v := range notification.Data {
		payload[k] = v
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("apns encode: %w", err)
	}

	url := d.host + "/3/device/" + deviceToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apns request: %w", err)
	}
	req.Header.Set("Authorization", "bearer "+authJWT)
	req.Header.Set("apns-topic", d.bundleID)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("Content-Type", "application/json")

	res, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("apns send: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		resBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("apns send: status %d: %s", res.StatusCode, strings.TrimSpace(string(resBody)))
	}
	return nil
}

// providerToken returns a cached ES256 provider token, re-signing when the
// cached token is missing or near the one-hour Apple validity limit.
func (d *APNSDelivery) providerToken() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.providerJWT != "" && time.Now().Before(d.tokenExp) {
		return d.providerJWT, nil
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": d.teamID,
		"iat": now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = d.keyID
	signed, err := token.SignedString(d.privateKey)
	if err != nil {
		return "", err
	}
	d.providerJWT = signed
	d.tokenExp = now.Add(55 * time.Minute)
	return d.providerJWT, nil
}

// SendSMS is not supported by APNs provider.
func (d *APNSDelivery) SendSMS(context.Context, string, string) error {
	return notifications.ErrDeliveryNotConfigured{Channel: "sms"}
}

// Name returns the provider name.
func (d *APNSDelivery) Name() string {
	return "apns"
}

// IsEnabled returns whether the provider is properly configured.
func (d *APNSDelivery) IsEnabled() bool {
	return d.enabled
}
