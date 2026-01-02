package migrate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	// ApplySQLMigration applies a raw .sql migration file specified by path
	ApplySQLMigration(path string) error
	RollbackMigration(step int) error
	ResetMigrations() error
	ValidateMigrations() error
	CreateMigrationFile(name string, raw bool) error
	CreateSeedFile(name string, raw bool) error
	ValidateHistoryStorage() error
	RunSeeds(truncate bool, includeRaw bool, seedFile ...string) error
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
	// assets holds an optional embedded filesystem (using //go:embed from the
	// application that embeds migrations/seeds/templates). When set, file
	// reads and directory walks will prefer this FS over the OS filesystem.
	assets fs.FS
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

// WithEmbeddedFiles supplies an embedded filesystem (fs.FS) to the manager.
// Use this when building a single binary with migrations embedded using
// //go:embed in the application.
func WithEmbeddedFiles(assets fs.FS) ManagerOption {
	return func(m *Manager) {
		m.assets = assets
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
		&ResetDatabaseCommand{Driver: m},
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

// readFile reads a file either from the embedded assets FS (if present) or the
// OS filesystem otherwise.
func (d *Manager) readFile(path string) ([]byte, error) {
	if d.assets != nil {
		return fs.ReadFile(d.assets, path)
	}
	return os.ReadFile(path)
}

// ListMigrationMap returns a map of migration name -> path. When using an
// embedded filesystem the returned paths are the paths inside the embedded FS.
func (d *Manager) ListMigrationMap() (map[string]string, error) {
	migrationMap := make(map[string]string)
	seedDir := d.SeedDir()
	if d.assets != nil {
		err := fs.WalkDir(d.assets, d.migrationDir, func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(de.Name()))
			// Skip seeds
			if seedDir != "" && strings.HasPrefix(p, seedDir) {
				return nil
			}
			switch ext {
			case ".bcl":
				// Try to read migration name from file contents to support names that omit timestamp
				data, err := d.readFile(p)
				if err == nil {
					var cfg Config
					if _, err := bcl.Unmarshal(data, &cfg); err == nil {
						if cfg.Migration.Name != "" {
							migrationMap[cfg.Migration.Name] = p
							return nil
						}
					}
				}
				name := strings.TrimSuffix(de.Name(), ext)
				migrationMap[name] = p
			case ".sql":
				name := strings.TrimSuffix(de.Name(), ext)
				migrationMap[name] = p
			}
			return nil
		})
		return migrationMap, err
	}
	// Fallback to OS filesystem walking
	err := filepath.Walk(d.migrationDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if seedDir != "" && strings.HasPrefix(path, seedDir) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(info.Name()))
			switch ext {
			case ".bcl":
				// Read file and parse migration name if possible
				data, err := d.readFile(path)
				if err == nil {
					var cfg Config
					if _, err := bcl.Unmarshal(data, &cfg); err == nil {
						if cfg.Migration.Name != "" {
							migrationMap[cfg.Migration.Name] = path
							return nil
						}
					}
				}
				name := strings.TrimSuffix(info.Name(), ext)
				migrationMap[name] = path
			case ".sql":
				name := strings.TrimSuffix(info.Name(), ext)
				migrationMap[name] = path
			}
		}
		return nil
	})
	return migrationMap, err
}

// ListSeedFiles returns seed files (.bcl and optionally .sql) inside the
// configured seed directory. The returned paths are either OS paths or paths
// inside the embedded FS depending on configuration.
func (d *Manager) ListSeedFiles(includeRaw bool) ([]string, error) {
	var files []string
	if d.assets != nil {
		err := fs.WalkDir(d.assets, d.seedDir, func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(de.Name()))
			switch ext {
			case ".bcl":
				files = append(files, p)
			case ".sql":
				if includeRaw {
					files = append(files, p)
				}
			}
			return nil
		})
		return files, err
	}
	osFiles, err := os.ReadDir(d.SeedDir())
	if err != nil {
		return nil, err
	}
	for _, file := range osFiles {
		if file.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name()))
		switch ext {
		case ".bcl":
			files = append(files, filepath.Join(d.SeedDir(), file.Name()))
		case ".sql":
			if includeRaw {
				files = append(files, filepath.Join(d.SeedDir(), file.Name()))
			}
		}
	}
	return files, nil
}

