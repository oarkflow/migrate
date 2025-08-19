package migrate

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/oarkflow/json"
	"github.com/oarkflow/squealx"
	"github.com/oarkflow/squealx/drivers/mysql"
	"github.com/oarkflow/squealx/drivers/postgres"
	"github.com/oarkflow/squealx/drivers/sqlite"
)

// isValidIdentifier checks if a string is a valid SQL identifier
// to prevent SQL injection attacks
func isValidIdentifier(name string) bool {
	if name == "" {
		return false
	}
	// Allow alphanumeric characters and underscores, must start with letter or underscore
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	return matched && len(name) <= 64 // reasonable length limit
}

// MigrationHistory holds a migration history record.
type MigrationHistory struct {
	Name        string    `json:"name" db:"name"`
	Version     string    `json:"version" db:"version"`
	Description string    `json:"description" db:"description"`
	Checksum    string    `json:"checksum" db:"checksum"`
	AppliedAt   time.Time `json:"applied_at" db:"applied_at"`
}

// HistoryDriver defines an interface to store migration history.
type HistoryDriver interface {
	Save(history MigrationHistory) error
	Load() ([]MigrationHistory, error)
	ValidateStorage() error
	// New method to remove a migration history record.
	Rollback(history ...MigrationHistory) error
}

// FileHistoryDriver implements the HistoryDriver interface using a file.
type FileHistoryDriver struct {
	filePath string
}

// NewFileHistoryDriver creates a new file-based history driver.
func NewFileHistoryDriver(filePath string) *FileHistoryDriver {
	return &FileHistoryDriver{
		filePath: filePath,
	}
}

func (f *FileHistoryDriver) Save(history MigrationHistory) error {
	histories, err := f.Load()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	histories = append(histories, history)
	data, err := json.Marshal(histories)
	if err != nil {
		return err
	}
	return os.WriteFile(f.filePath, data, 0644)
}

func (f *FileHistoryDriver) Load() ([]MigrationHistory, error) {
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []MigrationHistory{}, nil
		}
		return nil, err
	}
	var histories []MigrationHistory
	if err := json.Unmarshal(data, &histories); err != nil {
		return nil, err
	}
	return histories, nil
}

func (f *FileHistoryDriver) ValidateStorage() error {
	// If file does not exist, create an empty storage file.
	if _, err := os.Stat(f.filePath); os.IsNotExist(err) {
		empty := []byte("[]")
		if err := os.WriteFile(f.filePath, empty, 0644); err != nil {
			return err
		}
	}
	return nil
}

// DatabaseHistoryDriver implements the HistoryDriver interface using a database.
type DatabaseHistoryDriver struct {
	db      *squealx.DB
	dialect string
	table   string
}

func NewDB(dialect, dsn string) (*squealx.DB, error) {
	switch dialect {
	case "postgres":
		return postgres.Open(dsn, "postgres")
	case "mysql":
		return mysql.Open(dsn, "mysql")
	case "sqlite":
		return sqlite.Open(dsn, "sqlite")
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

func SetupMigrationHistoryTable(dialect string, db *squealx.DB, table string) error {
	dial := GetDialect(dialect)
	stmt := CreateTable{
		Name: table,
		Columns: []AddColumn{
			{Name: "id", Type: "number", PrimaryKey: true, AutoIncrement: true, Unique: true, Index: true},
			{Name: "name", Type: "string", Index: true, Size: 200},
			{Name: "version", Type: "string", Size: 10},
			{Name: "description", Type: "string", Size: 500},
			{Name: "checksum", Type: "string", Size: 100},
			{Name: "applied_at", Type: "datetime"},
		},
	}
	existsQuery := dial.TableExistsSQL(table)
	var exists bool
	err := db.Select(&exists, existsQuery)
	if err != nil {
		return err
	}
	if !exists {
		logger.Info().Msg("Setting up migration history table...")
		query, err := dial.CreateTableSQL(stmt, true)
		if err != nil {
			return err
		}
		queries := strings.Split(query, dial.EOS())
		for _, q := range queries {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if _, err := db.Exec(q); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewDatabaseHistoryDriver creates a new database history driver using squealx.
func NewDatabaseHistoryDriver(dialect, dsn string, tables ...string) (HistoryDriver, error) {
	db, err := NewDB(dialect, dsn)
	if err != nil {
		return nil, err
	}
	table := "migrations"
	if len(tables) > 0 {
		table = tables[0]
	}
	err = SetupMigrationHistoryTable(dialect, db, table)
	if err != nil {
		return nil, err
	}
	return &DatabaseHistoryDriver{db: db, dialect: dialect, table: table}, nil
}

func (d *DatabaseHistoryDriver) Save(history MigrationHistory) error {
	dial := GetDialect(d.dialect)
	cols := []string{"name", "version", "description", "checksum", "applied_at"}
	vals := []any{history.Name, history.Version, history.Description, history.Checksum, history.AppliedAt.Format(time.RFC3339)}
	query, args, err := dial.InsertSQL(d.table, cols, vals)
	if err != nil {
		return err
	}
	_, err = d.db.NamedExec(query, args)
	return err
}

func (d *DatabaseHistoryDriver) Load() ([]MigrationHistory, error) {
	var histories []MigrationHistory
	// Use parameterized query to prevent SQL injection
	query := `SELECT id, name, version, description, checksum, applied_at FROM migrations ORDER BY applied_at ASC`
	if d.table != "migrations" {
		// Validate table name to prevent SQL injection
		if !isValidIdentifier(d.table) {
			return nil, fmt.Errorf("invalid table name: %s", d.table)
		}
		query = fmt.Sprintf(`SELECT id, name, version, description, checksum, applied_at FROM "%s" ORDER BY applied_at ASC`, d.table)
	}
	err := d.db.Select(&histories, query)
	if err != nil {
		return nil, err
	}
	return histories, nil
}

func (d *DatabaseHistoryDriver) ValidateStorage() error {
	return SetupMigrationHistoryTable(d.dialect, d.db, d.table)
}

// FileHistoryDriver: implement Rollback by removing the record from the file.
func (f *FileHistoryDriver) Rollback(histories ...MigrationHistory) error {
	data, err := json.Marshal(histories)
	if err != nil {
		return err
	}
	return os.WriteFile(f.filePath, data, 0644)
}

// DatabaseHistoryDriver: implement Rollback by executing a DELETE query.
func (d *DatabaseHistoryDriver) Rollback(histories ...MigrationHistory) error {
	// Validate table name to prevent SQL injection
	if !isValidIdentifier(d.table) {
		return fmt.Errorf("invalid table name: %s", d.table)
	}

	if len(histories) == 0 {
		// Delete all records
		query := fmt.Sprintf(`DELETE FROM "%s"`, d.table)
		_, err := d.db.Exec(query)
		return err
	}

	// Use parameterized query to prevent SQL injection
	placeholders := make([]string, len(histories))
	args := make([]interface{}, len(histories))
	for i, h := range histories {
		placeholders[i] = "?"
		args[i] = h.Name
	}

	query := fmt.Sprintf(`DELETE FROM "%s" WHERE name NOT IN (%s)`, d.table, strings.Join(placeholders, ", "))
	_, err := d.db.Exec(query, args...)
	return err
}
