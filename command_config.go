package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oarkflow/cli/contracts"
	"github.com/oarkflow/json"
)

// ConfigCommand handles configuration management
type ConfigCommand struct {
	Driver IManager
}

func (c *ConfigCommand) Signature() string {
	return "config"
}

func (c *ConfigCommand) Description() string {
	return "Manage migration configuration"
}

func (c *ConfigCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{},
	}
}

func (c *ConfigCommand) Handle(ctx contracts.Context) error {
	return fmt.Errorf("please use config:init, config:validate, or config:show commands")
}

// ConfigInitCommand initializes a new configuration file
type ConfigInitCommand struct {
	Driver IManager
}

func (c *ConfigInitCommand) Signature() string {
	return "init"
}

func (c *ConfigInitCommand) Description() string {
	return "Initialize a new configuration file"
}

func (c *ConfigInitCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "path",
				Aliases: []string{"p"},
				Usage:   "Path to configuration file",
				Value:   "migrate.json",
			},
			{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite existing configuration file",
				Value:   "false",
			},
		},
	}
}

func (c *ConfigInitCommand) Handle(ctx contracts.Context) error {
	configPath := ctx.Option("path")
	if configPath == "" {
		configPath = "migrate.json"
	}

	force := ctx.Option("force") == "true"

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
	}

	// Create sample configuration
	if err := CreateSampleConfig(configPath); err != nil {
		return fmt.Errorf("failed to create configuration file: %w", err)
	}

	logger.Info().Msgf("Configuration file created: %s", configPath)
	logger.Info().Msg("Please edit the configuration file with your database settings")

	return nil
}

// ConfigValidateCommand validates a configuration file
type ConfigValidateCommand struct {
	Driver IManager
}

func (c *ConfigValidateCommand) Signature() string {
	return "config:validate"
}

func (c *ConfigValidateCommand) Description() string {
	return "Validate configuration file"
}

func (c *ConfigValidateCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "path",
				Aliases: []string{"p"},
				Usage:   "Path to configuration file",
				Value:   "migrate.json",
			},
		},
	}
}

func (c *ConfigValidateCommand) Handle(ctx contracts.Context) error {
	configPath := ctx.Option("path")
	if configPath == "" {
		configPath = "migrate.json"
	}

	// Load and validate configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	logger.Info().Msgf("Configuration file %s is valid", configPath)
	logger.Info().Msgf("Database driver: %s", config.Database.Driver)
	logger.Info().Msgf("Migration directory: %s", config.Migration.Directory)
	logger.Info().Msgf("Seed directory: %s", config.Seed.Directory)

	return nil
}

// ConfigShowCommand displays current configuration
type ConfigShowCommand struct {
	Driver IManager
}

func (c *ConfigShowCommand) Signature() string {
	return "config:show"
}

func (c *ConfigShowCommand) Description() string {
	return "Show current configuration"
}

func (c *ConfigShowCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "path",
				Aliases: []string{"p"},
				Usage:   "Path to configuration file",
				Value:   "migrate.json",
			},
			{
				Name:    "format",
				Aliases: []string{"f"},
				Usage:   "Output format (json, table)",
				Value:   "table",
			},
		},
	}
}

