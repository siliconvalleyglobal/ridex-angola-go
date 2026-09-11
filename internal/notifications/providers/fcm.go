package providers

import (
	"bytes"
	"context"
	"crypto/rsa"
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

const (
	fcmScope       = "https://www.googleapis.com/auth/firebase.messaging"
	fcmTokenLeeway = 5 * time.Minute
)

// Vars so tests can point them at a stub server.
var (
	fcmTokenURL = "https://oauth2.googleapis.com/token"
	fcmEndpoint = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
)

// serviceAccount is the subset of a Firebase service-account JSON we need.
type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	ProjectID   string `json:"project_id"`
}

// FCMDelivery implements DeliveryProvider using Firebase Cloud Messaging
// HTTP v1. Authentication uses a service-account JWT exchanged for a short
// lived OAuth2 access token; the token is cached until shortly before expiry.
type FCMDelivery struct {
	projectID   string
	enabled     bool
	clientEmail string
	privateKey  *rsa.PrivateKey

	mu          sync.Mutex
	accessToken string
	tokenExp    time.Time
	client      *http.Client
}

// NewFCMDelivery creates an FCM HTTP v1 delivery provider. credentials may be
// a path to a service-account JSON file or the JSON itself. It returns a
// disabled provider when no usable credentials are configured, and never
// fails hard so startup does not depend on push availability.
func NewFCMDelivery(projectID, credentials string) *FCMDelivery {
	if credentials == "" || projectID == "" {
		return &FCMDelivery{enabled: false}
	}
	raw := credentials
	if data, err := os.ReadFile(credentials); err == nil {
		raw = string(data)
	}
	var sa serviceAccount
	if err := json.Unmarshal([]byte(raw), &sa); err != nil {
		return &FCMDelivery{enabled: false}
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return &FCMDelivery{enabled: false}
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(sa.PrivateKey))
	if err != nil {
		return &FCMDelivery{enabled: false}
	}
	if sa.ProjectID != "" {
		projectID = sa.ProjectID
	}
	if projectID == "" {
		return &FCMDelivery{enabled: false}
	}
	return &FCMDelivery{
		projectID:   projectID,
		enabled:     true,
		clientEmail: sa.ClientEmail,
		privateKey:  key,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

// SendPush sends a push notification via FCM HTTP v1.
func (d *FCMDelivery) SendPush(ctx context.Context, deviceToken string, notification notifications.PushNotification) error {
	if !d.enabled {
		return notifications.ErrDeliveryNotConfigured{Channel: "push"}
	}
	if deviceToken == "" {
		return fmt.Errorf("device token is required")
	}

	token, err := d.oauthToken(ctx)
	if err != nil {
		return fmt.Errorf("fcm auth: %w", err)
	}

	data := make(map[string]string, len(notification.Data))
	for k, v := range notification.Data {
		data[k] = v
	}
	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"token": deviceToken,
			"notification": map[string]string{
				"title": notification.Title,
				"body":  notification.Body,
			},
			"data": data,
			"android": map[string]any{
				"priority": strings.ToUpper(priorityOrDefault(notification.Priority)),
			},
			"apns": map[string]any{
				"headers": map[string]string{"apns-priority": "10"},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("fcm encode: %w", err)
	}

	url := fmt.Sprintf(fcmEndpoint, d.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("fcm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	res, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		resBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("fcm send: status %d: %s", res.StatusCode, strings.TrimSpace(string(resBody)))
	}
	return nil
}

// oauthToken returns a cached access token, refreshing it through the
// service-account JWT grant when it is missing or near expiry.
func (d *FCMDelivery) oauthToken(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.accessToken != "" && time.Now().Before(d.tokenExp) {
		return d.accessToken, nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   d.clientEmail,
		"sub":   d.clientEmail,
		"aud":   fcmTokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"scope": fcmScope,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(d.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign assertion: %w", err)
	}

	form := strings.NewReader("grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=" + signed)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fcmTokenURL, form)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		resBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("token exchange: status %d: %s", res.StatusCode, strings.TrimSpace(string(resBody)))
	}

	var granted struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(res.Body).Decode(&granted); err != nil {
		return "", err
	}
	if granted.AccessToken == "" {
		return "", fmt.Errorf("token exchange returned no access token")
	}
	d.accessToken = granted.AccessToken
	d.tokenExp = now.Add(time.Duration(granted.ExpiresIn)*time.Second - fcmTokenLeeway)
	if d.tokenExp.Before(now) {
		d.tokenExp = now.Add(time.Minute)
	}
	return d.accessToken, nil
}

func priorityOrDefault(p string) string {
	if p == "high" {
		return "high"
	}
	return "normal"
}

// SendSMS is not supported by FCM provider.
func (d *FCMDelivery) SendSMS(context.Context, string, string) error {
	return notifications.ErrDeliveryNotConfigured{Channel: "sms"}
}

// Name returns the provider name.
func (d *FCMDelivery) Name() string {
	return "fcm"
}

// IsEnabled returns whether the provider is properly configured.
func (d *FCMDelivery) IsEnabled() bool {
	return d.enabled
}
