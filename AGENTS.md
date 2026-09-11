# RideX Angola Engineering Roadmap

This document records the remaining production gaps and the recommended implementation order for the RideX Angola backend.

## Current foundation

The backend currently includes:

- JWT authentication and rider/driver/admin roles
- Driver KYC submission and admin approval
- Driver availability and location tracking
- Transactional ride matching and expiring driver offers
- Ride lifecycle state guards
- Trip PIN generation and verification
- Cash payment confirmation and change calculation
- Payment-method selection for cash, Multicaixa, and card
- SAFT invoice creation and XML export foundation
- Background cleanup worker
- Admin and driver dashboard summary APIs
- Locally persisted in-app notifications with unread/read listing, read
  acknowledgement, per-user preferences, and a provider-neutral service
  boundary. No external push or SMS delivery is attempted.

## Production gaps

### P0: Launch blockers

- Implement real payment provider adapters for Multicaixa Express, AppyPay, VPOS, and/or ProxyPay.
- Add payment intent creation, provider webhooks, signature validation, idempotency, retries, reconciliation, refunds, receipts, commissions, and driver payouts.
- OTP request/verification and password-reset endpoints are implemented with
  hashed, expiring, attempt-limited challenges and an explicit no-op delivery
  boundary. Do not enable SMS until a provider is selected and documented.
- Add refresh-token rotation, token/session linkage, per-device revocation, password reset, login rate limiting, and production-secret validation.
- Add automated tests for authentication, ride authorization, state transitions, matching races, trip PINs, KYC, cash payments, invoices, and migrations.
- Make ride mutations and their audit events transactional.
- Add database migration execution and a documented admin bootstrap process.

### P1: Core product completeness

- Add external push notifications and SMS fallback for ride offers, driver
  arrival, trip completion, payments, invoices, and KYC status only after a
  provider is selected and documented. The current notification service is
  intentionally in-process and database-backed.
- Add rider dashboard aggregation: active ride, recent rides, invoices, payment methods, promotions, support, and notifications.
- Expand admin operations: user search, driver suspension/reactivation, ride intervention, payment disputes, refunds, KYC review, pricing/configuration, and audit logs.
- Add cancellation reasons, cancellation fees, no-show handling, offer retry limits, reassignment, ride expiry, and fare adjustment.
- Add driver ratings/reviews, earnings, payouts, and performance statistics.
- Emergency contacts, ride SOS, incident reporting, participant authorization,
  and ride audit events are implemented. Trip sharing, identity confirmation,
  and safety operations views remain.
- Add centralized Portuguese localization, `Accept-Language`, Portuguese SMS, invoice text, and Angola-specific AOA formatting.

### P2: Angola-specific differentiation

- Add airport transfer workflows with fixed fares, flight numbers, waiting time, and meet-and-greet.
- Add Luanda/Talatona/Kilamba/Viana and other service zones with operational rules and zone pricing.
- Add scheduled and intercity rides.
- Add corporate accounts, employee booking, approval workflows, cost centers, and monthly invoicing.
- Add low-bandwidth/offline driver support, request retries, queued location updates, and polling fallback.
- Add WhatsApp support and ride-specific support cases.

### P3: Maps and operations

- Add Google Maps or OSRM routing, ETA, route distance, and route polylines.
- Replace approximate PostgreSQL `POINT` distance queries with production geospatial indexing.
- Add location history, privacy rules, update rate limits, heading validation, and GPS-spoofing detection.
- Add operational metrics, demand heatmaps, driver staging areas, and surge controls.

## Technical hardening

- Add readiness and liveness endpoints with PostgreSQL and Valkey checks.
- Add request IDs, structured error codes, rate limiting, CORS, metrics, tracing, and centralized logs.
- Add OpenAPI documentation and API contract tests.
- Add backup/restore procedures, retention policies, and deployment runbooks.
- Keep secrets out of source control and reject unsafe default JWT secrets outside development.
- Keep generated SQL code synchronized with source queries using `sqlc generate`.

## Recommended implementation order

1. Authentication/session hardening and transactional ride consistency.
2. Automated unit, integration, API, and migration tests.
3. Payment provider abstraction and webhook/idempotency foundation.
4. OTP/SMS verification and Portuguese localization.
5. External push notifications and SMS fallback behind the existing
   provider-neutral notification service.
6. Admin operations and rider dashboard APIs.
7. Safety, support, cancellation, no-show, and dispute workflows.
8. Angola-specific airport, zone, corporate, and offline features.
9. Maps/ETA and production geospatial improvements.
10. Observability, deployment automation, backups, and launch readiness.

## Provider integration rule

Do not invent provider behavior or claim fiscal certification. Implement provider adapters only after selecting the provider, obtaining its API documentation and credentials, and confirming webhook/signature requirements. Keep external integrations behind interfaces so the core ride domain remains provider-independent.

## Validation requirement

Before considering a feature complete, run:

```bash
sqlc generate
gofmt -w .
go test ./...
go vet ./...
```

Features that change ride, payment, KYC, authentication, or invoice behavior must include targeted tests and migration coverage.
