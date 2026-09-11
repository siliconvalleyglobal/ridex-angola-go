package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTermiiDelivery_DeliverOTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}
		if !strings.Contains(r.URL.Path, "/api/v1/messaging/sms/send") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}

		// Verify form values
		if r.FormValue("api_key") != "test_api_key" {
			t.Errorf("expected api_key test_api_key, got %s", r.FormValue("api_key"))
			return
		}
		if r.FormValue("sender_id") != "RideXAO" {
			t.Errorf("expected sender_id RideXAO, got %s", r.FormValue("sender_id"))
			return
		}
		if r.FormValue("to") != "+244912123456" {
			t.Errorf("expected to +244912123456, got %s", r.FormValue("to"))
			return
		}
		if r.FormValue("channel") != "sms" {
			t.Errorf("expected channel sms, got %s", r.FormValue("channel"))
			return
		}

		response := map[string]interface{}{
			"status":  "success",
			"message": "SMS sent successfully",
			"data": map[string]string{
				"smsId": "sms_test_123",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create delivery and set BaseURL via reflection or direct assignment
	delivery := &TermiiDelivery{
		APIKey:   "test_api_key",
		SenderID: "RideXAO",
		BaseURL:  server.URL,
	}

	err := delivery.DeliverOTP(context.Background(), "+244912123456", "login", "123456")
	if err != nil {
		t.Fatalf("DeliverOTP failed: %v", err)
	}
}

func TestTermiiDelivery_DeliverOTP_MissingAPIKey(t *testing.T) {
	delivery := &TermiiDelivery{
		APIKey:   "",
		SenderID: "RideXAO",
		BaseURL:  "https://api.ng.stateapp.co",
	}

	err := delivery.DeliverOTP(context.Background(), "+244912123456", "login", "123456")
	if err == nil {
		t.Error("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestAfricaTalkingDelivery_DeliverOTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}
		if !strings.Contains(r.URL.Path, "/api/v1/messaging/sms") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok || username != "test_username" {
			t.Errorf("expected BasicAuth username test_username, got %s", username)
			return
		}
		if password != "test_api_key" {
			t.Errorf("expected BasicAuth password test_api_key, got %s", password)
			return
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if payload["to"] != "244912123456" {
			t.Errorf("expected to 244912123456, got %s", payload["to"])
			return
		}

		response := map[string]interface{}{
			"SMSMessageData": map[string]interface{}{
				"Message": "Success.",
				"Recipients": []map[string]interface{}{
					{
						"number":    "244912123456",
						"status":    "Success",
						"cost":      "N5.00",
						"messageId": "AT-CT-12345678901234567890",
						"pending":   false,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	delivery := &AfricaTalkingDelivery{
		APIKey:   "test_api_key",
		Username: "test_username",
		BaseURL:  server.URL,
	}

	err := delivery.DeliverOTP(context.Background(), "+244912123456", "login", "123456")
	if err != nil {
		t.Fatalf("DeliverOTP failed: %v", err)
	}
}

func TestAfricaTalkingDelivery_DeliverOTP_MissingCredentials(t *testing.T) {
	delivery := &AfricaTalkingDelivery{
		APIKey:   "",
		Username: "",
		BaseURL:  "https://api.africastalking.com",
	}

	err := delivery.DeliverOTP(context.Background(), "+244912123456", "login", "123456")
	if err == nil {
		t.Error("expected error for missing credentials")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestTwilioDelivery_DeliverOTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			return
		}
		if !strings.Contains(r.URL.Path, "/Messages.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}

		// Verify basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "ACtest_sid" {
			t.Errorf("expected BasicAuth username ACtest_sid, got %s", username)
			return
		}
		if password != "test_auth_token" {
			t.Errorf("expected BasicAuth password test_auth_token, got %s", password)
			return
		}

		// Verify form values
		if r.FormValue("To") != "+244912123456" {
			t.Errorf("expected To +244912123456, got %s", r.FormValue("To"))
			return
		}
		if r.FormValue("From") != "+24499999999" {
			t.Errorf("expected From +24499999999, got %s", r.FormValue("From"))
			return
		}

		response := map[string]interface{}{
			"sid":          "SMtest_sid",
			"date_created": time.Now().Format(time.RFC3339),
			"status":       "queued",
			"to":           "+244912123456",
			"from":         "+24499999999",
			"body":         "RideX Angola: Your OTP code for login is 123456. Valid for 5 minutes.",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	delivery := &TwilioDelivery{
		AccountSID: "ACtest_sid",
		AuthToken:  "test_auth_token",
		From:       "+24499999999",
		BaseURL:    server.URL,
	}

	err := delivery.DeliverOTP(context.Background(), "+244912123456", "login", "123456")
	if err != nil {
		t.Fatalf("DeliverOTP failed: %v", err)
	}
}

func TestTwilioDelivery_DeliverOTP_MissingCredentials(t *testing.T) {
	delivery := &TwilioDelivery{
		AccountSID: "",
		AuthToken:  "",
		From:       "",
		BaseURL:    "https://api.twilio.com",
	}

	err := delivery.DeliverOTP(context.Background(), "+244912123456", "login", "123456")
	if err == nil {
		t.Error("expected error for missing credentials")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestTermiiDelivery_FormatMessage(t *testing.T) {
	delivery := &TermiiDelivery{
		APIKey:   "test_key",
		SenderID: "RideXAO",
	}

	tests := []struct {
		purpose string
		code    string
		expect  string
	}{
		{"login", "123456", "Codigo OTP para login RideX Angola: 123456. Valido por 5 minutos."},
		{"registration", "123456", "Codigo OTP para registo RideX Angola: 123456. Valido por 5 minutos."},
		{"password_reset", "123456", "Codigo OTP para reset de password RideX Angola: 123456. Valido por 5 minutos."},
		{"phone_change", "123456", "Codigo OTP para alterar telemovel RideX Angola: 123456. Valido por 5 minutos."},
		{"unknown", "123456", "Codigo OTP RideX Angola: 123456. Valido por 5 minutos."},
	}

	for _, tt := range tests {
		result := delivery.FormatMessage(tt.purpose, tt.code)
		if result != tt.expect {
			t.Errorf("FormatMessage(%s, %s) = %q, want %q", tt.purpose, tt.code, result, tt.expect)
		}
	}
}

func TestAfricaTalkingDelivery_FormatMessage(t *testing.T) {
	delivery := &AfricaTalkingDelivery{
		APIKey:   "test_key",
		Username: "test_user",
	}

	tests := []struct {
		purpose string
		code    string
		expect  string
	}{
		{"login", "123456", "Código OTP para login RideX Angola: 123456. Valide por 5 minutos."},
		{"registration", "123456", "Código OTP para registo RideX Angola: 123456. Valide por 5 minutos."},
		{"password_reset", "123456", "Código OTP para redefinição de password RideX Angola: 123456. Valide por 5 minutos."},
		{"phone_change", "123456", "Código OTP para alterar telemóvel RideX Angola: 123456. Valide por 5 minutos."},
	}

	for _, tt := range tests {
		result := delivery.FormatMessage(tt.purpose, tt.code)
		if result != tt.expect {
			t.Errorf("FormatMessage(%s, %s) = %q, want %q", tt.purpose, tt.code, result, tt.expect)
		}
	}
}

func TestTwilioDelivery_FormatMessage(t *testing.T) {
	delivery := &TwilioDelivery{
		AccountSID: "ACtest_sid",
		AuthToken:  "test_token",
		From:       "+24499999999",
	}

	tests := []struct {
		purpose string
		code    string
		expect  string
	}{
		{"login", "123456", "RideX Angola: Your OTP code for login is 123456. Valid for 5 minutes."},
		{"registration", "123456", "RideX Angola: Your OTP code for registration is 123456. Valid for 5 minutes."},
		{"password_reset", "123456", "RideX Angola: Your OTP code for password reset is 123456. Valid for 5 minutes."},
		{"phone_change", "123456", "RideX Angola: Your OTP code for phone change is 123456. Valid for 5 minutes."},
	}

	for _, tt := range tests {
		result := delivery.FormatMessage(tt.purpose, tt.code)
		if result != tt.expect {
			t.Errorf("FormatMessage(%s, %s) = %q, want %q", tt.purpose, tt.code, result, tt.expect)
		}
	}
}
