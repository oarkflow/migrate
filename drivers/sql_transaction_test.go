package drivers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteTransactionalRollbackOnFailure(t *testing.T) {
	// Create temporary SQLite database file
	tmpDir := os.TempDir()
	dbPath := filepath.Join(tmpDir, "migrate_tx_test.db")
	_ = os.Remove(dbPath)
	drv, err := NewSQLiteDriver(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite driver: %v", err)
	}
	defer func() {
		_ = drv.DB().Close()
		_ = os.Remove(dbPath)
	}()

	// First: ensure that a failing statement causes rollback (table should not exist)
	bad := "CREATE TABLE tx_test (id INTEGER PRIMARY KEY); INSERT INTO tx_test (id) VALUES (1); INSRT INTO tx_test (id) VALUES (2);"
	err = drv.ApplySQL([]string{bad})
	if err == nil {
		t.Fatalf("expected error from malformed SQL but got nil")
	}
	// Check table does not exist (transaction should rollback)
	var count int
	err = drv.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='tx_test'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sqlite_master: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no table 'tx_test' after rollback, found %d", count)
	}

	// Second: successful multi-statement transaction should commit
	succ := "CREATE TABLE tx_ok (id INTEGER PRIMARY KEY); INSERT INTO tx_ok (id) VALUES (1);"
	if err := drv.ApplySQL([]string{succ}); err != nil {
		t.Fatalf("expected success applying SQL, got %v", err)
	}
	err = drv.DB().QueryRow("SELECT count(*) FROM tx_ok").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query tx_ok: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row in tx_ok after commit, got %d", count)
	}
}
