package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/oarkflow/bcl"
	"github.com/oarkflow/cli/contracts"
)

type HistoryCommand struct {
	Driver IManager
}

func (c *HistoryCommand) Signature() string {
	return "history"
}

func (c *HistoryCommand) Description() string {
	return "Generate a detailed HTML report of all changes for a specific object (table, view, function, procedure, trigger) from migration files."
}

func (c *HistoryCommand) Extend() contracts.Extend {
	return contracts.Extend{
		Flags: []contracts.Flag{
			{
				Name:    "object",
				Aliases: []string{"o"},
				Usage:   "Name of the object (table/view/function/procedure/trigger) to analyze",
				Value:   "",
			},
		},
	}
}

type MigrationChange struct {
	MigrationName string
	Date          time.Time
	Operation     string
	Details       string
}

func (c *HistoryCommand) Handle(ctx contracts.Context) error {
	objectName := ctx.Option("object")
	if objectName == "" {
		return fmt.Errorf("please specify --object=<name>")
	}
	objectName = strings.ToLower(objectName)
	files, err := os.ReadDir(c.Driver.MigrationDir())
	if err != nil {
		return fmt.Errorf("failed to read migration directory: %w", err)
	}
	type changeList []MigrationChange
	var changes changeList
	var finalTable *CreateTable
	var dropped bool
	var finalView *CreateView
	var finalFunction *CreateFunction
	var finalTrigger *CreateTrigger
	var finalProcedure *CreateProcedure

	var fileNames []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".bcl") {
			fileNames = append(fileNames, f.Name())
		}
	}
	sort.Strings(fileNames)

	for _, fname := range fileNames {
		path := filepath.Join(c.Driver.MigrationDir(), fname)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg Config
		if _, err := bcl.Unmarshal(data, &cfg); err != nil {
			continue
		}
		m := cfg.Migration
		createdAt := extractTimeFromFilename(fname)

		// TABLES
		for _, ct := range m.Up.CreateTable {
			if strings.EqualFold(ct.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "CreateTable",
					Details:       describeTableColumns(ct.Columns, ct.PrimaryKey),
				})
				cpy := ct
				finalTable = &cpy
				dropped = false
			}
		}
		for _, at := range m.Up.AlterTable {
			if strings.EqualFold(at.Name, objectName) && finalTable != nil && !dropped {
				for _, ac := range at.AddColumn {
					changes = append(changes, MigrationChange{
						MigrationName: m.Name,
						Date:          createdAt,
						Operation:     "AddColumn",
						Details:       describeColumn(ac),
					})
					finalTable.Columns = append(finalTable.Columns, ac)
				}
				for _, dc := range at.DropColumn {
					changes = append(changes, MigrationChange{
						MigrationName: m.Name,
						Date:          createdAt,
						Operation:     "DropColumn",
						Details:       fmt.Sprintf("Dropped column: <b>%s</b>", dc.Name),
					})
					if finalTable != nil {
						var newCols []AddColumn
						for _, col := range finalTable.Columns {
							if col.Name != dc.Name {
								newCols = append(newCols, col)
							}
						}
						finalTable.Columns = newCols
					}
				}
				for _, rc := range at.RenameColumn {
					changes = append(changes, MigrationChange{
						MigrationName: m.Name,
						Date:          createdAt,
						Operation:     "RenameColumn",
						Details:       fmt.Sprintf("Renamed column: <b>%s</b> to <b>%s</b>", rc.From, rc.To),
					})
					if finalTable != nil {
						for i, col := range finalTable.Columns {
							if col.Name == rc.From {
								finalTable.Columns[i].Name = rc.To
							}
						}
					}
				}
			}
		}
		for _, dt := range m.Up.DropTable {
			if strings.EqualFold(dt.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "DropTable",
					Details:       "Table dropped",
				})
				finalTable = nil
				dropped = true
			}
		}

		// VIEWS
		for _, cv := range m.Up.CreateView {
			if strings.EqualFold(cv.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "CreateView",
					Details:       describeView(cv),
				})
				cpy := cv
				finalView = &cpy
			}
		}
		for _, dv := range m.Up.DropView {
			if strings.EqualFold(dv.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "DropView",
					Details:       "View dropped",
				})
				finalView = nil
			}
		}
		for _, rv := range m.Up.RenameView {
			if strings.EqualFold(rv.OldName, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "RenameView",
					Details:       fmt.Sprintf("Renamed view: <b>%s</b> to <b>%s</b>", rv.OldName, rv.NewName),
				})
				if finalView != nil {
					finalView.Name = rv.NewName
				}
			}
		}

		// FUNCTIONS
		for _, cf := range m.Up.CreateFunction {
			if strings.EqualFold(cf.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "CreateFunction",
					Details:       describeFunction(cf),
				})
				cpy := cf
				finalFunction = &cpy
			}
		}
		for _, df := range m.Up.DropFunction {
			if strings.EqualFold(df.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "DropFunction",
					Details:       "Function dropped",
				})
				finalFunction = nil
			}
		}
		for _, rf := range m.Up.RenameFunction {
			if strings.EqualFold(rf.OldName, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "RenameFunction",
					Details:       fmt.Sprintf("Renamed function: <b>%s</b> to <b>%s</b>", rf.OldName, rf.NewName),
				})
				if finalFunction != nil {
					finalFunction.Name = rf.NewName
				}
			}
		}

		// PROCEDURES
		for _, cp := range m.Up.CreateProcedure {
			if strings.EqualFold(cp.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "CreateProcedure",
					Details:       describeProcedure(cp),
				})
				cpy := cp
				finalProcedure = &cpy
			}
		}
		for _, dp := range m.Up.DropProcedure {
			if strings.EqualFold(dp.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "DropProcedure",
					Details:       "Procedure dropped",
				})
				finalProcedure = nil
			}
		}
		for _, rp := range m.Up.RenameProcedure {
			if strings.EqualFold(rp.OldName, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "RenameProcedure",
					Details:       fmt.Sprintf("Renamed procedure: <b>%s</b> to <b>%s</b>", rp.OldName, rp.NewName),
				})
				if finalProcedure != nil {
					finalProcedure.Name = rp.NewName
				}
			}
		}

		// TRIGGERS
		for _, ct := range m.Up.CreateTrigger {
			if strings.EqualFold(ct.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "CreateTrigger",
					Details:       describeTrigger(ct),
				})
				cpy := ct
				finalTrigger = &cpy
			}
		}
		for _, dt := range m.Up.DropTrigger {
			if strings.EqualFold(dt.Name, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "DropTrigger",
					Details:       "Trigger dropped",
				})
				finalTrigger = nil
			}
		}
		for _, rt := range m.Up.RenameTrigger {
			if strings.EqualFold(rt.OldName, objectName) {
				changes = append(changes, MigrationChange{
					MigrationName: m.Name,
					Date:          createdAt,
					Operation:     "RenameTrigger",
					Details:       fmt.Sprintf("Renamed trigger: <b>%s</b> to <b>%s</b>", rt.OldName, rt.NewName),
				})
				if finalTrigger != nil {
					finalTrigger.Name = rt.NewName
				}
			}
		}
	}

	report := generateHTMLReport(objectName, changes, finalTable, finalView, finalFunction, finalProcedure, finalTrigger)
	reportPath := filepath.Join(".", fmt.Sprintf("history_%s_%d.html", objectName, time.Now().Unix()))
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		return fmt.Errorf("failed to write HTML report: %w", err)
	}
	fmt.Printf("History report generated: %s\n", reportPath)
	return nil
}

