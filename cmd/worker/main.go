package main

import (
	"context"
	"log"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ridex/ridex-angola/config"
	"github.com/ridex/ridex-angola/internal/db"
	"github.com/ridex/ridex-angola/internal/payment"
	"github.com/ridex/ridex-angola/internal/payment/providers"
	"github.com/ridex/ridex-angola/internal/payouts"
	payoutproviders "github.com/ridex/ridex-angola/internal/payouts/providers"
	"go.uber.org/zap"
)

const (
	cleanupInterval = time.Minute
	// Payment reconciliation polls the configured provider for charges whose
	// webhooks were missed. It only runs when PAYMENT_PROVIDER selects a
	// configured provider; otherwise the ledger relies on webhooks alone.
	reconcileInterval   = 5 * time.Minute
	reconcileBatchLimit = 25
	// Payout execution submits approved payouts to the configured executor and
	// polls in-flight ones. It only runs when PAYOUT_EXECUTOR is set.
	payoutExecutionInterval = 5 * time.Minute
	payoutBatchLimit        = 25
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		logger.Fatal("worker: failed to connect to db", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("worker: failed to ping db", zap.Error(err))
	}

	queries := db.New(pool)
	logger.Info("background worker started")

	// Payment status poller for reconciliation. Mirrors the API wiring: the
	// selected provider is only usable when its credentials are present.
	var statusPoller payment.StatusPoller
	selectedProvider := strings.ToLower(strings.TrimSpace(cfg.PaymentProvider))
	if selectedProvider != "" && selectedProvider != "none" {
		factory := providers.NewFactory(logger)
		registry, err := factory.CreateRegistry(
			factory.CreateAppyPay(cfg.AppyPayClientID, cfg.AppyPayClientSecret, cfg.AppyPayBaseURL, cfg.AppyPayGPOEnabled),
			factory.CreateVPOS(cfg.VPOSDeveloperID, cfg.VPOSAPIKey, cfg.VPOSBaseURL, cfg.VPOSWebhookSecret),
			factory.CreateProxyPay(cfg.ProxyPayAPIKey, cfg.ProxyPayBaseURL),
		)
		if err != nil {
			logger.Info("payment reconciliation disabled: no payment providers configured")
		} else if client, getErr := registry.Get(selectedProvider); getErr != nil {
			logger.Warn("payment reconciliation disabled: selected provider is not configured",
				zap.String("provider", selectedProvider), zap.Error(getErr))
		} else {
			statusPoller = payment.ProviderStatusPoller{Name: selectedProvider, Client: client}
			logger.Info("payment reconciliation enabled", zap.String("provider", selectedProvider),
				zap.Duration("interval", reconcileInterval))
		}
	}
	ledger := payment.NewService(queries).WithStatusPoller(statusPoller)

	// Payout executor for driver payout execution. "manual" uses the
	// deterministic in-process executor (submissions recorded, no real money
	// movement); any real banking/Multicaixa provider plugs in here later.
	var payoutLedger *payouts.Ledger
	executorName := strings.ToLower(strings.TrimSpace(cfg.PayoutExecutor))
	switch executorName {
	case "manual":
		payoutLedger = payouts.NewLedgerWithTx(queries, pool.Begin).WithExecutor(payouts.NewManualExecutor())
		logger.Info("payout execution enabled", zap.String("executor", executorName),
			zap.Duration("interval", payoutExecutionInterval))
	case "http":
		executor := payoutproviders.NewHTTPExecutor(cfg.PayoutExecutorURL, cfg.PayoutExecutorAPIKey, 0)
		if !executor.IsEnabled() {
			logger.Warn("payout execution disabled: http executor missing credentials",
				zap.Bool("hasURL", strings.TrimSpace(cfg.PayoutExecutorURL) != ""),
				zap.Bool("hasAPIKey", strings.TrimSpace(cfg.PayoutExecutorAPIKey) != ""))
			break
		}
		payoutLedger = payouts.NewLedgerWithTx(queries, pool.Begin).WithExecutor(executor)
		logger.Info("payout execution enabled", zap.String("executor", executorName),
			zap.String("url", cfg.PayoutExecutorURL),
			zap.Duration("interval", payoutExecutionInterval))
	case "", "none":
		// Payout execution disabled: admins settle payouts through the API.
	default:
		logger.Warn("payout execution disabled: unknown executor", zap.String("executor", executorName))
	}

	runCleanup := func() {
		if err := cleanup(ctx, pool, queries); err != nil {
			logger.Error("background cleanup failed", zap.Error(err))
		}
	}
	runReconcile := func() {
		if statusPoller == nil {
			return
		}
		result, err := ledger.ReconcilePending(ctx, reconcileBatchLimit)
		if err != nil {
			logger.Error("payment reconciliation failed", zap.Error(err))
			return
		}
		if result.Checked != 0 || result.Updated != 0 || result.Failed != 0 {
			logger.Info("payment reconciliation completed",
				zap.Int("checked", result.Checked), zap.Int("updated", result.Updated),
				zap.Int("skipped", result.Skipped), zap.Int("failed", result.Failed))
		}
	}

	runPayoutExec := func() {
		if payoutLedger == nil {
			return
		}
		submission, err := payoutLedger.ProcessPayoutSubmissions(ctx, payoutBatchLimit)
		if err != nil {
			logger.Error("payout submission failed", zap.Error(err))
			return
		}
		poll, err := payoutLedger.PollPayoutExecutions(ctx, payoutBatchLimit)
		if err != nil {
			logger.Error("payout polling failed", zap.Error(err))
			return
		}
		if submission.Checked != 0 || submission.Submitted != 0 || submission.Failed != 0 ||
			submission.Uncertain != 0 ||
			poll.Checked != 0 || poll.Updated != 0 || poll.Failed != 0 {
			logger.Info("payout execution completed",
				zap.Int("submittedChecked", submission.Checked), zap.Int("submitted", submission.Submitted),
				zap.Int("submissionFailed", submission.Failed), zap.Int("submissionUncertain", submission.Uncertain),
				zap.Int("polls", poll.Checked), zap.Int("settled", poll.Updated),
				zap.Int("pollSkipped", poll.Skipped), zap.Int("pollFailed", poll.Failed))
		}
	}

	runCleanup()
	runReconcile()
	runPayoutExec()

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	reconcileTicker := time.NewTicker(reconcileInterval)
	defer reconcileTicker.Stop()
	payoutTicker := time.NewTicker(payoutExecutionInterval)
	defer payoutTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("background worker stopped")
			return
		case <-ticker.C:
			runCleanup()
		case <-reconcileTicker.C:
			runReconcile()
		case <-payoutTicker.C:
			runPayoutExec()
		}
	}
}

func cleanup(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries) error {
	if err := queries.DeleteExpiredCodes(ctx); err != nil {
		return err
	}
	if err := queries.DeleteExpiredSessions(ctx); err != nil {
		return err
	}
	if err := queries.DeleteStaleDriverLocations(ctx); err != nil {
		return err
	}

	_, err := pool.Exec(ctx, `WITH expired AS (
		UPDATE ride_offers
		SET status = 'expired', responded_at = NOW()
		WHERE status = 'offered' AND expires_at <= NOW()
		RETURNING driver_id
	)
	UPDATE driver_availability
	SET is_online = true, updated_at = NOW()
	WHERE driver_id IN (SELECT driver_id FROM expired)`)
	return err
}
