package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/oarkflow/migrate"
)

func seedExampleMain() {
	fmt.Println("=== Seed Configuration Example ===")

	// Load configuration
	config, err := migrate.LoadConfig("migrate.json")
	if err != nil {
		log.Printf("Using default config: %v", err)
		config = migrate.DefaultConfig()
		config.Database.Driver = "sqlite"
		config.Database.Database = "example.db"
	}

	// Configure seeding settings
	config.Seed.DefaultRows = 25
	config.Seed.TruncateFirst = true
	config.Seed.BatchSize = 500

	fmt.Printf("Seed Configuration:\n")
	fmt.Printf("  Directory: %s\n", config.Seed.Directory)
	fmt.Printf("  Default Rows: %d\n", config.Seed.DefaultRows)
	fmt.Printf("  Truncate First: %t\n", config.Seed.TruncateFirst)
	fmt.Printf("  Batch Size: %d\n", config.Seed.BatchSize)

	// Create manager with seed configuration
	manager := migrate.NewManager(migrate.WithConfig(config))

	// Create seed directory if it doesn't exist
	seedDir := manager.SeedDir()
	if err := os.MkdirAll(seedDir, 0755); err != nil {
		log.Printf("Failed to create seed directory: %v", err)
		return
	}

	// Example 1: Create a comprehensive user seed file
	fmt.Println("\n1. Creating comprehensive user seed file...")
	userSeedContent := `Seed "comprehensive_users" {
    table = "users"

    Field "id" {
        value = "fake_uuid"
        unique = true
    }

    Field "email" {
        value = "fake_email"
        unique = true
    }

    Field "username" {
        value = "fake_string"
        size = 20
        unique = true
    }

    Field "first_name" {
        value = "fake_firstname"
    }

    Field "last_name" {
        value = "fake_lastname"
    }

    Field "full_name" {
        value = "expr: first_name.value + ' ' + last_name.value"
        data_type = "string"
    }

    Field "age" {
        value = "fake_age"
        data_type = "int"
    }

    Field "is_adult" {
        value = "expr: age.value >= 18 ? true : false"
        data_type = "boolean"
    }

    Field "phone" {
        value = "fake_phone"
    }

    Field "address" {
        value = "fake_address"
    }

    Field "city" {
        value = "fake_city"
    }

    Field "country" {
        value = "fake_country"
    }

    Field "company" {
        value = "fake_company"
    }

    Field "job_title" {
        value = "fake_jobtitle"
    }

    Field "salary" {
        value = "expr: is_adult.value ? fake_uint() * 1000 + 30000 : 0"
        data_type = "int"
    }

    Field "is_active" {
        value = "fake_bool"
        data_type = "boolean"
    }

    Field "registration_date" {
        value = "fake_pastdate"
        data_type = "datetime"
    }

    Field "last_login" {
        value = "fake_datetime"
        data_type = "datetime"
    }

    Field "profile_completion" {
        value = "expr: (first_name.value != '' ? 20 : 0) + (last_name.value != '' ? 20 : 0) + (phone.value != '' ? 20 : 0) + (address.value != '' ? 20 : 0) + (company.value != '' ? 20 : 0)"
        data_type = "int"
    }

    combine = ["email", "username"]
    condition = "if_not_exists"
    rows = ` + fmt.Sprintf("%d", config.Seed.DefaultRows) + `
}`

	userSeedFile := filepath.Join(seedDir, "comprehensive_users.bcl")
	if err := os.WriteFile(userSeedFile, []byte(userSeedContent), 0644); err != nil {
		log.Printf("Failed to create user seed file: %v", err)
		return
	}
	fmt.Printf("✅ Created user seed file: %s\n", userSeedFile)

	// Example 2: Create a product seed file with categories
	fmt.Println("\n2. Creating product seed file with categories...")
	productSeedContent := `Seed "products_with_categories" {
    table = "products"

    Field "id" {
        value = "fake_uuid"
        unique = true
    }

    Field "name" {
        value = "expr: fake_company() + ' ' + fake_string()"
        data_type = "string"
    }

    Field "description" {
        value = "expr: 'High quality ' + name.value + ' for all your needs'"
        data_type = "string"
    }

    Field "category" {
        value = "fake_status"
        data_type = "string"
    }

    Field "price" {
        value = "expr: fake_float64() * 1000 + 10"
        data_type = "decimal"
    }

    Field "discounted_price" {
        value = "expr: price.value * 0.9"
        data_type = "decimal"
    }

    Field "cost" {
        value = "expr: price.value * 0.6"
        data_type = "decimal"
    }

    Field "profit_margin" {
        value = "expr: ((price.value - cost.value) / price.value) * 100"
        data_type = "decimal"
    }

    Field "stock_quantity" {
        value = "fake_uint"
        data_type = "int"
    }

    Field "is_available" {
        value = "expr: stock_quantity.value > 0 ? true : false"
        data_type = "boolean"
    }

    Field "weight" {
        value = "expr: fake_float64() * 10 + 0.1"
        data_type = "decimal"
    }

    Field "dimensions" {
        value = "expr: fake_uint() + 'x' + fake_uint() + 'x' + fake_uint() + ' cm'"
        data_type = "string"
    }

    Field "manufacturer" {
        value = "fake_company"
    }

    Field "country_of_origin" {
        value = "fake_country"
    }

    Field "created_at" {
        value = "fake_pastdate"
        data_type = "datetime"
    }

    Field "updated_at" {
        value = "fake_datetime"
        data_type = "datetime"
    }

    Field "is_featured" {
        value = "expr: profit_margin.value > 30 ? true : false"
        data_type = "boolean"
    }

    combine = ["name", "manufacturer"]
    condition = "if_not_exists"
    rows = ` + fmt.Sprintf("%d", config.Seed.DefaultRows*2) + `
}`

	productSeedFile := filepath.Join(seedDir, "products_with_categories.bcl")
	if err := os.WriteFile(productSeedFile, []byte(productSeedContent), 0644); err != nil {
		log.Printf("Failed to create product seed file: %v", err)
		return
	}
	fmt.Printf("✅ Created product seed file: %s\n", productSeedFile)

	// Example 3: Create an orders seed file with relationships
	fmt.Println("\n3. Creating orders seed file with relationships...")
	orderSeedContent := `Seed "orders_with_relationships" {
    table = "orders"

    Field "id" {
        value = "fake_uuid"
        unique = true
    }

    Field "order_number" {
        value = "expr: 'ORD-' + fake_year() + '-' + fake_uint()"
        unique = true
        data_type = "string"
    }

    Field "customer_email" {
        value = "fake_email"
    }

    Field "customer_name" {
        value = "fake_name"
    }

    Field "customer_phone" {
        value = "fake_phone"
    }

    Field "shipping_address" {
        value = "fake_address"
    }

    Field "billing_address" {
        value = "expr: fake_bool() ? shipping_address.value : fake_address()"
        data_type = "string"
    }

    Field "order_date" {
        value = "fake_pastdate"
        data_type = "datetime"
    }

    Field "item_count" {
        value = "expr: fake_uint() % 10 + 1"
        data_type = "int"
    }

    Field "subtotal" {
        value = "expr: item_count.value * (fake_float64() * 100 + 20)"
        data_type = "decimal"
    }

    Field "tax_rate" {
        value = "expr: fake_float64() * 0.15 + 0.05"
        data_type = "decimal"
    }

    Field "tax_amount" {
        value = "expr: subtotal.value * tax_rate.value"
        data_type = "decimal"
    }

    Field "shipping_cost" {
        value = "expr: subtotal.value > 100 ? 0 : 15.99"
        data_type = "decimal"
    }

    Field "total_amount" {
        value = "expr: subtotal.value + tax_amount.value + shipping_cost.value"
        data_type = "decimal"
    }

    Field "payment_method" {
        value = "expr: fake_bool() ? 'credit_card' : (fake_bool() ? 'paypal' : 'bank_transfer')"
        data_type = "string"
    }

    Field "payment_status" {
        value = "expr: fake_bool() ? 'paid' : (fake_bool() ? 'pending' : 'failed')"
        data_type = "string"
    }

    Field "order_status" {
        value = "expr: payment_status.value == 'paid' ? (fake_bool() ? 'shipped' : 'processing') : 'cancelled'"
        data_type = "string"
    }

    Field "shipped_date" {
        value = "expr: order_status.value == 'shipped' ? fake_datetime() : null"
        data_type = "datetime"
    }

    Field "delivery_date" {
        value = "expr: shipped_date.value != null ? fake_futuredate() : null"
        data_type = "datetime"
    }

    Field "tracking_number" {
        value = "expr: order_status.value == 'shipped' ? 'TRK' + fake_uint() : null"
        data_type = "string"
    }

    Field "notes" {
        value = "expr: fake_bool() ? 'Customer requested ' + (fake_bool() ? 'fast delivery' : 'gift wrapping') : ''"
        data_type = "string"
    }

    combine = ["order_number"]
    condition = "if_not_exists"
    rows = ` + fmt.Sprintf("%d", config.Seed.DefaultRows/2) + `
}`

	orderSeedFile := filepath.Join(seedDir, "orders_with_relationships.bcl")
	if err := os.WriteFile(orderSeedFile, []byte(orderSeedContent), 0644); err != nil {
		log.Printf("Failed to create order seed file: %v", err)
		return
	}
	fmt.Printf("✅ Created order seed file: %s\n", orderSeedFile)

	// Example 4: Show how to run seeds with configuration
	fmt.Println("\n4. Seed execution configuration:")
	fmt.Printf("  Truncate tables before seeding: %t\n", config.Seed.TruncateFirst)
	fmt.Printf("  Batch size for inserts: %d\n", config.Seed.BatchSize)
	fmt.Printf("  Default rows per seed: %d\n", config.Seed.DefaultRows)

	// List all created seed files
	fmt.Println("\n5. Created seed files:")
	seedFiles, err := os.ReadDir(seedDir)
	if err != nil {
		log.Printf("Failed to read seed directory: %v", err)
		return
	}

	for i, file := range seedFiles {
		if filepath.Ext(file.Name()) == ".bcl" {
			fmt.Printf("  %d. %s\n", i+1, file.Name())
		}
	}

	fmt.Println("\n6. Example commands to run seeds:")
	fmt.Printf("  Run all seeds: go run main.go cli db:seed\n")
	fmt.Printf("  Run specific seed: go run main.go cli db:seed --file=%s\n", userSeedFile)
	fmt.Printf("  Run with truncate: go run main.go cli db:seed --truncate=true\n")

	fmt.Println("\n=== Seed Configuration Example Complete ===")
}

