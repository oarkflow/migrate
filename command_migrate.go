package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/oarkflow/bcl"
	"github.com/oarkflow/cli/contracts"
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
				Usage:   "Include raw .sql seed files after migration",
				Value:   "false",
			},
		},
	}
}

func (c *MigrateCommand) Handle(ctx contracts.Context) error {
	// Set verbose flag on Manager if -v is passed
	verbose := ctx.Option("v") != "" && ctx.Option("v") != "false"
	if mgr, ok := c.Driver.(*Manager); ok {
		mgr.Verbose = verbose
	}
	if err := c.Driver.ValidateHistoryStorage(); err != nil {
		logger.Error().Err(err).Msg("History storage validation failed")
		return fmt.Errorf("history storage validation failed: %w", err)
	}
	if err := acquireLock(); err != nil {
		logger.Error().Err(err).Msg("Cannot start migration (failed to acquire lock)")
		return fmt.Errorf("cannot start migration: %w", err)
	}
	defer func() {
		if err := releaseLock(); err != nil {
			logger.Printf("Warning releasing lock: %v", err)
		}
	}()
	if err := c.Driver.ValidateMigrations(); err != nil {
		logger.Printf("Validation warning: %v", err)
	}
	// Collect migration files (.bcl) - prefer Manager.ListMigrationMap when available
	var migrationFiles []string
	var readFile func(string) ([]byte, error)
	if mgr, ok := c.Driver.(*Manager); ok {
		migrationMap, err := mgr.ListMigrationMap()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to list migrations from manager")
			return fmt.Errorf("failed to list migrations: %w", err)
		}
		for _, p := range migrationMap {
			migrationFiles = append(migrationFiles, p)
		}
		readFile = mgr.readFile
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
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".bcl") {
				migrationFiles = append(migrationFiles, path)
			}
			return nil
		})
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to walk migration directory: %s", c.Driver.MigrationDir())
			return fmt.Errorf("failed to walk migration directory: %w", err)
		}
		readFile = os.ReadFile
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

	for _, path := range migrationFiles {
		name := strings.TrimSuffix(filepath.Base(path), ".bcl")
		data, err := readFile(path)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to read migration file %s from path %s", name, path)
			return fmt.Errorf("failed to read migration file %s: %w", name, err)
		}
		var cfg Config
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			logger.Error().Err(err).Msgf("Failed to unmarshal migration file %s", name)
			return fmt.Errorf("failed to unmarshal migration file %s: %w", name, err)
		}
		migration := cfg.Migration
		if err := requireFields(migration.Name); err != nil {
			logger.Error().Err(err).Msgf("Migration %s failed required field check", name)
			return fmt.Errorf("MigrateCommand.Handle: %w", err)
		}
		if migration.Disable {
			logger.Warn().Msgf("Migration '%s' is disabled. To enable it, set Disabled: false or remove the Disabled field.", migration.Name)
			continue
		}
		for _, val := range migration.Validate {
			if err := runPreUpChecks(val.PreUpChecks); err != nil {
				logger.Error().Err(err).Msgf("Pre-up validation failed for migration %s", migration.Name)
				return fmt.Errorf("pre-up validation failed for migration %s: %w", migration.Name, err)
			}
		}
		if err := c.Driver.ApplyMigration(migration); err != nil {
			logger.Error().Msgf("Failed to apply migration %s: %v", migration.Name, err)
			return fmt.Errorf("failed to apply migration %s: %w", migration.Name, err)
		}
		for _, val := range migration.Validate {
			if err := runPostUpChecks(val.PostUpChecks); err != nil {
				logger.Error().Err(err).Msgf("Post-up validation failed for migration %s", migration.Name)
				return fmt.Errorf("post-up validation failed for migration %s: %w", migration.Name, err)
			}
		}
		// --- SEEDING LOGIC ---
		if shouldSeed {
			for _, ct := range migration.Up.CreateTable {
				if err := requireFields(ct.Name); err != nil {
					logger.Error().Err(err).Msgf("Seed generation: missing required table name in migration %s", name)
					return fmt.Errorf("MigrateCommand.Handle (seed): %w", err)
				}
				seedDef := SeedDefinition{
					Name:  "auto_seed_" + ct.Name,
					Table: ct.Name,
					Rows:  seedRows,
				}
				for _, col := range ct.AddFields {
					if err := requireFields(col.Name); err != nil {
						logger.Error().Err(err).Msgf("Seed generation: missing required field name in table %s (migration %s)", ct.Name, name)
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
				queries, err := seedDef.ToSQL(c.Driver.(*Manager).dialect)
				if err != nil {
					logger.Error().Msgf("Failed to generate seed SQL for table %s: %v", ct.Name, err)
					return fmt.Errorf("failed to generate seed SQL for table %s: %w", ct.Name, err)
				}
				logger.Info().Msgf("Seeding table: %s", ct.Name)
				for _, q := range queries {
					logger.Info().Msgf("Seed SQL: %s", q.SQL)
					err := c.Driver.(*Manager).dbDriver.ApplySQL([]string{q.SQL}, q.Args)
					if err != nil {
						logger.Error().Err(err).Msgf("Failed to apply seed SQL for table %s: %s", ct.Name, q.SQL)
						return fmt.Errorf("failed to apply seed for table %s: %w", ct.Name, err)
					}
				}
			}
		}
	}
	if shouldSeed {
		if err := c.runSeedFilesAfterMigration(includeRaw); err != nil {
			logger.Error().Err(err).Msg("Running seed files after migration failed")
			return err
		}
	}
	return nil
}

func (c *MigrateCommand) runSeedFilesAfterMigration(includeRaw bool) error {
	seedDir := c.Driver.SeedDir()
	if seedDir == "" {
		return nil
	}
	var files []string
	if mgr, ok := c.Driver.(*Manager); ok {
		mgrFiles, err := mgr.ListSeedFiles(includeRaw)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to list seed files from manager")
			return fmt.Errorf("failed to list seed files: %w", err)
		}
		files = append(files, mgrFiles...)
	} else {
		entries, err := os.ReadDir(seedDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			logger.Error().Err(err).Msgf("Failed to read seed directory %s", seedDir)
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
	logger.Info().Msgf("Running %d seed file(s) after migration", len(files))
	if err := c.Driver.RunSeeds(false, includeRaw, files...); err != nil {
		logger.Error().Err(err).Msg("Failed to run seed files after migration")
		return fmt.Errorf("failed to apply seed files after migration: %w", err)
	}
	return nil
}
