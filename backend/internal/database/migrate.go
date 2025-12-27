package database

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog/log"
)

// MigrationsFS can be used to embed migrations in the binary
// To use, add this to your main package:
// //go:embed migrations/*.sql
// var migrationsFS embed.FS

// RunMigrations runs all pending migrations using embedded files
func RunMigrations(databaseURL string, migrationsFS embed.FS, migrationsPath string) error {
	d, err := iofs.New(migrationsFS, migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if err == migrate.ErrNilVersion {
		log.Info().Msg("No migrations applied yet")
	} else {
		log.Info().
			Uint("version", version).
			Bool("dirty", dirty).
			Msg("Database migration completed")
	}

	return nil
}

// RunMigrationsFromPath runs migrations from a file path
func RunMigrationsFromPath(databaseURL string, migrationsPath string) error {
	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if err == migrate.ErrNilVersion {
		log.Info().Msg("No migrations applied yet")
	} else {
		log.Info().
			Uint("version", version).
			Bool("dirty", dirty).
			Msg("Database migration completed")
	}

	return nil
}

// GetMigrationVersion returns the current migration version
func GetMigrationVersion(databaseURL string, migrationsPath string) (uint, bool, error) {
	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNilVersion {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}

// RollbackMigration rolls back the last migration
func RollbackMigration(databaseURL string, migrationsPath string, steps int) error {
	sourceURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	log.Info().Int("steps", steps).Msg("Database migration rolled back")
	return nil
}
