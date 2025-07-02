package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/oarkflow/bcl"
	"github.com/oarkflow/cli"
	"github.com/oarkflow/cli/console"
	"github.com/oarkflow/cli/contracts"
	"github.com/oarkflow/log"
	"github.com/oarkflow/squealx"
)

var (
	Name    = "Migration"
	Version = "v0.0.1"
)

var logger = log.Logger{
	TimeFormat: "15:04:05",
	Caller:     1,
	Writer: &log.ConsoleWriter{
		ColorOutput:    true,
		QuoteString:    true,
		EndWithMessage: true,
	},
}

type IDatabaseDriver interface {
	ApplySQL(queries []string, args ...any) error
	DB() *squealx.DB
}

type IManager interface {
	MigrationDir() string
	SeedDir() string
	ApplyMigration(m Migration) error
	RollbackMigration(step int) error
	ResetMigrations() error
	ValidateMigrations() error
	CreateMigrationFile(name string) error
	CreateSeedFile(name string) error
	ValidateHistoryStorage() error
	RunSeeds(truncate bool, seedFile ...string) error
}

type Manager struct {
	migrationDir  string
	seedDir       string
	dialect       string
	client        contracts.Cli
	dbDriver      IDatabaseDriver
	historyDriver HistoryDriver
	Verbose       bool
}

type ManagerOption func(*Manager)

func WithMigrationDir(dir string) ManagerOption {
	return func(m *Manager) {
		m.migrationDir = dir
	}
}

func WithSeedDir(dir string) ManagerOption {
	return func(m *Manager) {
		m.seedDir = dir
	}
}

func WithDriver(driver IDatabaseDriver) ManagerOption {
	return func(m *Manager) {
		m.dbDriver = driver
	}
}

func WithHistoryDriver(driver HistoryDriver) ManagerOption {
	return func(m *Manager) {
		m.historyDriver = driver
	}
}

func WithDialect(dialect string) ManagerOption {
	return func(m *Manager) {
		m.dialect = dialect
	}
}

func defaultManager(client contracts.Cli) *Manager {
	return &Manager{
		migrationDir:  "migrations",
		seedDir:       "migrations/seeds",
		dialect:       "postgres",
		historyDriver: NewFileHistoryDriver("migration_history.txt"),
		client:        client,
	}
}

func NewManager(opts ...ManagerOption) *Manager {

	cli.SetName(Name)
	cli.SetVersion(Version)
	app := cli.New()
	client := app.Instance.Client()
	m := defaultManager(client)
	for _, opt := range opts {
		opt(m)
	}
	if err := os.MkdirAll(m.migrationDir, fs.ModePerm); err != nil {
		logger.Fatal().Msgf("Failed to create migration directory: %v", err)
	}
	if err := os.MkdirAll(m.seedDir, fs.ModePerm); err != nil {
		logger.Fatal().Msgf("Failed to create migration directory: %v", err)
	}
	client.Register([]contracts.Command{
		console.NewListCommand(client),
		&MakeMigrationCommand{Driver: m},
		&MigrateCommand{Driver: m},
		&RollbackCommand{Driver: m},
		&ResetCommand{Driver: m},
		&ValidateCommand{Driver: m},
		&SeedCommand{Driver: m},
		&MakeSeedCommand{Driver: m},
	})
	return m
}

func (d *Manager) Run() {
	d.client.Run(os.Args, true)
}

func (d *Manager) SetDialect(dialect string) {
	d.dialect = dialect
}

func (d *Manager) GetDialect() string {
	return d.dialect
}

func (d *Manager) MigrationDir() string {
	return d.migrationDir
}

func (d *Manager) SeedDir() string {
	return d.seedDir
}

func (d *Manager) ValidateHistoryStorage() error {
	return d.historyDriver.ValidateStorage()
}

