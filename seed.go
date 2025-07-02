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
	DataType string `json:"data_type"`
}

type InsertQuery struct {
	SQL  string
	Args any
}

func (s SeedDefinition) ToSQL(dialect string) ([]InsertQuery, error) {
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
	for i := 0; i < s.Rows; i++ {
		var cols []string
		var vals []any
		rowValues := make(map[string]any)
		exprFields := make(map[string]*FieldDefinition)
		for idx, field := range s.Fields {
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
			switch strings.ToLower(field.DataType) {
			case "int", "integer", "number":
				if eval, err := strconv.Atoi(evaluated); err == nil {
					rowValues[field.Name] = eval
				} else {
					rowValues[field.Name] = evaluated
				}
			case "boolean", "bool":
				if eval, err := strconv.ParseBool(evaluated); err == nil {
					rowValues[field.Name] = eval
				} else {
					rowValues[field.Name] = evaluated
				}
			default:
				rowValues[field.Name] = evaluated
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
					continue // skip this expr for now
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
				rowValues[name] = result
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
		// Build columns and values in original order
		for _, field := range s.Fields {
			cols = append(cols, field.Name)
			val := rowValues[field.Name]
			var arg any = val
			if field.DataType != "" {
				dt := strings.ToLower(field.DataType)
				switch dt {
				case "boolean", "bool":
					switch v := val.(type) {
					case string:
						b, err := strconv.ParseBool(v)
						if err == nil {
							arg = b
						}
					}
				case "int", "integer", "number":
					switch v := val.(type) {
					case string:
						if eval, err := strconv.Atoi(v); err == nil {
							arg = eval
						}
					}
				}
			}
			vals = append(vals, arg)
		}
		q, qargs, err := dial.InsertSQL(s.Table, cols, vals)
		if err != nil {
			return nil, err
		}
		queries = append(queries, InsertQuery{SQL: q, Args: qargs})
	}
	return queries, nil
}

func getRandomValue(val string) string {
	if strings.Contains(val, "${ref(") {
		return "'random_fk'"
	}
	return val
}
