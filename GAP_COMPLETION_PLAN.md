# RideX Angola Gap Completion Plan

## Execution Strategy

This plan organizes all remaining gaps into implementation phases.
Each phase builds on the previous one and includes verification steps.

## Phase 1: Payment Provider Adapters (P0 - Launch Critical)

### 1.1 AppyPay Adapter
- [ ] Create AppyPay client implementation
- [ ] Implement payment intent creation
- [ ] Implement payment status query
- [ ] Implement refund support
- [ ] Add webhook signature verification (AppyPay specific)
- [ ] Add configuration in config package
- [ ] Wire in cmd/api/main.go

### 1.2 VPOS Adapter
- [ ] Create VPOS client implementation
- [ ] Implement payment intent creation
- [ ] Implement payment status query
- [ ] Implement refund support
- [ ] Add webhook signature verification (VPOS specific)
- [ ] Add configuration in config package
- [ ] Wire in cmd/api/main.go

### 1.3 ProxyPay Adapter
- [ ] Create ProxyPay client implementation
- [ ] Implement payment intent creation
- [ ] Add configuration in config package
- [ ] Wire in cmd/api/main.go

### 1.4 Payment Ledger Enhancements
- [ ] Add payment attempt tracking
- [ ] Add provider reference storage
- [ ] Add webhook event ledger
- [ ] Add refund records
- [ ] Add settlement records
- [ ] Add reconciliation status

## Phase 2: SMS/OTP Provider Integration (P0 - Launch Critical)

### 2.1 SMS Provider Interface
- [ ] Define SMS provider interface
- [ ] Create provider-agnostic SMS service

### 2.2 Provider Implementations
- [ ] Implement Twilio adapter (example)
- [ ] Implement Africa's Talking adapter (Angola-relevant)
- [ ] Implement generic HTTP SMS adapter

### 2.3 OTP Delivery Integration
- [ ] Wire SMS provider to OTPDelivery interface
- [ ] Add configuration for SMS provider
- [ ] Add SMS templates
- [ ] Add attempt limits and resend cooldowns
- [ ] Add delivery status tracking

## Phase 3: Authentication Hardening (P0 - Launch Critical)

### 3.1 Device/Session Management
- [ ] Add device/session listing endpoint
- [ ] Add session revocation endpoint
- [ ] Add device fingerprinting
- [ ] Add session metadata storage

### 3.2 Security Enhancements
- [ ] Add login rate limiting (per account)
- [ ] Add brute-force protection
- [ ] Add account lockout after failed attempts
- [ ] Add temporary throttling
- [ ] Add strong JWT secret validation in production
- [ ] Add password policy enforcement

### 3.3 Password Reset Flow
- [ ] Complete password reset implementation
- [ ] Add token expiration
- [ ] Add one-time use tokens

## Phase 4: Transactional Ride Consistency (P0 - Launch Critical)

### 4.1 Transaction Verification
- [ ] Review current transaction usage
- [ ] Ensure ride state changes and events are atomic
- [ ] Ensure driver availability changes are in same transaction
- [ ] Prevent partial dispatch states

### 4.2 Transaction Tests
- [ ] Add tests for transactional behavior
- [ ] Add concurrency tests for ride matching
- [ ] Add tests for race conditions

## Phase 5: Enhanced Test Coverage (P0 - Launch Critical)

### 5.1 Authentication Tests
- [ ] Add JWT regression tests (claimed done, verify)
- [ ] Add refresh-token replay tests
- [ ] Add OTP tests
- [ ] Add password reset tests
- [ ] Add session management tests

### 5.2 Ride Tests
- [ ] Add ride state-transition tests
- [ ] Add concurrent offer acceptance tests
- [ ] Add ride cancellation tests
- [ ] Add no-show handling tests

### 5.3 Domain Tests
- [ ] Add KYC approval gate tests
- [ ] Add trip PIN tests
- [ ] Add cash payment tests
- [ ] Add invoice XML tests
- [ ] Add payment webhook tests
- [ ] Add migration tests
- [ ] Add API integration tests

## Phase 6: Push Notifications (P1 - Core Product)

### 6.1 Notification Providers
- [ ] Define push notification provider interface
- [ ] Implement Firebase Cloud Messaging (FCM)
- [ ] Implement Apple Push Notification service (APNs)

