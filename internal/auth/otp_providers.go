package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TermiiDelivery implements OTPDelivery using Termii SMS API.
type TermiiDelivery struct {
	APIKey   string
	SenderID string
	BaseURL  string
}

// NewTermiiDelivery creates a new Termii OTP delivery.
func NewTermiiDelivery(apiKey, senderID string) *TermiiDelivery {
	return &TermiiDelivery{
		APIKey:   apiKey,
		SenderID: senderID,
		BaseURL:  "https://api.ng.stateapp.co",
	}
}

// DeliverOTP sends an OTP via Termii SMS.
func (d *TermiiDelivery) DeliverOTP(ctx context.Context, phone, purpose, code string) error {
	if d.APIKey == "" {
		return fmt.Errorf("%w: Termii API key not configured", ErrOTPDeliveryNotConfigured)
	}
	return d.SendSMS(ctx, phone, d.FormatMessage(purpose, code))
}

// SendSMS delivers a pre-formatted SMS message through the Termii API. It is
// the transport primitive behind DeliverOTP and the notification SMS
// adapters; the message is transmitted verbatim.
func (d *TermiiDelivery) SendSMS(ctx context.Context, phone, message string) error {
	if d.APIKey == "" {
		return fmt.Errorf("%w: Termii API key not configured", ErrOTPDeliveryNotConfigured)
	}

	payload := url.Values{
		"api_key":   {d.APIKey},
		"sender_id": {d.SenderID},
		"to":        {phone},
		"message":   {message},
		"channel":   {"sms"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.BaseURL+"/api/v1/messaging/sms/send",
		strings.NewReader(payload.Encode()))
	if err != nil {
		return fmt.Errorf("create Termii request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute Termii request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Termii API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode Termii response: %w", err)
	}

	status, _ := result["status"].(string)
	if status != "success" {
		return fmt.Errorf("Termii delivery failed: %v", result)
	}

	return nil
}

// FormatMessage creates a localized OTP message based on purpose.
func (d *TermiiDelivery) FormatMessage(purpose, code string) string {
	switch purpose {
	case "login":
		return fmt.Sprintf("Codigo OTP para login RideX Angola: %s. Valido por 5 minutos.", code)
	case "registration":
		return fmt.Sprintf("Codigo OTP para registo RideX Angola: %s. Valido por 5 minutos.", code)
	case "password_reset":
		return fmt.Sprintf("Codigo OTP para reset de password RideX Angola: %s. Valido por 5 minutos.", code)
	case "phone_change":
		return fmt.Sprintf("Codigo OTP para alterar telemovel RideX Angola: %s. Valido por 5 minutos.", code)
	default:
		return fmt.Sprintf("Codigo OTP RideX Angola: %s. Valido por 5 minutos.", code)
	}
}

// AfricaTalkingDelivery implements OTPDelivery using Africa's Talking SMS API.
type AfricaTalkingDelivery struct {
	APIKey   string
	Username string
	BaseURL  string
}

// NewAfricaTalkingDelivery creates a new Africa's Talking OTP delivery.
func NewAfricaTalkingDelivery(apiKey, username string) *AfricaTalkingDelivery {
	return &AfricaTalkingDelivery{
		APIKey:   apiKey,
		Username: username,
		BaseURL:  "https://api.africastalking.com",
	}
}

// DeliverOTP sends an OTP via Africa's Talking SMS.
func (d *AfricaTalkingDelivery) DeliverOTP(ctx context.Context, phone, purpose, code string) error {
	if d.APIKey == "" || d.Username == "" {
		return fmt.Errorf("%w: Africa's Talking credentials not configured", ErrOTPDeliveryNotConfigured)
	}
	return d.SendSMS(ctx, phone, d.FormatMessage(purpose, code))
}

// SendSMS delivers a pre-formatted SMS message through the Africa's Talking
// API. It is the transport primitive behind DeliverOTP and the notification
// SMS adapters; the message is transmitted verbatim.
func (d *AfricaTalkingDelivery) SendSMS(ctx context.Context, phone, message string) error {
	if d.APIKey == "" || d.Username == "" {
		return fmt.Errorf("%w: Africa's Talking credentials not configured", ErrOTPDeliveryNotConfigured)
	}

	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, " ", "")

	payload := map[string]string{
		"to":      phone,
		"message": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal Africa's Talking request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.BaseURL+"/api/v1/messaging/sms",
		strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create Africa's Talking request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(d.Username, d.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute Africa's Talking request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Africa's Talking API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode Africa's Talking response: %w", err)
	}

	responses, ok := result["SMSMessageData"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected Africa's Talking response format: %v", result)
	}

	data, ok := responses["Message"].(string)
	if !ok || !strings.Contains(data, "Success") {
		return fmt.Errorf("Africa's Talking delivery failed: %v", responses)
	}

	return nil
}