func (c *ConfigShowCommand) Handle(ctx contracts.Context) error {
	configPath := ctx.Option("path")
	if configPath == "" {
		configPath = "migrate.json"
	}

	format := ctx.Option("format")
	if format == "" {
		format = "table"
	}

	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply environment overrides
	config.ApplyEnvironmentOverrides()

	switch format {
	case "json":
		return c.showJSON(config)
	case "table":
		return c.showTable(config)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func (c *ConfigShowCommand) showJSON(config *MigrateConfig) error {
	// Use json marshal directly since SaveConfig writes to file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func (c *ConfigShowCommand) showTable(config *MigrateConfig) error {
	fmt.Println("=== Migration Configuration ===")
	fmt.Println()

	fmt.Println("Database:")
	fmt.Printf("  Driver:   %s\n", config.Database.Driver)
	fmt.Printf("  Host:     %s\n", config.Database.Host)
	fmt.Printf("  Port:     %d\n", config.Database.Port)
	fmt.Printf("  Username: %s\n", config.Database.Username)
	fmt.Printf("  Database: %s\n", config.Database.Database)
	fmt.Printf("  Timeout:  %d seconds\n", config.Database.Timeout)
	fmt.Println()

	fmt.Println("Migration:")
	fmt.Printf("  Directory:       %s\n", config.Migration.Directory)
	fmt.Printf("  Table Name:      %s\n", config.Migration.TableName)
	fmt.Printf("  Lock Timeout:    %d seconds\n", config.Migration.LockTimeout)
	fmt.Printf("  Batch Size:      %d\n", config.Migration.BatchSize)
	fmt.Printf("  Auto Rollback:   %t\n", config.Migration.AutoRollback)
	fmt.Printf("  Dry Run:         %t\n", config.Migration.DryRun)
	fmt.Printf("  Skip Validation: %t\n", config.Migration.SkipValidation)
	fmt.Println()

	fmt.Println("Seed:")
	fmt.Printf("  Directory:       %s\n", config.Seed.Directory)
	fmt.Printf("  Default Rows:    %d\n", config.Seed.DefaultRows)
	fmt.Printf("  Truncate First:  %t\n", config.Seed.TruncateFirst)
	fmt.Printf("  Batch Size:      %d\n", config.Seed.BatchSize)
	fmt.Println()

	fmt.Println("Logging:")
	fmt.Printf("  Level:   %s\n", config.Logging.Level)
	fmt.Printf("  Format:  %s\n", config.Logging.Format)
	fmt.Printf("  Output:  %s\n", config.Logging.Output)
	fmt.Printf("  Verbose: %t\n", config.Logging.Verbose)
	if config.Logging.LogFile != "" {
		fmt.Printf("  Log File: %s\n", config.Logging.LogFile)
	}
	fmt.Println()

	fmt.Println("Validation:")
	fmt.Printf("  Enabled:               %t\n", config.Validation.Enabled)
	fmt.Printf("  Strict Mode:           %t\n", config.Validation.StrictMode)
	fmt.Printf("  Max Identifier Length: %d\n", config.Validation.MaxIdentifierLen)
	fmt.Printf("  Require Description:   %t\n", config.Validation.RequireDescription)
	if len(config.Validation.ForbiddenNames) > 0 {
		fmt.Printf("  Forbidden Names:       %v\n", config.Validation.ForbiddenNames)
	}
	if len(config.Validation.AllowedDataTypes) > 0 {
		fmt.Printf("  Allowed Data Types:    %v\n", config.Validation.AllowedDataTypes)
	}

	return nil
}

// StatusCommand shows migration status
type StatusCommand struct {
	Driver IManager
}

func (c *StatusCommand) Signature() string {
	return "status"
}

func (c *StatusCommand) Description() string {
	return "Show migration status"
}

func (c *StatusCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Show detailed status",
				Value:   "false",
			},
		},
	}
}

func (c *StatusCommand) Handle(ctx contracts.Context) error {
	verbose := ctx.Option("verbose") == "true"

	// Get migration files
	files, err := os.ReadDir(c.Driver.MigrationDir())
	if err != nil {
		return fmt.Errorf("failed to read migration directory: %w", err)
	}

	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".bcl" {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}

	// Get applied migrations
	if err := c.Driver.ValidateHistoryStorage(); err != nil {
		return fmt.Errorf("failed to validate history storage: %w", err)
	}

	fmt.Printf("Migration Status\n")
	fmt.Printf("================\n\n")

	fmt.Printf("Migration Directory: %s\n", c.Driver.MigrationDir())
	fmt.Printf("Total Migration Files: %d\n", len(migrationFiles))

	if verbose {
		fmt.Printf("\nMigration Files:\n")
		for i, file := range migrationFiles {
			fmt.Printf("  %d. %s\n", i+1, file)
		}
	}

	return nil
}
