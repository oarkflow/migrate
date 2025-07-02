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

// Add MigrationGroup type definition
type MigrationGroup struct {
	Date          time.Time
	MigrationName string
	Actions       []MigrationChange
}

type objectInfo struct {
	Name string
	Type string
}

func (c *HistoryCommand) Handle(ctx contracts.Context) error {
	objectName := ctx.Option("object")
	files, err := os.ReadDir(c.Driver.MigrationDir())
	if err != nil {
		return fmt.Errorf("failed to read migration directory: %w", err)
	}

	var fileNames []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".bcl") {
			fileNames = append(fileNames, f.Name())
		}
	}
	sort.Strings(fileNames)

	// Collect all objects (tables, views, functions, procedures, triggers)
	objectSet := make(map[string]string) // name -> type
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
		for _, ct := range m.Up.CreateTable {
			objectSet[strings.ToLower(ct.Name)] = "table"
		}
		for _, cv := range m.Up.CreateView {
			objectSet[strings.ToLower(cv.Name)] = "view"
		}
		for _, cf := range m.Up.CreateFunction {
			objectSet[strings.ToLower(cf.Name)] = "function"
		}
		for _, cp := range m.Up.CreateProcedure {
			objectSet[strings.ToLower(cp.Name)] = "procedure"
		}
		for _, ct := range m.Up.CreateTrigger {
			objectSet[strings.ToLower(ct.Name)] = "trigger"
		}
	}

	// If no object specified, generate report for all objects
	if objectName == "" {
		var allObjects []objectInfo
		for name, typ := range objectSet {
			allObjects = append(allObjects, objectInfo{Name: name, Type: typ})
		}
		sort.Slice(allObjects, func(i, j int) bool { return allObjects[i].Name < allObjects[j].Name })
		report := generateHTMLReportAllObjects(allObjects, fileNames, c.Driver.MigrationDir())
		reportPath := filepath.Join(".", fmt.Sprintf("history_all_%d.html", time.Now().Unix()))
		if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
			return fmt.Errorf("failed to write HTML report: %w", err)
		}
		fmt.Printf("History report generated: %s\n", reportPath)
		return nil
	}

	objectName = strings.ToLower(objectName)
	files, err = os.ReadDir(c.Driver.MigrationDir())
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

	var fileNames2 []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".bcl") {
			fileNames2 = append(fileNames2, f.Name())
		}
	}
	sort.Strings(fileNames2)

	// Collect changes for the specified object
	for _, fname := range fileNames2 {
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
	migrationMap := make(map[string]*MigrationGroup)
	var migrationOrder []string

	for _, ch := range changes {
		key := ch.Date.Format("20060102150405") + "_" + ch.MigrationName
		if _, ok := migrationMap[key]; !ok {
			migrationMap[key] = &MigrationGroup{
				Date:          ch.Date,
				MigrationName: ch.MigrationName,
			}
			migrationOrder = append(migrationOrder, key)
		}
		migrationMap[key].Actions = append(migrationMap[key].Actions, ch)
	}

	report := generateHTMLReportAccordion2Col(objectName, migrationMap, migrationOrder, finalTable, finalView, finalFunction, finalProcedure, finalTrigger)
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

// Accordion HTML report generator
func generateHTMLReportAccordion(
	objectName string,
	migrationMap map[string]*MigrationGroup,
	migrationOrder []string,
	finalTable *CreateTable,
	finalView *CreateView,
	finalFunction *CreateFunction,
	finalProcedure *CreateProcedure,
	finalTrigger *CreateTrigger,
) string {
	html := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>History Report - ` + objectName + `</title>
<style>
body { font-family: Arial, sans-serif; margin: 2em; background: #f8f9fa; }
h1 { color: #333; }
.accordion { background: #fff; border-radius: 8px; border: 1px solid #ddd; margin-bottom: 1em; }
.accordion-header { cursor: pointer; padding: 1em; border-bottom: 1px solid #eee; background: #f1f3f4; font-weight: bold; }
.accordion-content { display: none; padding: 1em; }
.accordion.active .accordion-content { display: block; }
.accordion-header:after { content: "▼"; float: right; transition: transform 0.2s; }
.accordion.active .accordion-header:after { transform: rotate(-180deg); }
.event { margin-bottom: 1.5em; }
.event .op { font-weight: bold; color: #007bff; }
.event .details { margin-bottom: 0.5em; }
.event .migration { color: #888; font-size: 0.95em; }
.final-structure { background: #fff; border: 1px solid #ddd; padding: 1em; border-radius: 8px; }
th, td { padding: 0.5em 1em; border-bottom: 1px solid #eee; }
ul { margin: 0; padding-left: 1.2em; }
pre { background: #f4f4f4; padding: 0.5em; border-radius: 4px; }
</style>
</head>
<body>
<h1>History Report: ` + objectName + `</h1>
<div id="history-accordion">`

	for idx, key := range migrationOrder {
		group := migrationMap[key]
		html += `<div class="accordion` + func() string {
			if idx == 0 {
				return " active"
			}
			return ""
		}() + `">
  <div class="accordion-header" style="display:flex;flex-direction:column;align-items:flex-start;">
    <span class="accordion-title" style="font-weight:bold;color:#111;font-size:1.1em;">` + group.MigrationName + `</span>
    <span class="accordion-date" style="color:#888;font-size:0.95em;margin-top:0.2em;">` + group.Date.Format(time.RFC1123) + `</span>
  </div>
  <div class="accordion-content">`
		for _, ch := range group.Actions {
			html += `<div class="event">
  <div class="op">` + ch.Operation + `</div>
  <div class="details">` + ch.Details + `</div>
</div>
`
		}
		html += `</div></div>`
	}

	html += `</div>
</div>
<div class="structure-col">
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
</div>
</div>
<script>
document.querySelectorAll('.accordion-header').forEach(function(header) {
	header.addEventListener('click', function() {
		var acc = header.parentElement;
		acc.classList.toggle('active');
	});
});
</script>
</body>
</html>`
	return html
}

// 2-column layout for single object
func generateHTMLReportAccordion2Col(
	objectName string,
	migrationMap map[string]*MigrationGroup,
	migrationOrder []string,
	finalTable *CreateTable,
	finalView *CreateView,
	finalFunction *CreateFunction,
	finalProcedure *CreateProcedure,
	finalTrigger *CreateTrigger,
) string {
	html := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>History Report - ` + objectName + `</title>
<style>
body { font-family: Arial, sans-serif; margin: 0; background: #f8f9fa; }
h1 { color: #333; }
.main-container { display: flex; flex-direction: row; height: 100vh; }
.history-col { flex: 1.2; padding: 2em; overflow-y: auto; border-right: 1px solid #eee; background: #f8f9fa; }
.structure-col { flex: 1; padding: 2em; overflow-y: auto; background: #fff; }
.accordion { background: #fff; border-radius: 8px; border: 1px solid #ddd; margin-bottom: 1em; }
.accordion-header { cursor: pointer; padding: 1em; border-bottom: 1px solid #eee; background: #f1f3f4; }
.accordion-title { font-weight: bold; color: #111; font-size: 1.1em; }
.accordion-date { color: #888; font-size: 0.95em; margin-top: 0.2em; }
.accordion-content { display: none; padding: 1em; }
.accordion.active .accordion-content { display: block; }
.accordion-header:after { content: "▼"; float: right; transition: transform 0.2s; }
.accordion.active .accordion-header:after { transform: rotate(-180deg); }
.event { margin-bottom: 1.5em; }
.event .op { font-weight: bold; color: #007bff; }
.event .details { margin-bottom: 0.5em; }
.event .migration { color: #888; font-size: 0.95em; }
.final-structure { background: #fff; border: 1px solid #ddd; padding: 1.5em 1em 1em 1em; border-radius: 8px; margin-top: 1em; }
.structure-table { width: 100%; border-collapse: separate; border-spacing: 0; }
.structure-table th, .structure-table td { padding: 0.6em 1em; border-bottom: 1px solid #eee; text-align: left; }
.structure-table th { background: #f1f3f4; }
.flag-badge { display: inline-block; margin-right: 0.3em; margin-bottom: 0.1em; padding: 2px 7px; border-radius: 4px; font-size: 0.85em; font-weight: 500; }
.flag-pk { background: #007bff; color: #fff; }
.flag-ai { background: #28a745; color: #fff; }
.flag-unique { background: #ffc107; color: #333; }
.flag-index { background: #6c757d; color: #fff; }
.flag-null { background: #17a2b8; color: #fff; }
.flag-default { background: #e83e8c; color: #fff; }
.flag-check { background: #fd7e14; color: #fff; }
@media (max-width: 900px) {
	.main-container { flex-direction: column; }
	.history-col, .structure-col { padding: 1em; }
}
</style>
</head>
<body>
<h1 style="margin:1em 2em 0 2em;">History Report: All Objects</h1>
<div class="main-container">
	<div class="history-col">
<h1>History: ` + objectName + `</h1>
<div id="history-accordion">`
	// Accordion for each migration (date), grouping all actions under that date
	for idx, key := range migrationOrder {
		group := migrationMap[key]
		html += `<div class="accordion` + func() string {
			if idx == 0 {
				return " active"
			}
			return ""
		}() + `">
  <div class="accordion-header" style="display:flex;flex-direction:column;align-items:flex-start;">
    <span class="accordion-title" style="font-weight:bold;color:#111;font-size:1.1em;">` + group.MigrationName + `</span>
    <span class="accordion-date" style="color:#888;font-size:0.95em;margin-top:0.2em;">` + group.Date.Format(time.RFC1123) + `</span>
  </div>
  <div class="accordion-content">`
		for _, ch := range group.Actions {
			html += `<div class="event">
  <div class="op">` + ch.Operation + `</div>
  <div class="details">` + ch.Details + `</div>
</div>
`
		}
		html += `</div></div>`
	}
	html += `</div>
</div>
<div class="structure-col">
<h2>Final Structure</h2>
<div class="final-structure">`
	switch {
	case finalTable != nil:
		html += `<table class="structure-table"><tr><th>Column</th><th>Type</th><th>Flags</th><th>Default</th><th>Check</th></tr>`
		for _, col := range finalTable.Columns {
			flags := ""
			if col.PrimaryKey {
				flags += `<span class="flag-badge flag-pk">PK</span>`
			}
			if col.AutoIncrement {
				flags += `<span class="flag-badge flag-ai">AI</span>`
			}
			if col.Unique {
				flags += `<span class="flag-badge flag-unique">Unique</span>`
			}
			if col.Index {
				flags += `<span class="flag-badge flag-index">Index</span>`
			}
			if col.Nullable {
				flags += `<span class="flag-badge flag-null">Nullable</span>`
			}
			html += `<tr>
<td>` + col.Name + `</td>
<td><code>` + col.Type + `</code></td>
<td>` + flags + `</td>
<td>` + func() string {
				if col.Default != nil && col.Default != "" {
					return `<span class="flag-badge flag-default">` + fmt.Sprintf("%v", col.Default) + `</span>`
				}
				return ""
			}() + `</td>
<td>` + func() string {
				if col.Check != "" {
					return `<span class="flag-badge flag-check">` + col.Check + `</span>`
				}
				return ""
			}() + `</td>
</tr>`
		}
		html += `</table>`
		if len(finalTable.PrimaryKey) > 0 {
			html += `<div style="margin-top:1em;"><b>Primary Key:</b> <span class="flag-badge flag-pk">` + strings.Join(finalTable.PrimaryKey, ", ") + `</span></div>`
		}
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
</div>
</div>
<script>
document.querySelectorAll('.accordion-header').forEach(function(header) {
	header.addEventListener('click', function() {
		var acc = header.parentElement;
		acc.classList.toggle('active');
	});
});
</script>
</body>
</html>`
	return html
}

// 3-column layout for all objects
func generateHTMLReportAllObjects(
	allObjects []objectInfo,
	fileNames []string,
	migrationDir string,
) string {
	// Prepare per-object history and structure
	type ObjectReport struct {
		Name          string
		Type          string
		HistoryHTML   string
		StructureHTML string
	}
	reports := make(map[string]ObjectReport)
	for _, obj := range allObjects {
		// Collect changes and structure for each object
		var changes []MigrationChange
		var finalTable *CreateTable
		var dropped bool
		var finalView *CreateView
		var finalFunction *CreateFunction
		var finalTrigger *CreateTrigger
		var finalProcedure *CreateProcedure

		for _, fname := range fileNames {
			path := filepath.Join(migrationDir, fname)
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
				if strings.EqualFold(ct.Name, obj.Name) {
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
				if strings.EqualFold(at.Name, obj.Name) && finalTable != nil && !dropped {
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
				if strings.EqualFold(dt.Name, obj.Name) {
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
				if strings.EqualFold(cv.Name, obj.Name) {
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
				if strings.EqualFold(dv.Name, obj.Name) {
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
				if strings.EqualFold(rv.OldName, obj.Name) {
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
				if strings.EqualFold(cf.Name, obj.Name) {
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
				if strings.EqualFold(df.Name, obj.Name) {
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
				if strings.EqualFold(rf.OldName, obj.Name) {
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
				if strings.EqualFold(cp.Name, obj.Name) {
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
				if strings.EqualFold(dp.Name, obj.Name) {
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
				if strings.EqualFold(rp.OldName, obj.Name) {
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
				if strings.EqualFold(ct.Name, obj.Name) {
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
				if strings.EqualFold(dt.Name, obj.Name) {
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
				if strings.EqualFold(rt.OldName, obj.Name) {
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

		// Group changes by migration (date)
		migrationMap := make(map[string]*MigrationGroup)
		var migrationOrder []string
		for _, ch := range changes {
			key := ch.Date.Format("20060102150405") + "_" + ch.MigrationName
			if _, ok := migrationMap[key]; !ok {
				migrationMap[key] = &MigrationGroup{
					Date:          ch.Date,
					MigrationName: ch.MigrationName,
				}
				migrationOrder = append(migrationOrder, key)
			}
			migrationMap[key].Actions = append(migrationMap[key].Actions, ch)
		}

		// History HTML
		historyHTML := `<div id="history-accordion-` + obj.Name + `">`
		for idx, key := range migrationOrder {
			group := migrationMap[key]
			historyHTML += `<div class="accordion` + func() string {
				if idx == 0 {
					return " active"
				}
				return ""
			}() + `">
  <div class="accordion-header" style="display:flex;flex-direction:column;align-items:flex-start;">
    <span class="accordion-title" style="font-weight:bold;color:#111;font-size:1.1em;">` + group.MigrationName + `</span>
    <span class="accordion-date" style="color:#888;font-size:0.95em;margin-top:0.2em;">` + group.Date.Format(time.RFC1123) + `</span>
  </div>
  <div class="accordion-content">`
			for _, ch := range group.Actions {
				historyHTML += `<div class="event">
  <div class="op">` + ch.Operation + `</div>
  <div class="details">` + ch.Details + `</div>
</div>
`
			}
			historyHTML += `</div></div>`
		}
		historyHTML += `</div>`

		// Structure HTML
		structureHTML := `<h2 style="margin-top:0;">Final Structure</h2><div class="final-structure">`
		switch obj.Type {
		case "table":
			if finalTable != nil {
				structureHTML += `<table class="structure-table"><tr><th>Column</th><th>Type</th><th>Flags</th><th>Default</th><th>Check</th></tr>`
				for _, col := range finalTable.Columns {
					flags := ""
					if col.PrimaryKey {
						flags += `<span class="flag-badge flag-pk">PK</span>`
					}
					if col.AutoIncrement {
						flags += `<span class="flag-badge flag-ai">AI</span>`
					}
					if col.Unique {
						flags += `<span class="flag-badge flag-unique">Unique</span>`
					}
					if col.Index {
						flags += `<span class="flag-badge flag-index">Index</span>`
					}
					if col.Nullable {
						flags += `<span class="flag-badge flag-null">Nullable</span>`
					}
					structureHTML += `<tr>
<td>` + col.Name + `</td>
<td><code>` + col.Type + `</code></td>
<td>` + flags + `</td>
<td>` + func() string {
						if col.Default != nil && col.Default != "" {
							return `<span class="flag-badge flag-default">` + fmt.Sprintf("%v", col.Default) + `</span>`
						}
						return ""
					}() + `</td>
<td>` + func() string {
						if col.Check != "" {
							return `<span class="flag-badge flag-check">` + col.Check + `</span>`
						}
						return ""
					}() + `</td>
</tr>`
				}
				structureHTML += `</table>`
				if len(finalTable.PrimaryKey) > 0 {
					structureHTML += `<div style="margin-top:1em;"><b>Primary Key:</b> <span class="flag-badge flag-pk">` + strings.Join(finalTable.PrimaryKey, ", ") + `</span></div>`
				}
			} else {
				structureHTML += `<b>Object does not exist (dropped).</b>`
			}
		case "view":
			if finalView != nil {
				structureHTML += describeView(*finalView)
			} else {
				structureHTML += `<b>Object does not exist (dropped).</b>`
			}
		case "function":
			if finalFunction != nil {
				structureHTML += describeFunction(*finalFunction)
			} else {
				structureHTML += `<b>Object does not exist (dropped).</b>`
			}
		case "procedure":
			if finalProcedure != nil {
				structureHTML += describeProcedure(*finalProcedure)
			} else {
				structureHTML += `<b>Object does not exist (dropped).</b>`
			}
		case "trigger":
			if finalTrigger != nil {
				structureHTML += describeTrigger(*finalTrigger)
			} else {
				structureHTML += `<b>Object does not exist (dropped).</b>`
			}
		}
		structureHTML += `</div>`

		reports[obj.Name] = ObjectReport{
			Name:          obj.Name,
			Type:          obj.Type,
			HistoryHTML:   historyHTML,
			StructureHTML: structureHTML,
		}
	}

	// Build the 3-column layout
	html := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>History Report - All Objects</title>
<style>
body { font-family: Arial, sans-serif; margin: 0; background: #f8f9fa; }
h1 { color: #333; }
.layout3col { display: flex; flex-direction: row; height: 100vh; }
.sidebar { width: 260px; background: #222; color: #fff; padding: 0; overflow-y: auto; }
.sidebar h2 { font-size: 1.2em; margin: 0; padding: 1em 1em 0.5em 1em; color: #fff; }
.sidebar ul { list-style: none; margin: 0; padding: 0; }
.sidebar li { padding: 0.8em 1em; cursor: pointer; border-bottom: 1px solid #333; }
.sidebar li.active, .sidebar li:hover { background: #007bff; color: #fff; }
.main-content { flex: 1; display: flex; flex-direction: row; }
.history-col { flex: 1.2; padding: 2em; overflow-y: auto; border-right: 1px solid #eee; background: #f8f9fa; }
.structure-col { flex: 1; padding: 2em; overflow-y: auto; background: #fff; }
.accordion { background: #fff; border-radius: 8px; border: 1px solid #ddd; margin-bottom: 1em; }
.accordion-header { cursor: pointer; padding: 1em; border-bottom: 1px solid #eee; background: #f1f3f4; }
.accordion-title { font-weight: bold; color: #111; font-size: 1.1em; }
.accordion-date { color: #888; font-size: 0.95em; margin-top: 0.2em; }
.accordion-content { display: none; padding: 1em; }
.accordion.active .accordion-content { display: block; }
.accordion-header:after { content: "▼"; float: right; transition: transform 0.2s; }
.accordion.active .accordion-header:after { transform: rotate(-180deg); }
.event { margin-bottom: 1.5em; }
.event .op { font-weight: bold; color: #007bff; }
.event .details { margin-bottom: 0.5em; }
.event .migration { color: #888; font-size: 0.95em; }
.final-structure { background: #fff; border: 1px solid #ddd; padding: 1.5em 1em 1em 1em; border-radius: 8px; margin-top: 1em; }
.structure-table { width: 100%; border-collapse: separate; border-spacing: 0; }
.structure-table th, .structure-table td { padding: 0.6em 1em; border-bottom: 1px solid #eee; text-align: left; }
.structure-table th { background: #f1f3f4; }
.flag-badge { display: inline-block; margin-right: 0.3em; margin-bottom: 0.1em; padding: 2px 7px; border-radius: 4px; font-size: 0.85em; font-weight: 500; }
.flag-pk { background: #007bff; color: #fff; }
.flag-ai { background: #28a745; color: #fff; }
.flag-unique { background: #ffc107; color: #333; }
.flag-index { background: #6c757d; color: #fff; }
.flag-null { background: #17a2b8; color: #fff; }
.flag-default { background: #e83e8c; color: #fff; }
.flag-check { background: #fd7e14; color: #fff; }
@media (max-width: 900px) {
	.layout3col { flex-direction: column; }
	.sidebar { width: 100%; }
	.main-content { flex-direction: column; }
	.history-col, .structure-col { padding: 1em; }
}
</style>
</head>
<body>
<h1 style="margin:1em 2em 0 2em;">History Report: All Objects</h1>
<div class="layout3col">
	<div class="sidebar">
		<h2>Objects</h2>
		<ul id="object-list">`
	for _, obj := range allObjects {
		html += `<li data-obj="` + obj.Name + `" class="">` + obj.Name + ` <span style="font-size:0.85em;color:#aaa;">[` + obj.Type + `]</span></li>`
	}
	html += `</ul>
	</div>
	<div class="main-content">
		<div class="history-col" id="history-panel"></div>
		<div class="structure-col" id="structure-panel"></div>
	</div>
</div>
<script>
var reports = {};`
	for _, obj := range allObjects {
		rep := reports[obj.Name]
		html += `
reports["` + obj.Name + `"] = {
	history: ` + "`" + rep.HistoryHTML + "`" + `,
	structure: ` + "`" + rep.StructureHTML + "`" + `
};`
	}
	html += `
function selectObject(objName) {
	document.querySelectorAll('#object-list li').forEach(function(li) {
		li.classList.remove('active');
		if (li.getAttribute('data-obj') === objName) li.classList.add('active');
	});
	document.getElementById('history-panel').innerHTML = reports[objName].history;
	document.getElementById('structure-panel').innerHTML = reports[objName].structure;
	// Activate accordions
	document.querySelectorAll('.accordion-header').forEach(function(header) {
		header.addEventListener('click', function() {
			var acc = header.parentElement;
			acc.classList.toggle('active');
		});
	});
}
document.querySelectorAll('#object-list li').forEach(function(li) {
	li.addEventListener('click', function() {
		selectObject(li.getAttribute('data-obj'));
	});
});
if (Object.keys(reports).length > 0) {
	selectObject(Object.keys(reports)[0]);
}
</script>
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
