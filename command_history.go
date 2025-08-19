package migrate

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
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
			{
				Name:    "serve",
				Aliases: []string{"s"},
				Usage:   "Serve the HTML report at a local HTTP endpoint instead of writing to a file",
				Value:   "false",
			},
		},
	}
}

type MigrationChange struct {
	MigrationName   string
	Date            time.Time
	Operation       string
	Details         string // Now just raw details, not HTML
	Column          *AddColumn
	DropColumn      *DropColumn
	RenameColumn    *RenameColumn
	CreateTable     *CreateTable
	CreateView      *CreateView
	CreateFunction  *CreateFunction
	CreateProcedure *CreateProcedure
	CreateTrigger   *CreateTrigger
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
	serveFlag := ctx.Option("serve") == "true"

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

	if serveFlag {
		// Serve the HTML report at http://localhost:8080/history
		fmt.Println("Serving history report at http://localhost:8080/history (Press Ctrl+C to stop)")
		http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(report))
		})
		return http.ListenAndServe(":8080", nil)
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

func sortColumnsPriority(cols []AddColumn) []AddColumn {
	// Columns with PrimaryKey or AutoIncrement come first, preserving their original order
	var pri []AddColumn
	var rest []AddColumn
	for _, c := range cols {
		if c.PrimaryKey || c.AutoIncrement {
			pri = append(pri, c)
		} else {
			rest = append(rest, c)
		}
	}
	return append(pri, rest...)
}

func describeTableColumns(cols []AddColumn, pk []string) string {
	cols = sortColumnsPriority(cols)
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
		return fmt.Sprintf(
			` <span class="bg-green-600 text-white px-2 py-0.5 rounded text-xs">%s</span>`,
			label,
		)
	}
	return ""
}

func defaultBadge(def any) string {
	if def == nil || def == "" {
		return ""
	}
	return fmt.Sprintf(
		` <span class="bg-yellow-400 text-gray-900 px-2 py-0.5 rounded text-xs">Default: %v</span>`,
		def,
	)
}

