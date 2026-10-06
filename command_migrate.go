package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/oarkflow/cli/contracts"
	"github.com/oarkflow/zlog"
)

type MigrateCommand struct {
	Driver IManager
}

func (c *MigrateCommand) Signature() string {
	return "migrate"
}

func (c *MigrateCommand) Description() string {
	return "Migrate all migration files that are not already applied."
}

func (c *MigrateCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
			{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Force apply migrations ignoring checksum mismatches and statement errors",
				Value:   "false",
			},
			{
				Name:    "seed",
				Aliases: []string{"s"},
				Usage:   "Seed tables after migration",
				Value:   "false",
			},
			{
				Name:    "rows",
				Aliases: []string{"r"},
				Usage:   "Number of seed rows (default 10)",
				Value:   "10",
			},
			{
				Name:    "include-raw",
				Aliases: []string{"i"},
				Usage:   "Include raw .sql migrations and raw .sql seed files",
				Value:   "false",
			},
		},
	}
}

func (c *MigrateCommand) Handle(ctx contracts.Context) error {
	// contracts.Context is the CLI framework's own type; the cancellable
	// context.Context comes from the manager (set by Manager.Run).
	runCtx := c.Driver.Context()
	// Set verbose flag on Manager if -v is passed
	verbose := ctx.Option("v") != "" && ctx.Option("v") != "false"
	forceFlag := ctx.Option("f") != "" && ctx.Option("f") != "false"
	if mgr, ok := c.Driver.(*Manager); ok {
		mgr.Verbose = verbose
		if forceFlag {
			mgr.Force = true
			if mgr.dbDriver != nil {
				mgr.dbDriver.SetForce(true)
			}
		}
	}
	if err := c.Driver.ValidateHistoryStorage(runCtx); err != nil {
		logger.Error("History storage validation failed", zlog.Err(err))
		return fmt.Errorf("history storage validation failed: %w", err)
	}
	if err := acquireLock(); err != nil {
		logger.Error("Cannot start migration (failed to acquire lock)", zlog.Err(err))
		return fmt.Errorf("cannot start migration: %w", err)
	}
	defer func() {
		if err := releaseLock(); err != nil {
			logger.Info(fmt.Sprintf("Warning releasing lock: %v", err))
		}
	}()
	if err := c.Driver.ValidateMigrations(runCtx); err != nil {
		logger.Info(fmt.Sprintf("Validation warning: %v", err))
	}
	// Collect migration files (.bcl) - prefer Manager.ListMigrationMap when available
	var migrationFiles []string
	var readFile func(string) ([]byte, error)
	var readMigrations func(string) ([]Migration, error)
	if mgr, ok := c.Driver.(*Manager); ok {
		migrationMap, err := mgr.ListMigrationMap()
		if err != nil {
			logger.Error("Failed to list migrations from manager", zlog.Err(err))
			return fmt.Errorf("failed to list migrations: %w", err)
		}
		seenPaths := make(map[string]struct{}, len(migrationMap))
		for _, p := range migrationMap {
			if _, ok := seenPaths[p]; ok {
				continue
			}
			seenPaths[p] = struct{}{}
			migrationFiles = append(migrationFiles, p)
		}
		readFile = mgr.readFile
		readMigrations = func(path string) ([]Migration, error) {
			cached, err := mgr.readMigrationsBCL(path)
			if err != nil {
				return nil, err
			}
			return cached.migrations, nil
		}
	} else {
		seedDir := c.Driver.SeedDir()
		err := filepath.Walk(c.Driver.MigrationDir(), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Skip SeedDir and its subdirectories
			if seedDir != "" && strings.HasPrefix(path, seedDir) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(info.Name()))
				if ext == ".bcl" || ext == ".sql" {
					migrationFiles = append(migrationFiles, path)
				}
			}
			return nil
		})
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to walk migration directory: %s", c.Driver.MigrationDir()), zlog.Err(err))
			return fmt.Errorf("failed to walk migration directory: %w", err)
		}
		readFile = os.ReadFile
		readMigrations = func(path string) ([]Migration, error) {
			data, err := readFile(path)
			if err != nil {
				return nil, err
			}
			return ParseMigrationsBCL(data)
		}
	}

	seedFlag := ctx.Option("seed")
	seedRows := 10
	if rowsStr := ctx.Option("rows"); rowsStr != "" {
		if n, err := strconv.Atoi(rowsStr); err == nil && n > 0 {
			seedRows = n
		}
	}
	includeRawOption := ctx.Option("include-raw")
	includeRaw := includeRawOption == "true" || includeRawOption == "1"
	shouldSeed := seedFlag == "true" || seedFlag == "1"

	// Ensure migrations are applied in deterministic order by filename (timestamp prefix)
	sort.SliceStable(migrationFiles, func(i, j int) bool {
		return filepath.Base(migrationFiles[i]) < filepath.Base(migrationFiles[j])
	})

	for _, path := range migrationFiles {
		base := filepath.Base(path)
		ext := strings.ToLower(filepath.Ext(base))
		name := strings.TrimSuffix(base, ext)
		// Handle raw .sql migrations
		if ext == ".sql" {
			if !includeRaw {
				logger.Info(fmt.Sprintf("Skipping raw SQL migration (enable with --include-raw=true): %s", path))
				continue
			}
			if err := c.Driver.ApplySQLMigration(runCtx, path); err != nil {
				logger.Error(fmt.Sprintf("Failed to apply raw SQL migration %s", name), zlog.Err(err))
				if forceFlag {
					continue
				}
				return fmt.Errorf("failed to apply raw SQL migration %s: %w", name, err)
			}
			continue
		}

		// Default: .bcl migration
		migrations, err := readMigrations(path)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to parse migration file %s", name), zlog.Err(err))
			return fmt.Errorf("failed to parse migration file %s: %w", name, err)
		}
		if len(migrations) == 0 {
			return fmt.Errorf("migration file %s contains no Migration blocks", name)
		}
		for _, migration := range migrations {
			if err := c.applyParsedMigration(runCtx, migration, name, shouldSeed, seedRows, forceFlag); err != nil {
				return err
			}
		}
	}
	if shouldSeed {
		if err := c.runSeedFilesAfterMigration(runCtx, includeRaw); err != nil {
			logger.Error("Running seed files after migration failed", zlog.Err(err))
			return err
		}
	}
	return nil
}

