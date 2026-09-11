// Command migrate applies or rolls back the SQL migrations in migrations/.
//
// Examples:
//
//	go run ./cmd/migrate -command up
//	go run ./cmd/migrate -command status
//	go run ./cmd/migrate -command down -steps 1
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ridex/ridex-angola/config"
)

type migration struct {
	version int64
	name    string
	up      string
	down    string
}

func main() {
	command := flag.String("command", "up", "migration command: up, down, or status")
	dir := flag.String("dir", "migrations", "directory containing migration SQL files")
	steps := flag.Int("steps", 0, "number of migrations for down (default: 1)")
	flag.Parse()

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

	migrations, err := loadMigrations(*dir)
	if err != nil {
		fail(err)
	}
	if err := ensureTable(ctx, pool); err != nil {
		fail(err)
	}

	switch strings.ToLower(*command) {
	case "up":
		err = applyUp(ctx, pool, migrations)
	case "down":
		if *steps == 0 {
			*steps = 1
		}
		err = applyDown(ctx, pool, migrations, *steps)
	case "status":
		err = printStatus(ctx, pool, migrations)
	default:
		err = fmt.Errorf("unknown migration command %q", *command)
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "migration: %v\n", err)
	os.Exit(1)
}

func loadMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migration directory: %w", err)
	}
	ups := make(map[int64]migration)
	downs := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 || (strings.HasSuffix(entry.Name(), ".up.sql") == false && strings.HasSuffix(entry.Name(), ".down.sql") == false) {
			continue
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid migration filename %q: %w", entry.Name(), err)
		}
		base := strings.TrimSuffix(strings.TrimSuffix(parts[1], ".up.sql"), ".down.sql")
		path := filepath.Join(dir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			if _, exists := ups[version]; exists {
				return nil, fmt.Errorf("duplicate up migration version %d", version)
			}
			ups[version] = migration{version: version, name: base, up: string(body)}
		} else {
			if _, exists := downs[version]; exists {
				return nil, fmt.Errorf("duplicate down migration version %d", version)
			}
			downs[version] = string(body)
		}
	}
	out := make([]migration, 0, len(ups))
	for version, item := range ups {
		down, ok := downs[version]
		if !ok {
			return nil, fmt.Errorf("migration %d is missing a down file", version)
		}
		item.down = down
		item.version = version
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func ensureTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return err
}

func applied(ctx context.Context, pool *pgxpool.Pool) (map[int64]string, error) {
	rows, err := pool.Query(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64]string)
	for rows.Next() {
		var version int64
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			return nil, err
		}
		result[version] = name
	}
	return result, rows.Err()
}

func applyUp(ctx context.Context, pool *pgxpool.Pool, migrations []migration) error {
	done, err := applied(ctx, pool)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if _, ok := done[migration.version]; ok {
			continue
		}
		if err := runMigration(ctx, pool, migration.up, migration.version, migration.name, true); err != nil {
			return err
		}
		fmt.Printf("applied %06d_%s\n", migration.version, migration.name)
	}
	return nil
}

func applyDown(ctx context.Context, pool *pgxpool.Pool, migrations []migration, steps int) error {
	if steps < 1 {
		return errors.New("steps must be greater than zero")
	}
	done, err := applied(ctx, pool)
	if err != nil {
		return err
	}
	for i := len(migrations) - 1; i >= 0 && steps > 0; i-- {
		migration := migrations[i]
		if _, ok := done[migration.version]; !ok {
			continue
		}
		if err := runMigration(ctx, pool, migration.down, migration.version, migration.name, false); err != nil {
			return err
		}
		fmt.Printf("reverted %06d_%s\n", migration.version, migration.name)
		steps--
	}
	if steps > 0 {
		return fmt.Errorf("only %d applied migration(s) available to revert", steps)
	}
	return nil
}

func runMigration(ctx context.Context, pool *pgxpool.Pool, sql string, version int64, name string, up bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("execute migration %06d_%s: %w", version, name, err)
	}
	if up {
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, version, name); err != nil {
			return err
		}
	} else if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func printStatus(ctx context.Context, pool *pgxpool.Pool, migrations []migration) error {
	done, err := applied(ctx, pool)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		status := "pending"
		if name, ok := done[migration.version]; ok {
			status = "applied"
			if name != migration.name {
				status = "applied (name mismatch)"
			}
		}
		fmt.Printf("%06d %-24s %s\n", migration.version, migration.name, status)
	}
	return nil
}