func commentBadge(comment string) string {
	if comment == "" {
		return ""
	}
	return fmt.Sprintf(
		` <span class="bg-cyan-600 text-white px-2 py-0.5 rounded text-xs">Check: %s</span>`,
		comment,
	)
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
	Name           string
	Type           string
	History        []MigrationGroup
	FinalTable     *CreateTable
	FinalView      *CreateView
	FinalFunction  *CreateFunction
	FinalProcedure *CreateProcedure
	FinalTrigger   *CreateTrigger
	Dropped        bool
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
					// Sort columns for CreateTable in history
					sortedCT := ct
					sortedCT.Columns = sortColumnsPriority(sortedCT.Columns)
					changes = append(changes, MigrationChange{
						MigrationName: m.Name,
						Date:          createdAt,
						Operation:     "CreateTable",
						Details:       "",
						CreateTable:   &sortedCT,
					})
					cpy := sortedCT
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
							Details:       "",
							Column:        &ac,
						})
						finalTable.Columns = append(finalTable.Columns, ac)
						finalTable.Columns = sortColumnsPriority(finalTable.Columns)
					}
					for _, dc := range at.DropColumn {
						changes = append(changes, MigrationChange{
							MigrationName: m.Name,
							Date:          createdAt,
							Operation:     "DropColumn",
							Details:       "",
							DropColumn:    &dc,
						})
						if finalTable != nil {
							var newCols []AddColumn
							for _, col := range finalTable.Columns {
								if col.Name != dc.Name {
									newCols = append(newCols, col)
								}
							}
							finalTable.Columns = sortColumnsPriority(newCols)
						}
					}
					for _, rc := range at.RenameColumn {
						changes = append(changes, MigrationChange{
							MigrationName: m.Name,
							Date:          createdAt,
							Operation:     "RenameColumn",
							Details:       "",
							RenameColumn:  &rc,
						})
						if finalTable != nil {
							for i, col := range finalTable.Columns {
								if col.Name == rc.From {
									finalTable.Columns[i].Name = rc.To
								}
							}
							finalTable.Columns = sortColumnsPriority(finalTable.Columns)
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
						Details:       "",
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
						Details:       "",
						CreateView:    &cv,
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
						Details:       "",
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
						Details:       "",
						RenameColumn:  nil, // Not used for views
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
						MigrationName:  m.Name,
						Date:           createdAt,
						Operation:      "CreateFunction",
						Details:        "",
						CreateFunction: &cf,
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
						Details:       "",
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
						Details:       "",
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
						MigrationName:   m.Name,
						Date:            createdAt,
						Operation:       "CreateProcedure",
						Details:         "",
						CreateProcedure: &cp,
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
						Details:       "",
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
						Details:       "",
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
						Details:       "",
						CreateTrigger: &ct,
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
						Details:       "",
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
						Details:       "",
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

		reports[obj.Name] = ObjectReport{
			Name:           obj.Name,
			Type:           obj.Type,
			History:        history,
			FinalTable:     finalTable,
			FinalView:      finalView,
			FinalFunction:  finalFunction,
			FinalProcedure: finalProcedure,
			FinalTrigger:   finalTrigger,
			Dropped:        dropped,
		}
	}

	// Load and execute template
	tmplPath := filepath.Join("examples", "templates", "history.html")

	// Check if template file exists
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		// Fallback to embedded template if file doesn't exist
		return generateFallbackHTMLReport(allObjects, reports)
	}

	tmpl, err := template.New("history.html").
		Funcs(template.FuncMap{
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
			"join":     strings.Join, // Add join helper for template
		}).
		ParseFiles(tmplPath)
	if err != nil {
		// Fallback to embedded template on parse error
		return generateFallbackHTMLReport(allObjects, reports)
	}

	data := HistoryReportTemplateData{
		AllObjects: allObjects,
		Reports:    reports,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "history.html", data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

// generateFallbackHTMLReport creates a basic HTML report when template file is not available
func generateFallbackHTMLReport(allObjects []objectInfo, reports map[string]ObjectReport) (string, error) {
	var html strings.Builder

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
	   <meta charset="UTF-8">
	   <meta name="viewport" content="width=device-width, initial-scale=1.0">
	   <title>Migration History Report</title>
	   <style>
	       body { font-family: Arial, sans-serif; margin: 20px; }
	       .container { max-width: 1200px; margin: 0 auto; }
	       .object-section { margin-bottom: 30px; border: 1px solid #ddd; padding: 20px; }
	       .object-title { font-size: 24px; font-weight: bold; margin-bottom: 10px; }
	       .migration-group { margin-bottom: 20px; }
	       .migration-title { font-size: 18px; font-weight: bold; color: #333; }
	       .migration-date { color: #666; font-size: 14px; }
	       .action { margin: 10px 0; padding: 10px; background: #f5f5f5; }
	       .action-type { font-weight: bold; color: #0066cc; }
	       .column-list { list-style-type: none; padding: 0; }
	       .column-list li { margin: 5px 0; }
	       .badge { padding: 2px 6px; border-radius: 3px; font-size: 12px; margin-left: 5px; }
	       .badge-pk { background: #28a745; color: white; }
	       .badge-unique { background: #17a2b8; color: white; }
	       .badge-nullable { background: #6c757d; color: white; }
	       pre { background: #f8f9fa; padding: 10px; border-radius: 4px; overflow-x: auto; }
	   </style>
</head>
<body>
	   <div class="container">
	       <h1>Migration History Report</h1>
	       <p>Generated on: ` + time.Now().Format(time.RFC3339) + `</p>`)

	for _, obj := range allObjects {
		report := reports[obj.Name]
		html.WriteString(fmt.Sprintf(`
	       <div class="object-section">
	           <div class="object-title">%s (%s)</div>`, obj.Name, obj.Type))

		if len(report.History) == 0 {
			html.WriteString(`<p>No migration history found.</p>`)
		} else {
			for _, group := range report.History {
				html.WriteString(fmt.Sprintf(`
	           <div class="migration-group">
	               <div class="migration-title">%s</div>
	               <div class="migration-date">%s</div>`,
					group.MigrationName, group.Date.Format(time.RFC3339)))

				for _, action := range group.Actions {
					html.WriteString(fmt.Sprintf(`
	               <div class="action">
	                   <span class="action-type">%s</span>`, action.Operation))

					switch action.Operation {
					case "CreateTable":
						if action.CreateTable != nil {
							html.WriteString(`<ul class="column-list">`)
							for _, col := range action.CreateTable.Columns {
								html.WriteString(fmt.Sprintf(`<li><strong>%s</strong> <code>%s</code>`, col.Name, col.Type))
								if col.PrimaryKey {
									html.WriteString(`<span class="badge badge-pk">PK</span>`)
								}
								if col.Unique {
									html.WriteString(`<span class="badge badge-unique">Unique</span>`)
								}
								if col.Nullable {
									html.WriteString(`<span class="badge badge-nullable">Nullable</span>`)
								}
								html.WriteString(`</li>`)
							}
							html.WriteString(`</ul>`)
						}
					case "AddColumn":
						if action.Column != nil {
							html.WriteString(fmt.Sprintf(`: <strong>%s</strong> <code>%s</code>`, action.Column.Name, action.Column.Type))
						}
					case "DropColumn":
						if action.DropColumn != nil {
							html.WriteString(fmt.Sprintf(`: <strong>%s</strong>`, action.DropColumn.Name))
						}
					case "RenameColumn":
						if action.RenameColumn != nil {
							html.WriteString(fmt.Sprintf(`: <strong>%s</strong> → <strong>%s</strong>`, action.RenameColumn.From, action.RenameColumn.To))
						}
					}
					html.WriteString(`</div>`)
				}
				html.WriteString(`</div>`)
			}
		}

		// Show current state
		if report.FinalTable != nil && !report.Dropped {
			html.WriteString(`<h3>Current State</h3><ul class="column-list">`)
			for _, col := range report.FinalTable.Columns {
				html.WriteString(fmt.Sprintf(`<li><strong>%s</strong> <code>%s</code>`, col.Name, col.Type))
				if col.PrimaryKey {
					html.WriteString(`<span class="badge badge-pk">PK</span>`)
				}
				if col.Unique {
					html.WriteString(`<span class="badge badge-unique">Unique</span>`)
				}
				if col.Nullable {
					html.WriteString(`<span class="badge badge-nullable">Nullable</span>`)
				}
				html.WriteString(`</li>`)
			}
			html.WriteString(`</ul>`)
		} else if report.Dropped {
			html.WriteString(`<p><em>This object has been dropped.</em></p>`)
		}

		html.WriteString(`</div>`)
	}

	html.WriteString(`
	   </div>
</body>
</html>`)

	return html.String(), nil
}