func (c *MigrateCommand) runSeedFilesAfterMigration(ctx context.Context, includeRaw bool) error {
	seedDir := c.Driver.SeedDir()
	if seedDir == "" {
		return nil
	}
	var files []string
	if mgr, ok := c.Driver.(*Manager); ok {
		mgrFiles, err := mgr.ListSeedFiles(includeRaw)
		if err != nil {
			logger.Error("Failed to list seed files from manager", zlog.Err(err))
			return fmt.Errorf("failed to list seed files: %w", err)
		}
		files = append(files, mgrFiles...)
	} else {
		entries, err := os.ReadDir(seedDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			logger.Error(fmt.Sprintf("Failed to read seed directory %s", seedDir), zlog.Err(err))
			return fmt.Errorf("failed to read seed directory %s: %w", seedDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			switch ext {
			case ".bcl":
				files = append(files, filepath.Join(seedDir, entry.Name()))
			case ".sql":
				if includeRaw {
					files = append(files, filepath.Join(seedDir, entry.Name()))
				}
			}
		}
	}
	if len(files) == 0 {
		return nil
	}

	// Ensure seed files are run in deterministic order by filename
	sort.SliceStable(files, func(i, j int) bool {
		return filepath.Base(files[i]) < filepath.Base(files[j])
	})

	logger.Info(fmt.Sprintf("Running %d seed file(s) after migration", len(files)))
	if err := c.Driver.RunSeeds(ctx, false, includeRaw, files...); err != nil {
		logger.Error("Failed to run seed files after migration", zlog.Err(err))
		return fmt.Errorf("failed to apply seed files after migration: %w", err)
	}
	return nil
}

func (c *MigrateCommand) applyParsedMigration(ctx context.Context, migration Migration, fileName string, shouldSeed bool, seedRows int, forceFlag bool) error {
	if err := requireFields(migration.Name); err != nil {
		logger.Error(fmt.Sprintf("Migration %s failed required field check", fileName), zlog.Err(err))
		return fmt.Errorf("MigrateCommand.Handle: %w", err)
	}
	if migration.Disable {
		logger.Warn(fmt.Sprintf("Migration '%s' is disabled. To enable it, set Disabled: false or remove the Disabled field.", migration.Name))
		return nil
	}
	for _, val := range migration.Validate {
		if err := runPreUpChecks(val.PreUpChecks); err != nil {
			logger.Error(fmt.Sprintf("Pre-up validation failed for migration %s", migration.Name), zlog.Err(err))
			return fmt.Errorf("pre-up validation failed for migration %s: %w", migration.Name, err)
		}
	}
	if err := c.Driver.ApplyMigration(ctx, migration); err != nil {
		logger.Error(fmt.Sprintf("Failed to apply migration %s: %v", migration.Name, err))
		if forceFlag {
			return nil
		}
		return fmt.Errorf("failed to apply migration %s: %w", migration.Name, err)
	}
	for _, val := range migration.Validate {
		if err := runPostUpChecks(val.PostUpChecks); err != nil {
			logger.Error(fmt.Sprintf("Post-up validation failed for migration %s", migration.Name), zlog.Err(err))
			return fmt.Errorf("post-up validation failed for migration %s: %w", migration.Name, err)
		}
	}
	if shouldSeed {
		return c.autoSeedCreatedTables(ctx, migration, fileName, seedRows)
	}
	return nil
}

func (c *MigrateCommand) autoSeedCreatedTables(ctx context.Context, migration Migration, fileName string, seedRows int) error {
	mgr, ok := c.Driver.(*Manager)
	if !ok {
		return fmt.Errorf("automatic seeding requires *Manager driver")
	}
	for _, ct := range migration.Up.CreateTable {
		if err := requireFields(ct.Name); err != nil {
			logger.Error(fmt.Sprintf("Seed generation: missing required table name in migration %s", fileName), zlog.Err(err))
			return fmt.Errorf("MigrateCommand.Handle (seed): %w", err)
		}
		seedDef := SeedDefinition{
			Name:  "auto_seed_" + ct.Name,
			Table: ct.Name,
			Rows:  seedRows,
		}
		for _, col := range ct.AddFields {
			if err := requireFields(col.Name); err != nil {
				logger.Error(fmt.Sprintf("Seed generation: missing required field name in table %s (migration %s)", ct.Name, fileName), zlog.Err(err))
				return fmt.Errorf("MigrateCommand.Handle (seed field): %w", err)
			}
			if col.AutoIncrement || col.Nullable {
				continue
			}
			fd := FieldDefinition{
				Name:     col.Name,
				DataType: col.Type,
				Size:     col.Size,
			}
			if col.Default != nil {
				switch v := (col.Default).(type) {
				case string:
					if v == "now()" || v == "CURRENT_TIMESTAMP" {
						fd.Value = time.Now().Format(time.DateTime)
					} else {
						fd.Value = v
					}
				default:
					fd.Value = v
				}
				seedDef.Fields = append(seedDef.Fields, fd)
				continue
			}
			fakeFunc := "fake_string"
			switch strings.ToLower(col.Type) {
			case "int", "integer", "number", "smallint", "mediumint", "bigint", "tinyint":
				fakeFunc = "fake_uint"
			case "float", "double", "decimal", "numeric", "real":
				fakeFunc = "fake_float64"
			case "bool", "boolean":
				fakeFunc = "fake_bool"
			case "date":
				fakeFunc = "fake_date"
			case "datetime", "timestamp":
				fakeFunc = "fake_datetime"
			case "year":
				fakeFunc = "fake_year"
			default:
				fakeFunc = "fake_string"
				if col.Name == "status" {
					fakeFunc = "fake_status"
				}
			}
			fd.Value = fakeFunc
			seedDef.Fields = append(seedDef.Fields, fd)
		}
		queries, err := seedDef.ToSQL(mgr.dialect)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to generate seed SQL for table %s: %v", ct.Name, err))
			return fmt.Errorf("failed to generate seed SQL for table %s: %w", ct.Name, err)
		}
		logger.Info(fmt.Sprintf("Seeding table: %s", ct.Name))
		for _, q := range queries {
			logger.Info(fmt.Sprintf("Seed SQL: %s", q.SQL))
			if err := mgr.dbDriver.ApplySQL(ctx, []string{q.SQL}, q.Args); err != nil {
				logger.Error(fmt.Sprintf("Failed to apply seed SQL for table %s: %s", ct.Name, q.SQL), zlog.Err(err))
				return fmt.Errorf("failed to apply seed for table %s: %w", ct.Name, err)
			}
		}
	}
	return nil
}
