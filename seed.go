package migrate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oarkflow/bcl"
)

type SeedConfig struct {
	Seeds []SeedDefinition `json:"Seed"`
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

func (s SeedDefinition) ToSQL(dialect string) ([]string, error) {
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
	var queries []string
	for i := 0; i < s.Rows; i++ {
		var cols []string
		var vals []any
		for _, field := range s.Fields {
			val := fmt.Sprintf("%v", field.Value)
			cols = append(cols, field.Name)
			var evaluated string
			if field.Random {
				evaluated = getRandomValue(val)
			} else {
				evaluated = mutate(val)
			}
			if field.DataType != "" {
				dt := strings.ToLower(field.DataType)
				if dt == "string" || dt == "varchar" || dt == "char" || dt == "text" {
					if !(strings.HasPrefix(evaluated, "'") && strings.HasSuffix(evaluated, "'")) {
						evaluated = fmt.Sprintf("'%s'", evaluated)
					}
				} else if dt == "boolean" || dt == "bool" {
					b, err := strconv.ParseBool(evaluated)
					if err == nil {
						evaluated = fmt.Sprintf("%t", b)
					} else {
						evaluated = strings.ToLower(evaluated)
					}
				}
			}
			vals = append(vals, evaluated)
		}
		q, err := dial.InsertSQL(s.Table, cols, vals)
		if err != nil {
			return nil, err
		}
		queries = append(queries, q)
	}
	return queries, nil
}

func getRandomValue(val string) string {
	if strings.Contains(val, "${ref(") {
		return "'random_fk'"
	}
	return val
}
