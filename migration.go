package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/oarkflow/migrate/drivers"
)

const (
	DialectPostgres = "postgres"
	DialectMySQL    = "mysql"
	DialectSQLite   = "sqlite"
	lockFileName    = "migration.lock"
)

var tableSchemas = make(map[string]*CreateTable)
var schemaMutex sync.Mutex

type Config struct {
	Migration Migration `json:"Migration"`
}

type Migration struct {
	Name        string        `json:"name"`
	Version     string        `json:"Version"`
	Description string        `json:"Description"`
	Connection  string        `json:"Connection"`
	Up          Operation     `json:"Up"`
	Down        Operation     `json:"Down"`
	Transaction []Transaction `json:"Transaction"`
	Validate    []Validation  `json:"Validate"`
}

type Operation struct {
	Name                 string                 `json:"name"`
	AlterTable           []AlterTable           `json:"AlterTable,omitempty"`
	CreateTable          []CreateTable          `json:"CreateTable,omitempty"`
	DeleteData           []DeleteData           `json:"DeleteData,omitempty"`
	DropEnumType         []DropEnumType         `json:"DropEnumType,omitempty"`
	DropRowPolicy        []DropRowPolicy        `json:"DropRowPolicy,omitempty"`
	DropMaterializedView []DropMaterializedView `json:"DropMaterializedView,omitempty"`
	DropTable            []DropTable            `json:"DropTable,omitempty"`
	DropSchema           []DropSchema           `json:"DropSchema,omitempty"`
	RenameTable          []RenameTable          `json:"RenameTable,omitempty"`
	CreateView           []CreateView           `json:"CreateView,omitempty"`
	DropView             []DropView             `json:"DropView,omitempty"`
	RenameView           []RenameView           `json:"RenameView,omitempty"`
	CreateFunction       []CreateFunction       `json:"CreateFunction,omitempty"`
	DropFunction         []DropFunction         `json:"DropFunction,omitempty"`
	RenameFunction       []RenameFunction       `json:"RenameFunction,omitempty"`
	CreateProcedure      []CreateProcedure      `json:"CreateProcedure,omitempty"`
	DropProcedure        []DropProcedure        `json:"DropProcedure,omitempty"`
	RenameProcedure      []RenameProcedure      `json:"RenameProcedure,omitempty"`
	CreateTrigger        []CreateTrigger        `json:"CreateTrigger,omitempty"`
	DropTrigger          []DropTrigger          `json:"DropTrigger,omitempty"`
	RenameTrigger        []RenameTrigger        `json:"RenameTrigger,omitempty"`
}

type AlterTable struct {
	Name         string         `json:"name"`
	AddColumn    []AddColumn    `json:"AddColumn"`
	DropColumn   []DropColumn   `json:"DropColumn"`
	RenameColumn []RenameColumn `json:"RenameColumn"`
}

type CreateTable struct {
	Name       string      `json:"name"`
	Columns    []AddColumn `json:"Column"`
	PrimaryKey []string    `json:"PrimaryKey,omitempty"`
}

func (ct CreateTable) ToSQL(dialect string, up bool) (string, error) {
	if err := requireFields(ct.Name); err != nil {
		return "", fmt.Errorf("CreateTable: %w", err)
	}
	if len(ct.Columns) == 0 {
		return "", fmt.Errorf("CreateTable requires at least one column")
	}
	return GetDialect(dialect).CreateTableSQL(ct, up)
}

type AddColumn struct {
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	Nullable      bool        `json:"nullable"`
	Default       any         `json:"default,omitempty"`
	Check         string      `json:"check,omitempty"`
	Size          int         `json:"size,omitempty"`
	Scale         int         `json:"scale,omitempty"`
	AutoIncrement bool        `json:"auto_increment,omitempty"`
	PrimaryKey    bool        `json:"primary_key,omitempty"`
	Unique        bool        `json:"unique,omitempty"`
	Index         bool        `json:"index,omitempty"`
	ForeignKey    *ForeignKey `json:"foreign_key,omitempty"`
}

type ForeignKey struct {
	ReferenceTable  string `json:"reference_table"`
	ReferenceColumn string `json:"reference_column"`
	OnDelete        string `json:"on_delete,omitempty"`
	OnUpdate        string `json:"on_update,omitempty"`
}

type DropColumn struct {
	Name string `json:"name"`
}

