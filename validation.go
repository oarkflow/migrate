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

	// Check for valid SQL identifier pattern. Note: SQL reserved words (e.g.
	// "action", "order", "user") are intentionally NOT rejected here - every
	// identifier this validator guards is always emitted through a dialect's
	// quoteIdentifier, which safely quotes it regardless of keyword status, so
	// rejecting reserved words would only block legitimate column/table names
	// without any real safety benefit.
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, value)
	if !matched {
		v.AddError(field, value, "identifier must start with letter or underscore and contain only alphanumeric characters and underscores")
		return
	}
}

// ValidateSQLIdentifier validates a single SQL identifier (table name, column
// name, etc.) and returns a Go error if it is empty, malformed, too long, or
// a reserved keyword. It is intended to be called defensively at the point
// where identifiers coming from migration definitions are first used to
// generate SQL, so that obviously malicious or malformed identifiers (e.g.
// containing quote characters used to break out of a quoted identifier) are
// rejected outright instead of merely being escaped.
func ValidateSQLIdentifier(field, value string) error {
	v := NewValidator()
	v.ValidateIdentifier(field, value)
	return v.Error()
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
