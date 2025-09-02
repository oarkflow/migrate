package migrate

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s' (value: '%s'): %s", e.Field, e.Value, e.Message)
}

// Validator provides validation utilities for migration components
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		errors: make([]ValidationError, 0),
	}
}

// AddError adds a validation error
func (v *Validator) AddError(field, value, message string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors returns all validation errors
func (v *Validator) Errors() []ValidationError {
	return v.errors
}

// Error returns a formatted error message with all validation errors
func (v *Validator) Error() error {
	if !v.HasErrors() {
		return nil
	}

	var messages []string
	for _, err := range v.errors {
		messages = append(messages, err.Error())
	}

	return fmt.Errorf("validation failed:\n%s", strings.Join(messages, "\n"))
}

// ValidateIdentifier validates SQL identifiers (table names, field names, etc.)
func (v *Validator) ValidateIdentifier(field, value string) {
	if value == "" {
		v.AddError(field, value, "identifier cannot be empty")
		return
	}

	if len(value) > 64 {
		v.AddError(field, value, "identifier too long (max 64 characters)")
		return
	}

	// Check for valid SQL identifier pattern
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, value)
	if !matched {
		v.AddError(field, value, "identifier must start with letter or underscore and contain only alphanumeric characters and underscores")
		return
	}

	// Check for SQL reserved words
	if isReservedWord(value) {
		v.AddError(field, value, "identifier is a reserved SQL keyword")
	}
}

// ValidateDataType validates field data types
func (v *Validator) ValidateDataType(field, value string) {
	if value == "" {
		v.AddError(field, value, "data type cannot be empty")
		return
	}

	validTypes := map[string]bool{
		"string": true, "varchar": true, "text": true, "char": true,
		"longtext": true, "mediumtext": true, "tinytext": true,
		"number": true, "int": true, "integer": true, "serial": true,
		"bigserial": true, "smallint": true, "mediumint": true,
		"bigint": true, "tinyint": true, "float": true, "double": true,
		"decimal": true, "numeric": true, "real": true,
		"boolean": true, "bool": true, "date": true, "datetime": true,
		"time": true, "timestamp": true, "year": true,
		"blob": true, "mediumblob": true, "longblob": true,
		"binary": true, "varbinary": true, "enum": true, "set": true,
		"json": true, "jsonb": true, "bytea": true, "bit": true,
	}

	if !validTypes[strings.ToLower(value)] {
		v.AddError(field, value, "unsupported data type")
	}
}

// ValidateMigration validates a complete migration
func (v *Validator) ValidateMigration(m Migration) {
	v.ValidateIdentifier("migration.name", m.Name)

	if m.Version == "" {
		v.AddError("migration.version", m.Version, "version cannot be empty")
	}

	if m.Description == "" {
		v.AddError("migration.description", m.Description, "description cannot be empty")
	}

	// Validate Up operations
	v.validateOperation("up", m.Up)

	// Validate Down operations
	v.validateOperation("down", m.Down)
}

// validateOperation validates migration operations
func (v *Validator) validateOperation(prefix string, op Operation) {
	// Validate CreateTable operations
	for i, ct := range op.CreateTable {
		field := fmt.Sprintf("%s.create_table[%d]", prefix, i)
		v.ValidateIdentifier(field+".name", ct.Name)

		if len(ct.AddFields) == 0 {
			v.AddError(field+".fields", "", "table must have at least one field")
		}

		for j, col := range ct.AddFields {
			colField := fmt.Sprintf("%s.fields[%d]", field, j)
			v.ValidateIdentifier(colField+".name", col.Name)
			v.ValidateDataType(colField+".type", col.Type)

			// Validate size constraints
			if col.Size < 0 {
				v.AddError(colField+".size", fmt.Sprintf("%d", col.Size), "size cannot be negative")
			}

			if col.Scale < 0 {
				v.AddError(colField+".scale", fmt.Sprintf("%d", col.Scale), "scale cannot be negative")
			}

			if col.Scale > col.Size && col.Size > 0 {
				v.AddError(colField+".scale", fmt.Sprintf("%d", col.Scale), "scale cannot be greater than size")
			}
		}
	}

	// Validate AlterTable operations
	for i, at := range op.AlterTable {
		field := fmt.Sprintf("%s.alter_table[%d]", prefix, i)
		v.ValidateIdentifier(field+".name", at.Name)

		// Validate AddField operations
		for j, col := range at.AddFields {
			colField := fmt.Sprintf("%s.add_field[%d]", field, j)
			v.ValidateIdentifier(colField+".name", col.Name)
			v.ValidateDataType(colField+".type", col.Type)
		}

		// Validate DropField operations
		for j, col := range at.DropFields {
			colField := fmt.Sprintf("%s.drop_field[%d]", field, j)
			v.ValidateIdentifier(colField+".name", col.Name)
		}

		// Validate RenameField operations
		for j, col := range at.RenameFields {
			colField := fmt.Sprintf("%s.rename_field[%d]", field, j)
			v.ValidateIdentifier(colField+".from", col.From)
			v.ValidateIdentifier(colField+".to", col.To)
		}
	}

	// Validate DropTable operations
	for i, dt := range op.DropTable {
		field := fmt.Sprintf("%s.drop_table[%d]", prefix, i)
		v.ValidateIdentifier(field+".name", dt.Name)
	}
}