### 6.2 Notification Types
- [ ] Driver offer notifications
- [ ] Driver accepted notification
- [ ] Driver arriving notification
- [ ] Ride started/completed notifications
- [ ] Payment confirmation
- [ ] Invoice availability
- [ ] KYC approval/rejection

### 6.3 SMS Fallback
- [ ] OTP messages (from Phase 2)
- [ ] Ride status fallback
- [ ] Payment alerts
- [ ] Emergency notifications

## Phase 7: Admin Operations (P1 - Core Product)

### 7.1 User Management
- [ ] User search
- [ ] User profile management
- [ ] Suspend/reactivate users
- [ ] Suspend/reactivate drivers

### 7.2 Ride Intervention
- [ ] Ride intervention endpoint
- [ ] Ride reassignment
- [ ] Cancellation management
- [ ] Force-cancel/reassign

### 7.3 Financial Operations
- [ ] Refund processing
- [ ] Dispute management
- [ ] Payment operations dashboard

### 7.4 KYC Operations
- [ ] KYC review history
- [ ] KYC status management

### 7.5 Configuration
- [ ] Pricing configuration
- [ ] Service configuration
- [ ] Audit log viewer

## Phase 8: Rider Experience (P1 - Core Product)

### 8.1 Rider Dashboard Enhancements
- [ ] Saved places
- [ ] Favorite drivers
- [ ] Payment-method management
- [ ] Promo codes
- [ ] Ride scheduling (from migrations)
- [ ] Ride receipts
- [ ] Support tickets (from migrations)
- [ ] Ratings and reviews (from migrations)

## Phase 9: Driver Experience (P1 - Core Product)

### 9.1 Earnings & Payouts
- [ ] Driver earnings dashboard (claimed done, enhance)
- [ ] Payout history
- [ ] Daily/weekly/monthly statistics
- [ ] Driver ratings
- [ ] Vehicle profiles (from migrations)
- [ ] Document expiry reminders
- [ ] Driver performance metrics
- [ ] Shift/session tracking

## Phase 10: Ride Lifecycle Completeness (P1 - Core Product)

### 10.1 Cancellation System
- [ ] Cancellation reasons (from migrations)
- [ ] Cancellation fees
- [ ] No-show handling (from migrations)

### 10.2 Ride Management
- [ ] Ride timeout/expiry
- [ ] Offer retry limits
- [ ] Automatic reassignment
- [ ] Fare adjustment workflow

## Phase 11: Safety & Trust (P1 - Safety and Trust)

### 11.1 SOS/Emergency (claimed done, enhance)
- [ ] SOS button (from migrations)
- [ ] Emergency contacts (from migrations)
- [ ] Live trip sharing
- [ ] Emergency trip details
- [ ] Incident reports (from migrations)
- [ ] Safety escalation queue
- [ ] Admin emergency dashboard

### 11.2 Identity & Safety
- [ ] Driver selfie verification
- [ ] Rider/driver identity confirmation
- [ ] Vehicle make/model/color display (from migrations)
- [ ] License plate confirmation (from migrations)
- [ ] Suspicious trip detection
- [ ] GPS-spoofing detection

## Phase 12: Support System (P1 - Safety and Trust)

### 12.1 Support Tickets (from migrations)
- [ ] Rider support tickets
- [ ] Driver support tickets
- [ ] Ride-linked complaints
- [ ] Lost-and-found workflow
- [ ] Dispute status tracking (from migrations)
- [ ] Admin support queue
- [ ] Portuguese support templates

## Phase 13: Angola-Specific Features (P2)

### 13.1 Airport Transfers
- [ ] 4 de Fevereiro airport support
- [ ] Flight number tracking
- [ ] Meet-and-greet
- [ ] Fixed airport fares
- [ ] Waiting time
- [ ] Flight delay tracking
- [ ] Airport pickup zones

### 13.2 Service Zones (from migrations)
- [ ] Luanda zone
- [ ] Talatona zone
- [ ] Kilamba zone
- [ ] Viana zone
- [ ] Benfica zone
- [ ] Cacuaco zone
- [ ] Zone-specific pricing
- [ ] Zone driver rules
- [ ] Driver staging areas

