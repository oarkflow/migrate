package cmd

import (
	"github.com/oarkflow/squealx"

	"github.com/oarkflow/migrate"
)

type Config struct {
	squealx.Config
	MigrationDir   string
	MigrationTable string
}

func Run(dialect string, cfg ...Config) error {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	}
	var opts []migrate.ManagerOption
	if config.Config.Driver != "" {
		dsn := config.ToString()
		if dsn != "" {
			driver, err := migrate.NewDriver(config.Config.Driver, dsn)
			if err != nil {
				return err
			}
			opts = append(opts, migrate.WithDriver(driver))
			var tables []string
			if config.MigrationTable != "" {
				tables = append(tables, config.MigrationTable)
			}
			historyDriver, err := migrate.NewHistoryDriver("db", dialect, dsn, tables...)
			if err != nil {
				return err
			}
			opts = append(opts, migrate.WithHistoryDriver(historyDriver))
		}
	}
	if config.MigrationDir != "" {
		opts = append(opts, migrate.WithMigrationDir(config.MigrationDir))
	}
	manager := migrate.NewManager(opts...)
	manager.SetDialect(dialect)
	manager.Run()
	return nil
}
