package providers

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/ridex/ridex-angola/internal/notifications"
)

// fakeSMSSender implements both auth.OTPDelivery (DeliverOTP) and the raw
// SMSSender boundary, mirroring the real Termii/AT/Twilio providers.
type fakeSMSSender struct {
	otpPurpose string
	otpCode    string
	smsPhone   string
	smsMessage string
	smsCalls   int
	err        error
}

func (f *fakeSMSSender) DeliverOTP(_ context.Context, _, purpose, code string) error {
	f.otpPurpose = purpose
	f.otpCode = code
	return nil
}

func (f *fakeSMSSender) SendSMS(_ context.Context, phone, message string) error {
	f.smsCalls++
	f.smsPhone = phone
	f.smsMessage = message
	return f.err
}

// plainOTP implements only auth.OTPDelivery, mirroring NoopOTPDelivery.
type plainOTP struct {
	purpose string
	code    string
}

func (p *plainOTP) DeliverOTP(_ context.Context, _, purpose, code string) error {
	p.purpose = purpose
	p.code = code
	return nil
}

func TestSMSDeliverySendsMessageVerbatim(t *testing.T) {
	sender := &fakeSMSSender{}
	d := NewSMSDelivery(sender, "termii")

	if err := d.SendSMS(context.Background(), "+244912123456", "Sua corrida chegou ao local"); err != nil {
		t.Fatalf("SendSMS: %v", err)
	}
	if sender.smsCalls != 1 || sender.smsPhone != "+244912123456" || sender.smsMessage != "Sua corrida chegou ao local" {
		t.Fatalf("sender captured = calls %d phone %q message %q", sender.smsCalls, sender.smsPhone, sender.smsMessage)
	}
	if d.Name() != "sms_termii" {
		t.Fatalf("name = %q, want sms_termii", d.Name())
	}
}

func TestSMSDeliveryPushNotConfigured(t *testing.T) {
	d := NewSMSDelivery(&fakeSMSSender{}, "termii")
	err := d.SendPush(context.Background(), "tok", notifications.PushNotification{Title: "t", Body: "b"})
	if !errors.Is(err, notifications.ErrDeliveryNotConfigured{Channel: "push"}) {
		t.Fatalf("err = %v, want ErrDeliveryNotConfigured", err)
	}
}

func TestSMSDeliveryNilSenderNotConfigured(t *testing.T) {
	d := NewSMSDelivery(nil, "termii")
	if err := d.SendSMS(context.Background(), "+244912123456", "hello"); !errors.Is(err, notifications.ErrDeliveryNotConfigured{Channel: "sms"}) {
		t.Fatalf("err = %v, want ErrDeliveryNotConfigured", err)
	}
}

func TestSMSDeliverySurfacesTransportError(t *testing.T) {
	sender := &fakeSMSSender{err: errors.New("provider timeout")}
	d := NewSMSDelivery(sender, "twilio")
	if err := d.SendSMS(context.Background(), "+244912123456", "hello"); err == nil || err.Error() != "provider timeout" {
		t.Fatalf("err = %v, want provider timeout", err)
	}
}

func TestFactorySMSPrefersDirectAdapter(t *testing.T) {
	f := NewDeliveryFactory(zap.NewNop())
	p := f.Create("sms", "", "", "", "", "", "", false, &fakeSMSSender{}, "termii")
	sms, ok := p.(*SMSDelivery)
	if !ok {
		t.Fatalf("Create(sms) = %T, want *SMSDelivery", p)
	}
	if sms.Name() != "sms_termii" {
		t.Fatalf("name = %q, want sms_termii", sms.Name())
	}
}

func TestFactorySMSFallsBackForPlainOTP(t *testing.T) {
	f := NewDeliveryFactory(zap.NewNop())
	p := f.Create("sms", "", "", "", "", "", "", false, &plainOTP{}, "termii")
	if _, ok := p.(*SMSFallbackDelivery); !ok {
		t.Fatalf("Create(sms) = %T, want *SMSFallbackDelivery", p)
	}
}

func TestFactoryUnknownProviderIsNoop(t *testing.T) {
	f := NewDeliveryFactory(zap.NewNop())
	p := f.Create("carrier-pigeon", "", "", "", "", "", "", false, &plainOTP{}, "")
	if _, ok := p.(notifications.NoopDeliveryProvider); !ok {
		t.Fatalf("Create(unknown) = %T, want NoopDeliveryProvider", p)
	}
}