### 13.3 Scheduled Rides (from migrations)
- [ ] Future booking
- [ ] Reminder notifications
- [ ] Driver pre-assignment
- [ ] Scheduled cancellation rules
- [ ] Airport scheduling

### 13.4 Intercity Rides
- [ ] Luanda to Lobito
- [ ] Luanda to Benguela
- [ ] Luanda to Huambo
- [ ] Fixed intercity pricing
- [ ] Long-distance driver matching
- [ ] Driver rest/safety rules

### 13.5 Corporate Accounts (from migrations)
- [ ] Business accounts (from migrations)
- [ ] Employee management
- [ ] Ride approval workflows (from migrations)
- [ ] Cost centers
- [ ] Monthly billing
- [ ] Corporate invoices
- [ ] Spending limits (from migrations)

### 13.6 Low-Bandwidth Mode
- [ ] Offline ride state queue
- [ ] Retry-safe API calls
- [ ] Polling fallback
- [ ] Queued location updates
- [ ] Reduced payload mode
- [ ] Poor-network detection

### 13.7 WhatsApp Support
- [ ] Ride status lookup
- [ ] Support conversations
- [ ] Trip sharing
- [ ] Payment receipts
- [ ] Driver/rider notifications

## Phase 14: Maps & Dispatch (P2)

### 14.1 Routing & ETA
- [ ] Google Maps/Mapbox/OSRM integration
- [ ] Route distance calculation
- [ ] Accurate ETA
- [ ] Route polylines
- [ ] Traffic-aware estimates
- [ ] Driver navigation links

### 14.2 Geospatial (from migrations - PostGIS)
- [ ] PostGIS indexes
- [ ] Accurate distance calculations
- [ ] Bounding-box filtering
- [ ] Nearby-driver performance
- [ ] Location history

### 14.3 Dispatch Optimization
- [ ] Demand heatmaps
- [ ] Supply heatmaps
- [ ] Surge pricing
- [ ] Driver repositioning
- [ ] Smart batching
- [ ] Scheduled-demand forecasting

## Phase 15: Payments & Revenue (P2)

### 15.1 Payment Ledger (enhance from Phase 1)
- [ ] Payment intents (from Phase 1)
- [ ] Payment attempts
- [ ] Provider references
- [ ] Webhook event ledger (from Phase 1)
- [ ] Refund records (from Phase 1)
- [ ] Settlement records
- [ ] Reconciliation status

### 15.2 Driver Payouts
- [ ] Commission calculation
- [ ] Driver balance
- [ ] Payout requests
- [ ] Bank/mobile-money payout
- [ ] Payout approval
- [ ] Failed payout handling
- [ ] Payout reports

### 15.3 Pricing Engine (enhance)
- [ ] Zone pricing (from migrations)
- [ ] Airport fees
- [ ] Surge pricing
- [ ] Minimum fare
- [ ] Waiting fee
- [ ] Cancellation fee
- [ ] Corporate pricing (from migrations)
- [ ] Promo campaigns

### 15.4 Fiscal Compliance
- [ ] Validate official Angola SAF-T requirements
- [ ] Taxpayer/company fields
- [ ] Tax rates
- [ ] Invoice series
- [ ] Credit notes
- [ ] Debit notes
- [ ] Certified fiscal export
- [ ] Accountant/admin workflow

## Phase 16: Platform Hardening (P3)

### 16.1 Observability
- [ ] Request IDs (claimed done, verify)
- [ ] Structured error codes
- [ ] Metrics
- [ ] Distributed tracing
- [ ] Database metrics
- [ ] Matching metrics
- [ ] Payment metrics
- [ ] Alerting

### 16.2 Health & Readiness
- [ ] PostgreSQL readiness (claimed done, verify)
- [ ] Valkey readiness
- [ ] Provider readiness
- [ ] Worker health
- [ ] Matching-service health
- [ ] Dependency status endpoint

### 16.3 API Quality
- [ ] OpenAPI documentation
- [ ] API versioning policy
- [ ] Contract tests
- [ ] Consistent pagination (claimed done, verify)
- [ ] Consistent error format (claimed done, verify)
- [ ] Request validation (claimed done, verify)
- [ ] CORS configuration (claimed done, verify)