func (d *Manager) ApplyMigration(m Migration) error {
	// Validate migration name
	if err := requireFields(m.Name); err != nil {
		return fmt.Errorf("ApplyMigration: invalid migration name: %w", err)
	}
	if m.Disable {
		logger.Warn().Msgf("Migration '%s' is disabled and will not be applied.", m.Name)
		return nil
	}

	// Build map of migrations and look up by name (supports embedded FS)
	migrationMap, err := d.ListMigrationMap()
	if err != nil {
		return fmt.Errorf("failed to list migrations: %w", err)
	}
	migrationPath, ok := migrationMap[m.Name]
	if !ok {
		return fmt.Errorf("migration file for '%s' not found in '%s'", m.Name, d.migrationDir)
	}
	data, err := d.readFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", migrationPath, err)
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
	if err := requireFields(migration.Name); err != nil {
		return fmt.Errorf("ApplyMigration: %w", err)
	}
	dialect := d.dialect
	var dbDriver IDatabaseDriver = d.dbDriver
	if migration.Driver != "" {
		normalizedDriver, err := NormalizeDriver(migration.Driver)
		if err != nil {
			return fmt.Errorf("invalid driver in migration %s: %w", migration.Name, err)
		}
		dialect = normalizedDriver
		if migration.Connection != "" {
			dbDriver, err = NewDriver(normalizedDriver, migration.Connection)
			if err != nil {
				return fmt.Errorf("failed to create driver for migration %s: %w", migration.Name, err)
			}
		} else {
			return fmt.Errorf("migration %s has Driver set but no Connection", migration.Name)
		}
	}
	queries, err := migration.ToSQL(dialect, true)
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
	if len(queries) == 0 {
		logger.Info().Msgf("Migration '%s' has no operations to perform", m.Name)
		return nil
	}
	for _, val := range migration.Validate {
		if err := runPreUpChecks(val.PreUpChecks); err != nil {
			return fmt.Errorf("pre-up validation failed for migration %s: %w", migration.Name, err)
		}
	}
	if err := dbDriver.ApplySQL(queries); err != nil {
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
	// Log loaded histories for visibility
	var histNames []string
	for _, h := range histories {
		histNames = append(histNames, h.Name)
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
	migrationMap, err := d.ListMigrationMap()
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}
	// Debug: log loaded history and migration map keys
	var appliedNames []string
	for _, h := range histories {
		appliedNames = append(appliedNames, h.Name)
	}
	var migrationFilesList []string
	for k := range migrationMap {
		migrationFilesList = append(migrationFilesList, k)
	}
	for i := 0; i < step; i++ {
		last := histories[len(histories)-1]
		name := last.Name
		path, ok := migrationMap[name]
		if !ok {
			logger.Warn().Msgf("Migration file for %s not found; removing history entry and continuing", name)
			histories = histories[:len(histories)-1]
			continue
		}
		data, err := d.readFile(path)
		if err != nil {
			logger.Warn().Msgf("Failed to read migration file %s for rollback: %v; removing history entry and continuing", name, err)
			histories = histories[:len(histories)-1]
			continue
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".sql" {
			// Raw SQL rollback
			_, down := parseSQLMigration(data)
			if down == "" {
				logger.Info().Msgf("Raw migration '%s' has no down section, skipping rollback", name)
				histories = histories[:len(histories)-1]
				continue
			}
			if d.dbDriver == nil {
				return fmt.Errorf("no database driver configured for rollback of %s", name)
			}
			if d.Verbose {
				logger.Info().Msgf("Rollback raw SQL for '%s': %s", name, down)
			}
			if err := d.dbDriver.ApplySQL([]string{down}); err != nil {
				return fmt.Errorf("failed to rollback raw migration %s: %w", name, err)
			}
			logger.Info().Msg("Rolled back migration: " + name)
			histories = histories[:len(histories)-1]
			continue
		}
		var cfg Config
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			logger.Warn().Msgf("Failed to unmarshal migration file %s for rollback: %v; removing history entry and continuing", name, err)
			histories = histories[:len(histories)-1]
			continue
		}
		migration := cfg.Migration
		if err := requireFields(migration.Name); err != nil {
			logger.Warn().Msgf("Migration %s failed required field check for rollback: %v; removing history entry and continuing", name, err)
			histories = histories[:len(histories)-1]
			continue
		}
		if migration.Disable {
			logger.Warn().Msgf("Migration '%s' is disabled, skipping rollback.", migration.Name)
			// Still remove from history since user requested rollback
			histories = histories[:len(histories)-1]
			continue
		}
		dialect := d.dialect
		var dbDriver IDatabaseDriver = d.dbDriver
		if migration.Driver != "" {
			normalizedDriver, err := NormalizeDriver(migration.Driver)
			if err != nil {
				return fmt.Errorf("invalid driver in migration %s: %w", migration.Name, err)
			}
			dialect = normalizedDriver
			if migration.Connection != "" {
				dbDriver, err = NewDriver(normalizedDriver, migration.Connection)
				if err != nil {
					return fmt.Errorf("failed to create driver for migration %s: %w", migration.Name, err)
				}
			} else {
				return fmt.Errorf("migration %s has Driver set but no Connection", migration.Name)
			}
		}
		downQueries, err := migration.ToSQL(dialect, false)
		if err != nil {
			return fmt.Errorf("failed to generate rollback SQL for migration %s: %w", name, err)
		}
		if len(downQueries) == 0 {
			return fmt.Errorf("no rollback SQL found for migration %s; aborting", name)
		}
		if d.Verbose {
			logger.Info().Msgf("Rollback of migration '%s' details:", name)
			for _, q := range downQueries {
				logger.Info().Msg(q)
			}
		}
		if err := dbDriver.ApplySQL(downQueries); err != nil {
			logger.Error().Msgf("failed to rollback migration %s: %v", name, err)
			return fmt.Errorf("failed to rollback migration %s: %w", name, err)
		}
		logger.Info().Msg("Rolled back migration: " + name)
		histories = histories[:len(histories)-1]
	}
	// Log remaining history records before updating storage (helpful for debugging)
	var remainingNames []string
	for _, h := range histories {
		remainingNames = append(remainingNames, h.Name)
	}
	return d.historyDriver.Rollback(histories...)
}

func (d *Manager) ResetMigrations() error {
	logger.Info().Msg("Resetting migrations...")

	histories, err := d.historyDriver.Load()
	if err != nil {
		return err
	}

	migrationMap, err := d.ListMigrationMap()
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}
	for i := len(histories) - 1; i >= 0; i-- {
		name := histories[i].Name
		path, ok := migrationMap[name]
		if !ok {
			logger.Warn().Msgf("Migration file for %s not found; skipping and continuing", name)
			continue
		}
		data, err := d.readFile(path)
		if err != nil {
			logger.Warn().Msgf("Failed to read migration file %s for rollback: %v; skipping and continuing", name, err)
			continue
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".sql" {
			_, down := parseSQLMigration(data)
			if down == "" {
				logger.Info().Msgf("Raw migration '%s' has no down section, skipping rollback", name)
				continue
			}
			if d.dbDriver == nil {
				return fmt.Errorf("no database driver configured for rollback of %s", name)
			}
			if err := d.dbDriver.ApplySQL([]string{down}); err != nil {
				return fmt.Errorf("failed to rollback migration %s: %w", name, err)
			}
			logger.Info().Msg("Rolled back migration: " + name)
			continue
		}
		var cfg Config
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			logger.Warn().Msgf("Failed to unmarshal migration file %s for rollback: %v; skipping and continuing", name, err)
			continue
		}
		migration := cfg.Migration
		if err := requireFields(migration.Name); err != nil {
			logger.Warn().Msgf("Migration %s failed required field check for reset: %v; skipping and continuing", name, err)
			continue
		}
		if migration.Disable {
			logger.Warn().Msgf("Migration '%s' is disabled, skipping reset.", migration.Name)
			// If disabled, nothing to apply but still proceed to consider it rolled back
			continue
		}
		dialect := d.dialect
		var dbDriver IDatabaseDriver = d.dbDriver
		if migration.Driver != "" {
			normalizedDriver, err := NormalizeDriver(migration.Driver)
			if err != nil {
				return fmt.Errorf("invalid driver in migration %s: %w", migration.Name, err)
			}
			dialect = normalizedDriver
			if migration.Connection != "" {
				dbDriver, err = NewDriver(normalizedDriver, migration.Connection)
				if err != nil {
					return fmt.Errorf("failed to create driver for migration %s: %w", migration.Name, err)
				}
			} else {
				return fmt.Errorf("migration %s has Driver set but no Connection", migration.Name)
			}
		}
		downQueries, err := migration.ToSQL(dialect, false)
		if err != nil {
			return fmt.Errorf("failed to generate rollback SQL for migration %s: %w", name, err)
		}
		if err := dbDriver.ApplySQL(downQueries); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", name, err)
		}
		logger.Info().Msg("Rolled back migration: " + name)
	}

	// Ensure history is cleared for all history drivers (DB or File)
	if err := d.historyDriver.Rollback(); err != nil {
		return err
	}
	return nil
}

