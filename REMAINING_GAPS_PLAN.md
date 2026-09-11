# Remaining Gaps Implementation Plan

## Priority Order (based on launch impact)

### P0 - Launch Critical (Remaining)

1. **P0 #2: OTP and SMS Integration**
   - Wire up OTPDelivery interface
   - Implement SMS provider (Termii/Africa's Talking)
   - Add SMS delivery for OTP
   - Add rate limiting for OTP requests

2. **P0 #3: Authentication Hardening**
   - Add device/session listing endpoint
   - Add session revocation endpoint
   - Add login rate limiting
   - Add brute-force protection
   - Add account lockout

3. **P0 #5: Transactional Ride Consistency**
   - Verify ride mutations are atomic
   - Add tests for concurrent offer acceptance
   - Verify ride events are in same transaction

### P1 - Core Product (Next)

4. **P1 #6: Push Notifications**
   - Implement push notification service
   - Add Firebase/APNs integration
   - Wire notifications to ride events

5. **P1 #8: Admin Operations**
   - User search and profile management
   - Suspend/reactivate users
   - Ride intervention
   - KYC review history

6. **P1 #9: Rider Experience**
   - Saved places
   - Favorite drivers
   - Payment method management
   - Promo codes

7. **P1 #10: Driver Experience**
   - Earnings dashboard
   - Payout history
   - Driver statistics

8. **P1 #11: Ride Lifecycle Completeness**
   - Cancellation reasons/fees
   - No-show handling
   - Ride timeout/expiry
   - Automatic reassignment

### P1 - Safety & Trust

9. **P1 #12: SOS/Emergency Workflows**
   - SOS button API
   - Emergency contact management
   - Live trip sharing

10. **P1 #13: Identity & Trip Safety**
    - Driver selfie verification
    - Identity confirmation
    - Vehicle display

### P2 - Angola-Specific

11. **P2 #15: Airport Transfer**
    - Airport pickup zones
    - Fixed airport fares
    - Flight number tracking

12. **P2 #16: Luanda Zones**
    - Zone-specific pricing
    - Driver rules per zone
    - Driver staging areas

13. **P2 #17: Scheduled Rides**
    - Future booking
    - Reminder notifications
    - Scheduled cancellation rules

14. **P2 #18: Intercity Rides**
    - Fixed intercity pricing
    - Long-distance matching
    - Driver rest rules

### P2 - Payments & Revenue

15. **P2 #25: Payment Ledger**
    - Payment intents tracking
    - Provider references
    - Webhook event ledger
    - Refund records

16. **P2 #26: Driver Payouts**
    - Commission calculation
    - Driver balance
    - Payout requests
    - Bank/mobile-money payout

### Implementation Strategy

I'll work through these in batches:

**Batch 1 (Today)**: P0 #2, P0 #3, P0 #5
- Complete authentication and OTP gaps
- Verify transactional consistency

**Batch 2**: P1 #6, P1 #8, P1 #9, P1 #10
- Push notifications
- Admin operations
- User experiences

**Batch 3**: P1 #11, P1 #12, P1 #13, P2 #15-18
- Ride lifecycle
- Safety features
- Angola-specific features

**Batch 4**: P2 #25-26, P3 items
- Payment ledger
- Driver payouts
- Platform hardening

