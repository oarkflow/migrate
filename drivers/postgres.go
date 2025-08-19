package drivers

import (
	"fmt"
	"strings"

	"github.com/oarkflow/squealx"
	"github.com/oarkflow/squealx/drivers/postgres"
)

type PostgresDriver struct {
	db *squealx.DB
}

func NewPostgresDriverFromDB(db *squealx.DB) *PostgresDriver {
	return &PostgresDriver{db: db}
}

func NewPostgresDriver(dsn string) (*PostgresDriver, error) {
	db, err := postgres.Open(dsn, "postgres")
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &PostgresDriver{db: db}, nil
}

func (p *PostgresDriver) ApplySQL(migrations []string, args ...any) error {
	for _, query := range migrations {
		queries := strings.Split(query, ";")
		for _, q := range queries {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if len(args) > 0 {
				if _, err := p.db.NamedExec(q, args[0]); err != nil {
					return fmt.Errorf("failed to execute query [%s]: %w", query, err)
				}
			} else {
				if _, err := p.db.Exec(q); err != nil {
					return fmt.Errorf("failed to execute query [%s]: %w", query, err)
				}
			}
		}
	}
	return nil
}

func (p *PostgresDriver) DB() *squealx.DB {
	return p.db
}