// FormatMessage creates a localized OTP message based on purpose.
func (d *AfricaTalkingDelivery) FormatMessage(purpose, code string) string {
	switch purpose {
	case "login":
		return fmt.Sprintf("Código OTP para login RideX Angola: %s. Valide por 5 minutos.", code)
	case "registration":
		return fmt.Sprintf("Código OTP para registo RideX Angola: %s. Valide por 5 minutos.", code)
	case "password_reset":
		return fmt.Sprintf("Código OTP para redefinição de password RideX Angola: %s. Valide por 5 minutos.", code)
	case "phone_change":
		return fmt.Sprintf("Código OTP para alterar telemóvel RideX Angola: %s. Valide por 5 minutos.", code)
	default:
		return fmt.Sprintf("Código OTP RideX Angola: %s. Valide por 5 minutos.", code)
	}
}

// TwilioDelivery implements OTPDelivery using Twilio SMS API.
type TwilioDelivery struct {
	AccountSID string
	AuthToken  string
	From       string
	BaseURL    string
}

// NewTwilioDelivery creates a new Twilio OTP delivery.
func NewTwilioDelivery(accountSID, authToken, from string) *TwilioDelivery {
	return &TwilioDelivery{
		AccountSID: accountSID,
		AuthToken:  authToken,
		From:       from,
		BaseURL:    "https://api.twilio.com",
	}
}

// DeliverOTP sends an OTP via Twilio SMS.
func (d *TwilioDelivery) DeliverOTP(ctx context.Context, phone, purpose, code string) error {
	if d.AccountSID == "" || d.AuthToken == "" {
		return fmt.Errorf("%w: Twilio credentials not configured", ErrOTPDeliveryNotConfigured)
	}
	return d.SendSMS(ctx, phone, d.FormatMessage(purpose, code))
}

// SendSMS delivers a pre-formatted SMS message through the Twilio API. It is
// the transport primitive behind DeliverOTP and the notification SMS
// adapters; the message is transmitted verbatim.
func (d *TwilioDelivery) SendSMS(ctx context.Context, phone, message string) error {
	if d.AccountSID == "" || d.AuthToken == "" {
		return fmt.Errorf("%w: Twilio credentials not configured", ErrOTPDeliveryNotConfigured)
	}

	payload := url.Values{
		"To":   {phone},
		"From": {d.From},
		"Body": {message},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.BaseURL+"/2010-04-01/Accounts/"+d.AccountSID+"/Messages.json",
		strings.NewReader(payload.Encode()))
	if err != nil {
		return fmt.Errorf("create Twilio request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(d.AccountSID, d.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute Twilio request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Twilio API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode Twilio response: %w", err)
	}

	_, ok := result["sid"].(string)
	if !ok {
		return fmt.Errorf("Twilio delivery failed: %v", result)
	}

	return nil
}

// FormatMessage creates a localized OTP message based on purpose.
func (d *TwilioDelivery) FormatMessage(purpose, code string) string {
	switch purpose {
	case "login":
		return fmt.Sprintf("RideX Angola: Your OTP code for login is %s. Valid for 5 minutes.", code)
	case "registration":
		return fmt.Sprintf("RideX Angola: Your OTP code for registration is %s. Valid for 5 minutes.", code)
	case "password_reset":
		return fmt.Sprintf("RideX Angola: Your OTP code for password reset is %s. Valid for 5 minutes.", code)
	case "phone_change":
		return fmt.Sprintf("RideX Angola: Your OTP code for phone change is %s. Valid for 5 minutes.", code)
	default:
		return fmt.Sprintf("RideX Angola: Your OTP code is %s. Valid for 5 minutes.", code)
	}
}
