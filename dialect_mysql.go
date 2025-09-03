package migrate

import (
	"errors"
	"fmt"
	"strings"
)

type MySQLDialect struct{}

func (m *MySQLDialect) quoteIdentifier(id string) string {
	return fmt.Sprintf("`%s`", id)
}

func (m *MySQLDialect) TableExistsSQL(table string) string {
	return fmt.Sprintf(`SELECT COUNT(*) > 0 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'`, table)
}

func (m *MySQLDialect) CreateTableSQL(ct CreateTable, up bool) (string, error) {
	if err := requireFields(ct.Name); err != nil {
		return "", fmt.Errorf("MySQLDialect.CreateTableSQL: %w", err)
	}
	if up {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("CREATE TABLE %s (", m.quoteIdentifier(ct.Name)))
		var cols []string
		var pkCols []string
		for _, col := range ct.AddFields {
			colDef := fmt.Sprintf("%s %s", m.quoteIdentifier(col.Name), m.MapDataType(col.Type, col.Size, col.Scale, col.AutoIncrement))
			if col.AutoIncrement {
				colDef += " AUTO_INCREMENT"
			}
			if !col.Nullable {
				colDef += " NOT NULL"
			}
			if col.Default != "" {
				def := ConvertDefault(col.Default, col.Type)
				if strings.Contains(colDef, "NOT NULL") {
					if def != "NULL" {
						colDef += fmt.Sprintf(" DEFAULT %s", def)
					}
				} else {
					colDef += fmt.Sprintf(" DEFAULT %s", def)
				}
			}
			if col.Check != "" {
				colDef += fmt.Sprintf(" CHECK (%s)", col.Check)
			}
			cols = append(cols, colDef)
			if len(ct.PrimaryKey) == 0 && col.PrimaryKey {
				pkCols = append(pkCols, m.quoteIdentifier(col.Name))
			}
		}
		if len(ct.PrimaryKey) > 0 {
			var pkQuoted []string
			for _, col := range ct.PrimaryKey {
				pkQuoted = append(pkQuoted, m.quoteIdentifier(col))
			}
			cols = append(cols, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkQuoted, ", ")))
		} else if len(pkCols) > 0 {
			cols = append(cols, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
		}
		sb.WriteString(strings.Join(cols, ", "))
		sb.WriteString(");")
		var extra []string
		for _, col := range ct.AddFields {
			if col.Unique {
				extra = append(extra, fmt.Sprintf("CREATE UNIQUE INDEX uniq_%s_%s ON %s (%s);", ct.Name, col.Name, m.quoteIdentifier(ct.Name), m.quoteIdentifier(col.Name)))
			} else if col.Index {
				extra = append(extra, fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);", ct.Name, col.Name, m.quoteIdentifier(ct.Name), m.quoteIdentifier(col.Name)))
			}
		}
		if len(extra) > 0 {
			sb.WriteString("\n" + strings.Join(extra, "\n"))
		}
		return sb.String(), nil
	}
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", m.quoteIdentifier(ct.Name)), nil
}

func (m *MySQLDialect) RenameTableSQL(rt RenameTable) (string, error) {
	if err := requireFields(rt.OldName, rt.NewName); err != nil {
		return "", fmt.Errorf("MySQLDialect.RenameTableSQL: %w", err)
	}
	return fmt.Sprintf("RENAME TABLE %s TO %s;", m.quoteIdentifier(rt.OldName), m.quoteIdentifier(rt.NewName)), nil
}

func (m *MySQLDialect) DeleteDataSQL(dd DeleteData) (string, error) {
	return fmt.Sprintf("DELETE FROM %s WHERE %s;", m.quoteIdentifier(dd.Name), dd.Where), nil
}

func (m *MySQLDialect) DropEnumTypeSQL(de DropEnumType) (string, error) {
	return "", errors.New("enum types are not supported in MySQL")
}

