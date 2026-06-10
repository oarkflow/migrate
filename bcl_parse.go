package migrate

import (
	"fmt"

	"github.com/oarkflow/bcl"
)

type bclDocument struct {
	Migrations []bclMigration `bcl:"Migration,block"`
	Seeds      []bclSeed      `bcl:"Seed,block"`
}

type bclMigration struct {
	Name        string           `bcl:",id"`
	Version     string           `bcl:"Version"`
	Description string           `bcl:"Description"`
	Connection  string           `bcl:"Connection"`
	Driver      string           `bcl:"Driver"`
	Up          []bclOperation   `bcl:"Up,block"`
	Down        []bclOperation   `bcl:"Down,block"`
	Transaction []bclTransaction `bcl:"Transaction,block"`
	Validate    []bclValidation  `bcl:"Validate,block"`
	Disable     bool             `bcl:"Disable"`
}

type bclOperation struct {
	AlterTable           []bclAlterTable           `bcl:"AlterTable,block"`
	CreateTable          []bclCreateTable          `bcl:"CreateTable,block"`
	DeleteData           []bclDeleteData           `bcl:"DeleteData,block"`
	DropEnumType         []bclDropEnumType         `bcl:"DropEnumType,block"`
	DropRowPolicy        []bclDropRowPolicy        `bcl:"DropRowPolicy,block"`
	DropMaterializedView []bclDropMaterializedView `bcl:"DropMaterializedView,block"`
	DropTable            []bclDropTable            `bcl:"DropTable,block"`
	DropSchema           []bclDropSchema           `bcl:"DropSchema,block"`
	RenameTable          []bclRenameTable          `bcl:"RenameTable,block"`
	CreateView           []bclCreateView           `bcl:"CreateView,block"`
	DropView             []bclDropView             `bcl:"DropView,block"`
	RenameView           []bclRenameView           `bcl:"RenameView,block"`
	CreateFunction       []bclCreateFunction       `bcl:"CreateFunction,block"`
	DropFunction         []bclDropFunction         `bcl:"DropFunction,block"`
	RenameFunction       []bclRenameFunction       `bcl:"RenameFunction,block"`
	CreateProcedure      []bclCreateProcedure      `bcl:"CreateProcedure,block"`
	DropProcedure        []bclDropProcedure        `bcl:"DropProcedure,block"`
	RenameProcedure      []bclRenameProcedure      `bcl:"RenameProcedure,block"`
	CreateTrigger        []bclCreateTrigger        `bcl:"CreateTrigger,block"`
	DropTrigger          []bclDropTrigger          `bcl:"DropTrigger,block"`
	RenameTrigger        []bclRenameTrigger        `bcl:"RenameTrigger,block"`
}

type bclAlterTable struct {
	Name         string           `bcl:",id"`
	AddFields    []bclAddField    `bcl:"AddField,block"`
	DropFields   []bclDropField   `bcl:"DropField,block"`
	RenameFields []bclRenameField `bcl:"RenameField,block"`
}

type bclCreateTable struct {
	Name       string        `bcl:",id"`
	AddFields  []bclAddField `bcl:"Field,block"`
	PrimaryKey []string      `bcl:"PrimaryKey"`
}

type bclAddField struct {
	ID            string      `bcl:",id"`
	Name          string      `bcl:"name"`
	Type          string      `bcl:"type"`
	Nullable      bool        `bcl:"nullable"`
	Default       any         `bcl:"default"`
	Check         string      `bcl:"check"`
	Size          int         `bcl:"size"`
	Scale         int         `bcl:"scale"`
	AutoIncrement bool        `bcl:"auto_increment"`
	PrimaryKey    bool        `bcl:"primary_key"`
	Unique        bool        `bcl:"unique"`
	Index         bool        `bcl:"index"`
	ForeignKey    *ForeignKey `bcl:"foreign_key"`
}

type bclDropField struct {
	ID   string `bcl:",id"`
	Name string `bcl:"name"`
}

type bclRenameField struct {
	Name string `bcl:",id"`
	From string `bcl:"from"`
	To   string `bcl:"to"`
	Type string `bcl:"type"`
}

type bclRenameTable struct {
	Name    string `bcl:",id"`
	OldName string `bcl:"old_name"`
	NewName string `bcl:"new_name"`
}

type bclDeleteData struct {
	Name  string `bcl:",id"`
	Where string `bcl:"Where"`
}

