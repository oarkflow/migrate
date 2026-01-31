package drivers

import (
	"fmt"
	"strings"

	"github.com/oarkflow/squealx"
	"github.com/oarkflow/squealx/drivers/sqlite"
)

type SQLiteDriver struct {
	db *squealx.DB
}

func NewSQLiteDriverFromDB(db *squealx.DB) *SQLiteDriver {
	return &SQLiteDriver{db: db}
}

func NewSQLiteDriver(dbPath string) (*SQLiteDriver, error) {
	db, err := sqlite.Open(dbPath, "sqlite3")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}
	return &SQLiteDriver{db: db}, nil
}

func (s *SQLiteDriver) ApplySQL(migrations []string, args ...any) error {
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

	// Begin transaction
	if _, err := s.db.Exec("BEGIN;"); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Disable foreign key checks for rollback operations
	if isRollback {
		if _, err := s.db.Exec("PRAGMA foreign_keys = OFF;"); err != nil {
			_, _ = s.db.Exec("ROLLBACK;")
			return fmt.Errorf("failed to disable foreign key checks: %w", err)
		}
	}

	for _, q := range stmts {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if len(args) > 0 {
			if _, err := s.db.NamedExec(q, args[0]); err != nil {
				if isRollback && s.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = s.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		} else {
			if _, err := s.db.Exec(q); err != nil {
				if isRollback && s.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = s.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		}
	}

	// Re-enable foreign key checks if they were disabled
	if isRollback {
		if _, err := s.db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
			_, _ = s.db.Exec("ROLLBACK;")
			return fmt.Errorf("failed to re-enable foreign key checks: %w", err)
		}
	}

	if _, err := s.db.Exec("COMMIT;"); err != nil {
		_, _ = s.db.Exec("ROLLBACK;")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (m *SQLiteDriver) DB() *squealx.DB {
	return m.db
}
// isIgnorableError checks if an error can be safely ignored during rollback operations
func (s *SQLiteDriver) isIgnorableError(err error) bool {
	errStr := strings.ToLower(err.Error())
	// SQLite error messages for objects that don't exist
	return strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "no such column") ||
		strings.Contains(errStr, "no such index") ||
		strings.Contains(errStr, "no such trigger")
}