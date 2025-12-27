package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Configure zerolog for pretty console output
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Parse command line flags
	var (
		command       string
		steps         int
		migrationsDir string
		databaseURL   string
	)

	flag.StringVar(&command, "command", "up", "Migration command: up, down, force, version, drop")
	flag.IntVar(&steps, "steps", 0, "Number of migrations to run (0 = all)")
	flag.StringVar(&migrationsDir, "dir", "migrations", "Path to migrations directory")
	flag.StringVar(&databaseURL, "database", "", "Database URL (overrides DATABASE_URL env)")
	flag.Parse()

	// Get database URL from environment if not provided
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		log.Fatal().Msg("DATABASE_URL environment variable or -database flag is required")
	}

	// Get absolute path to migrations directory
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get absolute path for migrations directory")
	}

	// Create migration source URL
	sourceURL := fmt.Sprintf("file://%s", absPath)

	log.Info().
		Str("source", sourceURL).
		Str("command", command).
		Int("steps", steps).
		Msg("Starting migration")

	// Create migrate instance
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create migrate instance")
	}
	defer m.Close()

	// Execute command
	switch command {
	case "up":
		err = runUp(m, steps)
	case "down":
		err = runDown(m, steps)
	case "force":
		if steps == 0 {
			log.Fatal().Msg("Force command requires -steps flag with version number")
		}
		err = m.Force(steps)
	case "version":
		version, dirty, verr := m.Version()
		if verr != nil {
			if verr == migrate.ErrNilVersion {
				log.Info().Msg("No migrations have been applied yet")
				return
			}
			log.Fatal().Err(verr).Msg("Failed to get version")
		}
		log.Info().
			Uint("version", version).
			Bool("dirty", dirty).
			Msg("Current migration version")
		return
	case "drop":
		err = m.Drop()
	default:
		log.Fatal().Str("command", command).Msg("Unknown command")
	}

	if err != nil {
		if err == migrate.ErrNoChange {
			log.Info().Msg("No migrations to apply")
			return
		}
		log.Fatal().Err(err).Msg("Migration failed")
	}

	log.Info().Msg("Migration completed successfully")
}

func runUp(m *migrate.Migrate, steps int) error {
	if steps > 0 {
		return m.Steps(steps)
	}
	return m.Up()
}

func runDown(m *migrate.Migrate, steps int) error {
	if steps > 0 {
		return m.Steps(-steps)
	}
	return m.Down()
}
