package postgres

import (
	"context"
	"errors"
	"fmt"

	"coworking/db/migrations"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunMigrations applies all embedded migrations before serving requests.
func RunMigrations(databaseURL string) error {
	uri, err := MigrationURI(databaseURL)
	if err != nil {
		return err
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}
	migrator, err := migrate.NewWithSourceInstance("iofs", source, uri)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}

// Ping waits for PostgreSQL to become available until ctx is cancelled.
func Ping(ctx context.Context, databaseURL string) error {
	repo, err := NewBookingRepository(ctx, databaseURL)
	if err != nil {
		return err
	}
	repo.Close()
	return nil
}
