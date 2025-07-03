package migrate

import (
	"bytes"
	"fmt"
	"html/template"
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

	objectSet := make(map[string]string)
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

	var allObjects []objectInfo
	if objectName == "" {
		for name, typ := range objectSet {
			allObjects = append(allObjects, objectInfo{Name: name, Type: typ})
		}
		sort.Slice(allObjects, func(i, j int) bool { return allObjects[i].Name < allObjects[j].Name })
	} else {
		objectName = strings.ToLower(objectName)
		typ, ok := objectSet[objectName]
		if !ok {
			return fmt.Errorf("object %s not found", objectName)
		}
		allObjects = append(allObjects, objectInfo{Name: objectName, Type: typ})
	}

	report, err := generateHTMLReportAllObjectsTemplate(allObjects, fileNames, c.Driver.MigrationDir())
	if err != nil {
		return err
	}
	reportPath := filepath.Join(".", fmt.Sprintf("history_%s_%d.html", func() string {
		if objectName == "" {
			return "all"
		}
		return objectName
	}(), time.Now().Unix()))
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

type ObjectReport struct {
	Name          string
	Type          string
	History       []MigrationGroup
	StructureHTML template.HTML
}

type HistoryReportTemplateData struct {
	AllObjects []objectInfo
	Reports    map[string]ObjectReport
}

// Main template-based report generator
func generateHTMLReportAllObjectsTemplate(
	allObjects []objectInfo,
	fileNames []string,
	migrationDir string,
) (string, error) {
	reports := make(map[string]ObjectReport)
	for _, obj := range allObjects {
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
		var history []MigrationGroup
		for _, key := range migrationOrder {
			history = append(history, *migrationMap[key])
		}

		// Structure HTML
		var structureHTML string
		structureHTML += `<h2 style="margin-top:0;">Final Structure</h2><div class="final-structure">`
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
			History:       history,
			StructureHTML: template.HTML(structureHTML),
		}
	}

	// Load and execute template
	tmplPath := filepath.Join("templates/history.html")
	tmpl, err := template.New("history.html").
		Funcs(template.FuncMap{
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		}).
		ParseFiles(tmplPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := HistoryReportTemplateData{
		AllObjects: allObjects,
		Reports:    reports,
	}

	var buf bytes.Buffer
	// Use the base name of the template file for ExecuteTemplate
	if err := tmpl.ExecuteTemplate(&buf, "history.html", data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}