type bclDropEnumType struct {
	Name     string `bcl:",id"`
	IfExists bool   `bcl:"IfExists"`
}

type bclDropRowPolicy struct {
	Name     string `bcl:",id"`
	Table    string `bcl:"Table"`
	IfExists bool   `bcl:"if_exists"`
}

type bclDropMaterializedView struct {
	Name     string `bcl:",id"`
	IfExists bool   `bcl:"if_exists"`
}

type bclDropTable struct {
	Name    string `bcl:",id"`
	Cascade bool   `bcl:"Cascade"`
}

type bclDropSchema struct {
	Name     string `bcl:",id"`
	Cascade  bool   `bcl:"cascade"`
	IfExists bool   `bcl:"if_exists"`
}

type bclCreateView struct {
	Name       string `bcl:",id"`
	Definition string `bcl:"definition"`
	OrReplace  bool   `bcl:"or_replace"`
}

type bclDropView struct {
	Name     string `bcl:",id"`
	Cascade  bool   `bcl:"cascade"`
	IfExists bool   `bcl:"if_exists"`
}

type bclRenameView struct {
	Name    string `bcl:",id"`
	OldName string `bcl:"old_name"`
	NewName string `bcl:"new_name"`
}

type bclCreateFunction struct {
	Name       string `bcl:",id"`
	Definition string `bcl:"definition"`
	OrReplace  bool   `bcl:"or_replace"`
}

type bclDropFunction struct {
	Name     string `bcl:",id"`
	Cascade  bool   `bcl:"cascade"`
	IfExists bool   `bcl:"if_exists"`
}

type bclRenameFunction struct {
	Name    string `bcl:",id"`
	OldName string `bcl:"old_name"`
	NewName string `bcl:"new_name"`
}

type bclCreateProcedure struct {
	Name       string `bcl:",id"`
	Definition string `bcl:"definition"`
	OrReplace  bool   `bcl:"or_replace"`
}

type bclDropProcedure struct {
	Name     string `bcl:",id"`
	Cascade  bool   `bcl:"cascade"`
	IfExists bool   `bcl:"if_exists"`
}

type bclRenameProcedure struct {
	Name    string `bcl:",id"`
	OldName string `bcl:"old_name"`
	NewName string `bcl:"new_name"`
}

type bclCreateTrigger struct {
	Name       string `bcl:",id"`
	Definition string `bcl:"definition"`
	OrReplace  bool   `bcl:"or_replace"`
}

type bclDropTrigger struct {
	Name     string `bcl:",id"`
	Cascade  bool   `bcl:"cascade"`
	IfExists bool   `bcl:"if_exists"`
}

type bclRenameTrigger struct {
	Name    string `bcl:",id"`
	OldName string `bcl:"old_name"`
	NewName string `bcl:"new_name"`
}

type bclTransaction struct {
	Name           string `bcl:",id"`
	IsolationLevel string `bcl:"IsolationLevel"`
	Mode           string `bcl:"Mode"`
}

type bclValidation struct {
	Name         string   `bcl:",id"`
	PreUpChecks  []string `bcl:"PreUpChecks"`
	PostUpChecks []string `bcl:"PostUpChecks"`
}

type bclSeed struct {
	Name      string         `bcl:",id"`
	Table     string         `bcl:"table"`
	Fields    []bclSeedField `bcl:"Field,block"`
	Combine   []string       `bcl:"combine"`
	Condition string         `bcl:"condition"`
	Rows      int            `bcl:"rows"`
}

type bclSeedField struct {
	Name     string `bcl:",id"`
	Value    any    `bcl:"value"`
	Unique   bool   `bcl:"unique"`
	Random   bool   `bcl:"random"`
	Size     int    `bcl:"size"`
	DataType string `bcl:"data_type"`
}

func ParseMigrationsBCL(data []byte) ([]Migration, error) {
	var doc bclDocument
	if err := bcl.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	migrations := make([]Migration, 0, len(doc.Migrations))
	seen := make(map[string]struct{}, len(doc.Migrations))
	for i, item := range doc.Migrations {
		migration := item.toMigration()
		if migration.Name == "" {
			return nil, fmt.Errorf("migration block %d is missing a name", i+1)
		}
		if _, ok := seen[migration.Name]; ok {
			return nil, fmt.Errorf("duplicate migration name %q in BCL document", migration.Name)
		}
		seen[migration.Name] = struct{}{}
		migrations = append(migrations, migration)
	}
	return migrations, nil
}