// Example of advanced seed configuration
func advancedSeedExample() {
	fmt.Println("\n=== Advanced Seed Configuration ===")

	// Create a configuration with advanced seed settings
	config := &migrate.MigrateConfig{
		Database: migrate.DatabaseConfig{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "test_user",
			Password: "test_pass",
			Database: "test_db",
		},
		Seed: migrate.SeedingConfig{
			Directory:     "advanced_seeds",
			DefaultRows:   100,
			TruncateFirst: true,
			BatchSize:     2000,
		},
		Logging: migrate.LoggingConfig{
			Level:   "debug",
			Verbose: true,
		},
	}

	fmt.Printf("Advanced Seed Configuration:\n")
	fmt.Printf("  Large batch size: %d (for performance)\n", config.Seed.BatchSize)
	fmt.Printf("  High default rows: %d (for realistic data volume)\n", config.Seed.DefaultRows)
	fmt.Printf("  Truncate first: %t (for clean test data)\n", config.Seed.TruncateFirst)
	fmt.Printf("  Debug logging: %t (for detailed output)\n", config.Logging.Verbose)

	// Show how this would be used in practice
	manager := migrate.NewManager(migrate.WithConfig(config))
	fmt.Printf("  Manager seed directory: %s\n", manager.SeedDir())

	fmt.Println("\nThis configuration is optimized for:")
	fmt.Println("  - Large datasets (100+ rows per table)")
	fmt.Println("  - Performance testing (high batch size)")
	fmt.Println("  - Clean test environments (truncate first)")
	fmt.Println("  - Debugging seed issues (verbose logging)")
}
