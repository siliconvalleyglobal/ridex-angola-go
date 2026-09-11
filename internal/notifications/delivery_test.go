package notifications

import (
	"context"
	"errors"
	"testing"
)

func TestNoopDeliveryProvider(t *testing.T) {
	provider := NoopDeliveryProvider{}

	// Test SendPush
	err := provider.SendPush(context.Background(), "token123", PushNotification{
		Title: "Test",
		Body:  "Test body",
	})
	if !errors.Is(err, ErrDeliveryNotConfigured{Channel: "push"}) {
		var notConfigured ErrDeliveryNotConfigured
		if !errors.As(err, &notConfigured) || notConfigured.Channel != "push" {
			t.Errorf("SendPush() error = %v, want ErrDeliveryNotConfigured{push}", err)
		}
	}

	// Test SendSMS
	err = provider.SendSMS(context.Background(), "922123456", "Test message")
	if !errors.Is(err, ErrDeliveryNotConfigured{Channel: "sms"}) {
		var notConfigured ErrDeliveryNotConfigured
		if !errors.As(err, &notConfigured) || notConfigured.Channel != "sms" {
			t.Errorf("SendSMS() error = %v, want ErrDeliveryNotConfigured{sms}", err)
		}
	}

	// Test Name
	if provider.Name() != "noop" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "noop")
	}
}

func TestDeliveryService_Notify_PushSuccess(t *testing.T) {
	mockProvider := &mockDeliveryProvider{
		pushResult: nil, // success
	}
	service := NewDeliveryService(mockProvider)

	result, err := service.Notify(context.Background(), DeliveryInput{
		DeviceToken: "token123",
		Title:       "Ride Update",
		Body:        "Your driver is arriving",
	})

	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if !result.PushDelivered {
		t.Error("Notify() PushDelivered should be true")
	}
	if result.SMSDelivered {
		t.Error("Notify() SMSDelivered should be false")
	}
	if result.Provider != "mock" {
		t.Errorf("Notify() Provider = %q, want %q", result.Provider, "mock")
	}
}

func TestDeliveryService_Notify_SMSFallback(t *testing.T) {
	mockProvider := &mockDeliveryProvider{
		pushResult: ErrDeliveryNotConfigured{Channel: "push"},
		smsResult:  nil, // success
	}
	service := NewDeliveryService(mockProvider)

	result, err := service.Notify(context.Background(), DeliveryInput{
		DeviceToken: "token123",
		Phone:       "922123456",
		Title:       "Ride Update",
		Body:        "Your driver is arriving",
	})

	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if result.PushDelivered {
		t.Error("Notify() PushDelivered should be false")
	}
	if !result.SMSDelivered {
		t.Error("Notify() SMSDelivered should be true")
	}
}

func TestDeliveryService_Notify_NoDeviceToken(t *testing.T) {
	mockProvider := &mockDeliveryProvider{
		smsResult: nil, // success
	}
	service := NewDeliveryService(mockProvider)

	result, err := service.Notify(context.Background(), DeliveryInput{
		Phone: "922123456",
		Title: "Ride Update",
		Body:  "Your driver is arriving",
	})

	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if result.PushDelivered {
		t.Error("Notify() PushDelivered should be false")
	}
	if !result.SMSDelivered {
		t.Error("Notify() SMSDelivered should be true")
	}
}

func TestDeliveryService_Notify_NotConfigured(t *testing.T) {
	mockProvider := &mockDeliveryProvider{
		pushResult: ErrDeliveryNotConfigured{Channel: "push"},
		smsResult:  ErrDeliveryNotConfigured{Channel: "sms"},
	}
	service := NewDeliveryService(mockProvider)

	_, err := service.Notify(context.Background(), DeliveryInput{
		DeviceToken: "token123",
		Phone:       "922123456",
		Title:       "Ride Update",
		Body:        "Your driver is arriving",
	})

	if err == nil {
		t.Fatal("Notify() should return error when not configured")
	}
}

func TestDeliveryService_NilProvider(t *testing.T) {
	service := NewDeliveryService(nil)

	err := service.provider.SendPush(context.Background(), "token", PushNotification{})
	if err == nil {
		t.Error("SendPush() should return error with nil provider")
	}
}

// mockDeliveryProvider is a test mock for DeliveryProvider.
type mockDeliveryProvider struct {
	pushResult error
	smsResult  error
}

func (m *mockDeliveryProvider) SendPush(context.Context, string, PushNotification) error {
	return m.pushResult
}

func (m *mockDeliveryProvider) SendSMS(context.Context, string, string) error {
	return m.smsResult
}

func (m *mockDeliveryProvider) Name() string {
	return "mock"
}
