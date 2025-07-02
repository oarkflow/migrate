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
		return fmt.Errorf("history storage validation failed: %w", err)
	}
	if err := acquireLock(); err != nil {
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
	files, err := os.ReadDir(c.Driver.MigrationDir())
	if err != nil {
		return fmt.Errorf("failed to read migration directory: %w", err)
	}
	
	seedFlag := ctx.Option("seed")
	seedRows := 10
	if rowsStr := ctx.Option("rows"); rowsStr != "" {
		if n, err := strconv.Atoi(rowsStr); err == nil && n > 0 {
			seedRows = n
		}
	}
	shouldSeed := seedFlag == "true" || seedFlag == "1"
	
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".bcl") {
			continue
		}
		name := strings.TrimSuffix(file.Name(), ".bcl")
		path := filepath.Join(c.Driver.MigrationDir(), file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", name, err)
		}
		var cfg Config
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to unmarshal migration file %s: %w", name, err)
		}
		migration := cfg.Migration
		// Add requiredFields check for migration.Name
		if err := requireFields(migration.Name); err != nil {
			return fmt.Errorf("MigrateCommand.Handle: %w", err)
		}
		for _, val := range migration.Validate {
			if err := runPreUpChecks(val.PreUpChecks); err != nil {
				return fmt.Errorf("pre-up validation failed for migration %s: %w", migration.Name, err)
			}
		}
		if err := c.Driver.ApplyMigration(migration); err != nil {
			logger.Error().Msgf("Failed to apply migration %s: %v", migration.Name, err)
			return fmt.Errorf("failed to apply migration %s: %w", migration.Name, err)
		}
		for _, val := range migration.Validate {
			if err := runPostUpChecks(val.PostUpChecks); err != nil {
				return fmt.Errorf("post-up validation failed for migration %s: %w", migration.Name, err)
			}
		}
		
		// --- SEEDING LOGIC ---
		if shouldSeed {
			for _, ct := range migration.Up.CreateTable {
				// Add requiredFields check for ct.Name
				if err := requireFields(ct.Name); err != nil {
					return fmt.Errorf("MigrateCommand.Handle (seed): %w", err)
				}
				seedDef := SeedDefinition{
					Name:  "auto_seed_" + ct.Name,
					Table: ct.Name,
					Rows:  seedRows,
				}
				for _, col := range ct.Columns {
					// Add requiredFields check for col.Name
					if err := requireFields(col.Name); err != nil {
						return fmt.Errorf("MigrateCommand.Handle (seed column): %w", err)
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
					// Map data type to fake_ function
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
						return fmt.Errorf("failed to apply seed for table %s: %w", ct.Name, err)
					}
				}
			}
		}
	}
	return nil
}
