package drivers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/oarkflow/squealx"
	"github.com/oarkflow/squealx/drivers/mysql"
)

type MySQLDriver struct {
	db    *squealx.DB
	Force bool
}

func (m *MySQLDriver) SetForce(force bool) {
	m.Force = force
}

func NewMySQLDriverFromDB(db *squealx.DB) *MySQLDriver {
	return &MySQLDriver{db: db}
}

func NewMySQLDriver(ctx context.Context, dsn string) (*MySQLDriver, error) {
	db, err := mysql.Open(dsn, "mysql")
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to %s: %w", redactDSN(dsn), errors.New(redactDSN(err.Error())))
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database %s: %w", redactDSN(dsn), errors.New(redactDSN(err.Error())))
	}
	return &MySQLDriver{db: db}, nil
}

func (m *MySQLDriver) ApplySQL(ctx context.Context, migrations []string, args ...any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// cleanupCtx is used for the best-effort undo statements so they still
	// reach the database even when ctx has already been cancelled or timed out.
	cleanupCtx := context.WithoutCancel(ctx)
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

	// Force mode: MySQL DDL statements issue an implicit commit anyway, so a
	// surrounding transaction provides no real atomicity across statements.
	// We execute each statement individually and keep going past failures
	// (so a bad statement doesn't block the rest), but we track every
	// failure and never report success when one occurred: the caller (and
	// migration history) must see an accurate, honest partial-failure
	// result rather than a false "fully applied".
	if m.Force {
		var failures []error
		applied := 0
		for _, q := range stmts {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			var err error
			if len(args) > 0 {
				_, err = m.db.NamedExecContext(ctx, q, args[0])
			} else {
				_, err = m.db.ExecContext(ctx, q)
			}
			if err != nil {
				fmt.Printf("[force] warning: statement failed: %s: %v\n", q, err)
				failures = append(failures, fmt.Errorf("statement failed [%s]: %w", q, err))
				continue
			}
			applied++
		}
		if len(failures) > 0 {
			return fmt.Errorf("force mode: %d/%d statements applied, %d failed: %w", applied, len(stmts), len(failures), errors.Join(failures...))
		}
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
	if _, err := m.db.ExecContext(ctx, "START TRANSACTION;"); err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Disable foreign key checks for rollback operations
	if isRollback {
		if _, err := m.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0;"); err != nil {
			_, _ = m.db.ExecContext(cleanupCtx, "ROLLBACK;")
			return fmt.Errorf("failed to disable foreign key checks: %w", err)
		}
	}

	for _, q := range stmts {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if len(args) > 0 {
			if _, err := m.db.NamedExecContext(ctx, q, args[0]); err != nil {
				if isRollback && m.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = m.db.ExecContext(cleanupCtx, "ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		} else {
			if _, err := m.db.ExecContext(ctx, q); err != nil {
				if isRollback && m.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = m.db.ExecContext(cleanupCtx, "ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		}
	}

	// Re-enable foreign key checks if they were disabled
	if isRollback {
		if _, err := m.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1;"); err != nil {
			_, _ = m.db.ExecContext(cleanupCtx, "ROLLBACK;")
			return fmt.Errorf("failed to re-enable foreign key checks: %w", err)
		}
	}

	if _, err := m.db.ExecContext(ctx, "COMMIT;"); err != nil {
		_, _ = m.db.ExecContext(cleanupCtx, "ROLLBACK;")
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
		strings.Contains(errStr, "error 1451") // cannot delete or update a parent row (during rollback, ignore)
}
