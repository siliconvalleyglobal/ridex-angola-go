package angola

// Message keys for Portuguese (Angola) localization.
// These keys are used to retrieve localized messages throughout the application.

// Ride-related messages
const (
	MsgRideRequested     = "ride.requested"
	MsgDriverFound       = "ride.driver_found"
	MsgDriverAccepting   = "ride.driver_accepting"
	MsgDriverOnWay       = "ride.driver_on_way"
	MsgDriverArriving    = "ride.driver_arriving"
	MsgRideStarted       = "ride.started"
	MsgRideCompleted     = "ride.completed"
	MsgRideCancelled     = "ride.cancelled"
	MsgRideNoDriverFound = "ride.no_driver_found"
	MsgRideExpired       = "ride.expired"
	MsgRidePinRequired   = "ride.pin_required"
	MsgRidePinVerified   = "ride.pin_verified"
	MsgRidePinInvalid    = "ride.pin_invalid"
)

// Payment-related messages
const (
	MsgPaymentPending    = "payment.pending"
	MsgPaymentProcessing = "payment.processing"
	MsgPaymentCompleted  = "payment.completed"
	MsgPaymentFailed     = "payment.failed"
	MsgPaymentRefunded   = "payment.refunded"
	MsgPaymentCash       = "payment.cash"
	MsgPaymentCard       = "payment.card"
	MsgPaymentMulticaixa = "payment.multicaixa"
	MsgPaymentChangeDue  = "payment.change_due"
	MsgPaymentAmountDue  = "payment.amount_due"
)

// Invoice messages
const (
	MsgInvoiceCreated   = "invoice.created"
	MsgInvoiceSent      = "invoice.sent"
	MsgInvoiceAvailable = "invoice.available"
)

// KYC messages
const (
	MsgKycSubmitted = "kyc.submitted"
	MsgKycApproved  = "kyc.approved"
	MsgKycRejected  = "kyc.rejected"
	MsgKycPending   = "kyc.pending"
	MsgKycRequired  = "kyc.required"
)

// OTP/SMS messages
const (
	MsgOTPWelcome          = "otp.welcome"
	MsgOTPReceived         = "otp.received"
	MsgOTPSent             = "otp.sent"
	MsgOTPExpired          = "otp.expired"
	MsgOTPInvalid          = "otp.invalid"
	MsgOTPRequired         = "otp.required"
	MsgOTPTooManyAttempts  = "otp.too_many_attempts"
	MsgOTPRateLimited      = "otp.rate_limited"
	MsgOTPPhoneChanged     = "otp.phone_changed"
	MsgOTPVerificationCode = "otp.verification_code"
	MsgOTPLogin            = "otp.login"
	MsgOTPRegister         = "otp.register"
	MsgOTPPasswordReset    = "otp.password_reset"
	MsgOTPPhoneChange      = "otp.phone_change"
)

// Authentication messages
const (
	MsgAuthWelcome         = "auth.welcome"
	MsgAuthLoginSuccess    = "auth.login_success"
	MsgAuthLogoutSuccess   = "auth.logout_success"
	MsgAuthTokenExpired    = "auth.token_expired"
	MsgAuthUnauthorized    = "auth.unauthorized"
	MsgAuthForbidden       = "auth.forbidden"
	MsgAuthPasswordReset   = "auth.password_reset"
	MsgAuthPasswordChanged = "auth.password_changed"
	MsgAuthAccountLocked   = "auth.account_locked"
)

// Ride offer messages
const (
	MsgOfferReceived       = "offer.received"
	MsgOfferAccepted       = "offer.accepted"
	MsgOfferDeclined       = "offer.declined"
	MsgOfferExpired        = "offer.expired"
	MsgOfferNoDrivers      = "offer.no_drivers"
	MsgOfferTooManyDrivers = "offer.too_many_drivers"
)

// Safety messages
const (
	MsgSafetySOSTriggered    = "safety.sos_triggered"
	MsgSafetyContactNotified = "safety.contact_notified"
	MsgSafetyTripShared      = "safety.trip_shared"
	MsgSafetyEmergency       = "safety.emergency"
)

// Support messages
const (
	MsgSupportTicketCreated    = "support.ticket_created"
	MsgSupportTicketUpdated    = "support.ticket_updated"
	MsgSupportResponseReceived = "support.response_received"
)

// General messages
const (
	MsgGenericError        = "generic.error"
	MsgGenericSuccess      = "generic.success"
	MsgGenericNotFound     = "generic.not_found"
	MsgGenericUnauthorized = "generic.unauthorized"
	MsgGenericValidation   = "generic.validation"
	MsgGenericNetworkError = "generic.network_error"
	MsgGenericTimeout      = "generic.timeout"
)