func (d DropColumn) ToSQL(dialect, tableName string) (string, error) {
	if err := requireFields(tableName, d.Name); err != nil {
		return "", fmt.Errorf("DropColumn: %w", err)
	}
	return GetDialect(dialect).DropColumnSQL(d, tableName)
}

type RenameColumn struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type,omitempty"`
}

func (r RenameColumn) ToSQL(dialect, tableName string) (string, error) {
	if err := requireFields(tableName, r.From, r.To); err != nil {
		return "", fmt.Errorf("RenameColumn: %w", err)
	}
	return GetDialect(dialect).RenameColumnSQL(r, tableName)
}

type RenameTable struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func (rt RenameTable) ToSQL(dialect string) (string, error) {
	if err := requireFields(rt.OldName, rt.NewName); err != nil {
		return "", fmt.Errorf("RenameTable: %w", err)
	}
	return GetDialect(dialect).RenameTableSQL(rt)
}

type DeleteData struct {
	Name  string `json:"name"`
	Where string `json:"Where"`
}

func (d DeleteData) ToSQL(dialect string) (string, error) {
	if err := requireFields(d.Name); err != nil {
		return "", fmt.Errorf("DeleteData: %w", err)
	}
	return GetDialect(dialect).DeleteDataSQL(d)
}

type DropEnumType struct {
	Name     string `json:"name"`
	IfExists bool   `json:"IfExists"`
}

func (d DropEnumType) ToSQL(dialect string) (string, error) {
	if err := requireFields(d.Name); err != nil {
		return "", fmt.Errorf("DropEnumType: %w", err)
	}
	return GetDialect(dialect).DropEnumTypeSQL(d)
}

type DropRowPolicy struct {
	Name     string `json:"name"`
	Table    string `json:"Table"`
	IfExists bool   `json:"if_exists,omitempty"`
}

func (drp DropRowPolicy) ToSQL(dialect string) (string, error) {
	if err := requireFields(drp.Name); err != nil {
		return "", fmt.Errorf("DropRowPolicy: %w", err)
	}
	return GetDialect(dialect).DropRowPolicySQL(drp)
}

type DropMaterializedView struct {
	Name     string `json:"name"`
	IfExists bool   `json:"if_exists,omitempty"`
}

func (dmv DropMaterializedView) ToSQL(dialect string) (string, error) {
	if err := requireFields(dmv.Name); err != nil {
		return "", fmt.Errorf("DropMaterializedView: %w", err)
	}
	return GetDialect(dialect).DropMaterializedViewSQL(dmv)
}

type DropTable struct {
	Name    string `json:"name"`
	Cascade bool   `json:"cascade,omitempty"`
}

func (dt DropTable) ToSQL(dialect string) (string, error) {
	if err := requireFields(dt.Name); err != nil {
		return "", fmt.Errorf("DropTable: %w", err)
	}
	return GetDialect(dialect).DropTableSQL(dt)
}

type DropSchema struct {
	Name     string `json:"name"`
	Cascade  bool   `json:"cascade,omitempty"`
	IfExists bool   `json:"if_exists,omitempty"`
}

func (ds DropSchema) ToSQL(dialect string) (string, error) {
	if err := requireFields(ds.Name); err != nil {
		return "", fmt.Errorf("DropSchema: %w", err)
	}
	return GetDialect(dialect).DropSchemaSQL(ds)
}

type Transaction struct {
	Name           string `json:"name"`
	IsolationLevel string `json:"IsolationLevel"`
	Mode           string `json:"Mode"`
}

type Validation struct {
	Name         string   `json:"name"`
	PreUpChecks  []string `json:"PreUpChecks"`
	PostUpChecks []string `json:"PostUpChecks"`
}

func (a AddColumn) ToSQL(dialect, tableName string) ([]string, error) {
	if err := requireFields(tableName); err != nil {
		return nil, fmt.Errorf("AddColumn: %w", err)
	}
	return GetDialect(dialect).AddColumnSQL(a, tableName)
}

type CreateView struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	OrReplace  bool   `json:"or_replace,omitempty"`
}

func (cv CreateView) ToSQL(dialect string) (string, error) {
	if err := requireFields(cv.Name); err != nil {
		return "", fmt.Errorf("CreateView: %w", err)
	}
	return GetDialect(dialect).CreateViewSQL(cv)
}

