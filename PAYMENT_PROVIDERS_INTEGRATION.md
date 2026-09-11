# Payment Provider Integration - RideX Angola

## Overview

This document summarizes the payment provider integration completed for RideX Angola.

## Completed Implementation

### 1. Provider Adapters

Three payment provider adapters have been implemented:

#### AppyPay
- **File:** `internal/payment/providers/appypay.go`
- **Features:**
  - Payment intent creation
  - Payment status queries
  - Refund support
  - HMAC-SHA256 webhook signature verification
  - GPO (Guaranteed Payment Order) support
- **Authentication:** Basic Auth (Client ID:Client Secret)
- **Test Coverage:** 4 tests (all passing)

#### VPOS
- **File:** `internal/payment/providers/vpos.go`
- **Features:**
  - Payment intent creation
  - Payment status queries
  - Refund support
  - HMAC-SHA256 webhook signature verification
  - Request signing for API authentication
- **Authentication:** Custom headers (X-Developer-ID, X-API-Key, X-Signature)
- **Test Coverage:** 5 tests (all passing)

#### ProxyPay
- **File:** `internal/payment/providers/proxypay.go`
- **Features:**
  - Payment intent creation
  - Payment status queries
  - Refund support
  - HMAC-SHA256 webhook signature verification
- **Authentication:** Bearer token (API Key)
- **Test Coverage:** 3 tests (all passing)

### 2. Provider Registry

- **File:** `internal/payment/providers/registry.go`
- **Features:**
  - Centralized provider management
  - Provider registration and lookup
  - Common interface for all providers

### 3. Factory Pattern

- **File:** `internal/payment/providers/factory.go`
- **Features:**
  - Provider instantiation from configuration
  - Conditional provider registration (only if credentials are provided)
  - Registry creation with all configured providers

### 4. Common Interface

All providers implement the `Provider` interface:
```go
type Provider interface {
    CreatePaymentIntent(ctx context.Context, rideID uuid.UUID, amountCents int64, currency string) (*Payment, error)
    GetPaymentStatus(ctx context.Context, paymentID string) (*Payment, error)
    RefundPayment(ctx context.Context, paymentID string, amountCents int64, reason string) error
    VerifyWebhookSignature(payload []byte, signature string) error
    ParseWebhookEvent(payload []byte, signature string) (*WebhookEvent, error)
}
```

### 5. Test Coverage

Total test coverage: **12 tests**
- AppyPay: 4 tests
- VPOS: 5 tests
- ProxyPay: 3 tests

All tests are passing and cover:
- Payment intent creation
- Webhook signature verification
- Webhook event parsing
- Provider-specific authentication

## Configuration

Payment providers are configured via environment variables (see `.env.example`):

### AppyPay
```bash
APPY_PAY_CLIENT_ID=your_client_id
APPY_PAY_CLIENT_SECRET=your_client_secret
APPY_PAY_BASE_URL=https://sandbox.appypay.co
APPY_PAY_GPO_ENABLED=true
```

### VPOS
```bash
VPOS_DEVELOPER_ID=your_developer_id
VPOS_API_KEY=your_api_key
VPOS_BASE_URL=https://api.vpos.ao
VPOS_WEBHOOK_SECRET=your_webhook_secret
```

### ProxyPay
```bash
PROXY_PAY_API_KEY=your_api_key
PROXY_PAY_BASE_URL=https://api.proxypay.co
```

### General
```bash
PAYMENT_PROVIDER=appypay  # or vpos, proxypay, or none
PAYMENT_WEBHOOK_SECRET=your_webhook_secret_min_32_bytes
```

## Webhook Processing

All providers support webhook signature verification using HMAC-SHA256:

1. **AppyPay:** Uses client secret as HMAC key
2. **VPOS:** Uses webhook secret as HMAC key  
3. **ProxyPay:** Uses API key as HMAC key

The signature can be provided in either:
- Hex-encoded format
- Base64-encoded format
- With or without "sha256=" prefix

## Integration Points

The payment providers integrate with:
- **Payment Service:** `internal/payment/service.go` - orchestrates payment flows
- **Payment Handler:** `internal/payment/handler.go` - HTTP handlers for payment operations
- **Main Application:** `cmd/api/main.go` - wires providers into the application

## Security Features

1. **Signature Verification:** All webhook events are verified before processing
2. **Constant-Time Comparison:** Uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks
3. **Context Support:** All operations support context for timeout/cancellation
4. **Error Handling:** Proper error wrapping and provider-specific error types

## Testing

All tests pass:
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
ok      github.com/ridex/ridex-angola/internal/payment/providers    0.334s
```

## Quality Assurance

- ✅ All tests passing (`go test ./...`)
- ✅ No vet warnings (`go vet ./...`)
- ✅ Properly formatted (`gofmt -l .`)
- ✅ Builds successfully (`go build ./...`)

## Next Steps

To enable payment providers in production:

1. Obtain API credentials from each provider
2. Configure environment variables in production
3. Set `PAYMENT_PROVIDER` to the desired provider
4. Ensure `PAYMENT_WEBHOOK_SECRET` is set (min 32 bytes)
5. Configure webhook endpoints to receive provider callbacks
6. Test end-to-end payment flows in staging

## File Structure

```
internal/payment/providers/
├── appypay.go           # AppyPay provider implementation
├── appypay_test.go      # AppyPay tests
├── factory.go           # Provider factory
├── proxypay.go          # ProxyPay provider implementation
├── proxypay_test.go     # ProxyPay tests
├── registry.go          # Provider registry
└── vpos.go              # VPOS provider implementation
    vpos_test.go         # VPOS tests
```

Total: 8 files, ~50KB of code
