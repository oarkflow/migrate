package drivers

import (
	"fmt"
	"strings"

	"github.com/oarkflow/squealx"
	"github.com/oarkflow/squealx/drivers/mysql"
)

type MySQLDriver struct {
	db *squealx.DB
}

func NewMySQLDriverFromDB(db *squealx.DB) *MySQLDriver {
	return &MySQLDriver{db: db}
}

func NewMySQLDriver(dsn string) (*MySQLDriver, error) {
	db, err := mysql.Open(dsn, "mysql")
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &MySQLDriver{db: db}, nil
}

func (m *MySQLDriver) ApplySQL(migrations []string, args ...any) error {
	for _, query := range migrations {
		queries := strings.Split(query, ";")
		for _, q := range queries {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if len(args) > 0 {
				if _, err := m.db.NamedExec(q, args[0]); err != nil {
					return fmt.Errorf("failed to execute query [%s]: %w", query, err)
				}
			} else {
				if _, err := m.db.Exec(q); err != nil {
					return fmt.Errorf("failed to execute query [%s]: %w", query, err)
				}
			}
		}
	}
	return nil
}

func (m *MySQLDriver) DB() *squealx.DB {
	return m.db
}
