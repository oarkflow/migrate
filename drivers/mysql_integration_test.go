package drivers

import (
	"os"
	"testing"
)

// Integration test for MySQL driver; skipped unless TEST_MYSQL_DSN is provided.
func TestMySQLApplySQL_TransactionalRollbackOnFailure(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("skipping mysql integration test; set TEST_MYSQL_DSN to run")
	}
	m, err := NewMySQLDriver(dsn)
	if err != nil {
		t.Fatalf("failed to create mysql driver: %v", err)
	}
	defer func() { _ = m.DB().Close() }()

	// Cleanup if left over
	_, _ = m.DB().Exec("DROP TABLE IF EXISTS tx_mysql_test;")

	bad := `CREATE TABLE tx_mysql_test (id INT AUTO_INCREMENT PRIMARY KEY, note TEXT);
INSERT INTO tx_mysql_test (note) VALUES ('value;with;semicolons');
INSRT INTO tx_mysql_test (note) VALUES ('should fail');`
	err = m.ApplySQL([]string{bad})
	if err == nil {
		t.Fatalf("expected error from malformed SQL but got nil")
	}
	// verify table does not exist
	var count int
	err = m.DB().QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name='tx_mysql_test'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query information_schema: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no table 'tx_mysql_test' after rollback, found %d", count)
	}

	// Successful run
	succ := `CREATE TABLE tx_mysql_test (id INT AUTO_INCREMENT PRIMARY KEY, note TEXT);
INSERT INTO tx_mysql_test (note) VALUES ('ok');`
	if err := m.ApplySQL([]string{succ}); err != nil {
		t.Fatalf("expected success applying SQL, got %v", err)
	}
	err = m.DB().QueryRow("SELECT COUNT(*) FROM tx_mysql_test").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query tx_mysql_test: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row in tx_mysql_test after commit, got %d", count)
	}

	// cleanup
	_, _ = m.DB().Exec("DROP TABLE IF EXISTS tx_mysql_test;")
}
