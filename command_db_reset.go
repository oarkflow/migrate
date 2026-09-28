package migrate

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/oarkflow/cli/contracts"
)

// ResetDatabaseCommand deletes and recreates the configured database.
type ResetDatabaseCommand struct {
	Driver IManager
}

func (c *ResetDatabaseCommand) Signature() string {
	return "db:reset"
}

func (c *ResetDatabaseCommand) Description() string {
	return "Deletes and recreates the configured database (destructive)."
}

func (c *ResetDatabaseCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to configuration file",
				Value:   "",
			},
			{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Skip confirmation prompt",
				Value:   "false",
			},
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
		},
	}
}

func (c *ResetDatabaseCommand) Handle(ctx contracts.Context) error {
	// contracts.Context is the CLI framework's own type; the cancellable
	// context.Context comes from the manager (set by Manager.Run).
	runCtx := c.Driver.Context()
	configPath := ctx.Option("config")
	force := ctx.Option("force") == "true" || ctx.Option("force") == "1"
	verbose := ctx.Option("verbose") == "true" || ctx.Option("verbose") == "1"

	// If no config path from CLI, use the one from Manager (set via --config at startup)
	if configPath == "" {
		if mgr, ok := c.Driver.(*Manager); ok {
			configPath = mgr.ConfigPath()
		}
	}

	// Load config (or defaults) and apply env overrides
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg.ApplyEnvironmentOverrides()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if verbose {
		if mgr, ok := c.Driver.(*Manager); ok {
			mgr.Verbose = true
		}
	}

	// Warning & confirmation
	switch cfg.Database.Driver {
	case "sqlite":
		logger.Warn().Msgf("WARNING: This will permanently delete the sqlite database file at '%s'. All data will be lost.", cfg.Database.Database)
	default:
		logger.Warn().Msgf("WARNING: This will permanently DROP and RECREATE the database '%s' on %s:%d. All data will be lost.", cfg.Database.Database, cfg.Database.Host, cfg.Database.Port)
	}

	if !force {
		fmt.Printf("Type 'yes' to continue: ")
		r := bufio.NewReader(os.Stdin)
		resp, _ := r.ReadString('\n')
		resp = strings.TrimSpace(resp)
		if strings.ToLower(resp) != "yes" {
			logger.Info().Msg("Aborted.")
			return nil
		}
	}

	switch cfg.Database.Driver {
	case "postgres":
		return resetPostgres(runCtx, cfg)
	case "mysql":
		return resetMySQL(runCtx, cfg)
	case "sqlite":
		return resetSQLite(runCtx, cfg)
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}
}

func resetPostgres(ctx context.Context, cfg *MigrateConfig) error {
	// Connect to postgres DB to run drop/create
	admin := *cfg
	admin.Database.Database = "postgres"
	dsn := admin.GetDSN()
	connectCtx, cancel := cfg.ConnectContext(ctx)
	defer cancel()
	driver, err := NewDriver(connectCtx, "postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres for admin operations: %w", err)
	}
	name := strings.ReplaceAll(cfg.Database.Database, "\"", "\"\"")
	// Terminate existing connections to the target database so DROP can succeed
	terminate := fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid();", strings.ReplaceAll(cfg.Database.Database, "'", "''"))
	if err := driver.ApplySQL(ctx, []string{terminate}); err != nil {
		// Log as warning and continue; DROP may still fail if other connections remain
		logger.Warn().Msgf("failed to terminate existing connections to '%s': %v", cfg.Database.Database, err)
	}
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\";", name)
	// Create database from template0 to avoid inheriting objects from template1
	create := fmt.Sprintf("CREATE DATABASE \"%s\" TEMPLATE template0;", name)
	logger.Info().Msgf("Dropping database '%s'...", cfg.Database.Database)
	if err := driver.ApplySQL(ctx, []string{drop}); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}
	logger.Info().Msgf("Creating database '%s'...", cfg.Database.Database)
	if err := driver.ApplySQL(ctx, []string{create}); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	logger.Info().Msg("Database reset complete.")
	return nil
}

func resetMySQL(ctx context.Context, cfg *MigrateConfig) error {
	// Build admin DSN without a database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", cfg.Database.Username, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port)
	connectCtx, cancel := cfg.ConnectContext(ctx)
	defer cancel()
	driver, err := NewDriver(connectCtx, "mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to mysql for admin operations: %w", err)
	}
	name := strings.ReplaceAll(cfg.Database.Database, "`", "``")
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", name)
	create := fmt.Sprintf("CREATE DATABASE `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", name)
	logger.Info().Msgf("Dropping database '%s'...", cfg.Database.Database)
	if err := driver.ApplySQL(ctx, []string{drop}); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}
	logger.Info().Msgf("Creating database '%s'...", cfg.Database.Database)
	if err := driver.ApplySQL(ctx, []string{create}); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	logger.Info().Msg("Database reset complete.")
	return nil
}

func resetSQLite(ctx context.Context, cfg *MigrateConfig) error {
	path := cfg.Database.Database
	logger.Info().Msgf("Removing sqlite file '%s'...", path)
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove sqlite file: %w", err)
		}
	}
	// Recreate empty sqlite DB (driver will create file)
	connectCtx, cancel := cfg.ConnectContext(ctx)
	defer cancel()
	if _, err := NewDriver(connectCtx, "sqlite", path); err != nil {
		return fmt.Errorf("failed to create sqlite database: %w", err)
	}
	logger.Info().Msg("Database reset complete.")
	return nil
}