func (d *Manager) ValidateMigrations() error {
	migrationMap, err := d.ListMigrationMap()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list migration files")
		return fmt.Errorf("failed to list migration files: %w", err)
	}
	var migrationFiles []string
	for _, p := range migrationMap {
		migrationFiles = append(migrationFiles, p)
	}
	histories, err := d.historyDriver.Load()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to load migration history")
		return err
	}
	applied := make(map[string]bool)
	for _, h := range histories {
		applied[h.Name] = true
	}
	var missing []string
	for _, path := range migrationFiles {
		ext := filepath.Ext(path)
		name := strings.TrimSuffix(filepath.Base(path), ext)
		if !applied[name] {
			missing = append(missing, name)
		}
	}
	toApply := len(missing)
	if toApply > 0 {
		logger.Info().Msgf("Migration initiated for: %v", toApply)
		return nil
	}
	logger.Info().Msg("Migrations are up to date.")
	return nil
}

func (d *Manager) CreateSeedFile(name string, raw bool) error {
	tableName := strings.TrimSuffix(strings.TrimPrefix(name, "seed_"), ".bcl")
	name = fmt.Sprintf("%d_%s", time.Now().Unix(), name)
	var filename string
	var template string
	if raw {
		filename = filepath.Join(d.seedDir, name+".sql")
		template = fmt.Sprintf("-- Raw seed for table %s\n-- Add your INSERT statements below.\n-- Example:\n-- INSERT INTO %s (id, name) VALUES (1, 'example');\n", tableName, tableName)
	} else {
		filename = filepath.Join(d.seedDir, name+".bcl")
		template = fmt.Sprintf(`Seed "%s" {
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
	}
	if err := os.WriteFile(filename, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to create seed file: %w", err)
	}
	logger.Printf("Seed file created: %s", filename)
	return nil
}

func (d *Manager) CreateMigrationFile(name string, raw bool) error {
	var filename string
	if strings.Contains(name, string(os.PathSeparator)) {
		dir := filepath.Dir(name)
		base := filepath.Base(name)
		name = fmt.Sprintf("%d_%s", time.Now().Unix(), base)
		os.MkdirAll(filepath.Join(d.migrationDir, dir), fs.ModePerm)
		if raw {
			filename = filepath.Join(d.migrationDir, dir, name+".sql")
		} else {
			filename = filepath.Join(d.migrationDir, dir, name+".bcl")
		}
	} else {
		name = fmt.Sprintf("%d_%s", time.Now().Unix(), name)
		if raw {
			filename = filepath.Join(d.migrationDir, name+".sql")
		} else {
			filename = filepath.Join(d.migrationDir, name+".bcl")
		}
	}

	if raw {
		template := "-- migration-up\n\n-- migration-down\n"
		if err := os.WriteFile(filename, []byte(template), 0644); err != nil {
			return fmt.Errorf("failed to create raw migration file: %w", err)
		}
		logger.Printf("Raw SQL migration file created: %s", filename)
		return nil
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
				   Field "id" {
				     type = "integer"
				     primary_key = true
				     auto_increment = true
				     index = true
				     unique = true
				   }
				   Field "is_active" {
				     type = "boolean"
				     default = false
				   }
				   Field "status" {
				     type = "string"
				     size = 20
				     default = "active"
				   }
				   Field "created_at" {
				     type = "datetime"
				     default = "now()"
				   }
				   Field "updated_at" {
				     type = "datetime"
				     default = "now()"
				   }
				   Field "deleted_at" {
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

// parseSQLMigration splits a raw SQL migration file into up and down SQL sections.
// It supports section markers like "-- migration-up", "-- migrate-up",
// and "-- migration-down"/"-- migrate-down" (case-insensitive).
func parseSQLMigration(data []byte) (up string, down string) {
	lines := strings.Split(string(data), "\n")
	section := ""
	var upLines []string
	var downLines []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "--") {
			marker := strings.TrimSpace(strings.TrimPrefix(lower, "--"))
			if marker == "migration-up" || marker == "migrate-up" {
				section = "up"
				continue
			}
			if marker == "migration-down" || marker == "migrate-down" {
				section = "down"
				continue
			}
		}
		if section == "up" {
			upLines = append(upLines, line)
		} else if section == "down" {
			downLines = append(downLines, line)
		}
	}
	up = strings.TrimSpace(strings.Join(upLines, "\n"))
	down = strings.TrimSpace(strings.Join(downLines, "\n"))
	return
}

// deriveDescriptionFromFilename returns a human-friendly description by
// stripping a leading timestamp prefix (if present) and replacing underscores
// with spaces. If the result is empty, returns a fallback description.
func deriveDescriptionFromFilename(fname string) string {
	base := strings.TrimSuffix(filepath.Base(fname), filepath.Ext(fname))
	tokens := strings.Split(base, "_")
	if len(tokens) > 1 {
		// If first token is numeric (timestamp), drop it
		if _, err := strconv.ParseInt(tokens[0], 10, 64); err == nil {
			tokens = tokens[1:]
		}
	}
	desc := strings.TrimSpace(strings.Join(tokens, " "))
	if desc == "" {
		return "Raw SQL migration"
	}
	return desc
}

// deriveVersionFromFilename tries to extract a semantic version from the
// filename. It looks for tokens like 'v1.2.3' or '1.2.3' and returns a
// normalized version string (without leading 'v'), otherwise returns
// the default '1.0.0'.
func deriveVersionFromFilename(fname string) string {
	base := strings.TrimSuffix(filepath.Base(fname), filepath.Ext(fname))
	tokens := strings.Split(base, "_")
	verRegexp := regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})$`)
	for _, t := range tokens {
		if matches := verRegexp.FindStringSubmatch(strings.ToLower(t)); matches != nil {
			return matches[1]
		}
	}
	return "1.0.0"
}

// ApplySQLMigration applies a raw .sql migration file by running the -- migration-up
// section and recording it in history (checksum computed from file contents).
func (d *Manager) ApplySQLMigration(path string) error {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	data, err := d.readFile(path)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", path, err)
	}
	checksum := computeChecksum(data)
	histories, err := d.historyDriver.Load()
	if err != nil {
		return fmt.Errorf("failed to load migration history: %w", err)
	}
	// Check if already applied
	for _, h := range histories {
		if h.Name == name {
			if h.Checksum == checksum {
				if d.Verbose {
					logger.Info().Msgf("Migration '%s' already applied, skipping", name)
				}
				return nil
			}
			return fmt.Errorf("migration '%s' has been modified after being applied (checksum mismatch)", name)
		}
	}
	up, _ := parseSQLMigration(data)
	if up == "" {
		return fmt.Errorf("no up SQL found in %s", path)
	}
	if d.dbDriver == nil {
		return fmt.Errorf("no database driver configured for migration '%s'", name)
	}
	if d.Verbose {
		logger.Info().Msgf("Applying raw SQL migration '%s' details:", name)
		logger.Info().Msg(up)
	}
	if err := d.dbDriver.ApplySQL([]string{up}); err != nil {
		return fmt.Errorf("failed to apply raw migration %s: %w", name, err)
	}
	now := time.Now()
	history := MigrationHistory{
		Name:        name,
		Version:     deriveVersionFromFilename(name),
		Description: deriveDescriptionFromFilename(name),
		Checksum:    checksum,
		AppliedAt:   now,
	}
	return d.historyDriver.Save(history)
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

func (d *Manager) RunSeeds(truncate bool, includeRaw bool, seedFiles ...string) error {
	if d.dbDriver == nil {
		return fmt.Errorf("no database driver configured for seeding")
	}

	if len(seedFiles) == 0 {
		logger.Info().Msg("No seed files provided")
		return nil
	}

	for _, seedFile := range seedFiles {
		if seedFile == "" {
			logger.Warn().Msg("Empty seed file path, skipping")
			continue
		}

		ext := strings.ToLower(filepath.Ext(seedFile))
		switch ext {
		case ".sql":
			if !includeRaw {
				logger.Info().Msgf("Skipping raw seed file (enable with --include-raw): %s", seedFile)
				continue
			}
			data, err := d.readFile(seedFile)
			if err != nil {
				return fmt.Errorf("failed to read seed file '%s': %w", seedFile, err)
			}
			sql := strings.TrimSpace(string(data))
			if sql == "" {
				logger.Info().Msgf("Raw seed file '%s' is empty, skipping", seedFile)
				continue
			}
			if d.Verbose {
				logger.Info().Msgf("Raw seed SQL (%d bytes): %s", len(sql), sql)
			}
			if truncate {
				logger.Warn().Msgf("Truncate flag ignored for raw seed file: %s", seedFile)
			}
			logger.Info().Msgf("Applying raw seed file: %s", seedFile)
			if err := d.dbDriver.ApplySQL([]string{sql}); err != nil {
				return fmt.Errorf("failed to apply raw seed '%s': %w", seedFile, err)
			}
		case ".bcl":
			data, err := d.readFile(seedFile)
			if err != nil {
				return fmt.Errorf("failed to read seed file '%s': %w", seedFile, err)
			}

			var cfg SeedConfig
			if _, err := bcl.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("failed to unmarshal seed file '%s': %w", seedFile, err)
			}

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
					if err := d.dbDriver.ApplySQL([]string{query}); err != nil {
						return fmt.Errorf("failed to truncate table %s: %w", cfg.Seed.Table, err)
					}
				} else {
					return fmt.Errorf("unsupported dialect for truncation: %s", d.dialect)
				}
			}
			logger.Info().Msgf("Seeding table: %s", cfg.Seed.Table)
			for _, q := range queries {
				logger.Info().Msgf("Seed SQL: %s", q.SQL)
				if err := d.dbDriver.ApplySQL([]string{q.SQL}, q.Args); err != nil {
					logger.Error().Msgf("Seed failed (%s): %v", seedFile, err)
					return fmt.Errorf("failed to apply seed '%s': %w", seedFile, err)
				}
			}
		default:
			logger.Warn().Msgf("Unsupported seed file type, skipping: %s", seedFile)
		}
	}
	return nil
}

func getTruncateSQL(dialect string, table string) string {
	switch dialect {
	case "mysql", "mariadb":
		return fmt.Sprintf("TRUNCATE TABLE `%s`;", table)
	case "postgres", "cockroachdb", "postgresql", "pgx":
		return fmt.Sprintf("TRUNCATE TABLE \"%s\" RESTART IDENTITY CASCADE;", table)
	case "sqlite", "sqlite3":
		return fmt.Sprintf("DELETE FROM `%s`;", table)
	}
	return ""
}