### 16.4 Security
- [ ] Rate limiting (claimed done, verify)
- [ ] Security headers (claimed done, verify)
- [ ] CORS restrictions (claimed done, verify)
- [ ] Input size limits (claimed done, verify)
- [ ] Phone normalization
- [ ] Audit logging
- [ ] Secret rotation
- [ ] Sensitive-data redaction
- [ ] Dependency vulnerability scanning

### 16.5 Deployment
- [ ] Automated migration runner
- [ ] Database backup/restore
- [ ] Rollback procedures
- [ ] Staging environment
- [ ] Production environment
- [ ] CI pipeline
- [ ] Container health checks
- [ ] Graceful worker shutdown (claimed done, verify)
- [ ] Deployment runbook

### 16.6 Data Governance
- [ ] User data export
- [ ] Account deletion
- [ ] Data retention policies
- [ ] Location retention policies
- [ ] Payment retention policies
- [ ] Audit-log retention
- [ ] Privacy consent records
- [ ] Angola data-protection review

## Verification Checklist

Before considering each phase complete:

```bash
sqlc generate
gofmt -w .
go test ./...
go vet ./...
```

## Implementation Order

Based on dependencies and impact:

1. Phase 1: Payment Provider Adapters
2. Phase 2: SMS/OTP Provider Integration
3. Phase 3: Authentication Hardening
4. Phase 4: Transactional Ride Consistency
5. Phase 5: Enhanced Test Coverage
6. Phase 6: Push Notifications
7. Phase 7: Admin Operations
8. Phase 8: Rider Experience
9. Phase 9: Driver Experience
10. Phase 10: Ride Lifecycle Completeness
11. Phase 11: Safety & Trust
12. Phase 12: Support System
13. Phase 13: Angola-Specific Features
14. Phase 14: Maps & Dispatch
15. Phase 15: Payments & Revenue
16. Phase 16: Platform Hardening

## Progress Tracking

Each implementation will update this plan with:
- Completion date
- Files changed
- Tests added
- Verification results

## Completed: Payment Provider Adapters (P0 #1)

Date: 2024-01-10

### Implemented

1. **AppyPay Provider Adapter** (`internal/payment/providers/appypay.go`)
   - CreatePaymentIntent with GPO support
   - GetPaymentStatus
   - RefundPayment
   - HMAC-SHA256 webhook signature verification (supports hex and base64)
   - Webhook event parsing and validation
   - Event type mapping to provider-neutral events
   - Comprehensive unit tests

2. **VPOS Provider Adapter** (`internal/payment/providers/vpos.go`)
   - CreatePaymentIntent with HMAC-signed requests
   - GetPaymentStatus
   - RefundPayment
   - HMAC-SHA256 webhook signature verification
   - Webhook event parsing and validation
   - Request signing for API authentication
   - Comprehensive unit tests

3. **ProxyPay Provider Adapter** (`internal/payment/providers/proxypay.go`)
   - CreatePaymentIntent with Bearer token auth
   - GetPaymentStatus
   - RefundPayment
   - HMAC-SHA256 webhook signature verification
   - Webhook event parsing and validation
   - Comprehensive unit tests

4. **Provider Interface** (`internal/payment/providers/registry.go`)
   - Common Provider interface for all adapters
   - Common Payment and WebhookEvent structures
   - Registry for managing multiple providers

5. **Provider Factory** (`internal/payment/providers/factory.go`)
   - Factory for creating provider instances from config
   - Registry creation and provider registration

6. **Tests**
   - Unit tests for all three providers
   - Webhook signature verification tests
   - Webhook event parsing tests
   - API interaction tests with mock servers

### Test Results
- All provider tests passing (11 tests)
- All existing tests still passing
- go vet clean

### Files Created/Modified
- `internal/payment/providers/appypay.go` (new)
- `internal/payment/providers/vpos.go` (new)
- `internal/payment/providers/proxypay.go` (new)
- `internal/payment/providers/registry.go` (new)
- `internal/payment/providers/factory.go` (new)
- `internal/payment/providers/appypay_test.go` (new)
- `internal/payment/providers/vpos_test.go` (new)
- `internal/payment/providers/proxypay_test.go` (new)

### Next Steps
- Wire providers into the payment service
- Update webhook handlers to use provider-specific verifiers
- Add provider selection logic based on config
- Implement payment intent creation in the payment flow