func (d *Manager) ApplyMigration(m Migration) error {
	path := filepath.Join(d.migrationDir, m.Name+".bcl")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}
	checksum := computeChecksum(data)
	histories, err := d.historyDriver.Load()
	if err != nil {
		return err
	}
	for _, h := range histories {
		if h.Name == m.Name {
			if h.Checksum == checksum {
				return nil
			}
			return errors.New("changes found in migration file after migration was applied")
		}
	}
	var cfg Config
	if _, err := bcl.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal migration file: %w", err)
	}
	migration := cfg.Migration
	queries, err := migration.ToSQL(d.dialect, true)
	if err != nil {
		return fmt.Errorf("failed to generate SQL: %w", err)
	}
	if d.Verbose {
		logger.Info().Msgf("Migration '%s' details:", m.Name)
		for _, q := range queries {
			logger.Info().Msg(q)
		}
	}
	if d.dbDriver == nil {
		return fmt.Errorf("no database driver configured")
	}
	for _, val := range migration.Validate {
		if err := runPreUpChecks(val.PreUpChecks); err != nil {
			return fmt.Errorf("pre-up validation failed for migration %s: %w", migration.Name, err)
		}
	}
	if err := d.dbDriver.ApplySQL(queries); err != nil {
		return fmt.Errorf("failed to apply migration %s: %w", m.Name, err)
	}
	for _, val := range migration.Validate {
		if err := runPostUpChecks(val.PostUpChecks); err != nil {
			return fmt.Errorf("post-up validation failed for migration %s: %w", migration.Name, err)
		}
	}
	now := time.Now()
	logger.Info().Msgf("Applied migration: %s at %v", m.Name, now.Format(time.DateTime))
	history := MigrationHistory{
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Checksum:    checksum,
		AppliedAt:   now,
	}
	return d.historyDriver.Save(history)
}

func (d *Manager) RollbackMigration(step int) error {
	histories, err := d.historyDriver.Load()
	if err != nil {
		return err
	}
	total := len(histories)
	if step <= 0 || step > total {
		return fmt.Errorf("rollback step %d is out of range, total applied: %d", step, total)
	}
	for i := 0; i < step; i++ {
		last := histories[len(histories)-1]
		name := last.Name
		path := filepath.Join(d.migrationDir, name+".bcl")
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s for rollback: %w", name, err)
		}
		var cfg Config
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to unmarshal migration file %s for rollback: %w", name, err)
		}
		migration := cfg.Migration
		downQueries, err := migration.ToSQL(d.dialect, false)
		if err != nil {
			return fmt.Errorf("failed to generate rollback SQL for migration %s: %w", name, err)
		}
		// If in verbose mode, show rollback details.
		if d.Verbose {
			logger.Info().Msgf("Rollback of migration '%s' details:", name)
			for _, q := range downQueries {
				logger.Info().Msg(q)
			}
		}
		if err := d.dbDriver.ApplySQL(downQueries); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", name, err)
		}
		logger.Info().Msg("Rolled back migration: " + name)
		histories = histories[:len(histories)-1]
	}
	return d.historyDriver.Rollback(histories...)
}

func (d *Manager) ResetMigrations() error {
	logger.Info().Msg("Resetting migrations...")

	histories, err := d.historyDriver.Load()
	if err != nil {
		return err
	}

	for i := len(histories) - 1; i >= 0; i-- {
		name := histories[i].Name
		path := filepath.Join(d.migrationDir, name+".bcl")
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s for rollback: %w", name, err)
		}
		var cfg Config
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to unmarshal migration file %s for rollback: %w", name, err)
		}
		migration := cfg.Migration
		downQueries, err := migration.ToSQL(d.dialect, false)
		if err != nil {
			return fmt.Errorf("failed to generate rollback SQL for migration %s: %w", name, err)
		}
		if err := d.dbDriver.ApplySQL(downQueries); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", name, err)
		}
		logger.Info().Msg("Rolled back migration: " + name)
	}

	if fh, ok := d.historyDriver.(*FileHistoryDriver); ok {
		return os.WriteFile(fh.filePath, []byte("[]"), 0644)
	}
	return nil
}

func (d *Manager) ValidateMigrations() error {
	files, err := os.ReadDir(d.migrationDir)
	if err != nil {
		return fmt.Errorf("failed to read migration directory: %w", err)
	}
	histories, err := d.historyDriver.Load()
	if err != nil {
		return err
	}

	applied := make(map[string]bool)
	for _, h := range histories {
		applied[h.Name] = true
	}
	var missing []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".bcl") {
			name := strings.TrimSuffix(file.Name(), ".bcl")
			if !applied[name] {
				missing = append(missing, name)
			}
		}
	}
	toApply := len(missing)
	if toApply > 0 {
		logger.Info().Msgf("Migration initiated for: %v migration(s)", toApply)
		return nil
	}
	logger.Info().Msg("Migrations are up to date.")
	return nil
}