func ParseMigrationBCL(data []byte) (Migration, error) {
	migrations, err := ParseMigrationsBCL(data)
	if err != nil {
		return Migration{}, err
	}
	if len(migrations) == 0 {
		return Migration{}, fmt.Errorf("no Migration blocks found")
	}
	return migrations[0], nil
}

func FindMigrationBCL(data []byte, name string) (Migration, error) {
	migrations, err := ParseMigrationsBCL(data)
	if err != nil {
		return Migration{}, err
	}
	for _, migration := range migrations {
		if migration.Name == name {
			return migration, nil
		}
	}
	return Migration{}, fmt.Errorf("migration %q not found in BCL document", name)
}

func ParseSeedsBCL(data []byte) ([]SeedDefinition, error) {
	var doc bclDocument
	if err := bcl.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	seeds := make([]SeedDefinition, 0, len(doc.Seeds))
	seen := make(map[string]struct{}, len(doc.Seeds))
	for i, item := range doc.Seeds {
		seed := item.toSeedDefinition()
		if seed.Name == "" {
			return nil, fmt.Errorf("seed block %d is missing a name", i+1)
		}
		if _, ok := seen[seed.Name]; ok {
			return nil, fmt.Errorf("duplicate seed name %q in BCL document", seed.Name)
		}
		seen[seed.Name] = struct{}{}
		seeds = append(seeds, seed)
	}
	return seeds, nil
}

func ParseSeedBCL(data []byte) (SeedDefinition, error) {
	seeds, err := ParseSeedsBCL(data)
	if err != nil {
		return SeedDefinition{}, err
	}
	if len(seeds) == 0 {
		return SeedDefinition{}, fmt.Errorf("no Seed blocks found")
	}
	return seeds[0], nil
}

func (m bclMigration) toMigration() Migration {
	return Migration{
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Connection:  m.Connection,
		Driver:      m.Driver,
		Up:          mergeBCLOperations(m.Up),
		Down:        mergeBCLOperations(m.Down),
		Transaction: mapSlice(m.Transaction, func(v bclTransaction) Transaction { return v.toTransaction() }),
		Validate:    mapSlice(m.Validate, func(v bclValidation) Validation { return v.toValidation() }),
		Disable:     m.Disable,
	}
}

func mergeBCLOperations(items []bclOperation) Operation {
	var out Operation
	for _, item := range items {
		op := item.toOperation()
		out.AlterTable = append(out.AlterTable, op.AlterTable...)
		out.CreateTable = append(out.CreateTable, op.CreateTable...)
		out.DeleteData = append(out.DeleteData, op.DeleteData...)
		out.DropEnumType = append(out.DropEnumType, op.DropEnumType...)
		out.DropRowPolicy = append(out.DropRowPolicy, op.DropRowPolicy...)
		out.DropMaterializedView = append(out.DropMaterializedView, op.DropMaterializedView...)
		out.DropTable = append(out.DropTable, op.DropTable...)
		out.DropSchema = append(out.DropSchema, op.DropSchema...)
		out.RenameTable = append(out.RenameTable, op.RenameTable...)
		out.CreateView = append(out.CreateView, op.CreateView...)
		out.DropView = append(out.DropView, op.DropView...)
		out.RenameView = append(out.RenameView, op.RenameView...)
		out.CreateFunction = append(out.CreateFunction, op.CreateFunction...)
		out.DropFunction = append(out.DropFunction, op.DropFunction...)
		out.RenameFunction = append(out.RenameFunction, op.RenameFunction...)
		out.CreateProcedure = append(out.CreateProcedure, op.CreateProcedure...)
		out.DropProcedure = append(out.DropProcedure, op.DropProcedure...)
		out.RenameProcedure = append(out.RenameProcedure, op.RenameProcedure...)
		out.CreateTrigger = append(out.CreateTrigger, op.CreateTrigger...)
		out.DropTrigger = append(out.DropTrigger, op.DropTrigger...)
		out.RenameTrigger = append(out.RenameTrigger, op.RenameTrigger...)
	}
	return out
}

