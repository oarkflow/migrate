package main

import (
	"fmt"
	"log"
	"os"

	"github.com/oarkflow/migrate"
)

func mai1n() {
	// Example 1: Using configuration file
	fmt.Println("=== Example 1: Using Configuration File ===")
	configExample()

	fmt.Println("\n=== Example 2: Environment Variable Override ===")
	envExample()

	fmt.Println("\n=== Example 3: Programmatic Configuration ===")
	programmaticExample()

	fmt.Println("\n=== Example 4: Configuration Validation ===")
	validationExample()
}

// configExample demonstrates loading configuration from file
func configExample() {
	// Create a manager using configuration file
	manager, err := migrate.NewManagerFromConfig("migrate.json")
	if err != nil {
		log.Printf("Failed to create manager from config: %v", err)
		return
	}

	fmt.Printf("Manager created with config:\n")
	fmt.Printf("  Migration Directory: %s\n", manager.MigrationDir())
	fmt.Printf("  Seed Directory: %s\n", manager.SeedDir())
	fmt.Printf("  Dialect: %s\n", manager.GetDialect())
}

// envExample demonstrates environment variable overrides
func envExample() {
	// Set environment variables
	os.Setenv("MIGRATE_DB_HOST", "production-db.example.com")
	os.Setenv("MIGRATE_DB_PORT", "5432")
	os.Setenv("MIGRATE_DB_USERNAME", "prod_user")
	os.Setenv("MIGRATE_DB_DATABASE", "production_db")
	os.Setenv("MIGRATE_VERBOSE", "true")
	os.Setenv("MIGRATE_LOG_LEVEL", "debug")

	// Load configuration with environment overrides
	config, err := migrate.LoadConfig("migrate.json")
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}

	// Apply environment overrides
	config.ApplyEnvironmentOverrides()

	fmt.Printf("Configuration with environment overrides:\n")
	fmt.Printf("  Database Host: %s\n", config.Database.Host)
	fmt.Printf("  Database Port: %d\n", config.Database.Port)
	fmt.Printf("  Database Username: %s\n", config.Database.Username)
	fmt.Printf("  Database Name: %s\n", config.Database.Database)
	fmt.Printf("  Verbose Logging: %t\n", config.Logging.Verbose)
	fmt.Printf("  Log Level: %s\n", config.Logging.Level)

	// Clean up environment variables
	os.Unsetenv("MIGRATE_DB_HOST")
	os.Unsetenv("MIGRATE_DB_PORT")
	os.Unsetenv("MIGRATE_DB_USERNAME")
	os.Unsetenv("MIGRATE_DB_DATABASE")
	os.Unsetenv("MIGRATE_VERBOSE")
	os.Unsetenv("MIGRATE_LOG_LEVEL")
}

// programmaticExample demonstrates creating configuration programmatically
func programmaticExample() {
	// Create configuration programmatically
	config := &migrate.MigrateConfig{
		Database: migrate.DatabaseConfig{
			Driver:   "sqlite",
			Database: "example.db",
			Timeout:  30,
		},
		Migration: migrate.MigrationConfig{
			Directory:      "custom_migrations",
			TableName:      "schema_versions",
			LockTimeout:    600,
			BatchSize:      50,
			AutoRollback:   true,
			DryRun:         false,
			SkipValidation: false,
		},
		Seed: migrate.SeedingConfig{
			Directory:     "custom_seeds",
			DefaultRows:   25,
			TruncateFirst: true,
			BatchSize:     500,
		},
		Logging: migrate.LoggingConfig{
			Level:   "debug",
			Format:  "json",
			Output:  "file",
			Verbose: true,
			LogFile: "migration.log",
		},
		Validation: migrate.ValidationConfig{
			Enabled:            true,
			StrictMode:         true,
			MaxIdentifierLen:   50,
			RequireDescription: true,
			ForbiddenNames:     []string{"temp", "tmp", "test", "debug"},
			AllowedDataTypes:   []string{"string", "integer", "boolean", "datetime"},
		},
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		log.Printf("Configuration validation failed: %v", err)
		return
	}

	fmt.Printf("Programmatic configuration created:\n")
	fmt.Printf("  Database: %s (%s)\n", config.Database.Database, config.Database.Driver)
	fmt.Printf("  Migration Directory: %s\n", config.Migration.Directory)
	fmt.Printf("  Migration Table: %s\n", config.Migration.TableName)
	fmt.Printf("  Lock Timeout: %d seconds\n", config.Migration.LockTimeout)
	fmt.Printf("  Batch Size: %d\n", config.Migration.BatchSize)
	fmt.Printf("  Auto Rollback: %t\n", config.Migration.AutoRollback)
	fmt.Printf("  Seed Directory: %s\n", config.Seed.Directory)
	fmt.Printf("  Default Seed Rows: %d\n", config.Seed.DefaultRows)
	fmt.Printf("  Truncate First: %t\n", config.Seed.TruncateFirst)
	fmt.Printf("  Log Level: %s\n", config.Logging.Level)
	fmt.Printf("  Log Format: %s\n", config.Logging.Format)
	fmt.Printf("  Validation Enabled: %t\n", config.Validation.Enabled)
	fmt.Printf("  Strict Mode: %t\n", config.Validation.StrictMode)
	fmt.Printf("  Max Identifier Length: %d\n", config.Validation.MaxIdentifierLen)
	fmt.Printf("  Forbidden Names: %v\n", config.Validation.ForbiddenNames)
	fmt.Printf("  Allowed Data Types: %v\n", config.Validation.AllowedDataTypes)

	// Create manager with this configuration
	manager := migrate.NewManager(migrate.WithConfig(config))
	fmt.Printf("  Manager Dialect: %s\n", manager.GetDialect())
	fmt.Printf("  Manager Migration Dir: %s\n", manager.MigrationDir())
	fmt.Printf("  Manager Seed Dir: %s\n", manager.SeedDir())
}

// validationExample demonstrates configuration validation
func validationExample() {
	// Create an invalid configuration
	invalidConfig := &migrate.MigrateConfig{
		Database: migrate.DatabaseConfig{
			Driver:   "invalid_driver",
			Host:     "",
			Port:     -1,
			Database: "",
		},
		Migration: migrate.MigrationConfig{
			Directory:   "",
			TableName:   "123invalid",
			LockTimeout: -1,
			BatchSize:   0,
		},
		Seed: migrate.SeedingConfig{
			Directory:   "",
			DefaultRows: -5,
			BatchSize:   0,
		},
		Logging: migrate.LoggingConfig{
			Level:  "invalid_level",
			Format: "invalid_format",
			Output: "invalid_output",
		},
		Validation: migrate.ValidationConfig{
			MaxIdentifierLen: -1,
			ForbiddenNames:   []string{"", "valid_name"},
		},
	}

	// Try to validate
	err := invalidConfig.Validate()
	if err != nil {
		fmt.Printf("Configuration validation failed (as expected):\n%v\n", err)
	}

	// Create a valid configuration
	validConfig := migrate.DefaultConfig()
	validConfig.Database.Username = "test_user"
	validConfig.Database.Password = "test_pass"
	validConfig.Database.Database = "test_db"

	err = validConfig.Validate()
	if err != nil {
		fmt.Printf("Valid configuration failed validation: %v\n", err)
	} else {
		fmt.Printf("Valid configuration passed validation ✓\n")
		fmt.Printf("  DSN: %s\n", validConfig.GetDSN())
	}
}
