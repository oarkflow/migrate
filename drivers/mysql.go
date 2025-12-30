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
	// Flatten statements
	var stmts []string
	for _, query := range migrations {
		parts := splitSQLStatements(query)
		for _, q := range parts {
			if strings.TrimSpace(q) != "" {
				stmts = append(stmts, q)
			}
		}
	}
	if len(stmts) == 0 {
		return nil
	}
	// Start transaction
	if _, err := m.db.Exec("START TRANSACTION;"); err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	for _, q := range stmts {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if len(args) > 0 {
			if _, err := m.db.NamedExec(q, args[0]); err != nil {
				_, _  = m.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		} else {
			if _, err := m.db.Exec(q); err != nil {
				_, _  = m.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		}
	}
	if _, err := m.db.Exec("COMMIT;"); err != nil {
		_, _  = m.db.Exec("ROLLBACK;")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (m *MySQLDriver) DB() *squealx.DB {
	return m.db
}