func (op bclOperation) toOperation() Operation {
	return Operation{
		AlterTable:           mapSlice(op.AlterTable, func(v bclAlterTable) AlterTable { return v.toAlterTable() }),
		CreateTable:          mapSlice(op.CreateTable, func(v bclCreateTable) CreateTable { return v.toCreateTable() }),
		DeleteData:           mapSlice(op.DeleteData, func(v bclDeleteData) DeleteData { return v.toDeleteData() }),
		DropEnumType:         mapSlice(op.DropEnumType, func(v bclDropEnumType) DropEnumType { return v.toDropEnumType() }),
		DropRowPolicy:        mapSlice(op.DropRowPolicy, func(v bclDropRowPolicy) DropRowPolicy { return v.toDropRowPolicy() }),
		DropMaterializedView: mapSlice(op.DropMaterializedView, func(v bclDropMaterializedView) DropMaterializedView { return v.toDropMaterializedView() }),
		DropTable:            mapSlice(op.DropTable, func(v bclDropTable) DropTable { return v.toDropTable() }),
		DropSchema:           mapSlice(op.DropSchema, func(v bclDropSchema) DropSchema { return v.toDropSchema() }),
		RenameTable:          mapSlice(op.RenameTable, func(v bclRenameTable) RenameTable { return v.toRenameTable() }),
		CreateView:           mapSlice(op.CreateView, func(v bclCreateView) CreateView { return v.toCreateView() }),
		DropView:             mapSlice(op.DropView, func(v bclDropView) DropView { return v.toDropView() }),
		RenameView:           mapSlice(op.RenameView, func(v bclRenameView) RenameView { return v.toRenameView() }),
		CreateFunction:       mapSlice(op.CreateFunction, func(v bclCreateFunction) CreateFunction { return v.toCreateFunction() }),
		DropFunction:         mapSlice(op.DropFunction, func(v bclDropFunction) DropFunction { return v.toDropFunction() }),
		RenameFunction:       mapSlice(op.RenameFunction, func(v bclRenameFunction) RenameFunction { return v.toRenameFunction() }),
		CreateProcedure:      mapSlice(op.CreateProcedure, func(v bclCreateProcedure) CreateProcedure { return v.toCreateProcedure() }),
		DropProcedure:        mapSlice(op.DropProcedure, func(v bclDropProcedure) DropProcedure { return v.toDropProcedure() }),
		RenameProcedure:      mapSlice(op.RenameProcedure, func(v bclRenameProcedure) RenameProcedure { return v.toRenameProcedure() }),
		CreateTrigger:        mapSlice(op.CreateTrigger, func(v bclCreateTrigger) CreateTrigger { return v.toCreateTrigger() }),
		DropTrigger:          mapSlice(op.DropTrigger, func(v bclDropTrigger) DropTrigger { return v.toDropTrigger() }),
		RenameTrigger:        mapSlice(op.RenameTrigger, func(v bclRenameTrigger) RenameTrigger { return v.toRenameTrigger() }),
	}
}

func (at bclAlterTable) toAlterTable() AlterTable {
	return AlterTable{
		Name:         at.Name,
		AddFields:    mapSlice(at.AddFields, func(v bclAddField) AddField { return v.toAddField() }),
		DropFields:   mapSlice(at.DropFields, func(v bclDropField) DropField { return v.toDropField() }),
		RenameFields: mapSlice(at.RenameFields, func(v bclRenameField) RenameField { return v.toRenameField() }),
	}
}

func (ct bclCreateTable) toCreateTable() CreateTable {
	return CreateTable{
		Name:       ct.Name,
		AddFields:  mapSlice(ct.AddFields, func(v bclAddField) AddField { return v.toAddField() }),
		PrimaryKey: ct.PrimaryKey,
	}
}

func (f bclAddField) toAddField() AddField {
	return AddField{
		Name:          firstNonEmpty(f.ID, f.Name),
		Type:          f.Type,
		Nullable:      f.Nullable,
		Default:       f.Default,
		Check:         f.Check,
		Size:          f.Size,
		Scale:         f.Scale,
		AutoIncrement: f.AutoIncrement,
		PrimaryKey:    f.PrimaryKey,
		Unique:        f.Unique,
		Index:         f.Index,
		ForeignKey:    f.ForeignKey,
	}
}

func (f bclDropField) toDropField() DropField {
	return DropField{Name: firstNonEmpty(f.ID, f.Name)}
}

func (f bclRenameField) toRenameField() RenameField {
	return RenameField{Name: f.Name, From: f.From, To: f.To, Type: f.Type}
}

func (rt bclRenameTable) toRenameTable() RenameTable {
	return RenameTable{OldName: firstNonEmpty(rt.OldName, rt.Name), NewName: rt.NewName}
}

