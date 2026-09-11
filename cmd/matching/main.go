package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ridex/ridex-angola/config"
	"go.uber.org/zap"
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
		logger.Fatal("matching: failed to connect to db", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("matching: failed to ping db", zap.Error(err))
	}

	logger.Info("matching service started", zap.String("grpc_port", cfg.GRPCPort))
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("matching service stopped")
			return
		case <-ticker.C:
			if err := matchOne(ctx, pool); err != nil {
				logger.Error("matching cycle failed", zap.Error(err))
			}
		}
	}
}

func matchOne(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `WITH expired AS (
		UPDATE ride_offers SET status = 'expired', responded_at = NOW()
		WHERE status = 'offered' AND expires_at <= NOW()
		RETURNING driver_id
	)
	UPDATE driver_availability SET is_online = true, updated_at = NOW()
	WHERE driver_id IN (SELECT driver_id FROM expired)`); err != nil {
		return err
	}

	var rideID uuid.UUID
	var pickupX, pickupY float64
	err = tx.QueryRow(ctx, `SELECT id, pickup_point[0], pickup_point[1]
		FROM rides
		WHERE status = 'requested' AND driver_id IS NULL
		  AND (requested_pickup_at IS NULL OR requested_pickup_at <= NOW())
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&rideID, &pickupX, &pickupY)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	var driverID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT l.driver_id
		FROM driver_locations l
		JOIN driver_availability a ON a.driver_id = l.driver_id AND a.is_online
		JOIN users u ON u.id = l.driver_id AND u.is_active AND u.role = 'driver'
		JOIN driver_kyc k ON k.driver_id = l.driver_id AND k.status = 'approved'
		WHERE l.updated_at > NOW() - interval '10 minutes'
		  AND a.updated_at > NOW() - interval '10 minutes'
		ORDER BY sqrt(power((l.location[0] - $1::float8)::numeric, 2) +
		             power((l.location[1] - $2::float8)::numeric, 2))
		LIMIT 1`, pickupX, pickupY).Scan(&driverID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `INSERT INTO ride_offers (ride_id, driver_id, status, expires_at)
		VALUES ($1, $2, 'offered', NOW() + interval '15 seconds')`, rideID, driverID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE driver_availability
		SET is_online = false, updated_at = NOW()
		WHERE driver_id = $1`, driverID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO ride_events (ride_id, event_type, payload, created_by)
		VALUES ($1, 'offer_created', '{}'::jsonb, $2)`, rideID, driverID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
