package main

import (
	"path/filepath"
	"testing"
)

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(migrations) != 31 {
		t.Fatalf("loaded %d migrations, want 31", len(migrations))
	}
	for i, migration := range migrations {
		if migration.version != int64(i+1) {
			t.Fatalf("migration %d has version %d", i, migration.version)
		}
		if migration.up == "" || migration.down == "" {
			t.Fatalf("migration %d has an empty up or down script", migration.version)
		}
	}
}