func (d *Manager) CreateSeedFile(name string) error {
	tableName := strings.TrimSuffix(strings.TrimPrefix(name, "seed_"), ".bcl")
	name = fmt.Sprintf("%d_%s", time.Now().Unix(), name)
	filename := filepath.Join(d.seedDir, name+".bcl")
	template := fmt.Sprintf(`Seed "%s" {
    table = "%s"
    Field "id" {
        value = "fake_uuid"
        unique = true
    }
    Field "is_active" {
        value = true
    }
	rows = 2
}`, name, tableName)
	if err := os.WriteFile(filename, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to create seed file: %w", err)
	}
	logger.Printf("Seed file created: %s", filename)
	return nil
}

func (d *Manager) CreateMigrationFile(name string) error {
	name = fmt.Sprintf("%d_%s", time.Now().Unix(), name)
	filename := filepath.Join(d.migrationDir, name+".bcl")
	tokens := strings.Split(name, "_")
	var template string
	if len(tokens) < 2 {
		template = defaultTemplate(name)
	} else {
		op := strings.ToLower(tokens[1])
		objType := ""
		if len(tokens) > 2 {
			last := strings.ToLower(tokens[len(tokens)-1])
			if last == "table" || last == "view" || last == "function" || last == "trigger" {
				objType = last
			}
		}
		switch op {
		case "create":
			switch objType {
			case "view":
				viewName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Create view %s."
  Connection = "default"
  Up {
    CreateView "%s" {
      # Define view SQL query.
    }
  }
  Down {
    DropView "%s" {
      Cascade = true
    }
  }
}`, name, viewName, viewName, viewName)
			case "function":

				funcName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Create function %s."
  Connection = "default"
  Up {
    CreateFunction "%s" {
      # Define function signature and body.
    }
  }
  Down {
    DropFunction "%s" {
      Cascade = true
    }
  }
}`, name, funcName, funcName, funcName)
			case "trigger":
				triggerName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Create trigger %s."
  Connection = "default"
  Up {
    CreateTrigger "%s" {
      # Define trigger logic.
    }
  }
  Down {
    DropTrigger "%s" {
      Cascade = true
    }
  }
}`, name, triggerName, triggerName, triggerName)
			default:
				var table string
				if objType == "table" {
					table = strings.Join(tokens[2:len(tokens)-1], "_")
				} else {
					table = strings.Join(tokens[2:], "_")
				}
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Create table %s."
  Connection = "default"
  Up {
    CreateTable "%s" {
      Column "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
        index = true
        unique = true
      }
      Column "is_active" {
        type = "boolean"
        default = false
      }
      Column "status" {
        type = "string"
        size = 20
        default = "active"
      }
      Column "created_at" {
        type = "datetime"
        default = "now()"
      }
      Column "updated_at" {
        type = "datetime"
        default = "now()"
      }
      Column "deleted_at" {
        type = "datetime"
        nullable = true
      }
    }
  }
  Down {
    DropTable "%s" {
      Cascade = true
    }
  }
}
`, name, table, table, table)
			}
		case "alter":
			switch objType {
			case "view":
				viewName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Alter view %s."
  Connection = "default"
  Up {
    AlterView "%s" {
      # Define alterations for view.
    }
  }
  Down {
    # Define rollback for view alterations.
  }
}`, name, viewName, viewName)
			case "function":
				funcName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Alter function %s."
  Connection = "default"
  Up {
    AlterFunction "%s" {
      # Define function alterations.
    }
  }
  Down {
    # Define rollback for function alterations.
  }
}`, name, funcName, funcName)
			case "trigger":
				triggerName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Alter trigger %s."
  Connection = "default"
  Up {
    AlterTrigger "%s" {
      # Define trigger alterations.
    }
  }
  Down {
    # Define rollback for trigger alterations.
  }
}`, name, triggerName, triggerName)
			default:

				var table string
				if objType == "table" {
					table = strings.Join(tokens[2:len(tokens)-1], "_")
				} else {
					table = strings.Join(tokens[2:], "_")
				}
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Alter table %s."
  Connection = "default"
  Up {
    AlterTable "%s" {
      # Define alterations here.
    }
  }
  Down {
    # Define rollback for alterations.
  }
}`, name, table, table)
			}
		case "drop":
			switch objType {
			case "view":
				viewName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Drop view %s."
  Connection = "default"
  Up {
    DropView "%s" {
      Cascade = true
    }
  }
  Down {
    # Optionally define rollback for view drop.
  }
}`, name, viewName, viewName)
			case "function":
				funcName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Drop function %s."
  Connection = "default"
  Up {
    DropFunction "%s" {
      Cascade = true
    }
  }
  Down {
    # Optionally define rollback for function drop.
  }
}`, name, funcName, funcName)
			case "trigger":
				triggerName := strings.Join(tokens[2:len(tokens)-1], "_")
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Drop trigger %s."
  Connection = "default"
  Up {
    DropTrigger "%s" {
      Cascade = true
    }
  }
  Down {
    # Optionally define rollback for trigger drop.
  }
}`, name, triggerName, triggerName)
			default:

				var table string
				if objType == "table" {
					table = strings.Join(tokens[2:len(tokens)-1], "_")
				} else {
					table = strings.Join(tokens[2:], "_")
				}
				template = fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "Drop table %s."
  Connection = "default"
  Up {
    DropTable "%s" {
      Cascade = true
    }
  }
  Down {
    # Optionally define rollback for table drop.
  }
}`, name, table, table)
			}
		default:
			template = defaultTemplate(name)
		}
	}
	if err := os.WriteFile(filename, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}
	logger.Printf("Migration file created: %s", filename)
	return nil
}

