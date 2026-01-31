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

	// Check if this is a rollback operation (contains DROP statements)
	isRollback := false
	for _, q := range stmts {
		l := strings.ToLower(strings.TrimSpace(q))
		if strings.HasPrefix(l, "drop table") || strings.HasPrefix(l, "drop view") || strings.HasPrefix(l, "drop function") {
			isRollback = true
			break
		}
	}

	// Start transaction
	if _, err := m.db.Exec("START TRANSACTION;"); err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Disable foreign key checks for rollback operations
	if isRollback {
		if _, err := m.db.Exec("SET FOREIGN_KEY_CHECKS = 0;"); err != nil {
			_, _ = m.db.Exec("ROLLBACK;")
			return fmt.Errorf("failed to disable foreign key checks: %w", err)
		}
	}

	for _, q := range stmts {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if len(args) > 0 {
			if _, err := m.db.NamedExec(q, args[0]); err != nil {
				if isRollback && m.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = m.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		} else {
			if _, err := m.db.Exec(q); err != nil {
				if isRollback && m.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = m.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		}
	}

	// Re-enable foreign key checks if they were disabled
	if isRollback {
		if _, err := m.db.Exec("SET FOREIGN_KEY_CHECKS = 1;"); err != nil {
			_, _ = m.db.Exec("ROLLBACK;")
			return fmt.Errorf("failed to re-enable foreign key checks: %w", err)
		}
	}

	if _, err := m.db.Exec("COMMIT;"); err != nil {
		_, _ = m.db.Exec("ROLLBACK;")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (m *MySQLDriver) DB() *squealx.DB {
	return m.db
}

// isIgnorableError checks if an error can be safely ignored during rollback operations
func (m *MySQLDriver) isIgnorableError(err error) bool {
	errStr := strings.ToLower(err.Error())
	// MySQL error codes for objects that don't exist or dependency issues during rollback
	return strings.Contains(errStr, "doesn't exist") ||
		strings.Contains(errStr, "unknown table") ||
		strings.Contains(errStr, "unknown column") ||
		strings.Contains(errStr, "error 1051") || // unknown table
		strings.Contains(errStr, "error 1054") || // unknown column
		strings.Contains(errStr, "error 1217") || // foreign key constraint fails (during rollback, ignore)
		strings.Contains(errStr, "error 1451")    // cannot delete or update a parent row (during rollback, ignore)
}