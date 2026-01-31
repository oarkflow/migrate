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

	// If the set of statements includes database-level operations (CREATE/DROP/ALTER DATABASE)
	// they cannot be executed inside a transaction in Postgres. Execute all statements
	// individually (without BEGIN/COMMIT) when any such statement is present.
	hasDBStmt := false
	for _, q := range stmts {
		l := strings.ToLower(strings.TrimSpace(q))
		if strings.HasPrefix(l, "drop database") || strings.HasPrefix(l, "create database") || strings.HasPrefix(l, "alter database") {
			hasDBStmt = true
			break
		}
	}

	if hasDBStmt {
		for _, q := range stmts {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if len(args) > 0 {
				if _, err := p.db.NamedExec(q, args[0]); err != nil {
					return fmt.Errorf("failed to execute query [%s]: %w", q, err)
				}
			} else {
				if _, err := p.db.Exec(q); err != nil {
					return fmt.Errorf("failed to execute query [%s]: %w", q, err)
				}
			}
		}
		return nil
	}

	// Begin transaction
	if _, err := p.db.Exec("BEGIN;"); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Disable foreign key checks for rollback operations
	if isRollback {
		if _, err := p.db.Exec("SET session_replication_role = replica;"); err != nil {
			_, _ = p.db.Exec("ROLLBACK;")
			return fmt.Errorf("failed to disable foreign key constraints: %w", err)
		}
	}

	// Execute statements with error tolerance for rollback operations
	for _, q := range stmts {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if len(args) > 0 {
			if _, err := p.db.NamedExec(q, args[0]); err != nil {
				if isRollback && p.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = p.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		} else {
			if _, err := p.db.Exec(q); err != nil {
				if isRollback && p.isIgnorableError(err) {
					continue // Skip errors for non-existent objects during rollback
				}
				_, _ = p.db.Exec("ROLLBACK;")
				return fmt.Errorf("failed to execute query [%s]: %w", q, err)
			}
		}
	}

	// Re-enable foreign key checks if they were disabled
	if isRollback {
		if _, err := p.db.Exec("SET session_replication_role = DEFAULT;"); err != nil {
			_, _ = p.db.Exec("ROLLBACK;")
			return fmt.Errorf("failed to re-enable foreign key constraints: %w", err)
		}
	}

	// Commit
	if _, err := p.db.Exec("COMMIT;"); err != nil {
		_, _ = p.db.Exec("ROLLBACK;")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// isIgnorableError checks if an error can be safely ignored during rollback operations
func (p *PostgresDriver) isIgnorableError(err error) bool {
	errStr := strings.ToLower(err.Error())
	// PostgreSQL error codes for objects that don't exist or dependency issues during rollback
	return strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "42p01") || // undefined_table
		strings.Contains(errStr, "42703") || // undefined_column
		strings.Contains(errStr, "42883") || // undefined_function
		strings.Contains(errStr, "42p02") || // undefined_parameter
		strings.Contains(errStr, "2bp01")    // dependent_objects_still_exist (during rollback, ignore)
}

func (p *PostgresDriver) DB() *squealx.DB {
	return p.db
}