func (m *MySQLDialect) DropRowPolicySQL(drp DropRowPolicy) (string, error) {
	return "", errors.New("DROP ROW POLICY is not supported in MySQL")
}

func (m *MySQLDialect) DropMaterializedViewSQL(dmv DropMaterializedView) (string, error) {
	return "", errors.New("DROP MATERIALIZED VIEW is not supported in MySQL")
}

func (m *MySQLDialect) DropTableSQL(dt DropTable) (string, error) {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", m.quoteIdentifier(dt.Name)), nil
}

func (m *MySQLDialect) DropSchemaSQL(ds DropSchema) (string, error) {
	return "", errors.New("DROP SCHEMA is not supported in MySQL")
}

func (m *MySQLDialect) AddFieldSQL(ac AddField, tableName string) ([]string, error) {
	if err := requireFields(ac.Name, tableName); err != nil {
		return nil, fmt.Errorf("MySQLDialect.AddFieldSQL: %w", err)
	}
	var queries []string
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s ", m.quoteIdentifier(tableName), m.quoteIdentifier(ac.Name)))
	sb.WriteString(m.MapDataType(ac.Type, ac.Size, ac.Scale, ac.AutoIncrement))
	if ac.AutoIncrement {
		sb.WriteString(" AUTO_INCREMENT")
	}
	if !ac.Nullable {
		sb.WriteString(" NOT NULL")
	}
	if ac.Default != "" {
		def := ConvertDefault(ac.Default, ac.Type)
		if !ac.Nullable {
			if def != "NULL" {
				sb.WriteString(fmt.Sprintf(" DEFAULT %s", def))
			}
		} else {
			sb.WriteString(fmt.Sprintf(" DEFAULT %s", def))
		}
	}
	if ac.Check != "" {
		sb.WriteString(fmt.Sprintf(" CHECK (%s)", ac.Check))
	}
	sb.WriteString(";")
	queries = append(queries, sb.String())
	if ac.Unique {
		queries = append(queries, fmt.Sprintf("CREATE UNIQUE INDEX uniq_%s_%s ON %s (%s);", tableName, ac.Name, tableName, ac.Name))
	}
	if ac.Index {
		queries = append(queries, fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);", tableName, ac.Name, tableName, ac.Name))
	}
	if ac.ForeignKey != nil {
		fk := ac.ForeignKey
		sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT fk_%s FOREIGN KEY (%s) REFERENCES %s(%s)", tableName, ac.Name, ac.Name, fk.ReferenceTable, fk.ReferenceField)
		if fk.OnDelete != "" {
			sql += fmt.Sprintf(" ON DELETE %s", fk.OnDelete)
		}
		if fk.OnUpdate != "" {
			sql += fmt.Sprintf(" ON UPDATE %s", fk.OnUpdate)
		}
		queries = append(queries, sql+";")
	}
	return queries, nil
}

func (m *MySQLDialect) DropFieldSQL(dc DropField, tableName string) (string, error) {
	if err := requireFields(dc.Name, tableName); err != nil {
		return "", fmt.Errorf("MySQLDialect.DropFieldSQL: %w", err)
	}
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", m.quoteIdentifier(tableName), m.quoteIdentifier(dc.Name)), nil
}

func (m *MySQLDialect) RenameFieldSQL(rc RenameField, tableName string) (string, error) {
	if err := requireFields(tableName); err != nil {
		return "", fmt.Errorf("MySQLDialect.RenameFieldSQL: %w", err)
	}
	if rc.Type == "" {
		return "", errors.New("MySQL requires field type for renaming field")
	}
	from := rc.From
	if from == "" && rc.Name != "" {
		from = rc.Name
	}
	if from == "" {
		return "", errors.New("MySQL requires field name for renaming field")
	}
	if rc.To == "" {
		return "", errors.New("MySQL requires new field name for renaming field")
	}
	return fmt.Sprintf("ALTER TABLE %s CHANGE %s %s %s;", m.quoteIdentifier(tableName), m.quoteIdentifier(from), m.quoteIdentifier(rc.To), rc.Type), nil
}