func (d bclDeleteData) toDeleteData() DeleteData {
	return DeleteData{Name: d.Name, Where: d.Where}
}

func (d bclDropEnumType) toDropEnumType() DropEnumType {
	return DropEnumType{Name: d.Name, IfExists: d.IfExists}
}

func (d bclDropRowPolicy) toDropRowPolicy() DropRowPolicy {
	return DropRowPolicy{Name: d.Name, Table: d.Table, IfExists: d.IfExists}
}

func (d bclDropMaterializedView) toDropMaterializedView() DropMaterializedView {
	return DropMaterializedView{Name: d.Name, IfExists: d.IfExists}
}

func (d bclDropTable) toDropTable() DropTable {
	return DropTable{Name: d.Name, Cascade: d.Cascade}
}

func (d bclDropSchema) toDropSchema() DropSchema {
	return DropSchema{Name: d.Name, Cascade: d.Cascade, IfExists: d.IfExists}
}

func (v bclCreateView) toCreateView() CreateView {
	return CreateView{Name: v.Name, Definition: v.Definition, OrReplace: v.OrReplace}
}

func (v bclDropView) toDropView() DropView {
	return DropView{Name: v.Name, Cascade: v.Cascade, IfExists: v.IfExists}
}

func (v bclRenameView) toRenameView() RenameView {
	return RenameView{OldName: firstNonEmpty(v.OldName, v.Name), NewName: v.NewName}
}

func (f bclCreateFunction) toCreateFunction() CreateFunction {
	return CreateFunction{Name: f.Name, Definition: f.Definition, OrReplace: f.OrReplace}
}

func (f bclDropFunction) toDropFunction() DropFunction {
	return DropFunction{Name: f.Name, Cascade: f.Cascade, IfExists: f.IfExists}
}

func (f bclRenameFunction) toRenameFunction() RenameFunction {
	return RenameFunction{OldName: firstNonEmpty(f.OldName, f.Name), NewName: f.NewName}
}

func (p bclCreateProcedure) toCreateProcedure() CreateProcedure {
	return CreateProcedure{Name: p.Name, Definition: p.Definition, OrReplace: p.OrReplace}
}

func (p bclDropProcedure) toDropProcedure() DropProcedure {
	return DropProcedure{Name: p.Name, Cascade: p.Cascade, IfExists: p.IfExists}
}

func (p bclRenameProcedure) toRenameProcedure() RenameProcedure {
	return RenameProcedure{OldName: firstNonEmpty(p.OldName, p.Name), NewName: p.NewName}
}

func (t bclCreateTrigger) toCreateTrigger() CreateTrigger {
	return CreateTrigger{Name: t.Name, Definition: t.Definition, OrReplace: t.OrReplace}
}

func (t bclDropTrigger) toDropTrigger() DropTrigger {
	return DropTrigger{Name: t.Name, Cascade: t.Cascade, IfExists: t.IfExists}
}

func (t bclRenameTrigger) toRenameTrigger() RenameTrigger {
	return RenameTrigger{OldName: firstNonEmpty(t.OldName, t.Name), NewName: t.NewName}
}

func (t bclTransaction) toTransaction() Transaction {
	return Transaction{Name: t.Name, IsolationLevel: t.IsolationLevel, Mode: t.Mode}
}

func (v bclValidation) toValidation() Validation {
	return Validation{Name: v.Name, PreUpChecks: v.PreUpChecks, PostUpChecks: v.PostUpChecks}
}

func (s bclSeed) toSeedDefinition() SeedDefinition {
	return SeedDefinition{
		Name:      s.Name,
		Table:     s.Table,
		Fields:    mapSlice(s.Fields, func(v bclSeedField) FieldDefinition { return v.toFieldDefinition() }),
		Combine:   s.Combine,
		Condition: s.Condition,
		Rows:      s.Rows,
	}
}

func (f bclSeedField) toFieldDefinition() FieldDefinition {
	return FieldDefinition{
		Name:     f.Name,
		Value:    f.Value,
		Unique:   f.Unique,
		Random:   f.Random,
		Size:     f.Size,
		DataType: f.DataType,
	}
}

func mapSlice[T any, U any](items []T, fn func(T) U) []U {
	if len(items) == 0 {
		return nil
	}
	out := make([]U, 0, len(items))
	for _, item := range items {
		out = append(out, fn(item))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
