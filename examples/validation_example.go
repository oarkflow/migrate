package main

import (
	"fmt"
	"log"

	"github.com/oarkflow/migrate"
)

func validationExampleMain() {
	fmt.Println("=== Migration Validation Example ===")

	// Example 1: Validate a good migration
	fmt.Println("\n1. Validating a good migration:")
	goodMigration := migrate.Migration{
		Name:        "create_users_table",
		Version:     "1.0.0",
		Description: "Create users table with proper validation",
		Up: migrate.Operation{
			CreateTable: []migrate.CreateTable{
				{
					Name: "users",
					Columns: []migrate.AddColumn{
						{
							Name:          "id",
							Type:          "integer",
							PrimaryKey:    true,
							AutoIncrement: true,
						},
						{
							Name:   "email",
							Type:   "string",
							Size:   255,
							Unique: true,
						},
						{
							Name: "name",
							Type: "string",
							Size: 100,
						},
						{
							Name:  "age",
							Type:  "integer",
							Check: "age >= 0 AND age <= 150",
						},
						{
							Name:     "created_at",
							Type:     "datetime",
							Default:  "now()",
							Nullable: false,
						},
					},
				},
			},
		},
		Down: migrate.Operation{
			DropTable: []migrate.DropTable{
				{Name: "users", Cascade: true},
			},
		},
	}

	validator := migrate.NewValidator()
	validator.ValidateMigration(goodMigration)

	if validator.HasErrors() {
		fmt.Printf("❌ Validation failed:\n")
		for _, err := range validator.Errors() {
			fmt.Printf("  - %s\n", err.Error())
		}
	} else {
		fmt.Printf("✅ Migration validation passed!\n")
	}

	// Example 2: Validate a bad migration
	fmt.Println("\n2. Validating a bad migration:")
	badMigration := migrate.Migration{
		Name:        "", // Empty name - should fail
		Version:     "", // Empty version - should fail
		Description: "", // Empty description - should fail
		Up: migrate.Operation{
			CreateTable: []migrate.CreateTable{
				{
					Name:    "123invalid",          // Invalid table name - should fail
					Columns: []migrate.AddColumn{}, // No columns - should fail
				},
				{
					Name: "users",
					Columns: []migrate.AddColumn{
						{
							Name:  "user-name",    // Invalid column name - should fail
							Type:  "invalid_type", // Invalid data type - should fail
							Size:  -1,             // Negative size - should fail
							Scale: 10,             // Scale > size - should fail
						},
					},
				},
			},
		},
	}

	validator2 := migrate.NewValidator()
	validator2.ValidateMigration(badMigration)

	if validator2.HasErrors() {
		fmt.Printf("❌ Validation failed (as expected):\n")
		for _, err := range validator2.Errors() {
			fmt.Printf("  - %s\n", err.Error())
		}
	} else {
		fmt.Printf("✅ Migration validation passed (unexpected!)\n")
	}

	// Example 3: Configuration validation with custom rules
	fmt.Println("\n3. Configuration validation with custom rules:")

	config := &migrate.MigrateConfig{
		Database: migrate.DatabaseConfig{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "test_user",
			Password: "test_pass",
			Database: "test_db",
		},
		Migration: migrate.MigrationConfig{
			Directory:      "migrations",
			TableName:      "schema_migrations",
			LockTimeout:    300,
			BatchSize:      100,
			AutoRollback:   false,
			DryRun:         false,
			SkipValidation: false,
		},
		Seed: migrate.SeedingConfig{
			Directory:     "seeds",
			DefaultRows:   10,
			TruncateFirst: false,
			BatchSize:     1000,
		},
		Logging: migrate.LoggingConfig{
			Level:   "info",
			Format:  "text",
			Output:  "console",
			Verbose: false,
		},
		Validation: migrate.ValidationConfig{
			Enabled:            true,
			StrictMode:         true,
			MaxIdentifierLen:   64,
			RequireDescription: true,
			ForbiddenNames:     []string{"temp", "tmp", "test"},
			AllowedDataTypes:   []string{"string", "integer", "boolean", "datetime", "decimal"},
		},
	}

	if err := config.Validate(); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
	} else {
		fmt.Printf("✅ Configuration validation passed!\n")
		fmt.Printf("  DSN: %s\n", config.GetDSN())
	}

	// Example 4: Test identifier validation
	fmt.Println("\n4. Testing identifier validation:")

	testIdentifiers := []struct {
		name     string
		value    string
		expected bool
	}{
		{"valid table name", "users", true},
		{"valid with underscore", "user_profiles", true},
		{"valid with numbers", "table123", true},
		{"empty name", "", false},
		{"starts with number", "123table", false},
		{"contains spaces", "user table", false},
		{"contains hyphens", "user-table", false},
		{"too long", "this_is_a_very_long_table_name_that_exceeds_the_maximum_length_limit_for_identifiers", false},
		{"reserved word", "select", false},
		{"reserved word uppercase", "TABLE", false},
	}

	for _, test := range testIdentifiers {
		validator := migrate.NewValidator()
		validator.ValidateIdentifier("test_field", test.value)

		hasErrors := validator.HasErrors()
		if hasErrors == !test.expected {
			if test.expected {
				fmt.Printf("✅ '%s' (%s): correctly validated as valid\n", test.name, test.value)
			} else {
				fmt.Printf("✅ '%s' (%s): correctly validated as invalid\n", test.name, test.value)
			}
		} else {
			fmt.Printf("❌ '%s' (%s): validation result unexpected\n", test.name, test.value)
		}
	}

	// Example 5: Test data type validation
	fmt.Println("\n5. Testing data type validation:")

	testDataTypes := []struct {
		name     string
		value    string
		expected bool
	}{
		{"string type", "string", true},
		{"integer type", "integer", true},
		{"boolean type", "boolean", true},
		{"datetime type", "datetime", true},
		{"case insensitive", "STRING", true},
		{"empty type", "", false},
		{"invalid type", "invalid_type", false},
		{"typo", "integr", false},
	}

	for _, test := range testDataTypes {
		validator := migrate.NewValidator()
		validator.ValidateDataType("test_field", test.value)

		hasErrors := validator.HasErrors()
		if hasErrors == !test.expected {
			if test.expected {
				fmt.Printf("✅ '%s' (%s): correctly validated as valid\n", test.name, test.value)
			} else {
				fmt.Printf("✅ '%s' (%s): correctly validated as invalid\n", test.name, test.value)
			}
		} else {
			fmt.Printf("❌ '%s' (%s): validation result unexpected\n", test.name, test.value)
		}
	}

	fmt.Println("\n=== Validation Example Complete ===")
}

