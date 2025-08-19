package main

import (
	"fmt"
	"log"

	"github.com/oarkflow/migrate"
)

func main() {
	// Example using the new configuration system
	fmt.Println("Starting migration tool with configuration...")

	// Try to load configuration from file
	manager, err := migrate.NewManagerFromConfig("migrate.json")
	if err != nil {
		log.Printf("Failed to load from config file, using defaults: %v", err)

		// Fallback to default configuration
		config := migrate.DefaultConfig()
		fmt.Println(config.Database)
		config.Database.Username = "postgres"
		config.Database.Password = "postgres"
		config.Database.Database = "sujit"

		manager = migrate.NewManager(migrate.WithConfig(config))
	}

	fmt.Printf("Manager configured with:\n")
	fmt.Printf("  Migration Directory: %s\n", manager.MigrationDir())
	fmt.Printf("  Seed Directory: %s\n", manager.SeedDir())
	fmt.Printf("  Dialect: %s\n", manager.GetDialect())

	// Run the CLI
	manager.Run()
}
