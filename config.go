package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oarkflow/json"
)

// MigrateConfig represents the configuration for the migration system
type MigrateConfig struct {
	// Database configuration
	Database DatabaseConfig `json:"database"`

	// Migration settings
	Migration MigrationConfig `json:"migration"`

	// Seed settings
	Seed SeedingConfig `json:"seed"`

	// Logging settings
	Logging LoggingConfig `json:"logging"`

	// Validation settings
	Validation ValidationConfig `json:"validation"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode,omitempty"`
	Charset  string `json:"charset,omitempty"`
	Timeout  int    `json:"timeout,omitempty"`
}

// MigrationConfig holds migration-specific settings
type MigrationConfig struct {
	Directory      string `json:"directory"`
	TableName      string `json:"table_name"`
	LockTimeout    int    `json:"lock_timeout"`
	BatchSize      int    `json:"batch_size"`
	AutoRollback   bool   `json:"auto_rollback"`
	DryRun         bool   `json:"dry_run"`
	SkipValidation bool   `json:"skip_validation"`
}

// SeedingConfig holds seeding-specific settings
type SeedingConfig struct {
	Directory     string `json:"directory"`
	DefaultRows   int    `json:"default_rows"`
	TruncateFirst bool   `json:"truncate_first"`
	BatchSize     int    `json:"batch_size"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level   string `json:"level"`
	Format  string `json:"format"`
	Output  string `json:"output"`
	Verbose bool   `json:"verbose"`
	LogFile string `json:"log_file,omitempty"`
}

// ValidationConfig holds validation settings
type ValidationConfig struct {
	Enabled            bool     `json:"enabled"`
	StrictMode         bool     `json:"strict_mode"`
	AllowedDataTypes   []string `json:"allowed_data_types,omitempty"`
	ForbiddenNames     []string `json:"forbidden_names,omitempty"`
	MaxIdentifierLen   int      `json:"max_identifier_length"`
	RequireDescription bool     `json:"require_description"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *MigrateConfig {
	return &MigrateConfig{
		Database: DatabaseConfig{
			Driver:  "postgres",
			Host:    "localhost",
			Port:    5432,
			Timeout: 30,
		},
		Migration: MigrationConfig{
			Directory:      "migrations",
			TableName:      "migrations",
			LockTimeout:    300, // 5 minutes
			BatchSize:      100,
			AutoRollback:   false,
			DryRun:         false,
			SkipValidation: false,
		},
		Seed: SeedingConfig{
			Directory:     "migrations/seeds",
			DefaultRows:   10,
			TruncateFirst: false,
			BatchSize:     1000,
		},
		Logging: LoggingConfig{
			Level:   "info",
			Format:  "text",
			Output:  "console",
			Verbose: false,
		},
		Validation: ValidationConfig{
			Enabled:            true,
			StrictMode:         false,
			MaxIdentifierLen:   64,
			RequireDescription: true,
		},
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(configPath string) (*MigrateConfig, error) {
	// Start with default config
	config := DefaultConfig()

	// If no config file specified, try to load a default config file (./migrate.json)
	if configPath == "" {
		if _, err := os.Stat("migrate.json"); err == nil {
			configPath = "migrate.json"
		} else {
			return config, nil
		}
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse based on file extension
	ext := strings.ToLower(filepath.Ext(configPath))
	switch ext {
	case ".json":
		err = json.Unmarshal(data, config)
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to a file
func (c *MigrateConfig) SaveConfig(configPath string) error {
	// Validate configuration before saving
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *MigrateConfig) Validate() error {
	validator := NewValidator()

	// Validate database config
	if c.Database.Driver == "" {
		validator.AddError("database.driver", c.Database.Driver, "driver cannot be empty")
	} else {
		validDrivers := []string{"postgres", "mysql", "sqlite"}
		valid := false
		for _, driver := range validDrivers {
			if c.Database.Driver == driver {
				valid = true
				break
			}
		}
		if !valid {
			validator.AddError("database.driver", c.Database.Driver, "unsupported database driver")
		}
	}

	if c.Database.Host == "" && c.Database.Driver != "sqlite" {
		validator.AddError("database.host", c.Database.Host, "host cannot be empty for non-sqlite databases")
	}

	if c.Database.Port <= 0 && c.Database.Driver != "sqlite" {
		validator.AddError("database.port", fmt.Sprintf("%d", c.Database.Port), "port must be positive for non-sqlite databases")
	}

	if c.Database.Database == "" {
		validator.AddError("database.database", c.Database.Database, "database name cannot be empty")
	}

	// Validate migration config
	if c.Migration.Directory == "" {
		validator.AddError("migration.directory", c.Migration.Directory, "migration directory cannot be empty")
	}

	if c.Migration.TableName == "" {
		validator.AddError("migration.table_name", c.Migration.TableName, "migration table name cannot be empty")
	} else {
		validator.ValidateIdentifier("migration.table_name", c.Migration.TableName)
	}

	if c.Migration.LockTimeout <= 0 {
		validator.AddError("migration.lock_timeout", fmt.Sprintf("%d", c.Migration.LockTimeout), "lock timeout must be positive")
	}

	if c.Migration.BatchSize <= 0 {
		validator.AddError("migration.batch_size", fmt.Sprintf("%d", c.Migration.BatchSize), "batch size must be positive")
	}

	// Validate seed config
	if c.Seed.Directory == "" {
		validator.AddError("seed.directory", c.Seed.Directory, "seed directory cannot be empty")
	}

	if c.Seed.DefaultRows <= 0 {
		validator.AddError("seed.default_rows", fmt.Sprintf("%d", c.Seed.DefaultRows), "default rows must be positive")
	}

	if c.Seed.BatchSize <= 0 {
		validator.AddError("seed.batch_size", fmt.Sprintf("%d", c.Seed.BatchSize), "batch size must be positive")
	}

	// Validate logging config
	validLevels := []string{"debug", "info", "warn", "error"}
	valid := false
	for _, level := range validLevels {
		if c.Logging.Level == level {
			valid = true
			break
		}
	}
	if !valid {
		validator.AddError("logging.level", c.Logging.Level, "invalid log level")
	}

	validFormats := []string{"text", "json"}
	valid = false
	for _, format := range validFormats {
		if c.Logging.Format == format {
			valid = true
			break
		}
	}
	if !valid {
		validator.AddError("logging.format", c.Logging.Format, "invalid log format")
	}

	validOutputs := []string{"console", "file", "both"}
	valid = false
	for _, output := range validOutputs {
		if c.Logging.Output == output {
			valid = true
			break
		}
	}
	if !valid {
		validator.AddError("logging.output", c.Logging.Output, "invalid log output")
	}

	// Validate validation config
	if c.Validation.MaxIdentifierLen <= 0 {
		validator.AddError("validation.max_identifier_length", fmt.Sprintf("%d", c.Validation.MaxIdentifierLen), "max identifier length must be positive")
	}

	// Validate forbidden names
	for i, name := range c.Validation.ForbiddenNames {
		if name == "" {
			validator.AddError(fmt.Sprintf("validation.forbidden_names[%d]", i), name, "forbidden name cannot be empty")
		}
	}

	return validator.Error()
}

// GetDSN returns the database connection string
func (c *MigrateConfig) GetDSN() string {
	switch c.Database.Driver {
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s",
			c.Database.Host, c.Database.Port, c.Database.Username, c.Database.Database)

		if c.Database.Password != "" {
			dsn += fmt.Sprintf(" password=%s", c.Database.Password)
		}

		if c.Database.SSLMode != "" {
			dsn += fmt.Sprintf(" sslmode=%s", c.Database.SSLMode)
		} else {
			dsn += " sslmode=disable"
		}

		return dsn

	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			c.Database.Username, c.Database.Password,
			c.Database.Host, c.Database.Port, c.Database.Database)

		if c.Database.Charset != "" {
			dsn += fmt.Sprintf("?charset=%s", c.Database.Charset)
		} else {
			dsn += "?charset=utf8mb4"
		}

		return dsn

	case "sqlite":
		return c.Database.Database

	default:
		return ""
	}
}

// ApplyEnvironmentOverrides applies environment variable overrides
func (c *MigrateConfig) ApplyEnvironmentOverrides() {
	if host := os.Getenv("MIGRATE_DB_HOST"); host != "" {
		c.Database.Host = host
	}

	if port := os.Getenv("MIGRATE_DB_PORT"); port != "" {
		if p, err := parsePort(port); err == nil {
			c.Database.Port = p
		}
	}

	if username := os.Getenv("MIGRATE_DB_USERNAME"); username != "" {
		c.Database.Username = username
	}

	if password := os.Getenv("MIGRATE_DB_PASSWORD"); password != "" {
		c.Database.Password = password
	}

	if database := os.Getenv("MIGRATE_DB_DATABASE"); database != "" {
		c.Database.Database = database
	}

	if driver := os.Getenv("MIGRATE_DB_DRIVER"); driver != "" {
		c.Database.Driver = driver
	}

	if migrationDir := os.Getenv("MIGRATE_MIGRATION_DIR"); migrationDir != "" {
		c.Migration.Directory = migrationDir
	}

	if seedDir := os.Getenv("MIGRATE_SEED_DIR"); seedDir != "" {
		c.Seed.Directory = seedDir
	}

	if logLevel := os.Getenv("MIGRATE_LOG_LEVEL"); logLevel != "" {
		c.Logging.Level = logLevel
	}

	if verbose := os.Getenv("MIGRATE_VERBOSE"); verbose == "true" || verbose == "1" {
		c.Logging.Verbose = true
	}
}

// parsePort parses a port string to integer
func parsePort(portStr string) (int, error) {
	var port int
	_, err := fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		return 0, err
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port number: %d", port)
	}
	return port, nil
}

// CreateSampleConfig creates a sample configuration file
func CreateSampleConfig(configPath string) error {
	config := DefaultConfig()

	// Add some sample values
	config.Database.Username = "your_username"
	config.Database.Password = "your_password"
	config.Database.Database = "your_database"

	// Add comments as a separate JSON structure for documentation
	sample := map[string]interface{}{
		"_comment": "Sample configuration file for migrate tool",
		"database": map[string]interface{}{
			"_comment": "Database connection settings",
			"driver":   config.Database.Driver,
			"host":     config.Database.Host,
			"port":     config.Database.Port,
			"username": config.Database.Username,
			"password": config.Database.Password,
			"database": config.Database.Database,
			"ssl_mode": "disable",
			"charset":  "utf8mb4",
			"timeout":  config.Database.Timeout,
		},
		"migration": map[string]interface{}{
			"_comment":        "Migration settings",
			"directory":       config.Migration.Directory,
			"table_name":      config.Migration.TableName,
			"lock_timeout":    config.Migration.LockTimeout,
			"batch_size":      config.Migration.BatchSize,
			"auto_rollback":   config.Migration.AutoRollback,
			"dry_run":         config.Migration.DryRun,
			"skip_validation": config.Migration.SkipValidation,
		},
		"seed": map[string]interface{}{
			"_comment":       "Seed settings",
			"directory":      config.Seed.Directory,
			"default_rows":   config.Seed.DefaultRows,
			"truncate_first": config.Seed.TruncateFirst,
			"batch_size":     config.Seed.BatchSize,
		},
		"logging": map[string]interface{}{
			"_comment": "Logging settings",
			"level":    config.Logging.Level,
			"format":   config.Logging.Format,
			"output":   config.Logging.Output,
			"verbose":  config.Logging.Verbose,
			"log_file": "/path/to/migrate.log",
		},
		"validation": map[string]interface{}{
			"_comment":              "Validation settings",
			"enabled":               config.Validation.Enabled,
			"strict_mode":           config.Validation.StrictMode,
			"max_identifier_length": config.Validation.MaxIdentifierLen,
			"require_description":   config.Validation.RequireDescription,
			"forbidden_names":       []string{"temp", "tmp", "test"},
		},
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(sample, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sample config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sample config file: %w", err)
	}

	return nil
}