// isReservedWord checks if a word is a reserved SQL keyword
func isReservedWord(word string) bool {
	reservedWords := map[string]bool{
		"select": true, "insert": true, "update": true, "delete": true,
		"create": true, "drop": true, "alter": true, "table": true,
		"index": true, "view": true, "database": true, "schema": true,
		"primary": true, "foreign": true, "key": true, "constraint": true,
		"unique": true, "not": true, "null": true, "default": true,
		"check": true, "references": true, "on": true, "cascade": true,
		"restrict": true, "set": true, "action": true, "match": true,
		"full": true, "partial": true, "simple": true, "initially": true,
		"deferred": true, "immediate": true, "deferrable": true,
		"from": true, "where": true, "group": true, "having": true,
		"order": true, "by": true, "limit": true, "offset": true,
		"union": true, "intersect": true, "except": true, "all": true,
		"distinct": true, "as": true, "join": true, "inner": true,
		"left": true, "right": true, "outer": true,
		"cross": true, "natural": true, "using": true, "and": true,
		"or": true, "in": true, "exists": true, "between": true,
		"like": true, "ilike": true, "similar": true, "to": true,
		"escape": true, "is": true, "true": true, "false": true,
		"unknown": true, "case": true, "when": true, "then": true,
		"else": true, "end": true, "cast": true, "extract": true,
		"position": true, "substring": true, "trim": true, "leading": true,
		"trailing": true, "both": true, "for": true, "collate": true,
		"user": true, "current_user": true, "session_user": true,
		"system_user": true, "current_date": true, "current_time": true,
		"current_timestamp": true, "localtime": true, "localtimestamp": true,
		"current_role": true, "current_catalog": true, "current_schema": true,
		"authorization": true, "binary": true, "collation": true,
		"column": true, "current": true, "cursor": true,
		"day": true, "dec": true, "decimal": true, "declare": true,
		"do": true, "double": true, "each": true, "elseif": true,
		"enclosed": true, "escaped": true, "exit": true, "explain": true,
		"float": true, "float4": true, "float8": true, "force": true,
		"function": true, "grant": true, "high_priority": true, "hour": true,
		"ignore": true, "int": true, "int1": true, "int2": true,
		"int3": true, "int4": true, "int8": true, "integer": true,
		"interval": true, "into": true, "iterate": true, "keys": true,
		"kill": true, "leave": true, "lines": true, "load": true,
		"lock": true, "long": true, "longblob": true, "longtext": true,
		"loop": true, "low_priority": true, "mediumblob": true, "mediumint": true,
		"mediumtext": true, "middleint": true, "minute": true, "mod": true,
		"month": true, "no": true, "numeric": true, "optimize": true,
		"option": true, "optionally": true, "out": true, "outfile": true,
		"precision": true, "procedure": true, "purge": true, "read": true,
		"real": true, "rename": true, "repeat": true, "replace": true,
		"require": true, "return": true, "revoke": true, "rlike": true,
		"second": true, "separator": true, "show": true, "smallint": true,
		"soname": true, "spatial": true, "sql": true, "sqlexception": true,
		"sqlstate": true, "sqlwarning": true, "ssl": true, "starting": true,
		"straight_join": true, "terminated": true, "text": true, "time": true,
		"timestamp": true, "tinyblob": true, "tinyint": true, "tinytext": true,
		"trigger": true, "undo": true, "unlock": true, "unsigned": true,
		"usage": true, "use": true, "utc_date": true, "utc_time": true,
		"utc_timestamp": true, "values": true, "varbinary": true, "varchar": true,
		"varcharacter": true, "varying": true, "while": true, "with": true,
		"write": true, "x509": true, "xor": true, "year": true,
		"year_month": true, "zerofill": true,
	}

	return reservedWords[strings.ToLower(word)]
}