func defaultTemplate(name string) string {
	return fmt.Sprintf(`Migration "%s" {
  Version = "1.0.0"
  Description = "New migration"
  Connection = "default"
  Up {
    # Define migration operations here.
  }
  Down {
    # Define rollback operations here.
  }
}`, name)
}

type MakeMigrationCommand struct {
	Driver IManager
}

func (c *MakeMigrationCommand) Signature() string {
	return "make:migration"
}

func (c *MakeMigrationCommand) Description() string {
	return "Creates a new migration file in the designated directory."
}

func (c *MakeMigrationCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
		},
	}
}

func (c *MakeMigrationCommand) Handle(ctx contracts.Context) error {
	name := ctx.Argument(0)
	if name == "" {
		return errors.New("migration name is required")
	}
	return c.Driver.CreateMigrationFile(name)
}

type MakeSeedCommand struct {
	Driver IManager
}

func (c *MakeSeedCommand) Signature() string {
	return "make:seed"
}

func (c *MakeSeedCommand) Description() string {
	return "Creates a new seed file for table in the designated directory."
}

func (c *MakeSeedCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
		},
	}
}

func (c *MakeSeedCommand) Handle(ctx contracts.Context) error {
	name := ctx.Argument(0)
	if name == "" {
		return errors.New("seed name is required")
	}
	return c.Driver.CreateSeedFile(name)
}

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

func acquireLock() error {
	if _, err := os.Stat(lockFileName); err == nil {
		return fmt.Errorf("migration lock already acquired")
	}
	f, err := os.Create(lockFileName)
	if err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	f.Close()
	return nil
}

func releaseLock() error {
	if err := os.Remove(lockFileName); err != nil {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}
	return nil
}

func runPreUpChecks(checks []string) error {
	for _, check := range checks {
		logger.Printf("Executing PreUpCheck: %s", check)

		if strings.Contains(strings.ToLower(check), "fail") {
			return fmt.Errorf("PreUp check failed: %s", check)
		}
	}
	logger.Info().Msg("All PreUpChecks passed.")
	return nil
}

func runPostUpChecks(checks []string) error {
	for _, check := range checks {
		logger.Printf("Executing PostUpCheck: %s", check)

		if strings.Contains(strings.ToLower(check), "fail") {
			return fmt.Errorf("PostUp check failed: %s", check)
		}
	}
	logger.Info().Msg("All PostUpChecks passed.")
	return nil
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
				seedDef := SeedDefinition{
					Name:  "auto_seed_" + ct.Name,
					Table: ct.Name,
					Rows:  seedRows,
				}
				for _, col := range ct.Columns {
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

type RollbackCommand struct {
	Driver IManager
}

func (c *RollbackCommand) Signature() string {
	return "migration:rollback"
}

func (c *RollbackCommand) Description() string {
	return "Rolls back migrations. Optionally specify --step=<n>."
}

func (c *RollbackCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
		},
	}
}

