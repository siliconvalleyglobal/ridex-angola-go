// Command seed upserts local demo accounts for the RideX studio.
//
//	go run ./cmd/seed
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ridex/ridex-angola/config"
	"github.com/ridex/ridex-angola/internal/auth"
)

type demoUser struct {
	Phone    string
	Name     string
	Role     string
	Password string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		fail(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fail(err)
	}

	users := []demoUser{
		{Phone: "+244923000000", Name: "Ana Admin", Role: "admin", Password: "DemoPass123!"},
		{Phone: "+244923000001", Name: "Rita Rider", Role: "rider", Password: "DemoPass123!"},
		{Phone: "+244923000002", Name: "Dario Driver", Role: "driver", Password: "DemoPass123!"},
	}

	for _, user := range users {
		hash, err := auth.HashPassword(user.Password)
		if err != nil {
			fail(err)
		}
		var id string
		err = pool.QueryRow(ctx, `
			INSERT INTO users (phone, name, role, password_hash, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (phone) DO UPDATE SET
				name = EXCLUDED.name,
				role = EXCLUDED.role,
				password_hash = EXCLUDED.password_hash,
				is_active = true,
				updated_at = NOW()
			RETURNING id::text`,
			user.Phone, user.Name, user.Role, hash,
		).Scan(&id)
		if err != nil {
			fail(fmt.Errorf("upsert %s: %w", user.Phone, err))
		}
		fmt.Printf("seeded %s %s %s\n", user.Role, user.Phone, id)

		if user.Role != "driver" {
			continue
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO driver_kyc (driver_id, id_document, license_number, vehicle_plate, status, submitted_at, reviewed_at, review_note)
			VALUES ($1::uuid, 'BI-DEMO-002', 'Carta-DEMO-002', 'LD-00-00-AA', 'approved', NOW(), NOW(), 'local studio seed')
			ON CONFLICT (driver_id) DO UPDATE SET
				status = 'approved',
				reviewed_at = NOW(),
				review_note = EXCLUDED.review_note`, id); err != nil {
			fail(fmt.Errorf("kyc %s: %w", user.Phone, err))
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO driver_availability (driver_id, is_online, updated_at)
			VALUES ($1::uuid, true, NOW())
			ON CONFLICT (driver_id) DO UPDATE SET is_online = true, updated_at = NOW()`, id); err != nil {
			fail(fmt.Errorf("availability %s: %w", user.Phone, err))
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO driver_locations (driver_id, location, heading, updated_at)
			VALUES ($1::uuid, point(13.215, -9.455), 90, NOW())
			ON CONFLICT (driver_id) DO UPDATE SET location = EXCLUDED.location, heading = EXCLUDED.heading, updated_at = NOW()`, id); err != nil {
			fail(fmt.Errorf("location %s: %w", user.Phone, err))
		}
	}
	fmt.Println("demo accounts ready (password DemoPass123!)")
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "seed: %v\n", err)
	os.Exit(1)
}
