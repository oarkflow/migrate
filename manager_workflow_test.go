package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newSQLiteWorkflowManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	migrationDir := filepath.Join(dir, "migrations")
	seedDir := filepath.Join(migrationDir, "seeds")
	dbPath := filepath.Join(dir, "workflow.db")

	driver, err := NewDriver(DialectSQLite, dbPath)
	if err != nil {
		t.Fatalf("NewDriver sqlite: %v", err)
	}
	historyDriver, err := NewHistoryDriver("db", DialectSQLite, dbPath, "migrations")
	if err != nil {
		t.Fatalf("NewHistoryDriver sqlite: %v", err)
	}
	return NewManager(
		WithMigrationDir(migrationDir),
		WithSeedDir(seedDir),
		WithDialect(DialectSQLite),
		WithDriver(driver),
		WithHistoryDriver(historyDriver),
	)
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func testMultiRootMigrationBCL() string {
	return `
Migration "001_create_accounts" {
  Version = "1.0.0"
  Description = "Create accounts."
  Up {
    CreateTable "accounts" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "name" {
        type = "string"
        size = 64
      }
    }
  }
  Down {
    DropTable "accounts" {}
  }
}

Migration "002_create_projects" {
  Version = "1.0.0"
  Description = "Create projects."
  Up {
    CreateTable "projects" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "title" {
        type = "string"
        size = 100
      }
    }
  }
  Down {
    DropTable "projects" {}
  }
}
`
}

func TestManagerMultiRootMigrateRollbackResetSQLite(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	migrationFile := filepath.Join(manager.MigrationDir(), "001_multi.bcl")
	writeTestFile(t, migrationFile, testMultiRootMigrationBCL())

	migrationMap, err := manager.ListMigrationMap()
	if err != nil {
		t.Fatalf("ListMigrationMap: %v", err)
	}
	if len(migrationMap) != 2 {
		t.Fatalf("len(migrationMap) = %d, want 2", len(migrationMap))
	}

	migrations, err := ParseMigrationsBCL([]byte(testMultiRootMigrationBCL()))
	if err != nil {
		t.Fatalf("ParseMigrationsBCL: %v", err)
	}
	for _, migration := range migrations {
		if err := manager.ApplyMigration(migration); err != nil {
			t.Fatalf("ApplyMigration(%s): %v", migration.Name, err)
		}
	}

	assertSQLiteTableExists(t, manager, "accounts", true)
	assertSQLiteTableExists(t, manager, "projects", true)

	histories, err := manager.historyDriver.Load()
	if err != nil {
		t.Fatalf("history Load: %v", err)
	}
	if len(histories) != 2 {
		t.Fatalf("len(histories) = %d, want 2", len(histories))
	}

	if err := manager.RollbackMigration(1); err != nil {
		t.Fatalf("RollbackMigration: %v", err)
	}
	assertSQLiteTableExists(t, manager, "accounts", true)
	assertSQLiteTableExists(t, manager, "projects", false)

	histories, err = manager.historyDriver.Load()
	if err != nil {
		t.Fatalf("history Load after rollback: %v", err)
	}
	if len(histories) != 1 || histories[0].Name != "001_create_accounts" {
		t.Fatalf("histories after rollback = %#v", histories)
	}

	if err := manager.ResetMigrations(); err != nil {
		t.Fatalf("ResetMigrations: %v", err)
	}
	assertSQLiteTableExists(t, manager, "accounts", false)

	histories, err = manager.historyDriver.Load()
	if err != nil {
		t.Fatalf("history Load after reset: %v", err)
	}
	if len(histories) != 0 {
		t.Fatalf("len(histories after reset) = %d, want 0", len(histories))
	}
}

