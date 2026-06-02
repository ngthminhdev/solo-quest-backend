package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"solo_quest_backend/internal/database"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "up":
		fmt.Println("Running pending migrations...")
		fmt.Printf("Database: %s\n", sanitizeURL(databaseURL))
		if err := database.RunMigrations(databaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations completed successfully.")

	case "down":
		fmt.Println("Rolling back last migration...")
		fmt.Printf("Database: %s\n", sanitizeURL(databaseURL))
		fmt.Println("WARNING: This may cause data loss. Ensure you have a backup.")
		if err := database.RunMigrationsDown(databaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Rollback completed.")

	case "version":
		version, dirty, err := database.GetMigrationVersion(databaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Current migration version: %d (dirty: %v)\n", version, dirty)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("SoloQuest Database Migration Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/migrate/main.go up       Run all pending migrations")
	fmt.Println("  go run cmd/migrate/main.go down      Roll back last migration (DANGEROUS)")
	fmt.Println("  go run cmd/migrate/main.go version   Show current migration version")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  DATABASE_URL      PostgreSQL connection string (required)")
	fmt.Println("  MIGRATIONS_PATH   Path to migrations directory (default: ./migrations)")
}

// sanitizeURL hides password from connection string for safe logging.
func sanitizeURL(url string) string {
	// Simple sanitization: show host/db but hide credentials
	for i := 0; i < len(url); i++ {
		if url[i] == '@' {
			return "***@" + url[i+1:]
		}
	}
	return url
}
