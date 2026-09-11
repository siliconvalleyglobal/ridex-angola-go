// cmd/api/main.go — RideX Angola HTTP API server.
//
// Starts a Gin server on the configured port with:
//   - health check endpoint
//   - suggested-fare and create-bid ride endpoints (migrated from TS Elysia API)
//   - zap structured logging middleware
//   - graceful shutdown on SIGINT/SIGTERM
//   - configuration from .env via the config package
//   - PostgreSQL connection via pgx+v5 and sqlc-generated queries
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ridex/ridex-angola/config"
	"github.com/ridex/ridex-angola/internal/airport"
	"github.com/ridex/ridex-angola/internal/auth"
	"github.com/ridex/ridex-angola/internal/business"
	"github.com/ridex/ridex-angola/internal/cash"
	"github.com/ridex/ridex-angola/internal/dashboard"
	"github.com/ridex/ridex-angola/internal/db"
	"github.com/ridex/ridex-angola/internal/drivers"
	"github.com/ridex/ridex-angola/internal/identity"
	"github.com/ridex/ridex-angola/internal/invoices"
	"github.com/ridex/ridex-angola/internal/middleware"
	"github.com/ridex/ridex-angola/internal/notifications"
	pushproviders "github.com/ridex/ridex-angola/internal/notifications/providers"
	"github.com/ridex/ridex-angola/internal/payment"
	"github.com/ridex/ridex-angola/internal/payment/providers"
	"github.com/ridex/ridex-angola/internal/payouts"
	"github.com/ridex/ridex-angola/internal/pricing"
	"github.com/ridex/ridex-angola/internal/rides"
	"github.com/ridex/ridex-angola/internal/safety"
	"github.com/ridex/ridex-angola/internal/support"
	"github.com/ridex/ridex-angola/internal/trust"
	"github.com/ridex/ridex-angola/internal/zones"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Set Gin mode based on log level
	gin.SetMode(gin.DebugMode)
	if cfg.LogLevel == "info" || cfg.LogLevel == "warn" || cfg.LogLevel == "error" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin engine
	r := gin.New()
	r.Use(middleware.RecoveryJSON())
	r.Use(middleware.RequestID())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins, cfg.CORSAllowCredentials))
	r.Use(middleware.RequestSizeLimit(cfg.MaxRequestBodyBytes))
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.NewRateLimiter(120, time.Minute))
	r.Use(zapGinMiddleware(logger))

	// ── Database ──────────────────────────────────────────────────────────
	pool, err := pgxpool.New(context.Background(), cfg.DSN())
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		logger.Fatal("failed to ping database", zap.Error(err))
	}
	logger.Info("connected to postgres", zap.String("addr", cfg.ValkeyAddr()))

	q := db.New(pool)

	// ── Auth ──────────────────────────────────────────────────────────────
	tokens := auth.NewTokenManager(
		cfg.JWTAccessSecret, cfg.JWTRefreshSecret,
		cfg.JWTAccessExpirySec, cfg.JWTRefreshExpirySec,
	)
	// OTP delivery is configured via the SMSProvider environment variable.
	// Available providers: termii, africastalking, twilio, noop (default).
	// When no provider is configured, OTP challenges are still persisted and
	// hashed, but no SMS is delivered (development mode).
	otpFactory := auth.NewOTPDeliveryFactory(logger)
	otpDelivery := otpFactory.Create(
		cfg.SMSProvider,
		cfg.TermiiAPIKey, cfg.TermiiSenderID,
		cfg.ATSciptKey, cfg.ATUsername,
		cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFrom,
	)
	authHandler := auth.NewHandlerWithOTP(q, tokens, cfg.JWTRefreshExpirySec, otpDelivery,
		cfg.OTPCodeExpirySec, cfg.OTPCodeLength, time.Minute)
	rideHandler := rides.NewHandlerWithTx(q, pool.Begin)
	// In-app notifications with optional external push fan-out. PUSH_PROVIDER
	// selects fcm or apns; without a provider, notifications persist in-app only.
	notificationService := notifications.NewService(q)
	deliveryFactory := pushproviders.NewDeliveryFactory(logger)
	deliveryProvider := deliveryFactory.Create(cfg.PushProvider,
		cfg.FCMCredentials, cfg.FCMProjectID,
		cfg.APNSTeamID, cfg.APNSKeyID, cfg.APNSBundleID, cfg.APNSPrivateKey, cfg.APNSProduction,
		otpDelivery, cfg.SMSProvider)
	notificationService.WithPush(deliveryProvider, q)
	notificationHandler := notifications.NewHandlerWithTokens(notificationService, q)
	businessHandler := business.NewHandler(q)
	pricer := pricing.NewService(q)
	driverHandler := drivers.NewHandler(q)
	invoiceHandler := invoices.NewHandler(q)
	trustHandler := trust.NewHandlerWithNotifications(q, notificationService)
	cashHandler := cash.NewHandler(q)
	dashboardHandler := dashboard.NewHandler(q)
	safetyHandler := safety.NewHandler(q)
	supportHandler := support.NewHandler(q)
	zoneHandler := zones.NewHandler(q)
	airportHandler := airport.NewHandler(q, logger)
	identityHandler := identity.NewHandler(identity.NewService(q))
	// Payout ledger + wallet APIs. Money movements (withdrawal, settle, fail)
	// run atomically against the provider DB transaction.
	payoutMethods := payouts.DefaultPayoutMethods()
	if raw := strings.TrimSpace(os.Getenv("PAYOUT_METHODS")); raw != "" {
		payoutMethods = payouts.NewPayoutMethods(raw)
		logger.Info("payout methods configured", zap.String("methods", raw))
	}
	payoutLedger := payouts.NewLedgerWithTx(q, pool.Begin).WithMethods(payoutMethods)
	payoutHandler := payouts.NewHandler(payoutLedger)
	// Payment provider clients are created from configuration. A provider is
	// only usable when its credentials are present; PAYMENT_PROVIDER selects
	// the charger used for intent creation. Without a selection the ledger
	// stays in local-only mode and no provider calls are made.
	providerFactory := providers.NewFactory(logger)
	appyPayClient := providerFactory.CreateAppyPay(cfg.AppyPayClientID, cfg.AppyPayClientSecret, cfg.AppyPayBaseURL, cfg.AppyPayGPOEnabled)
	vposClient := providerFactory.CreateVPOS(cfg.VPOSDeveloperID, cfg.VPOSAPIKey, cfg.VPOSBaseURL, cfg.VPOSWebhookSecret)
	proxyPayClient := providerFactory.CreateProxyPay(cfg.ProxyPayAPIKey, cfg.ProxyPayBaseURL)

	var activeCharger payment.Charger
	var activeRefunder payment.Refunder
	var activeStatusPoller payment.StatusPoller
	providerAdapters := make(map[string]payment.ProviderWebhookAdapter, 3)
	if appyPayClient != nil {
		providerAdapters["appypay"] = appyPayClient
	}
	if vposClient != nil {
		providerAdapters["vpos"] = vposClient
	}
	if proxyPayClient != nil {
		providerAdapters["proxypay"] = proxyPayClient
	}
	selectedProvider := strings.ToLower(strings.TrimSpace(cfg.PaymentProvider))
	if selectedProvider != "" && selectedProvider != "none" {
		registry, err := providerFactory.CreateRegistry(appyPayClient, vposClient, proxyPayClient)
		if err == nil {
			if client, getErr := registry.Get(selectedProvider); getErr == nil {
				activeCharger = payment.ProviderCharger{Provider: client}
				activeRefunder = payment.ProviderRefunder{Provider: client}
				activeStatusPoller = payment.ProviderStatusPoller{Name: selectedProvider, Client: client}
				logger.Info("payment provider charger enabled", zap.String("provider", selectedProvider))
			} else {
				logger.Warn("selected payment provider is not configured; falling back to ledger-only intents",
					zap.String("provider", selectedProvider), zap.Error(getErr))
			}
		} else {
			logger.Warn("no payment providers configured; falling back to ledger-only intents", zap.Error(err))
		}
	}
	paymentHandler := payment.NewHandlerWithCharger(q, map[string]payment.WebhookVerifier{
		"appypay":  payment.NewHMACSHA256Verifier(cfg.PaymentWebhookSecret),
		"vpos":     payment.NewHMACSHA256Verifier(cfg.PaymentWebhookSecret),
		"proxypay": payment.NewHMACSHA256Verifier(cfg.PaymentWebhookSecret),
	}, activeCharger).WithAdapters(providerAdapters).
		WithRefunder(activeRefunder).
		WithStatusPoller(activeStatusPoller)

	// ── Routes ────────────────────────────────────────────────────────────

	// Root welcome
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Welcome to Ridex Angola API (Go/Gin)")
	})

	// Health endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   "ridex-angola-api",
		})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database unavailable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// API v1 group
	v1 := r.Group("/api/v1")
	{
		// Public auth endpoints
		authRoutes := v1.Group("/auth")
		authRoutes.Use(middleware.NewRateLimiter(20, time.Minute))
		{
			authRoutes.POST("/register", authHandler.Register)
			authRoutes.POST("/login", authHandler.Login)
			authRoutes.POST("/refresh", authHandler.Refresh)
			authRoutes.POST("/otp/request", authHandler.RequestOTP)
			authRoutes.POST("/otp/verify", authHandler.VerifyOTP)
			authRoutes.POST("/password-reset/request", authHandler.RequestPasswordReset)
			authRoutes.POST("/password-reset/verify", authHandler.VerifyPasswordResetOTP)
			authRoutes.POST("/password-reset", authHandler.CompletePasswordReset)
			authRoutes.POST("/password-reset/complete", authHandler.CompletePasswordReset)
		}

		// Authenticated endpoints
		protected := v1.Group("", tokens.Middleware())
		{
			protected.GET("/auth/me", authHandler.Me)
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/notifications/unread", notificationHandler.ListUnread)
			protected.GET("/notifications/read", notificationHandler.ListRead)
			protected.POST("/notifications/:id/read", notificationHandler.MarkRead)
			protected.PATCH("/notifications/:id/read", notificationHandler.MarkRead)
			protected.GET("/notifications/preferences", notificationHandler.GetPreferences)
			protected.POST("/notifications/preferences", notificationHandler.SavePreferences)
			protected.PUT("/notifications/preferences", notificationHandler.SavePreferences)
			protected.PATCH("/notifications/preferences", notificationHandler.SavePreferences)
			protected.DELETE("/notifications/preferences", notificationHandler.DeletePreferences)
			protected.POST("/notifications/devices", notificationHandler.RegisterDevice)
			protected.DELETE("/notifications/devices/:token", notificationHandler.UnregisterDevice)
			// Corporate accounts reuse rider users as account members.
			b := protected.Group("/business", auth.RequireRole("rider"))
			{
				b.POST("/accounts", businessHandler.CreateAccount)
				b.GET("/accounts", businessHandler.ListAccounts)
				b.GET("/accounts/:id", businessHandler.GetAccount)
				b.POST("/accounts/:id/members", businessHandler.AddMember)
				b.GET("/accounts/:id/members", businessHandler.ListMembers)
				b.PATCH("/accounts/:id/members/:userId", businessHandler.UpdateMember)
				b.PUT("/accounts/:id/members/:userId", businessHandler.UpdateMember)
				b.PUT("/accounts/:id/limits", businessHandler.UpdateLimits)
				b.PATCH("/accounts/:id/limits", businessHandler.UpdateLimits)
				b.GET("/accounts/:id/usage", businessHandler.Usage)
				b.GET("/accounts/:id/authorizations", businessHandler.PendingAuthorizations)
			}
			protected.GET("/emergency-contacts", safetyHandler.ListContacts)
			protected.POST("/emergency-contacts", safetyHandler.CreateContact)
			protected.GET("/emergency-contacts/:id", safetyHandler.GetContact)
			protected.PUT("/emergency-contacts/:id", safetyHandler.UpdateContact)
			protected.DELETE("/emergency-contacts/:id", safetyHandler.DeleteContact)

			// Driver location tracking + discovery
			d := protected.Group("/drivers", auth.RequireRole("driver"))
			{
				d.POST("/location", driverHandler.UpdateLocation)
				d.PUT("/availability", driverHandler.SetAvailability)
				d.POST("/vehicle-profile", driverHandler.CreateVehicleProfile)
				d.GET("/vehicle-profile", driverHandler.GetVehicleProfile)
				d.PUT("/vehicle-profile", driverHandler.UpdateVehicleProfile)
				d.PATCH("/vehicle-profile", driverHandler.UpdateVehicleProfile)
				d.POST("/vehicle", driverHandler.CreateVehicleProfile)
				d.GET("/vehicle", driverHandler.GetVehicleProfile)
				d.PUT("/vehicle", driverHandler.UpdateVehicleProfile)
				d.PATCH("/vehicle", driverHandler.UpdateVehicleProfile)
			}
			protected.GET("/drivers/nearby", driverHandler.Nearby)
			protected.GET("/drivers/offers", auth.RequireRole("driver"), rideHandler.Offers)
			protected.POST("/rides/:id/invoice", auth.RequireRole("rider"), invoiceHandler.Create)
			protected.GET("/invoices/:id", invoiceHandler.Get)
			protected.GET("/invoices/:id/export", invoiceHandler.ExportXML)
			protected.PUT("/rides/:id/payment-method", auth.RequireRole("rider"), trustHandler.SetPaymentMethod)
			protected.POST("/rides/:id/trip-pin", auth.RequireRole("rider"), trustHandler.CreatePIN)
			protected.POST("/rides/:id/trip-pin/verify", auth.RequireRole("driver"), trustHandler.VerifyPIN)
			protected.POST("/drivers/kyc", auth.RequireRole("driver"), trustHandler.SubmitKYC)
			protected.GET("/drivers/kyc", auth.RequireRole("driver"), trustHandler.GetKYC)
			protected.GET("/admin/kyc", auth.RequireRole("admin"), trustHandler.ListKYC)
			protected.POST("/admin/kyc/:driverId/review", auth.RequireRole("admin"), trustHandler.ReviewKYC)
			protected.POST("/identity/verification", identityHandler.Submit)
			protected.GET("/identity/verification", identityHandler.GetMine)
			protected.GET("/admin/identity-verifications", auth.RequireRole("admin"), identityHandler.AdminPending)
			protected.POST("/admin/identity-verifications/:verificationId/review", auth.RequireRole("admin"), identityHandler.AdminReview)
			protected.GET("/admin/dashboard", auth.RequireRole("admin"), dashboardHandler.AdminSummary)
			protected.POST("/admin/service-zones", auth.RequireRole("admin"), zoneHandler.Create)
			protected.GET("/admin/service-zones", auth.RequireRole("admin"), zoneHandler.List)
			protected.GET("/admin/service-zones/:id", auth.RequireRole("admin"), zoneHandler.Get)
			protected.PUT("/admin/service-zones/:id", auth.RequireRole("admin"), zoneHandler.Update)
			protected.PATCH("/admin/service-zones/:id", auth.RequireRole("admin"), zoneHandler.Update)
			protected.DELETE("/admin/service-zones/:id", auth.RequireRole("admin"), zoneHandler.Delete)
			protected.GET("/admin/users", auth.RequireRole("admin"), dashboardHandler.AdminUsers)
			protected.PATCH("/admin/users/:userId/status", auth.RequireRole("admin"), dashboardHandler.AdminSetUserStatus)
			protected.PUT("/admin/users/:userId/status", auth.RequireRole("admin"), dashboardHandler.AdminSetUserStatus)
			protected.GET("/admin/rides", auth.RequireRole("admin"), dashboardHandler.AdminRides)
			protected.GET("/admin/rides/:rideId", auth.RequireRole("admin"), dashboardHandler.AdminRideAudit)
			protected.GET("/admin/rides/:rideId/audit", auth.RequireRole("admin"), dashboardHandler.AdminRideAudit)
			protected.GET("/admin/payments", auth.RequireRole("admin"), dashboardHandler.AdminPayments)
			protected.GET("/admin/payments/:paymentId", auth.RequireRole("admin"), dashboardHandler.AdminPaymentAudit)
			protected.GET("/admin/payments/:paymentId/audit", auth.RequireRole("admin"), dashboardHandler.AdminPaymentAudit)
			protected.POST("/admin/payments/:paymentId/refund", auth.RequireRole("admin"), paymentHandler.Refund)
			protected.POST("/admin/reconciliation/payments", auth.RequireRole("admin"), paymentHandler.Reconcile)
			protected.GET("/admin/payouts", auth.RequireRole("admin"), payoutHandler.AdminPayoutQueue)
			protected.POST("/admin/payouts/:payoutId/approve", auth.RequireRole("admin"), payoutHandler.ApprovePayout)
			protected.POST("/admin/payouts/:payoutId/complete", auth.RequireRole("admin"), payoutHandler.CompletePayout)
			protected.POST("/admin/payouts/:payoutId/fail", auth.RequireRole("admin"), payoutHandler.FailPayout)
			protected.GET("/drivers/dashboard", auth.RequireRole("driver"), dashboardHandler.DriverSummary)
			protected.GET("/drivers/earnings", auth.RequireRole("driver"), dashboardHandler.DriverEarnings)
			protected.GET("/drivers/wallet", auth.RequireRole("driver"), payoutHandler.Wallet)
			protected.GET("/drivers/wallet/transactions", auth.RequireRole("driver"), payoutHandler.WalletTransactions)
			protected.POST("/drivers/payouts", auth.RequireRole("driver"), payoutHandler.RequestWithdrawal)
			protected.GET("/drivers/payouts", auth.RequireRole("driver"), payoutHandler.DriverPayouts)
			protected.GET("/riders/dashboard", auth.RequireRole("rider"), dashboardHandler.RiderSummary)
			rider := protected.Group("/riders", auth.RequireRole("rider"))
			{
				rider.GET("/saved-places", dashboardHandler.RiderSavedPlaces)
				rider.GET("/promos", dashboardHandler.RiderPromos)
				rider.GET("/loyalty", dashboardHandler.RiderLoyalty)
				rider.GET("/support-tickets", dashboardHandler.RiderSupportTickets)
			}
			protected.GET("/support-tickets", supportHandler.ListMySupportTickets)
			protected.GET("/admin/support-tickets", auth.RequireRole("admin"), supportHandler.AdminSupportTickets)
			protected.PATCH("/admin/support-tickets/:ticketId/status", auth.RequireRole("admin"), supportHandler.AdminUpdateSupportTicket)
			protected.PUT("/admin/support-tickets/:ticketId/status", auth.RequireRole("admin"), supportHandler.AdminUpdateSupportTicket)
			protected.GET("/admin/disputes", auth.RequireRole("admin"), supportHandler.AdminDisputes)
			protected.GET("/admin/sos", auth.RequireRole("admin"), safetyHandler.AdminActiveSOS)
			protected.POST("/admin/sos/:sosId/resolve", auth.RequireRole("admin"), safetyHandler.AdminResolveSOS)
			protected.PATCH("/admin/disputes/:disputeId/status", auth.RequireRole("admin"), supportHandler.AdminUpdateDispute)
			protected.PUT("/admin/disputes/:disputeId/status", auth.RequireRole("admin"), supportHandler.AdminUpdateDispute)
			protected.POST("/payments/intents", auth.RequireRole("rider"), paymentHandler.CreateIntent)
			protected.POST("/business/rides/:rideId/authorization/:decision",
				auth.RequireRole("rider"), businessHandler.DecideAuthorization)
			r := protected.Group("/rides")
			{
				r.POST("", auth.RequireRole("rider"), rideHandler.Create)
				r.GET("/open", auth.RequireRole("driver"), rideHandler.Open)
				r.GET("", rideHandler.List)
				r.GET("/:id", rideHandler.Get)
				r.POST("/:id/accept", auth.RequireRole("driver"), rideHandler.Accept)
				r.POST("/:id/decline", auth.RequireRole("driver"), rideHandler.Decline)
				r.POST("/:id/arrive", auth.RequireRole("driver"), rideHandler.Arrive)
				r.POST("/:id/start", auth.RequireRole("driver"), rideHandler.Start)
				r.POST("/:id/complete", auth.RequireRole("driver"), rideHandler.Complete)
				r.POST("/:id/cash-payment", auth.RequireRole("driver"), cashHandler.Confirm)
				r.POST("/:id/cancel", rideHandler.Cancel)
				r.POST("/:id/no-show", auth.RequireRole("driver"), rideHandler.NoShow)
				r.POST("/:id/reviews", supportHandler.CreateReview)
				r.POST("/:id/review", supportHandler.CreateReview)
				r.GET("/:id/reviews", supportHandler.ListReviews)
				r.POST("/:id/support-tickets", supportHandler.CreateSupportTicket)
				r.GET("/:id/support-tickets", supportHandler.ListSupportTickets)
				r.POST("/:id/disputes", supportHandler.CreateDispute)
				r.GET("/:id/disputes", supportHandler.ListDisputes)
				r.POST("/:id/sos", safetyHandler.TriggerSOS)
				r.POST("/:id/incidents", safetyHandler.ReportIncident)
				r.GET("/:id/safety", safetyHandler.ListSafetyEvents)
			}
		}

		// Airport transfers: fixed-fare workflows with flight numbers, waiting
		// time, passenger details and meet-and-greet for pickup/dropoff rides.
		// Accessible to riders and drivers once authenticated.
		a := protected.Group("/airport")
		{
			a.GET("/airports", airportHandler.ListAirports)
			a.GET("/airports/:id", airportHandler.GetAirport)
			a.GET("/airports/iata/:code", airportHandler.GetAirportByIATA)
			a.POST("/transfers", airportHandler.CreateTransfer)
			a.GET("/transfers", airportHandler.ListTransfers)
			a.GET("/transfers/:id", airportHandler.GetTransfer)
			a.PATCH("/transfers/:id", airportHandler.UpdateTransfer)
			a.POST("/transfers/:id/cancel", airportHandler.CancelTransfer)
		}

		// Provider-neutral webhook ingress. Provider adapters must translate
		// documented metadata to these headers; no provider calls are made here.
		v1.POST("/payments/webhooks/:provider/:providerChargeID", paymentHandler.Webhook)
		v1.POST("/payments/webhooks/:provider", paymentHandler.Webhook)

		rides := v1.Group("/rides")
		{
			rides.GET("/suggested-fare", func(c *gin.Context) {
				var q struct {
					FromLat float64 `form:"fromLat" binding:"required"`
					FromLng float64 `form:"fromLng" binding:"required"`
					ToLat   float64 `form:"toLat" binding:"required"`
					ToLng   float64 `form:"toLng" binding:"required"`
				}
				if err := c.ShouldBindQuery(&q); err != nil {
					c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
						"error": "fromLat, fromLng, toLat, toLng query params are required",
					})
					return
				}
				quote, err := pricer.Suggest(c.Request.Context(), q.FromLat, q.FromLng, q.ToLat, q.ToLng)
				if err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to compute fare"})
					return
				}
				c.JSON(http.StatusOK, quote)
			})

			rides.POST("/create-bid", func(c *gin.Context) {
				var body map[string]interface{}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
						"error": "invalid JSON body",
					})
					return
				}
				bidID := "bid_" + uuid.New().String()[:8]
				c.JSON(http.StatusOK, gin.H{
					"status":    "PENDING",
					"bidId":     bidID,
					"createdAt": time.Now().UTC().Format(time.RFC3339),
					"body":      body,
				})
			})
		}
	}

	// Ride expiry reaper: rides still 'requested' with no driver after the
	// TTL are cancelled with reason 'expired'. A minute-grained ticker is
	// enough; the reaper itself is idempotent and SKIP LOCKED so overlapping
	// ticks cannot double-process a batch.
	expiryStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				expired, err := rides.ExpireStaleRidesWithTTL(
					context.Background(), q, pool.Begin, rides.DefaultRideExpiryTTL)
				if err != nil {
					logger.Warn("ride expiry sweep failed", zap.Error(err))
				} else if expired > 0 {
					logger.Info("expired stale ride requests", zap.Int("count", expired))
				}
			case <-expiryStop:
				return
			}
		}
	}()
	defer close(expiryStop)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.ServerPort),
		Handler:      r,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting server",
			zap.String("addr", srv.Addr),
			zap.String("port", cfg.ServerPort),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ServerShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}

// zapGinMiddleware returns a Gin middleware that logs each request with zap.
func zapGinMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info("request completed",
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}