func TestManagerRunSeedsMultiRootSQLite(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	if err := manager.dbDriver.ApplySQL([]string{`CREATE TABLE seed_targets (id TEXT PRIMARY KEY, label TEXT NOT NULL);`}); err != nil {
		t.Fatalf("create seed_targets: %v", err)
	}
	seedFile := filepath.Join(manager.SeedDir(), "multi_seed.bcl")
	writeTestFile(t, seedFile, `
Seed "first_seed" {
  table = "seed_targets"
  Field "id" {
    value = "first"
  }
  Field "label" {
    value = "one"
  }
  rows = 1
}

Seed "second_seed" {
  table = "seed_targets"
  Field "id" {
    value = "second"
  }
  Field "label" {
    value = "two"
  }
  rows = 1
}
`)

	if err := manager.RunSeeds(false, false, seedFile); err != nil {
		t.Fatalf("RunSeeds: %v", err)
	}

	var count int
	if err := manager.dbDriver.DB().Select(&count, `SELECT COUNT(*) FROM seed_targets`); err != nil {
		t.Fatalf("count seed rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("seed row count = %d, want 2", count)
	}
}

func TestManagerMixedRawSQLAndBCLResetSQLite(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	rawFile := filepath.Join(manager.MigrationDir(), "001_raw.sql")
	writeTestFile(t, rawFile, `
-- migration-up
CREATE TABLE raw_items (id INTEGER PRIMARY KEY, note TEXT);
INSERT INTO raw_items (id, note) VALUES (1, 'raw;keeps;semicolons');

-- migration-down
DROP TABLE IF EXISTS raw_items;
`)
	bclSrc := `
Migration "002_create_bcl_items" {
  Version = "1.0.0"
  Description = "Create BCL items."
  Up {
    CreateTable "bcl_items" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "name" {
        type = "string"
      }
    }
  }
  Down {
    DropTable "bcl_items" {}
  }
}
`
	writeTestFile(t, filepath.Join(manager.MigrationDir(), "002_bcl.bcl"), bclSrc)

	if err := manager.ApplySQLMigration(rawFile); err != nil {
		t.Fatalf("ApplySQLMigration: %v", err)
	}
	migration, err := ParseMigrationBCL([]byte(bclSrc))
	if err != nil {
		t.Fatalf("ParseMigrationBCL: %v", err)
	}
	if err := manager.ApplyMigration(migration); err != nil {
		t.Fatalf("ApplyMigration: %v", err)
	}
	assertSQLiteTableExists(t, manager, "raw_items", true)
	assertSQLiteTableExists(t, manager, "bcl_items", true)

	if err := manager.ResetMigrations(); err != nil {
		t.Fatalf("ResetMigrations: %v", err)
	}
	assertSQLiteTableExists(t, manager, "raw_items", false)
	assertSQLiteTableExists(t, manager, "bcl_items", false)
}

func TestMigrateCommandUsesIncludeRawForRawSQLMigrations(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	rawFile := filepath.Join(manager.MigrationDir(), "001_raw.sql")
	writeTestFile(t, rawFile, `
-- migration-up
CREATE TABLE raw_command_items (id INTEGER PRIMARY KEY);

-- migration-down
DROP TABLE IF EXISTS raw_command_items;
`)

	cmd := &MigrateCommand{Driver: manager}
	if err := cmd.Handle(testContext{options: map[string]string{"force": "false", "seed": "false", "include-raw": "false"}}); err != nil {
		t.Fatalf("Handle without include raw: %v", err)
	}
	assertSQLiteTableExists(t, manager, "raw_command_items", false)

	manager = newSQLiteWorkflowManager(t)
	rawFile = filepath.Join(manager.MigrationDir(), "001_raw.sql")
	writeTestFile(t, rawFile, `
-- migration-up
CREATE TABLE raw_command_items (id INTEGER PRIMARY KEY);

-- migration-down
DROP TABLE IF EXISTS raw_command_items;
`)
	cmd = &MigrateCommand{Driver: manager}
	if err := cmd.Handle(testContext{options: map[string]string{"force": "false", "seed": "false", "include-raw": "true"}}); err != nil {
		t.Fatalf("Handle with include raw: %v", err)
	}
	assertSQLiteTableExists(t, manager, "raw_command_items", true)
}

func TestValidateMigrationsRejectsRawSQLWithoutUpSection(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	writeTestFile(t, filepath.Join(manager.MigrationDir(), "001_bad.sql"), `CREATE TABLE bad_raw (id INTEGER);`)

	err := manager.ValidateMigrations()
	if err == nil {
		t.Fatal("expected raw SQL validation error")
	}
	if !strings.Contains(err.Error(), "must include a non-empty -- migration-up section") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackRawSQLWithoutDownFailsUnlessForced(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	rawFile := filepath.Join(manager.MigrationDir(), "001_raw.sql")
	writeTestFile(t, rawFile, `
-- migration-up
CREATE TABLE raw_without_down (id INTEGER PRIMARY KEY);
`)
	if err := manager.ApplySQLMigration(rawFile); err != nil {
		t.Fatalf("ApplySQLMigration: %v", err)
	}

	err := manager.RollbackMigration(1)
	if err == nil {
		t.Fatal("expected rollback error for missing down section")
	}
	if !strings.Contains(err.Error(), "has no down SQL") {
		t.Fatalf("error = %v", err)
	}

	histories, err := manager.historyDriver.Load()
	if err != nil {
		t.Fatalf("history Load: %v", err)
	}
	if len(histories) != 1 {
		t.Fatalf("len(histories) = %d, want history retained after failed rollback", len(histories))
	}

	manager.Force = true
	if err := manager.RollbackMigration(1); err != nil {
		t.Fatalf("forced RollbackMigration: %v", err)
	}
	histories, err = manager.historyDriver.Load()
	if err != nil {
		t.Fatalf("history Load after forced rollback: %v", err)
	}
	if len(histories) != 0 {
		t.Fatalf("len(histories after forced rollback) = %d, want 0", len(histories))
	}
}

func TestRunSeedsReturnsApplyErrorUnlessForced(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	if err := manager.dbDriver.ApplySQL([]string{`CREATE TABLE seed_unique (id TEXT PRIMARY KEY);`}); err != nil {
		t.Fatalf("create seed_unique: %v", err)
	}
	seedFile := filepath.Join(manager.SeedDir(), "bad_seed.bcl")
	writeTestFile(t, seedFile, `
Seed "bad_seed" {
  table = "seed_unique"
  Field "id" {
    value = "dup"
  }
  rows = 2
}
`)
	err := manager.RunSeeds(false, false, seedFile)
	if err == nil {
		t.Fatal("expected seed apply error")
	}
	if !strings.Contains(err.Error(), "seed failed") {
		t.Fatalf("error = %v", err)
	}

	manager.Force = true
	if err := manager.RunSeeds(false, false, seedFile); err != nil {
		t.Fatalf("forced RunSeeds: %v", err)
	}
}

func TestHistoryReportIncludesMultiRootMigrations(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	migrationFile := filepath.Join(manager.MigrationDir(), "001_multi.bcl")
	writeTestFile(t, migrationFile, testMultiRootMigrationBCL())

	readMigrations := func(path string) ([]Migration, error) {
		cached, err := manager.readMigrationsBCL(path)
		if err != nil {
			return nil, err
		}
		return cached.migrations, nil
	}
	report, err := generateHTMLReportAllObjectsTemplate(
		[]objectInfo{{Name: "accounts", Type: "table"}, {Name: "projects", Type: "table"}},
		[]string{migrationFile},
		manager.MigrationDir(),
		readMigrations,
	)
	if err != nil {
		t.Fatalf("generateHTMLReportAllObjectsTemplate: %v", err)
	}
	for _, want := range []string{"001_create_accounts", "002_create_projects", "accounts", "projects"} {
		if !strings.Contains(report, want) {
			t.Fatalf("history report missing %q", want)
		}
	}
}

func TestManagerRejectsDuplicateMigrationNamesAcrossFiles(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	writeTestFile(t, filepath.Join(manager.MigrationDir(), "001_a.bcl"), `Migration "dup" {}`)
	writeTestFile(t, filepath.Join(manager.MigrationDir(), "002_b.bcl"), `Migration "dup" {}`)

	_, err := manager.ListMigrationMap()
	if err == nil {
		t.Fatal("expected duplicate migration error")
	}
	if !strings.Contains(err.Error(), `duplicate migration name "dup"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestManagerMigrationParseCache(t *testing.T) {
	manager := newSQLiteWorkflowManager(t)
	migrationFile := filepath.Join(manager.MigrationDir(), "001_multi.bcl")
	writeTestFile(t, migrationFile, testMultiRootMigrationBCL())

	first, err := manager.readMigrationsBCL(migrationFile)
	if err != nil {
		t.Fatalf("first readMigrationsBCL: %v", err)
	}
	second, err := manager.readMigrationsBCL(migrationFile)
	if err != nil {
		t.Fatalf("second readMigrationsBCL: %v", err)
	}
	if first.checksum == "" || second.checksum != first.checksum {
		t.Fatalf("cache checksums differ: %q vs %q", first.checksum, second.checksum)
	}
	if len(manager.migrationBCL) != 1 {
		t.Fatalf("len(migrationBCL cache) = %d, want 1", len(manager.migrationBCL))
	}
}

func TestParsedMigrationGeneratesMySQLAndPostgresSQL(t *testing.T) {
	src := []byte(`
Migration "001_create_accounts" {
  Version = "1.0.0"
  Description = "Create accounts."
  Up {
    CreateTable "accounts" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "email" {
        type = "string"
        size = 120
        unique = true
      }
    }
  }
  Down {
    DropTable "accounts" {}
  }
}
`)
	migration, err := ParseMigrationBCL(src)
	if err != nil {
		t.Fatalf("ParseMigrationBCL: %v", err)
	}
	for _, dialect := range []string{DialectMySQL, DialectPostgres} {
		queries, err := migration.ToSQL(dialect, true)
		if err != nil {
			t.Fatalf("ToSQL(%s): %v", dialect, err)
		}
		if len(queries) == 0 {
			t.Fatalf("ToSQL(%s) returned no queries", dialect)
		}
		joined := strings.Join(queries, "\n")
		if !strings.Contains(strings.ToLower(joined), "create table") || !strings.Contains(joined, "accounts") {
			t.Fatalf("ToSQL(%s) unexpected SQL: %s", dialect, joined)
		}
	}
}

type testContext struct {
	args    []string
	options map[string]string
}

func (c testContext) Argument(index int) string {
	if index < 0 || index >= len(c.args) {
		return ""
	}
	return c.args[index]
}

func (c testContext) Arguments() []string {
	return c.args
}

func (c testContext) Option(key string) string {
	return c.options[key]
}

func assertSQLiteTableExists(t *testing.T, manager *Manager, table string, want bool) {
	t.Helper()
	var exists bool
	query := GetDialect(DialectSQLite).TableExistsSQL(table)
	if err := manager.dbDriver.DB().Select(&exists, query); err != nil {
		t.Fatalf("table exists query for %s: %v", table, err)
	}
	if exists != want {
		t.Fatalf("table %s exists = %t, want %t", table, exists, want)
	}
}