func (c *RollbackCommand) Handle(ctx contracts.Context) error {
	verbose := ctx.Option("v") != "" && ctx.Option("v") != "false"
	if mgr, ok := c.Driver.(*Manager); ok {
		mgr.Verbose = verbose
	}
	stepStr := ctx.Option("step")
	step := 1
	if stepStr != "" {
		var err error
		step, err = strconv.Atoi(stepStr)
		if err != nil {
			return fmt.Errorf("invalid step value: %w", err)
		}
	}
	return c.Driver.RollbackMigration(step)
}

type ResetCommand struct {
	Driver IManager
}

func (c *ResetCommand) Signature() string {
	return "migration:reset"
}

func (c *ResetCommand) Description() string {
	return "Resets migrations by rolling back and reapplying all migrations."
}

func (c *ResetCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
		},
	}
}

func (c *ResetCommand) Handle(ctx contracts.Context) error {
	return c.Driver.ResetMigrations()
}

type ValidateCommand struct {
	Driver IManager
}

func (c *ValidateCommand) Signature() string {
	return "migration:validate"
}

func (c *ValidateCommand) Description() string {
	return "Validates the migration history against migration files."
}

func (c *ValidateCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
				Value:   "false",
			},
		},
	}
}

func (c *ValidateCommand) Handle(ctx contracts.Context) error {
	return c.Driver.ValidateMigrations()
}

type SeedCommand struct {
	Driver IManager
}

func (c *SeedCommand) Signature() string {
	return "db:seed"
}

func (c *SeedCommand) Description() string {
	return "Run database seeds from a seed file."
}

func (c *SeedCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "Seed file to run",
				Value:   "",
			},
			{
				Name:    "truncate",
				Aliases: []string{"t"},
				Usage:   "Truncate tables before seeding",
				Value:   "false",
			},
		},
	}
}

func (c *SeedCommand) Handle(ctx contracts.Context) error {
	var files []string
	seedFile := ctx.Option("file")
	truncateOption := ctx.Option("truncate")
	truncate := truncateOption == "true" || truncateOption == "1"
	if seedFile != "" {
		files = append(files, seedFile)
	} else {
		osFiles, _ := os.ReadDir(c.Driver.SeedDir())
		for _, file := range osFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".bcl") {
				files = append(files, filepath.Join(c.Driver.SeedDir(), file.Name()))
			}
		}
	}
	if len(files) == 0 {
		logger.Printf("No seed files found in %s", c.Driver.SeedDir())
		return nil
	}
	return c.Driver.RunSeeds(truncate, files...)
}

func (d *Manager) RunSeeds(truncate bool, seedFiles ...string) error {
	for _, seedFile := range seedFiles {
		data, err := os.ReadFile(seedFile)
		if err != nil {
			return fmt.Errorf("failed to read seed file: %w", err)
		}
		var cfg SeedConfig
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to unmarshal seed file: %w", err)
		}
		queries, err := cfg.Seed.ToSQL(d.dialect)
		if err != nil {
			return fmt.Errorf("failed to generate seed SQL: %w", err)
		}
		if d.dbDriver == nil {
			return fmt.Errorf("no database driver configured")
		}
		if truncate {
			query := getTruncateSQL(d.dialect, cfg.Seed.Table)
			if query != "" {
				logger.Info().Msgf("Truncating table: %s", cfg.Seed.Table)
				if d.Verbose {
					logger.Info().Msgf("Truncate SQL: %s", query)
				}
				err := d.dbDriver.ApplySQL([]string{query})
				if err != nil {
					return fmt.Errorf("failed to truncate table %s: %w", cfg.Seed.Table, err)
				}
			} else {
				return fmt.Errorf("unsupported dialect for truncation: %s", d.dialect)
			}
		}
		logger.Info().Msgf("Seeding table: %s", cfg.Seed.Table)
		for _, q := range queries {
			logger.Info().Msgf("Seed SQL: %s", q.SQL)
			err := d.dbDriver.ApplySQL([]string{q.SQL}, q.Args)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getTruncateSQL(dialect string, table string) string {
	switch dialect {
	case "mysql", "mariadb":
		return fmt.Sprintf("TRUNCATE TABLE `%s`;", table)
	case "postgres", "cockroachdb":
		return fmt.Sprintf("TRUNCATE TABLE \"%s\" RESTART IDENTITY CASCADE;", table)
	case "sqlite", "sqlite3":
		return fmt.Sprintf("DELETE FROM `%s`;", table)
	}
	return ""
}