// Example of using validation in a real migration workflow
func migrationWorkflowExample() {
	fmt.Println("\n=== Migration Workflow with Validation ===")

	// Load configuration
	config, err := migrate.LoadConfig("migrate.json")
	if err != nil {
		log.Printf("Using default config: %v", err)
		config = migrate.DefaultConfig()
		config.Database.Username = "test"
		config.Database.Password = "test"
		config.Database.Database = "test_db"
	}

	// Enable strict validation
	config.Validation.Enabled = true
	config.Validation.StrictMode = true
	config.Validation.RequireDescription = true

	// Create manager with validation enabled
	manager := migrate.NewManager(migrate.WithConfig(config))

	fmt.Printf("Manager created with validation enabled:\n")
	fmt.Printf("  Migration Directory: %s\n", manager.MigrationDir())
	fmt.Printf("  Seed Directory: %s\n", manager.SeedDir())
	fmt.Printf("  Validation Enabled: %t\n", config.Validation.Enabled)
	fmt.Printf("  Strict Mode: %t\n", config.Validation.StrictMode)
	fmt.Printf("  Require Description: %t\n", config.Validation.RequireDescription)
	fmt.Printf("  Max Identifier Length: %d\n", config.Validation.MaxIdentifierLen)

	// Example migration that would be validated
	migration := migrate.Migration{
		Name:        "add_user_preferences",
		Version:     "1.1.0",
		Description: "Add user preferences table with proper constraints",
		Up: migrate.Operation{
			CreateTable: []migrate.CreateTable{
				{
					Name: "user_preferences",
					Columns: []migrate.AddColumn{
						{Name: "id", Type: "integer", PrimaryKey: true, AutoIncrement: true},
						{Name: "user_id", Type: "integer"},
						{Name: "preference_key", Type: "string", Size: 100},
						{Name: "preference_value", Type: "text"},
						{Name: "created_at", Type: "datetime", Default: "now()"},
					},
				},
			},
		},
		Down: migrate.Operation{
			DropTable: []migrate.DropTable{
				{Name: "user_preferences", Cascade: true},
			},
		},
	}

	// Validate the migration
	validator := migrate.NewValidator()
	validator.ValidateMigration(migration)

	if validator.HasErrors() {
		fmt.Printf("❌ Migration validation failed:\n")
		for _, err := range validator.Errors() {
			fmt.Printf("  - %s\n", err.Error())
		}
	} else {
		fmt.Printf("✅ Migration validation passed! Ready to apply.\n")
	}
}