func extractTimeFromFilename(fname string) time.Time {
	parts := strings.Split(fname, "_")
	if len(parts) > 0 {
		if ts, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
	}
	return time.Time{}
}

func describeTableColumns(cols []AddColumn, pk []string) string {
	var lines []string
	for _, c := range cols {
		lines = append(lines, describeColumn(c))
	}
	if len(pk) > 0 {
		lines = append(lines, fmt.Sprintf("<b>Primary Key:</b> %s", strings.Join(pk, ", ")))
	}
	return "<ul><li>" + strings.Join(lines, "</li><li>") + "</li></ul>"
}

func describeColumn(c AddColumn) string {
	return fmt.Sprintf(
		"<b>%s</b> <code>%s</code>%s%s%s%s%s%s%s",
		c.Name,
		c.Type,
		boolBadge("PK", c.PrimaryKey),
		boolBadge("AI", c.AutoIncrement),
		boolBadge("Unique", c.Unique),
		boolBadge("Index", c.Index),
		boolBadge("Nullable", c.Nullable),
		defaultBadge(c.Default),
		commentBadge(c.Check),
	)
}

func boolBadge(label string, b bool) string {
	if b {
		return fmt.Sprintf(` <span style="background:#28a745;color:#fff;padding:2px 6px;border-radius:4px;font-size:0.85em">%s</span>`, label)
	}
	return ""
}

