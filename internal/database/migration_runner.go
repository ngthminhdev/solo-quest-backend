package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	pkglogger "solo_quest_backend/pkg/logger"
	"go.uber.org/zap"
)

// MigrationsPath is the path to the migrations directory.
// Can be overridden via MIGRATIONS_PATH env var.
var MigrationsPath = getMigrationsPath()

func getMigrationsPath() string {
	if p := os.Getenv("MIGRATIONS_PATH"); p != "" {
		return p
	}
	return "./migrations"
}

// RunMigrations runs all pending SQL migrations against the given database URL.
// This is the primary schema migration mechanism. SQL migrations are the source of truth.
func RunMigrations(databaseURL string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database for migrations: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres migration driver: %w", err)
	}

	absPath, err := filepath.Abs(MigrationsPath)
	if err != nil {
		return fmt.Errorf("failed to resolve migrations path: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+absPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		pkglogger.L.Warn("could not read migration version", zap.Error(err))
	} else {
		pkglogger.L.Info("migrations completed",
			zap.Uint("version", version),
			zap.Bool("dirty", dirty),
		)
	}

	return nil
}

// RunMigrationsToVersion runs SQL migrations up to the target version against the given database URL.
func RunMigrationsToVersion(databaseURL string, targetVersion uint) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres migration driver: %w", err)
	}

	absPath, err := filepath.Abs(MigrationsPath)
	if err != nil {
		return fmt.Errorf("failed to resolve migrations path: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+absPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Migrate(targetVersion); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations to version %d: %w", targetVersion, err)
	}

	return nil
}

// RunMigrationsDown rolls back one migration step.
// WARNING: Do not run on dev/prod without explicit intent.
func RunMigrationsDown(databaseURL string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres migration driver: %w", err)
	}

	absPath, err := filepath.Abs(MigrationsPath)
	if err != nil {
		return fmt.Errorf("failed to resolve migrations path: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+absPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("failed to roll back migration: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		pkglogger.L.Warn("could not read migration version after rollback", zap.Error(err))
	} else {
		pkglogger.L.Info("migration rolled back",
			zap.Uint("version", version),
			zap.Bool("dirty", dirty),
		)
	}

	return nil
}

// GetMigrationVersion returns the current migration version and dirty state.
func GetMigrationVersion(databaseURL string) (uint, bool, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return 0, false, fmt.Errorf("failed to open database for version check: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return 0, false, fmt.Errorf("failed to create postgres migration driver: %w", err)
	}

	absPath, err := filepath.Abs(MigrationsPath)
	if err != nil {
		return 0, false, fmt.Errorf("failed to resolve migrations path: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+absPath, "postgres", driver)
	if err != nil {
		return 0, false, fmt.Errorf("failed to initialize migrate instance: %w", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		return 0, false, err
	}

	return version, dirty, nil
}
