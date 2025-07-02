package migrate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oarkflow/bcl"
	"github.com/oarkflow/expr"
	"github.com/oarkflow/expr/vm"
)

type SeedConfig struct {
	Seed SeedDefinition `json:"Seed"`
}

type SeedDefinition struct {
	Name      string            `json:"name"`
	Table     string            `json:"table"`
	Fields    []FieldDefinition `json:"Field"`
	Combine   []string          `json:"combine"`
	Condition string            `json:"condition"`
	Rows      int               `json:"rows"`
}

type FieldDefinition struct {
	Name     string `json:"name"`
	Value    any    `json:"value"`
	Unique   bool   `json:"unique"`
	Random   bool   `json:"random"`
	Size     int    `json:"size"`
	DataType string `json:"data_type"`
}

type InsertQuery struct {
	SQL  string
	Args map[string]any
}

type SeedHistory struct {
	Name      string `json:"name"`
	AppliedAt string `json:"applied_at"`
}

func convertSeedValue(val any, dataType string) any {
	switch strings.ToLower(dataType) {
	case "int", "integer", "number":
		switch v := val.(type) {
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		case int:
			return v
		}
	case "boolean", "bool":
		switch v := val.(type) {
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		case bool:
			return v
		}
	}
	return val
}

func (s SeedDefinition) ToSQL(dialect string) ([]InsertQuery, error) {
	// Check required fields for SeedDefinition
	if err := requireFields(s.Name, s.Table); err != nil {
		return nil, fmt.Errorf("SeedDefinition.ToSQL: %w", err)
	}
	mutate := func(val string) string {
		if strings.HasPrefix(val, "fake_") {
			fn, ok := bcl.LookupFunction(val)
			if ok {
				rs, err := fn()
				if err == nil {
					switch rs := rs.(type) {
					case string:
						return rs
					default:
						return fmt.Sprintf("%v", rs)
					}
				}
			}
		}
		return val
	}
	dial := GetDialect(dialect)
	exprMap := make(map[string]*vm.Program)
	findDeps := func(exprStr string) []string {
		var deps []string
		parts := strings.FieldsFunc(exprStr, func(r rune) bool {
			return !(r == '_' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
		})
		for _, p := range parts {
			if strings.HasSuffix(p, ".value") {
				name := strings.TrimSuffix(p, ".value")
				if name != "" {
					deps = append(deps, name)
				}
			}
		}
		return deps
	}
	var queries []InsertQuery
	uniqueSet := make(map[string]map[any]struct{})
	for _, field := range s.Fields {
		if field.Unique {
			if uniqueSet[field.Name] == nil {
				uniqueSet[field.Name] = make(map[any]struct{})
			}
		}
	}
	for i := 0; i < s.Rows; i++ {
		var cols []string
		valMap := make(map[string]any)
		rowValues := make(map[string]any)
		exprFields := make(map[string]*FieldDefinition)
		for idx, field := range s.Fields {
			// Check required fields for FieldDefinition
			if err := requireFields(field.Name); err != nil {
				return nil, fmt.Errorf("SeedDefinition.ToSQL (field): %w", err)
			}
			val := fmt.Sprintf("%v", field.Value)
			if strings.HasPrefix(val, "expr:") {
				exprFields[field.Name] = &s.Fields[idx]
				continue
			}
			var evaluated string
			if field.Random {
				evaluated = getRandomValue(val)
			} else {
				evaluated = mutate(val)
			}
			rowValues[field.Name] = convertSeedValue(evaluated, field.DataType)
		}
		// Improved unique constraint enforcement
		for _, field := range s.Fields {
			if field.Unique {
				val := rowValues[field.Name]
				maxAttempts := 100
				attempts := 0
				origVal := fmt.Sprintf("%v", field.Value)
				for {
					if _, exists := uniqueSet[field.Name][val]; !exists {
						break
					}
					attempts++
					if attempts >= maxAttempts {
						return nil, fmt.Errorf("could not generate unique value for field '%s' after %d attempts", field.Name, maxAttempts)
					}
					// Regenerate value using mutate or getRandomValue as appropriate
					var newVal string
					if field.Random {
						newVal = getRandomValue(origVal)
					} else {
						newVal = mutate(origVal)
					}
					val = convertSeedValue(newVal, field.DataType)
				}
				uniqueSet[field.Name][val] = struct{}{}
				rowValues[field.Name] = val
			}
		}
		remaining := len(exprFields)
		for remaining > 0 {
			progress := false
			for name, field := range exprFields {
				exprStr := strings.TrimSpace(strings.TrimPrefix(fmt.Sprintf("%v", field.Value), "expr:"))
				deps := findDeps(exprStr)
				missing := false
				for _, dep := range deps {
					if _, ok := rowValues[dep]; !ok {
						missing = true
						break
					}
				}
				if missing {
					continue
				}
				ctx := map[string]map[string]any{}
				for k, v := range rowValues {
					if mv, ok := v.(map[string]any); ok {
						ctx[k] = mv
					} else {
						ctx[k] = map[string]any{"value": v}
					}
				}
				for k, v := range rowValues {
					ctx[k+".value"] = map[string]any{"value": v}
				}
				program, ok := exprMap[exprStr]
				if !ok {
					var err error
					program, err = expr.Compile(exprStr, expr.Env(ctx))
					if err != nil {
						return nil, fmt.Errorf("expr compile error for field '%s': %w", field.Name, err)
					}
					exprMap[exprStr] = program
				}
				result, err := expr.Run(program, ctx)
				if err != nil {
					return nil, fmt.Errorf("expr eval error for field '%s': %w", field.Name, err)
				}
				rowValues[name] = convertSeedValue(result, field.DataType)
				delete(exprFields, name)
				progress = true
			}
			if !progress {
				names := make([]string, 0, len(exprFields))
				for n := range exprFields {
					names = append(names, n)
				}
				return nil, fmt.Errorf("could not resolve expr fields: %v", names)
			}
			remaining = len(exprFields)
		}
		for _, field := range s.Fields {
			cols = append(cols, field.Name)
			valMap[field.Name] = rowValues[field.Name]
		}
		q, argMap, err := dial.InsertSQL(s.Table, cols, colsToArgs(cols, valMap))
		if err != nil {
			return nil, err
		}
		queries = append(queries, InsertQuery{SQL: q, Args: argMap})
	}
	return queries, nil
}

func colsToArgs(cols []string, valMap map[string]any) []any {
	args := make([]any, len(cols))
	for i, col := range cols {
		args[i] = valMap[col]
	}
	return args
}

func getRandomValue(val string) string {
	if strings.Contains(val, "${ref(") {
		return "'random_fk'"
	}
	return val
}