func defaultBadge(def any) string {
	if def == nil || def == "" {
		return ""
	}
	return fmt.Sprintf(` <span style="background:#ffc107;color:#333;padding:2px 6px;border-radius:4px;font-size:0.85em">Default: %v</span>`, def)
}

func commentBadge(comment string) string {
	if comment == "" {
		return ""
	}
	return fmt.Sprintf(` <span style="background:#17a2b8;color:#fff;padding:2px 6px;border-radius:4px;font-size:0.85em">Check: %s</span>`, comment)
}

func describeView(cv CreateView) string {
	return fmt.Sprintf("<b>Name:</b> %s<br><b>Definition:</b><pre>%s</pre>", cv.Name, cv.Definition)
}

func describeFunction(cf CreateFunction) string {
	return fmt.Sprintf("<b>Name:</b> %s<br><b>Definition:</b><pre>%s</pre>", cf.Name, cf.Definition)
}

func describeProcedure(cp CreateProcedure) string {
	return fmt.Sprintf("<b>Name:</b> %s<br><b>Definition:</b><pre>%s</pre>", cp.Name, cp.Definition)
}

func describeTrigger(ct CreateTrigger) string {
	return fmt.Sprintf("<b>Name:</b> %s<br><b>Definition:</b><pre>%s</pre>", ct.Name, ct.Definition)
}

func generateHTMLReport(objectName string, changes []MigrationChange, finalTable *CreateTable, finalView *CreateView, finalFunction *CreateFunction, finalProcedure *CreateProcedure, finalTrigger *CreateTrigger) string {
	html := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>History Report - ` + objectName + `</title>
<style>
body { font-family: Arial, sans-serif; margin: 2em; background: #f8f9fa; }
h1 { color: #333; }
.timeline { border-left: 3px solid #007bff; margin: 2em 0; padding-left: 2em; }
.event { margin-bottom: 2em; position: relative; }
.event:before { content: ""; position: absolute; left: -2.1em; top: 0.5em; width: 1em; height: 1em; background: #007bff; border-radius: 50%; }
.event .date { color: #888; font-size: 0.95em; }
.event .op { font-weight: bold; color: #007bff; }
.final-structure { background: #fff; border: 1px solid #ddd; padding: 1em; border-radius: 8px; }
th, td { padding: 0.5em 1em; border-bottom: 1px solid #eee; }
ul { margin: 0; padding-left: 1.2em; }
pre { background: #f4f4f4; padding: 0.5em; border-radius: 4px; }
</style>
</head>
<body>
<h1>History Report: ` + objectName + `</h1>
<div class="timeline">
`
	for _, ch := range changes {
		html += `<div class="event">
  <div class="date">` + ch.Date.Format(time.RFC1123) + `</div>
  <div class="op">` + ch.Operation + `</div>
  <div class="details">` + ch.Details + `</div>
  <div class="migration">Migration: <b>` + ch.MigrationName + `</b></div>
</div>
`
	}
	html += `</div>
<h2>Final Structure</h2>
<div class="final-structure">`
	switch {
	case finalTable != nil:
		html += `<table><tr><th>Column</th><th>Type</th><th>PK</th><th>AI</th><th>Unique</th><th>Index</th><th>Nullable</th><th>Default</th><th>Check</th></tr>`
		for _, col := range finalTable.Columns {
			html += `<tr><td>` + col.Name + `</td><td>` + col.Type + `</td><td>` + boolStr(col.PrimaryKey) + `</td><td>` + boolStr(col.AutoIncrement) + `</td><td>` + boolStr(col.Unique) + `</td><td>` + boolStr(col.Index) + `</td><td>` + boolStr(col.Nullable) + `</td><td>` + fmt.Sprintf("%v", col.Default) + `</td><td>` + col.Check + `</td></tr>`
		}
		html += `</table>`
	case finalView != nil:
		html += describeView(*finalView)
	case finalFunction != nil:
		html += describeFunction(*finalFunction)
	case finalProcedure != nil:
		html += describeProcedure(*finalProcedure)
	case finalTrigger != nil:
		html += describeTrigger(*finalTrigger)
	default:
		html += `<b>Object does not exist (dropped).</b>`
	}
	html += `</div>
</body>
</html>`
	return html
}

func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
