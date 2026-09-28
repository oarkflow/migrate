package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// chdirTemp switches the process working directory to a fresh temp dir for
// the duration of the test and restores it afterwards. acquireLock/releaseLock
// and the default-config lookup used by lockTimeout() are both relative to
// the current working directory, so lock tests need an isolated cwd.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
	return dir
}

// writeLockTimeoutConfig writes a migrate.json in dir with the given
// migration.lock_timeout (in seconds). Other fields are populated with
// otherwise-valid defaults so config.Validate() succeeds when timeoutSeconds
// is positive.
func writeLockTimeoutConfig(t *testing.T, dir string, timeoutSeconds int) {
	t.Helper()
	raw := map[string]any{
		"database": map[string]any{
			"driver":   "sqlite",
			"database": filepath.Join(dir, "test.db"),
		},
		"migration": map[string]any{
			"directory":    "migrations",
			"table_name":   "migrations",
			"lock_timeout": timeoutSeconds,
			"batch_size":   100,
		},
		"seed": map[string]any{
			"directory":    "migrations/seeds",
			"default_rows": 10,
			"batch_size":   1000,
		},
		"logging": map[string]any{
			"level":  "info",
			"format": "text",
			"output": "console",
		},
		"validation": map[string]any{
			"max_identifier_length": 64,
		},
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "migrate.json"), data, 0644); err != nil {
		t.Fatalf("write migrate.json: %v", err)
	}
}

func writeRawLock(t *testing.T, info lockInfo) {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal lock info: %v", err)
	}
	if err := os.WriteFile(lockFileName, data, 0644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
}

func TestAcquireLockRecordsMetadataAndBlocksSecondAcquire(t *testing.T) {
	chdirTemp(t)

	if err := acquireLock(); err != nil {
		t.Fatalf("acquireLock: %v", err)
	}
	defer releaseLock()

	info, err := readLockInfo(lockFileName)
	if err != nil {
		t.Fatalf("readLockInfo: %v", err)
	}
	if info.PID != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), info.PID)
	}
	if info.Hostname == "" {
		t.Errorf("expected hostname to be recorded")
	}
	if info.AcquiredAt.IsZero() || time.Since(info.AcquiredAt) > time.Minute {
		t.Errorf("expected a recent AcquiredAt timestamp, got %v", info.AcquiredAt)
	}

	if err := acquireLock(); err == nil {
		t.Fatalf("expected second acquireLock to fail while the lock is held")
	} else if !strings.Contains(err.Error(), "pid=") {
		t.Errorf("expected error to include the lock holder's pid, got: %v", err)
	}
}

func TestAcquireLockReclaimsStaleLockWhenTimeoutConfigured(t *testing.T) {
	dir := chdirTemp(t)
	writeLockTimeoutConfig(t, dir, 1) // 1 second timeout

	writeRawLock(t, lockInfo{
		PID:        999999,
		Hostname:   "stale-host",
		AcquiredAt: time.Now().Add(-10 * time.Second),
	})

	if err := acquireLock(); err != nil {
		t.Fatalf("expected stale lock to be reclaimed, got error: %v", err)
	}
	defer releaseLock()

	info, err := readLockInfo(lockFileName)
	if err != nil {
		t.Fatalf("readLockInfo: %v", err)
	}
	if info.PID != os.Getpid() {
		t.Errorf("expected fresh lock owned by current pid %d, got %d", os.Getpid(), info.PID)
	}
}

func TestAcquireLockKeepsFreshLockEvenWithTimeoutConfigured(t *testing.T) {
	dir := chdirTemp(t)
	writeLockTimeoutConfig(t, dir, 300) // 5 minute timeout

	writeRawLock(t, lockInfo{
		PID:        999999,
		Hostname:   "other-host",
		AcquiredAt: time.Now(),
	})

	if err := acquireLock(); err == nil {
		releaseLock()
		t.Fatalf("expected a lock well within its timeout to block acquisition")
	}
}

func TestAcquireLockWithoutConfiguredTimeoutNeverAutoExpires(t *testing.T) {
	dir := chdirTemp(t)
	// lock_timeout of 0 fails config.Validate(), so LoadConfig("") returns an
	// error and lockTimeout() falls back to "never auto-expire" (0).
	writeLockTimeoutConfig(t, dir, 0)

	writeRawLock(t, lockInfo{
		PID:        999999,
		Hostname:   "stale-host",
		AcquiredAt: time.Now().Add(-999999 * time.Second),
	})

	err := acquireLock()
	if err == nil {
		t.Fatalf("expected acquireLock to refuse to auto-remove a lock with no configured timeout")
	}
	if !strings.Contains(err.Error(), "pid=999999") || !strings.Contains(err.Error(), "manually") {
		t.Errorf("expected error to reference the stale lock's pid and a manual-removal hint, got: %v", err)
	}
}

func TestValidateMigrationIdentifiersRejectsMalformedTableName(t *testing.T) {
	m := Migration{
		Name: "bad_migration",
		Up: Operation{
			CreateTable: []CreateTable{
				{
					Name: "users; DROP TABLE users;--",
					AddFields: []AddField{
						{Name: "id", Type: "integer"},
					},
				},
			},
		},
	}
	if err := validateMigrationIdentifiers(m); err == nil {
		t.Fatalf("expected validation error for malicious table identifier")
	}
}

func TestValidateMigrationIdentifiersAcceptsWellFormedMigration(t *testing.T) {
	m := Migration{
		Name: "good_migration",
		Up: Operation{
			CreateTable: []CreateTable{
				{
					Name: "users",
					AddFields: []AddField{
						{Name: "id", Type: "integer"},
						{Name: "email", Type: "string"},
					},
				},
			},
		},
		Down: Operation{
			DropTable: []DropTable{{Name: "users"}},
		},
	}
	if err := validateMigrationIdentifiers(m); err != nil {
		t.Fatalf("expected no validation error, got: %v", err)
	}
}
