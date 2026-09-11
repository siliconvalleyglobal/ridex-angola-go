# Payment Provider Adapters Implementation

## Overview

This implementation adds real payment provider adapters for RideX Angola, completing a critical P0 gap from the production readiness roadmap. The implementation provides adapters for three major Angolan payment providers:

- **AppyPay** - Mobile payment platform
- **VPOS** - Virtual Point of Sale system
- **ProxyPay** - Payment gateway

## Files Created

### Core Provider Implementations

1. **`internal/payment/providers/appypay.go`**
   - AppyPay client with full API integration
   - HMAC-SHA256 webhook signature verification
   - Payment intent creation
   - Payment status queries
   - Refund support
   - GPO (Guaranteed Payment Order) support

2. **`internal/payment/providers/vpos.go`**
   - VPOS client with full API integration
   - Custom HMAC signature for request authentication
   - Webhook signature verification
   - Payment intent creation
   - Payment status queries
   - Refund support

3. **`internal/payment/providers/proxypay.go`**
   - ProxyPay client with full API integration
   - Bearer token authentication
   - Webhook signature verification
   - Payment intent creation
   - Payment status queries
   - Refund support

### Provider Infrastructure

4. **`internal/payment/providers/registry.go`**
   - Provider registry for managing multiple payment providers
   - Common Provider interface
   - Common Payment and WebhookEvent types

5. **`internal/payment/providers/factory.go`**
   - Factory for creating provider instances from config
   - Registry creation helper

### Tests

6. **`internal/payment/providers/appypay_test.go`**
   - CreatePaymentIntent test with mocked server
   - Webhook signature verification tests
   - Webhook event parsing tests
   - Auth header generation tests

7. **`internal/payment/providers/vpos_test.go`**
   - CreatePaymentIntent test with mocked server
   - Webhook signature verification tests
   - Webhook event parsing tests
   - Request signing tests

8. **`internal/payment/providers/proxypay_test.go`**
   - CreatePaymentIntent test with mocked server
   - Webhook signature verification tests
   - Webhook event parsing tests

## Key Features

### 1. Provider Interface
All providers implement a common interface:
```go
type Provider interface {
    CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*Payment, error)
    GetPaymentStatus(ctx context.Context, paymentID string) (*Payment, error)
    RefundPayment(ctx context.Context, paymentID string, amountCents int64, reason string) error
    VerifyWebhookSignature(payload []byte, signature string) error
    ParseWebhookEvent(payload []byte, signature string) (*WebhookEvent, error)
}
```

### 2. Webhook Security
All providers implement HMAC-SHA256 webhook signature verification:
- Support for both hex and base64 encoded signatures
- Support for `sha256=` prefix
- Constant-time comparison to prevent timing attacks
- Configurable webhook secrets

### 3. Idempotency Ready
The payment intent creation uses ride UUIDs as references, enabling idempotency key generation in the payment service layer.

### 4. Provider-Neutral Events
Webhook events are converted to a provider-neutral format:
- `payment.processing` - Payment created/initialized
- `payment.completed` - Payment successful
- `payment.failed` - Payment failed/cancelled
- `payment.refunded` - Payment refunded

## Integration Points

### Configuration
The providers use existing configuration from `config.Config`:
- `AppyPayClientID`, `AppyPayClientSecret`, `AppyPayBaseURL`, `AppyPayGPOEnabled`
- `VPOSDeveloperID`, `VPOSAPIKey`, `VPOSBaseURL`, `VPOSWebhookSecret`
- `ProxyPayAPIKey`, `ProxyPayBaseURL`

### Payment Service Integration
The providers are designed to integrate with the existing payment service:
- Use `uuid.UUID` for ride identification
- Return amounts in cents (int64)
- Support AOA currency
- Compatible with existing webhook verification framework

## Testing

