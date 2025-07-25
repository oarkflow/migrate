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
	tmplPath := filepath.Join("templates/history.html")
	tmpl, err := template.New("history.html").
		Funcs(template.FuncMap{
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
			"join":     strings.Join, // Add join helper for template
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
	if err := tmpl.ExecuteTemplate(&buf, "history.html", data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}