func (m *MySQLDialect) MapDataType(genericType string, size, scale int, autoIncrement bool) string {
	return ConvertType(strings.ToLower(genericType), "mysql", size, scale, autoIncrement)
}

func (m *MySQLDialect) WrapInTransaction(queries []string) []string {
	tx := []string{"START TRANSACTION;"}
	tx = append(tx, queries...)
	tx = append(tx, "COMMIT;")
	return tx
}

func (m *MySQLDialect) WrapInTransactionWithConfig(queries []string, trans Transaction) []string {
	var beginStmt string
	if trans.IsolationLevel != "" {
		beginStmt = fmt.Sprintf("SET TRANSACTION ISOLATION LEVEL %s; START TRANSACTION;", trans.IsolationLevel)
	} else {
		beginStmt = "START TRANSACTION;"
	}
	tx := []string{beginStmt}
	tx = append(tx, queries...)
	tx = append(tx, "COMMIT;")
	return tx
}

func (m *MySQLDialect) CreateViewSQL(cv CreateView) (string, error) {
	if cv.OrReplace {
		return fmt.Sprintf("CREATE OR REPLACE VIEW %s AS %s;", m.quoteIdentifier(cv.Name), cv.Definition), nil
	}
	return fmt.Sprintf("CREATE VIEW %s AS %s;", m.quoteIdentifier(cv.Name), cv.Definition), nil
}

func (m *MySQLDialect) DropViewSQL(dv DropView) (string, error) {
	cascade := ""
	if dv.Cascade {
		cascade = " CASCADE"
	}
	if dv.IfExists {
		return fmt.Sprintf("DROP VIEW IF EXISTS %s%s;", m.quoteIdentifier(dv.Name), cascade), nil
	}
	return fmt.Sprintf("DROP VIEW %s%s;", m.quoteIdentifier(dv.Name), cascade), nil
}

func (m *MySQLDialect) RenameViewSQL(rv RenameView) (string, error) {
	return "", errors.New("RENAME VIEW is not supported in MySQL")
}

func (m *MySQLDialect) CreateFunctionSQL(cf CreateFunction) (string, error) {
	return "", errors.New("CREATE FUNCTION is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) DropFunctionSQL(df DropFunction) (string, error) {
	return "", errors.New("DROP FUNCTION is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) RenameFunctionSQL(rf RenameFunction) (string, error) {
	return "", errors.New("RENAME FUNCTION is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) CreateProcedureSQL(cp CreateProcedure) (string, error) {
	return "", errors.New("CREATE PROCEDURE is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) DropProcedureSQL(dp DropProcedure) (string, error) {
	return "", errors.New("DROP PROCEDURE is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) RenameProcedureSQL(rp RenameProcedure) (string, error) {
	return "", errors.New("RENAME PROCEDURE is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) CreateTriggerSQL(ct CreateTrigger) (string, error) {
	return "", errors.New("CREATE TRIGGER is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) DropTriggerSQL(dt DropTrigger) (string, error) {
	return "", errors.New("DROP TRIGGER is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) RenameTriggerSQL(rt RenameTrigger) (string, error) {
	return "", errors.New("RENAME TRIGGER is not supported in this MySQL dialect implementation")
}

func (m *MySQLDialect) InsertSQL(table string, fields []string, values []any) (string, map[string]any, error) {
	var quotedCols []string
	argMap := make(map[string]any)
	var namedParams []string
	for i, col := range fields {
		quotedCols = append(quotedCols, m.quoteIdentifier(col))
		paramName := ":" + col
		namedParams = append(namedParams, paramName)
		argMap[col] = values[i]
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		m.quoteIdentifier(table),
		strings.Join(quotedCols, ", "),
		strings.Join(namedParams, ", "),
	)
	return query, argMap, nil
}
func (m *MySQLDialect) EOS() string {
	return ";"
}
