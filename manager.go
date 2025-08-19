package migrate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oarkflow/bcl"
	"github.com/oarkflow/cli"
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
	command       []contracts.Command
}

type ManagerOption func(*Manager)

func WithMigrationDir(dir string) ManagerOption {
	return func(m *Manager) {
		m.migrationDir = dir
	}
}

func WithCommands(commands ...contracts.Command) ManagerOption {
	return func(m *Manager) {
		m.command = append(m.command, commands...)
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

func WithConfig(config *MigrateConfig) ManagerOption {
	return func(m *Manager) {
		// Apply configuration settings
		m.migrationDir = config.Migration.Directory
		m.seedDir = config.Seed.Directory
		m.dialect = config.Database.Driver
		m.Verbose = config.Logging.Verbose

		// Set up database driver if configuration is complete
		if config.Database.Driver != "" && config.Database.Database != "" {
			dsn := config.GetDSN()
			if dsn != "" {
				driver, err := NewDriver(config.Database.Driver, dsn)
				if err == nil {
					m.dbDriver = driver

					// Set up history driver
					historyDriver, err := NewHistoryDriver("db", config.Database.Driver, dsn, config.Migration.TableName)
					if err == nil {
						m.historyDriver = historyDriver
					}
				}
			}
		}
	}
}

func WithClient(client contracts.Cli) ManagerOption {
	return func(m *Manager) {
		m.client = client
	}
}

func defaultManager() *Manager {
	return &Manager{
		migrationDir:  "migrations",
		seedDir:       "migrations/seeds",
		dialect:       "postgres",
		historyDriver: NewFileHistoryDriver("migration_history.txt"),
	}
}

func NewManager(opts ...ManagerOption) *Manager {
	m := defaultManager()
	for _, opt := range opts {
		opt(m)
	}
	if err := os.MkdirAll(m.migrationDir, fs.ModePerm); err != nil {
		logger.Fatal().Msgf("Failed to create migration directory: %v", err)
	}
	if err := os.MkdirAll(m.seedDir, fs.ModePerm); err != nil {
		logger.Fatal().Msgf("Failed to create migration directory: %v", err)
	}
	return m
}

func GetCommands(m *Manager) []contracts.Command {
	return []contracts.Command{
		&MakeMigrationCommand{Driver: m},
		&MigrateCommand{Driver: m},
		&RollbackCommand{Driver: m},
		&ResetCommand{Driver: m},
		&ValidateCommand{Driver: m},
		&SeedCommand{Driver: m},
		&MakeSeedCommand{Driver: m},
		&HistoryCommand{Driver: m},
		&ConfigCommand{Driver: m},
		&ConfigInitCommand{Driver: m},
		&ConfigValidateCommand{Driver: m},
		&ConfigShowCommand{Driver: m},
		&StatusCommand{Driver: m},
	}
}

// NewManagerFromConfig creates a new manager from configuration file
func NewManagerFromConfig(configPath string, opts ...ManagerOption) (*Manager, error) {
	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply environment overrides
	config.ApplyEnvironmentOverrides()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create manager with configuration
	allOpts := []ManagerOption{WithConfig(config)}
	allOpts = append(allOpts, opts...)

	manager := NewManager(allOpts...)
	return manager, nil
}

func (d *Manager) Run(clients ...contracts.Cli) {
	var client contracts.Cli
	if len(clients) > 0 {
		client = clients[0]
	} else if d.client != nil {
		client = d.client
	} else {
		cli.SetName(Name)
		cli.SetVersion(Version)
		app := cli.New()
		client = app.Instance.Client()
	}
	cmds := append(GetCommands(d), d.command...)
	client.Register(cmds)
	client.Run(os.Args, true)
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
	// Validate migration name
	if err := requireFields(m.Name); err != nil {
		return fmt.Errorf("ApplyMigration: invalid migration name: %w", err)
	}

	path := filepath.Join(d.migrationDir, m.Name+".bcl")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", path, err)
	}

	checksum := computeChecksum(data)
	histories, err := d.historyDriver.Load()
	if err != nil {
		return fmt.Errorf("failed to load migration history: %w", err)
	}

	// Check if migration already applied
	for _, h := range histories {
		if h.Name == m.Name {
			if h.Checksum == checksum {
				if d.Verbose {
					logger.Info().Msgf("Migration '%s' already applied, skipping", m.Name)
				}
				return nil
			}
			return fmt.Errorf("migration '%s' has been modified after being applied (checksum mismatch)", m.Name)
		}
	}
	var cfg Config
	if _, err := bcl.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal migration file: %w", err)
	}
	migration := cfg.Migration
	// Add requiredFields check for migration.Name
	if err := requireFields(migration.Name); err != nil {
		return fmt.Errorf("ApplyMigration: %w", err)
	}
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
		return fmt.Errorf("no database driver configured for migration '%s'", m.Name)
	}

	// Validate that we have operations to perform
	if len(queries) == 0 {
		logger.Info().Msgf("Migration '%s' has no operations to perform", m.Name)
		return nil
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
	if d.dbDriver == nil {
		return fmt.Errorf("no database driver configured for rollback")
	}

	histories, err := d.historyDriver.Load()
	if err != nil {
		return fmt.Errorf("failed to load migration history: %w", err)
	}

	total := len(histories)
	if total == 0 {
		logger.Info().Msg("No migrations to rollback")
		return nil
	}

	if step <= 0 {
		return fmt.Errorf("rollback step must be positive, got: %d", step)
	}

	if step > total {
		logger.Info().Msgf("Requested rollback steps (%d) exceeds total applied migrations (%d), rolling back all", step, total)
		step = total
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
		// Add requiredFields check for migration.Name
		if err := requireFields(migration.Name); err != nil {
			return fmt.Errorf("RollbackMigration: %w", err)
		}
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
		// Add requiredFields check for migration.Name
		if err := requireFields(migration.Name); err != nil {
			return fmt.Errorf("ResetMigrations: %w", err)
		}
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
	var filename string
	if strings.Contains(name, string(os.PathSeparator)) {
		dir := filepath.Dir(name)
		name = filepath.Base(name)
		name = fmt.Sprintf("%d_%s", time.Now().Unix(), name)
		os.MkdirAll(filepath.Join(d.migrationDir, dir), fs.ModePerm)
		filename = filepath.Join(d.migrationDir, dir, name+".bcl")
	} else {
		filename = filepath.Join(d.migrationDir, name+".bcl")
	}

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

func (d *Manager) RunSeeds(truncate bool, seedFiles ...string) error {
	if d.dbDriver == nil {
		return fmt.Errorf("no database driver configured for seeding")
	}

	if len(seedFiles) == 0 {
		logger.Info().Msg("No seed files provided")
		return nil
	}

	for _, seedFile := range seedFiles {
		// Validate seed file path
		if seedFile == "" {
			logger.Warn().Msg("Empty seed file path, skipping")
			continue
		}

		data, err := os.ReadFile(seedFile)
		if err != nil {
			return fmt.Errorf("failed to read seed file '%s': %w", seedFile, err)
		}

		var cfg SeedConfig
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to unmarshal seed file '%s': %w", seedFile, err)
		}

		// Validate seed configuration
		if err := requireFields(cfg.Seed.Name, cfg.Seed.Table); err != nil {
			return fmt.Errorf("invalid seed configuration in '%s': %w", seedFile, err)
		}

		queries, err := cfg.Seed.ToSQL(d.dialect)
		if err != nil {
			return fmt.Errorf("failed to generate seed SQL for '%s': %w", seedFile, err)
		}

		if len(queries) == 0 {
			logger.Info().Msgf("Seed file '%s' generated no queries, skipping", seedFile)
			continue
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