All tests pass successfully:
```bash
$ go test ./internal/payment/providers/... -v
=== RUN   TestAppyPayClient_CreatePaymentIntent
--- PASS: TestAppyPayClient_CreatePaymentIntent (0.00s)
=== RUN   TestAppyPayClient_VerifyWebhookSignature
--- PASS: TestAppyPayClient_VerifyWebhookSignature (0.00s)
=== RUN   TestAppyPayClient_ParseWebhookEvent
--- PASS: TestAppyPayClient_ParseWebhookEvent (0.00s)
=== RUN   TestAppyPayClient_AuthHeader
--- PASS: TestAppyPayClient_AuthHeader (0.00s)
=== RUN   TestProxyPayClient_CreatePaymentIntent
--- PASS: TestProxyPayClient_CreatePaymentIntent (0.00s)
=== RUN   TestProxyPayClient_VerifyWebhookSignature
--- PASS: TestProxyPayClient_VerifyWebhookSignature (0.00s)
=== RUN   TestProxyPayClient_ParseWebhookEvent
--- PASS: TestProxyPayClient_ParseWebhookEvent (0.00s)
=== RUN   TestVPOSClient_CreatePaymentIntent
--- PASS: TestVPOSClient_CreatePaymentIntent (0.00s)
=== RUN   TestVPOSClient_VerifyWebhookSignature
--- PASS: TestVPOSClient_VerifyWebhookSignature (0.00s)
=== RUN   TestVPOSClient_ParseWebhookEvent
--- PASS: TestVPOSClient_ParseWebhookEvent (0.00s)
=== RUN   TestVPOSClient_SignRequest
--- PASS: TestVPOSClient_SignRequest (0.00s)
PASS
ok      github.com/ridex/ridex-angola/internal/payment/providers    0.455s
```

Full test suite passes:
```bash
$ go test ./...
ok      github.com/ridex/ridex-angola/internal/auth        0.300s
ok      github.com/ridex/ridex-angola/internal/config      0.010s
ok      github.com/ridex/ridex-angola/internal/dashboard   0.009s
ok      github.com/ridex/ridex-angola/internal/driver     0.398s
ok      github.com/ridex/ridex-angola/internal/kyc        0.301s
ok      github.com/ridex/ridex-angola/internal/metrics    0.010s
ok      github.com/ridex/ridex-angola/internal/middleware 0.009s
ok      github.com/ridex/ridex-angola/internal/migrations 0.014s
ok      github.com/ridex/ridex-angola/internal/notifications    0.315s
ok      github.com/ridex/ridex-angola/internal/payment    0.328s
ok      github.com/ridex/ridex-angola/internal/payment/providers    0.455s
ok      github.com/ridex/ridex-angola/internal/pricing    0.296s
ok      github.com/ridex/ridex-angola/internal/rides      0.317s
ok      github.com/ridex/ridex-angola/internal/safety     0.308s
ok      github.com/ridex/ridex-angola/internal/support    0.307s
ok      github.com/ridex/ridex-angola/internal/zones      0.397s
```

## Next Steps

To complete the payment integration:

1. **Wire providers in main.go** - Create provider instances and register them in the payment service
2. **Add payment intent creation to payment service** - Use the provider registry to create payment intents
3. **Add webhook handlers** - Create HTTP handlers for provider webhooks
4. **Add provider-specific webhook verification** - Use provider-specific signature verification
5. **Add payment status polling** - Implement background job to check payment status
6. **Add refund workflow** - Integrate refund functionality into admin operations
7. **Add provider selection logic** - Allow users to choose payment provider
8. **Add payment method management** - Store user payment preferences

## Security Considerations

- All webhook signatures are verified using HMAC-SHA256
- Constant-time comparison prevents timing attacks
- Secrets are validated in production (min 32 bytes)
- Request signatures use provider-specific algorithms
- No sensitive data logged

## Compliance

- Follows provider API conventions
- Supports Angola AOA currency
- Ready for SAFT invoice integration
- Provider-agnostic design allows easy addition of new providers