type DropView struct {
	Name     string `json:"name"`
	Cascade  bool   `json:"cascade,omitempty"`
	IfExists bool   `json:"if_exists,omitempty"`
}

func (dv DropView) ToSQL(dialect string) (string, error) {
	if err := requireFields(dv.Name); err != nil {
		return "", fmt.Errorf("DropView: %w", err)
	}
	return GetDialect(dialect).DropViewSQL(dv)
}

type RenameView struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func (rv RenameView) ToSQL(dialect string) (string, error) {
	if err := requireFields(rv.OldName); err != nil {
		return "", fmt.Errorf("RenameView: %w", err)
	}
	return GetDialect(dialect).RenameViewSQL(rv)
}

type CreateFunction struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	OrReplace  bool   `json:"or_replace,omitempty"`
}

func (cf CreateFunction) ToSQL(dialect string) (string, error) {
	if err := requireFields(cf.Name); err != nil {
		return "", fmt.Errorf("CreateFunction: %w", err)
	}
	return GetDialect(dialect).CreateFunctionSQL(cf)
}

type DropFunction struct {
	Name     string `json:"name"`
	Cascade  bool   `json:"cascade,omitempty"`
	IfExists bool   `json:"if_exists,omitempty"`
}

func (df DropFunction) ToSQL(dialect string) (string, error) {
	if err := requireFields(df.Name); err != nil {
		return "", fmt.Errorf("DropFunction: %w", err)
	}
	return GetDialect(dialect).DropFunctionSQL(df)
}

type RenameFunction struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func (rf RenameFunction) ToSQL(dialect string) (string, error) {
	if err := requireFields(rf.OldName, rf.NewName); err != nil {
		return "", fmt.Errorf("RenameFunction: %w", err)
	}
	return GetDialect(dialect).RenameFunctionSQL(rf)
}

type CreateProcedure struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	OrReplace  bool   `json:"or_replace,omitempty"`
}

func (cp CreateProcedure) ToSQL(dialect string) (string, error) {
	if err := requireFields(cp.Name); err != nil {
		return "", fmt.Errorf("CreateProcedure: %w", err)
	}
	return GetDialect(dialect).CreateProcedureSQL(cp)
}

type DropProcedure struct {
	Name     string `json:"name"`
	Cascade  bool   `json:"cascade,omitempty"`
	IfExists bool   `json:"if_exists,omitempty"`
}

func (dp DropProcedure) ToSQL(dialect string) (string, error) {
	if err := requireFields(dp.Name); err != nil {
		return "", fmt.Errorf("DropProcedure: %w", err)
	}
	return GetDialect(dialect).DropProcedureSQL(dp)
}

type RenameProcedure struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func (rp RenameProcedure) ToSQL(dialect string) (string, error) {
	if err := requireFields(rp.OldName, rp.NewName); err != nil {
		return "", fmt.Errorf("RenameProcedure: %w", err)
	}
	return GetDialect(dialect).RenameProcedureSQL(rp)
}

type CreateTrigger struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	OrReplace  bool   `json:"or_replace,omitempty"`
}

func (ct CreateTrigger) ToSQL(dialect string) (string, error) {
	if err := requireFields(ct.Name); err != nil {
		return "", fmt.Errorf("CreateTrigger: %w", err)
	}
	return GetDialect(dialect).CreateTriggerSQL(ct)
}

type DropTrigger struct {
	Name     string `json:"name"`
	Cascade  bool   `json:"cascade,omitempty"`
	IfExists bool   `json:"if_exists,omitempty"`
}

func (dt DropTrigger) ToSQL(dialect string) (string, error) {
	if err := requireFields(dt.Name); err != nil {
		return "", fmt.Errorf("DropTrigger: %w", err)
	}
	return GetDialect(dialect).DropTriggerSQL(dt)
}

type RenameTrigger struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func (rt RenameTrigger) ToSQL(dialect string) (string, error) {
	if err := requireFields(rt.OldName, rt.NewName); err != nil {
		return "", fmt.Errorf("RenameTrigger: %w", err)
	}
	return GetDialect(dialect).RenameTriggerSQL(rt)
}

