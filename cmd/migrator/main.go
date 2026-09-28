package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/oarkflow/squealx"

	"github.com/oarkflow/migrate"
)

type Config struct {
	squealx.Config
	ConfigFile     string // path to a JSON config file (e.g., migrate.json)
	MigrationDir   string
	MigrationTable string
}

// extractConfigFromArgs looks for --config or -c in the provided args and
// returns the config path and a new args slice with those flags removed so the
// CLI parser doesn't see them.
func extractConfigFromArgs(args []string) (string, []string) {
	var cfg string
	out := make([]string, 0, len(args))
	if len(args) > 0 {
		out = append(out, args[0])
	}
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--config" || a == "-c" {
			if i+1 < len(args) {
				cfg = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "--config=") {
			cfg = strings.TrimPrefix(a, "--config=")
			continue
		}
		if strings.HasPrefix(a, "-c=") {
			cfg = strings.TrimPrefix(a, "-c=")
			continue
		}
		out = append(out, a)
	}
	return cfg, out
}

func Run(ctx context.Context, dialect string, cfg ...Config) error {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	}

	// Priority: explicit ConfigFile in config param, otherwise look at CLI args
	if config.ConfigFile != "" {
		if _, err := os.Stat(config.ConfigFile); err != nil {
			return err
		}
		manager, err := migrate.NewManagerFromConfig(ctx, config.ConfigFile)
		if err != nil {
			return err
		}
		if dialect != "" {
			manager.SetDialect(dialect)
		}
		manager.Run(ctx)
		return nil
	}

	// Look for --config on the command line and strip it so it doesn't affect
	// other commands.
	if cfgPath, filtered := extractConfigFromArgs(os.Args); cfgPath != "" {
		if _, err := os.Stat(cfgPath); err != nil {
			return err
		}
		manager, err := migrate.NewManagerFromConfig(ctx, cfgPath)
		if err != nil {
			return err
		}
		if dialect != "" {
			manager.SetDialect(dialect)
		}
		// Replace os.Args so CLI won't see the --config flag
		os.Args = filtered
		manager.Run(ctx)
		return nil
	}

	// If no explicit config is provided, auto-load default migrate.json when present.
	// This keeps plain `migrator ... migrate` behavior intuitive.
	if _, err := os.Stat("migrate.json"); err == nil {
		manager, err := migrate.NewManagerFromConfig(ctx, "migrate.json")
		if err != nil {
			return err
		}
		if dialect != "" {
			manager.SetDialect(dialect)
		}
		manager.Run(ctx)
		return nil
	}

	var opts []migrate.ManagerOption
	if config.Config.Driver != "" {
		dsn := config.ToString()
		if dsn != "" {
			driver, err := migrate.NewDriver(ctx, config.Config.Driver, dsn)
			if err != nil {
				return err
			}
			opts = append(opts, migrate.WithDriver(driver))
			var tables []string
			if config.MigrationTable != "" {
				tables = append(tables, config.MigrationTable)
			}
			historyDriver, err := migrate.NewHistoryDriver(ctx, "db", dialect, dsn, tables...)
			if err != nil {
				return err
			}
			opts = append(opts, migrate.WithHistoryDriver(historyDriver))
		}
	}
	if config.MigrationDir != "" {
		opts = append(opts, migrate.WithMigrationDir(config.MigrationDir))
	}
	manager := migrate.NewManager(append(opts, migrate.WithContext(ctx))...)
	manager.SetDialect(dialect)
	manager.Run(ctx)
	return nil
}

func main() {
	// A single root context for the whole process: Ctrl+C / SIGTERM cancels it,
	// which cancels any in-flight migration/rollback/seed all the way down to
	// the database calls instead of leaving work hanging.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Strip --config/-c early so the CLI doesn't see it
	cfgPath, filtered := extractConfigFromArgs(os.Args)
	if cfgPath != "" {
		// ensure file exists early
		if _, err := os.Stat(cfgPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Args = filtered
		if err := Run(ctx, "", Config{ConfigFile: cfgPath}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// No config file; pass through and let Run handle other configuration
	if err := Run(ctx, ""); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
