package migrate

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// TestApplyMigrationRespectsCancelledContext proves the context threaded from
// the CLI down to the database calls is actually honored: with an already
// cancelled context, ApplyMigration fails with context.Canceled and the
// migration's SQL is never executed (no table, no history row).
func TestApplyMigrationRespectsCancelledContext(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	migrationFile := filepath.Join(manager.MigrationDir(), "001_multi.bcl")
	writeTestFile(t, migrationFile, testMultiRootMigrationBCL())

	migrations, err := ParseMigrationsBCL([]byte(testMultiRootMigrationBCL()))
	if err != nil {
		t.Fatalf("ParseMigrationsBCL: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("no migrations parsed")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := manager.ApplyMigration(ctx, migrations[0]); !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplyMigration with cancelled context: err = %v, want context.Canceled", err)
	}

	// The SQL must not have run, and nothing must have been recorded.
	assertSQLiteTableExists(t, manager, "accounts", false)

	histories, err := manager.historyDriver.Load(t.Context())
	if err != nil {
		t.Fatalf("history Load: %v", err)
	}
	if len(histories) != 0 {
		t.Fatalf("len(histories) = %d, want 0 after cancelled migration", len(histories))
	}
}

// TestApplySQLRespectsCancelledContext is the same guarantee one level lower,
// at the database driver itself.
func TestApplySQLRespectsCancelledContext(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := manager.dbDriver.ApplySQL(ctx, []string{`CREATE TABLE cancelled_target (id INTEGER PRIMARY KEY);`})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplySQL with cancelled context: err = %v, want context.Canceled", err)
	}
	assertSQLiteTableExists(t, manager, "cancelled_target", false)
}