func handleSQLiteAlterTable(at AlterTable) ([]string, error) {
	schemaMutex.Lock()
	defer schemaMutex.Unlock()
	origSchema, ok := tableSchemas[at.Name]
	if !ok {
		return nil, fmt.Errorf("table schema for %s not found; cannot recreate table for alteration", at.Name)
	}
	newSchema := *origSchema
	renameMap := make(map[string]string)
	if len(at.DropColumn) > 0 || len(at.RenameColumn) > 0 {
		for _, dropCol := range at.DropColumn {
			found := false
			newCols := []AddColumn{}
			for _, col := range newSchema.Columns {
				if col.Name == dropCol.Name {
					found = true
					continue
				}
				newCols = append(newCols, col)
			}
			if !found {
				return nil, fmt.Errorf("column %s not found in table %s for dropping", dropCol.Name, at.Name)
			}
			newSchema.Columns = newCols
			newPK := []string{}
			for _, pk := range newSchema.PrimaryKey {
				if pk != dropCol.Name {
					newPK = append(newPK, pk)
				}
			}
			newSchema.PrimaryKey = newPK
		}
		for _, renameCol := range at.RenameColumn {
			found := false
			for i, col := range newSchema.Columns {
				if col.Name == renameCol.From {
					newSchema.Columns[i].Name = renameCol.To
					found = true
					renameMap[renameCol.From] = renameCol.To
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("column %s not found in table %s for renaming", renameCol.From, at.Name)
			}
			for i, pk := range newSchema.PrimaryKey {
				if pk == renameCol.From {
					newSchema.PrimaryKey[i] = renameCol.To
				}
			}
		}
		sqliteDialect, _ := GetDialect(DialectSQLite).(*SQLiteDialect)
		queries, err := sqliteDialect.RecreateTableForAlter(at.Name, newSchema, renameMap)
		if err != nil {
			return nil, fmt.Errorf("failed to recreate table for SQLite alteration: %w", err)
		}
		tableSchemas[at.Name] = &newSchema
		return queries, nil
	}
	var queries []string
	for _, addCol := range at.AddColumn {
		qList, err := addCol.ToSQL(DialectSQLite, at.Name)
		if err != nil {
			return nil, err
		}
		queries = append(queries, qList...)
		newSchema.Columns = append(newSchema.Columns, addCol)
		if addCol.PrimaryKey {
			newSchema.PrimaryKey = append(newSchema.PrimaryKey, addCol.Name)
		}
	}
	tableSchemas[at.Name] = &newSchema
	return queries, nil
}

// helper to append non-empty string results from a slice of structs with ToSQL(dialect, tableName)
func appendNonEmptyQueriesString[T any](queries []string, items []T, fn func(T) (string, error)) ([]string, error) {
	for _, item := range items {
		q, err := fn(item)
		if err != nil {
			return nil, err
		}
		if q != "" {
			queries = append(queries, q)
		}
	}
	return queries, nil
}

func (at AlterTable) ToSQL(dialect string) ([]string, error) {
	if err := requireFields(at.Name); err != nil {
		return nil, fmt.Errorf("AlterTable: %w", err)
	}
	if dialect == DialectSQLite {
		return handleSQLiteAlterTable(at)
	}
	var queries []string
	for _, addCol := range at.AddColumn {
		qList, err := addCol.ToSQL(dialect, at.Name)
		if err != nil {
			return nil, fmt.Errorf("error in AddColumn: %w", err)
		}
		if len(qList) > 0 {
			queries = append(queries, qList...)
		}
	}
	var err error
	queries, err = appendNonEmptyQueriesString(queries, at.DropColumn, func(dc DropColumn) (string, error) {
		return dc.ToSQL(dialect, at.Name)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropColumn: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, at.RenameColumn, func(rc RenameColumn) (string, error) {
		return rc.ToSQL(dialect, at.Name)
	})
	if err != nil {
		return nil, fmt.Errorf("error in RenameColumn: %w", err)
	}
	return queries, nil
}

func (op Operation) ToSQL(dialect string) ([]string, error) {
	var queries []string
	for _, ct := range op.CreateTable {
		q, err := ct.ToSQL(dialect, true)
		if err != nil {
			return nil, fmt.Errorf("error in CreateTable: %w", err)
		}
		if q != "" {
			queries = append(queries, q)
		}
		if dialect == DialectSQLite {
			schemaMutex.Lock()
			cpy := ct
			tableSchemas[ct.Name] = &cpy
			schemaMutex.Unlock()
		}
	}
	for _, at := range op.AlterTable {
		qList, err := at.ToSQL(dialect)
		if err != nil {
			return nil, fmt.Errorf("error in AlterTable: %w", err)
		}
		if len(qList) > 0 {
			queries = append(queries, qList...)
		}
	}
	var err error
	queries, err = appendNonEmptyQueriesString(queries, op.DeleteData, func(dd DeleteData) (string, error) {
		return dd.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DeleteData: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropEnumType, func(de DropEnumType) (string, error) {
		return de.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropEnumType: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropRowPolicy, func(drp DropRowPolicy) (string, error) {
		return drp.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropRowPolicy: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropMaterializedView, func(dmv DropMaterializedView) (string, error) {
		return dmv.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropMaterializedView: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropTable, func(dt DropTable) (string, error) {
		return dt.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropTable: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropSchema, func(ds DropSchema) (string, error) {
		return ds.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropSchema: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.RenameTable, func(rt RenameTable) (string, error) {
		return rt.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in RenameTable: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.CreateView, func(cv CreateView) (string, error) {
		return cv.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in CreateView: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropView, func(dv DropView) (string, error) {
		return dv.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropView: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.RenameView, func(rv RenameView) (string, error) {
		return rv.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in RenameView: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.CreateFunction, func(cf CreateFunction) (string, error) {
		return cf.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in CreateFunction: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropFunction, func(df DropFunction) (string, error) {
		return df.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropFunction: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.RenameFunction, func(rf RenameFunction) (string, error) {
		return rf.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in RenameFunction: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.CreateProcedure, func(cp CreateProcedure) (string, error) {
		return cp.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in CreateProcedure: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropProcedure, func(dp DropProcedure) (string, error) {
		return dp.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropProcedure: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.RenameProcedure, func(rp RenameProcedure) (string, error) {
		return rp.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in RenameProcedure: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.CreateTrigger, func(ct CreateTrigger) (string, error) {
		return ct.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in CreateTrigger: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.DropTrigger, func(dt DropTrigger) (string, error) {
		return dt.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in DropTrigger: %w", err)
	}
	queries, err = appendNonEmptyQueriesString(queries, op.RenameTrigger, func(rt RenameTrigger) (string, error) {
		return rt.ToSQL(dialect)
	})
	if err != nil {
		return nil, fmt.Errorf("error in RenameTrigger: %w", err)
	}
	return queries, nil
}

func (m Migration) ToSQL(dialect string, up bool) ([]string, error) {
	var queries []string
	var ops Operation
	if up {
		ops = m.Up
	} else {
		ops = m.Down
	}
	qList, err := ops.ToSQL(dialect)
	if err != nil {
		return nil, fmt.Errorf("error in migration operation: %w", err)
	}
	queries = append(queries, qList...)
	return queries, nil
}

// RunSeeds executes the seed SQL statements for a given SeedDefinition.
func RunSeeds(seed SeedDefinition, dialect string, dbDriver IDatabaseDriver) error {
	queries, err := seed.ToSQL(dialect)
	if err != nil {
		return err
	}
	for _, q := range queries {
		if err := dbDriver.ApplySQL([]string{q.SQL}, q.Args); err != nil {
			return fmt.Errorf("failed to apply seed: %w", err)
		}
	}
	return nil
}

func computeChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func NewDriver(driver string, dsn string) (IDatabaseDriver, error) {
	switch driver {
	case "mysql":
		return drivers.NewMySQLDriver(dsn)
	case "postgres":
		return drivers.NewPostgresDriver(dsn)
	case "sqlite":
		return drivers.NewSQLiteDriver(dsn)
	}
	return nil, fmt.Errorf("unsupported driver: %s", driver)
}

// NewHistoryDriver returns an implementation of HistoryDriver (file, db, etc.)
func NewHistoryDriver(driver, dialect, config string, tables ...string) (HistoryDriver, error) {
	switch driver {
	case "file":
		return NewFileHistoryDriver(config), nil
	case "db":
		return NewDatabaseHistoryDriver(dialect, config, tables...)
	default:
		return nil, fmt.Errorf("unsupported history driver: %s", driver)
	}
}

func requireFields(fields ...string) error {
	for _, f := range fields {
		if f == "" {
			return fmt.Errorf("required field is empty")
		}
	}
	return nil
}
