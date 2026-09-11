package auth

import (
	"context"
	"testing"
)

// mockOTPDelivery is a test mock that captures the delivered message.
type mockOTPDelivery struct {
	deliveredPhone   string
	deliveredPurpose string
	deliveredMessage string
}

func (m *mockOTPDelivery) DeliverOTP(ctx context.Context, phone, purpose, message string) error {
	m.deliveredPhone = phone
	m.deliveredPurpose = purpose
	m.deliveredMessage = message
	return nil
}

func TestLocalizedOTPDelivery_Login(t *testing.T) {
	mock := &mockOTPDelivery{}
	delivery := NewLocalizedOTPDelivery(mock)

	err := delivery.DeliverOTP(context.Background(), "922123456", "login", "123456")
	if err != nil {
		t.Fatalf("DeliverOTP() error = %v", err)
	}

	expected := "Código OTP para login RideX Angola: 123456. Valido por 5 minutos."
	if mock.deliveredMessage != expected {
		t.Errorf("DeliverOTP() message = %q, want %q", mock.deliveredMessage, expected)
	}
}

func TestLocalizedOTPDelivery_Register(t *testing.T) {
	mock := &mockOTPDelivery{}
	delivery := NewLocalizedOTPDelivery(mock)

	err := delivery.DeliverOTP(context.Background(), "922123456", "registration", "654321")
	if err != nil {
		t.Fatalf("DeliverOTP() error = %v", err)
	}

	expected := "Código OTP para registo RideX Angola: 654321. Valido por 5 minutos."
	if mock.deliveredMessage != expected {
		t.Errorf("DeliverOTP() message = %q, want %q", mock.deliveredMessage, expected)
	}
}

func TestLocalizedOTPDelivery_PasswordReset(t *testing.T) {
	mock := &mockOTPDelivery{}
	delivery := NewLocalizedOTPDelivery(mock)

	err := delivery.DeliverOTP(context.Background(), "922123456", "password_reset", "987654")
	if err != nil {
		t.Fatalf("DeliverOTP() error = %v", err)
	}

	expected := "Código OTP para redefinição de password RideX Angola: 987654. Valido por 5 minutos."
	if mock.deliveredMessage != expected {
		t.Errorf("DeliverOTP() message = %q, want %q", mock.deliveredMessage, expected)
	}
}

func TestLocalizedOTPDelivery_PhoneChange(t *testing.T) {
	mock := &mockOTPDelivery{}
	delivery := NewLocalizedOTPDelivery(mock)

	err := delivery.DeliverOTP(context.Background(), "922123456", "phone_change", "456789")
	if err != nil {
		t.Fatalf("DeliverOTP() error = %v", err)
	}

	expected := "Código OTP para alterar telemóvel RideX Angola: 456789. Valido por 5 minutos."
	if mock.deliveredMessage != expected {
		t.Errorf("DeliverOTP() message = %q, want %q", mock.deliveredMessage, expected)
	}
}

func TestLocalizedOTPDelivery_UnknownPurpose(t *testing.T) {
	mock := &mockOTPDelivery{}
	delivery := NewLocalizedOTPDelivery(mock)

	err := delivery.DeliverOTP(context.Background(), "922123456", "unknown", "111222")
	if err != nil {
		t.Fatalf("DeliverOTP() error = %v", err)
	}

	expected := "O seu código de verificação é: 111222"
	if mock.deliveredMessage != expected {
		t.Errorf("DeliverOTP() message = %q, want %q", mock.deliveredMessage, expected)
	}
}

// smsCapturingOTP implements both DeliverOTP and SendSMS, mirroring the real
// Termii/Africa's Talking/Twilio providers.
type smsCapturingOTP struct {
	mockOTPDelivery
	sentPhone   string
	sentMessage string
}

func (s *smsCapturingOTP) SendSMS(_ context.Context, phone, message string) error {
	s.sentPhone = phone
	s.sentMessage = message
	return nil
}

func TestLocalizedOTPDelivery_UsesRawSMSWithoutOTPWrap(t *testing.T) {
	inner := &smsCapturingOTP{}
	delivery := NewLocalizedOTPDelivery(inner)

	if err := delivery.DeliverOTP(context.Background(), "922123456", "login", "123456"); err != nil {
		t.Fatalf("DeliverOTP() error = %v", err)
	}
	expected := "Código OTP para login RideX Angola: 123456. Valido por 5 minutos."
	if inner.sentMessage != expected {
		t.Fatalf("sent message = %q, want %q (verbatim, no re-framing)", inner.sentMessage, expected)
	}
	if inner.deliveredPurpose != "" || inner.deliveredMessage != "" {
		t.Fatalf("inner DeliverOTP called with purpose %q message %q; want raw SendSMS path", inner.deliveredPurpose, inner.deliveredMessage)
	}
}
