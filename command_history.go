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
	Date           time.Time
	MigrationName  string
	Actions        []MigrationChange
	StructureAfter string // HTML snapshot after this migration
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
	AllObjects      []objectInfo
	Reports         map[string]ObjectReport
	TotalMigrations int
	LastUpdated     string
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

		// For structure snapshots after each migration
		var migrationGroups []MigrationGroup

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
						var newCols []AddColumn
						for _, col := range finalTable.Columns {
							if col.Name != dc.Name {
								newCols = append(newCols, col)
							}
						}
						finalTable.Columns = sortColumnsPriority(newCols)
					}
					for _, rc := range at.RenameColumn {
						changes = append(changes, MigrationChange{
							MigrationName: m.Name,
							Date:          createdAt,
							Operation:     "RenameColumn",
							Details:       "",
							RenameColumn:  &rc,
						})
						for i, col := range finalTable.Columns {
							if col.Name == rc.From {
								finalTable.Columns[i].Name = rc.To
							}
						}
						finalTable.Columns = sortColumnsPriority(finalTable.Columns)
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
		// Track structure state after each migration group
		var tableState *CreateTable
		var viewState *CreateView
		var functionState *CreateFunction
		var procedureState *CreateProcedure
		var triggerState *CreateTrigger
		var droppedState bool

		for _, ch := range changes {
			key := ch.Date.Format("20060102150405") + "_" + ch.MigrationName
			if _, ok := migrationMap[key]; !ok {
				// Copy current state for this migration group
				migrationMap[key] = &MigrationGroup{
					Date:          ch.Date,
					MigrationName: ch.MigrationName,
				}
				migrationOrder = append(migrationOrder, key)
			}
			migrationMap[key].Actions = append(migrationMap[key].Actions, ch)

			// Apply migration to state snapshot
			switch obj.Type {
			case "table":
				switch ch.Operation {
				case "CreateTable":
					if ch.CreateTable != nil {
						cpy := *ch.CreateTable
						tableState = &cpy
						droppedState = false
					}
				case "AddColumn":
					if tableState != nil && ch.Column != nil {
						tableState.Columns = append(tableState.Columns, *ch.Column)
						tableState.Columns = sortColumnsPriority(tableState.Columns)
					}
				case "DropColumn":
					if tableState != nil && ch.DropColumn != nil {
						var newCols []AddColumn
						for _, col := range tableState.Columns {
							if col.Name != ch.DropColumn.Name {
								newCols = append(newCols, col)
							}
						}
						tableState.Columns = sortColumnsPriority(newCols)
					}
				case "RenameColumn":
					if tableState != nil && ch.RenameColumn != nil {
						for i, col := range tableState.Columns {
							if col.Name == ch.RenameColumn.From {
								tableState.Columns[i].Name = ch.RenameColumn.To
							}
						}
						tableState.Columns = sortColumnsPriority(tableState.Columns)
					}
				case "DropTable":
					tableState = nil
					droppedState = true
				}
			case "view":
				switch ch.Operation {
				case "CreateView":
					if ch.CreateView != nil {
						cpy := *ch.CreateView
						viewState = &cpy
					}
				case "DropView":
					viewState = nil
				case "RenameView":
					// Not used for views in this context
				}
			case "function":
				switch ch.Operation {
				case "CreateFunction":
					if ch.CreateFunction != nil {
						cpy := *ch.CreateFunction
						functionState = &cpy
					}
				case "DropFunction":
					functionState = nil
				case "RenameFunction":
					// Not used for functions in this context
				}
			case "procedure":
				switch ch.Operation {
				case "CreateProcedure":
					if ch.CreateProcedure != nil {
						cpy := *ch.CreateProcedure
						procedureState = &cpy
					}
				case "DropProcedure":
					procedureState = nil
				case "RenameProcedure":
					// Not used for procedures in this context
				}
			case "trigger":
				switch ch.Operation {
				case "CreateTrigger":
					if ch.CreateTrigger != nil {
						cpy := *ch.CreateTrigger
						triggerState = &cpy
					}
				case "DropTrigger":
					triggerState = nil
				case "RenameTrigger":
					// Not used for triggers in this context
				}
			}
		}

		// Compute structure snapshot after each migration group
		for _, key := range migrationOrder {
			group := migrationMap[key]
			var structHTML string
			switch obj.Type {
			case "table":
				if tableState != nil && !droppedState {
					structHTML = generateTableHTML(tableState)
				} else {
					structHTML = "<b>Object does not exist (dropped).</b>"
				}
			case "view":
				if viewState != nil {
					structHTML = generateViewHTML(viewState)
				} else {
					structHTML = "<b>Object does not exist (dropped).</b>"
				}
			case "function":
				if functionState != nil {
					structHTML = generateFunctionHTML(functionState)
				} else {
					structHTML = "<b>Object does not exist (dropped).</b>"
				}
			case "procedure":
				if procedureState != nil {
					structHTML = generateProcedureHTML(procedureState)
				} else {
					structHTML = "<b>Object does not exist (dropped).</b>"
				}
			case "trigger":
				if triggerState != nil {
					structHTML = generateTriggerHTML(triggerState)
				} else {
					structHTML = "<b>Object does not exist (dropped).</b>"
				}
			}
			group.StructureAfter = structHTML
			migrationGroups = append(migrationGroups, *group)
		}

		reports[obj.Name] = ObjectReport{
			Name:           obj.Name,
			Type:           obj.Type,
			History:        migrationGroups,
			FinalTable:     finalTable,
			FinalView:      finalView,
			FinalFunction:  finalFunction,
			FinalProcedure: finalProcedure,
			FinalTrigger:   finalTrigger,
			Dropped:        dropped,
		}
	}

	// Calculate TotalMigrations and LastUpdated for template
	totalMigrations := len(fileNames)
	var lastUpdated string
	if totalMigrations > 0 {
		lastUpdated = extractTimeFromFilename(fileNames[len(fileNames)-1]).Format("2006-01-02 15:04:05")
	} else {
		lastUpdated = "N/A"
	}

	// Load and execute template
	tmplPath := filepath.Join("examples", "templates", "history.html")

	// Check if template file exists
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		return generateFallbackHTMLReport(allObjects, reports)
	}

	tmpl, err := template.New("history.html").
		Funcs(template.FuncMap{
			"safeHTML": func(s string) template.HTML { return template.HTML(s) },
			"join":     strings.Join,
			"sub":      func(a, b int) int { return a - b }, // For "What's New" section
		}).
		ParseFiles(tmplPath)
	if err != nil {
		return generateFallbackHTMLReport(allObjects, reports)
	}

	data := HistoryReportTemplateData{
		AllObjects:      allObjects,
		Reports:         reports,
		TotalMigrations: totalMigrations,
		LastUpdated:     lastUpdated,
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
	<title>Migration History Dashboard</title>
	<script src="https://cdn.tailwindcss.com"></script>
	<style>
		.hero-gradient { background: linear-gradient(90deg, #2563eb 0%, #38bdf8 100%); }
		#feedback-btn { position: fixed; bottom: 24px; right: 24px; z-index: 50; background: linear-gradient(90deg, #2563eb 0%, #38bdf8 100%); color: white; border-radius: 9999px; box-shadow: 0 2px 8px rgba(37,99,235,0.15); padding: 0.75rem 1.5rem; font-weight: 600; cursor: pointer; transition: box-shadow 0.2s; }
		#feedback-btn:hover { box-shadow: 0 4px 16px rgba(37,99,235,0.25); background: linear-gradient(90deg, #38bdf8 0%, #2563eb 100%);}
		#back-to-top { position: fixed; bottom: 80px; right: 32px; z-index: 40; background: #fff; color: #2563eb; border-radius: 9999px; box-shadow: 0 2px 8px rgba(37,99,235,0.15); padding: 0.5rem 1rem; font-weight: 600; cursor: pointer; border: 1px solid #2563eb; display: none;}
		#back-to-top.show { display: block;}
	</style>
</head>
<body class="bg-gray-50 text-gray-800 font-sans text-sm">
	<!-- Hero Section -->
	<section class="hero-gradient py-8 px-6 flex items-center justify-between text-white shadow-lg mb-2">
		<div class="flex items-center space-x-4">
			<span class="inline-block w-16 h-16 bg-white/20 rounded-full flex items-center justify-center text-3xl font-bold">🗂️</span>
			<div>
				<h1 class="text-3xl font-bold tracking-tight mb-1">Migration History Dashboard</h1>
				<p class="text-base opacity-90">Track, explore, and understand your database changes visually.<br>
				<span class="text-xs opacity-80">Your database story, beautifully visualized.</span></p>
			</div>
		</div>
		<div class="hidden md:flex flex-col items-end space-y-2">
			<span class="bg-white/10 px-3 py-1 rounded-full text-sm font-semibold shadow">Powered by <a href="https://github.com/oarkflow/migrate" class="underline">Migrate</a></span>
		</div>
	</section>
	<!-- Quick Stats -->
	<section class="w-full py-2 px-6 flex items-center justify-between border-b border-gray-200 bg-gradient-to-r from-blue-50 to-blue-100">
		<div class="flex space-x-6">
			<div class="flex items-center space-x-2 bg-white rounded-lg px-3 py-2 shadow-sm">
				<span class="font-semibold">Objects:</span>
				<span class="text-blue-700 text-lg font-bold">` + fmt.Sprintf("%d", len(allObjects)) + `</span>
			</div>
			<div class="flex items-center space-x-2 bg-white rounded-lg px-3 py-2 shadow-sm">
				<span class="font-semibold">Migrations:</span>
				<span class="text-green-700 text-lg font-bold">` + fmt.Sprintf("%d", countTotalMigrations(reports)) + `</span>
			</div>
			<div class="flex items-center space-x-2 bg-white rounded-lg px-3 py-2 shadow-sm">
				<span class="font-semibold">Last Updated:</span>
				<span class="text-purple-700 text-lg font-bold">` + time.Now().Format("2006-01-02 15:04:05") + `</span>
			</div>
		</div>
		<div class="hidden sm:flex items-center space-x-2">
			<span class="text-xs text-gray-500">Tip: Use the sidebar to filter objects.</span>
		</div>
	</section>
	<!-- What's New Section -->
	<section class="w-full px-6 py-4 bg-white border-b border-gray-200">
		<h2 class="text-lg font-bold mb-2 flex items-center">What's New?</h2>
		<div class="flex flex-wrap gap-4">
`)
	// What's New: show latest migration for each object
	for _, obj := range allObjects {
		report := reports[obj.Name]
		if len(report.History) > 0 {
			latest := report.History[len(report.History)-1]
			html.WriteString(`<div class="bg-blue-50 rounded-lg px-4 py-2 shadow-sm flex flex-col min-w-[220px]">
				<div class="flex items-center space-x-2 mb-1">
					<span class="font-semibold text-blue-700">` + report.Name + `</span>
					<span class="text-xs text-gray-400">[` + report.Type + `]</span>
				</div>
				<div class="text-xs text-gray-600 mb-1">` + latest.Date.Format("2006-01-02 15:04:05") + `</div>
				<div class="text-sm font-medium text-gray-800">`)
			for _, act := range latest.Actions {
				html.WriteString(`<span class="inline-block bg-blue-600 text-white px-2 py-0.5 rounded-full mr-1">` + act.Operation + `</span>`)
			}
			html.WriteString(`</div></div>`)
		}
	}
	html.WriteString(`</div></section>
	<div class="flex h-[calc(100vh-110px)] overflow-hidden">
		<!-- Sidebar -->
		<aside id="sidebar" class="w-64 bg-gradient-to-b from-blue-900 to-blue-700 text-gray-100 flex-shrink-0 overflow-y-auto shadow-lg border-r border-blue-800">
			<nav class="mt-2">
				<h2 class="px-4 text-xs font-semibold uppercase tracking-wider text-blue-200 mb-1">Objects</h2>
				<div class="px-4 mb-2">
					<input id="object-search" type="text" placeholder="Search objects..." class="w-full px-2 py-1 rounded bg-blue-800 text-gray-100 border border-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-400 text-sm" />
				</div>
				<ul id="object-list" class="mt-1">
`)
	for _, obj := range allObjects {
		html.WriteString(`<li data-obj="` + obj.Name + `" class="px-4 py-2 rounded hover:bg-blue-600 hover:text-white cursor-pointer transition-colors text-sm select-none flex items-center space-x-2">
			<span class="font-medium">` + obj.Name + `</span>
			<span class="text-xs text-blue-200 ml-1">[` + obj.Type + `]</span>
		</li>`)
	}
	html.WriteString(`</ul>
			</nav>
		</aside>
		<!-- Main Content -->
		<main class="flex-1 flex flex-col overflow-hidden">
			<div class="flex-1 flex flex-col sm:flex-row overflow-auto">
				<section class="sm:w-full p-3 bg-white overflow-y-auto">
					<div class="border-b border-gray-200 mb-4 flex items-center justify-between">
						<div class="flex space-x-2" id="tab-buttons">
							<button id="tab-structure-btn" class="px-4 py-2 text-sm font-medium text-blue-700 bg-blue-50 rounded-t border border-b-0 border-gray-200 focus:outline-none transition-colors duration-150 hover:bg-blue-100" data-tab="structure" type="button">Final Structure</button>
							<button id="tab-history-btn" class="px-4 py-2 text-sm font-medium text-gray-700 bg-white rounded-t border border-b-0 border-gray-200 focus:outline-none transition-colors duration-150 hover:bg-blue-100" data-tab="history" type="button">History</button>
						</div>
					</div>
					<div>
						<div id="structure-panel" class="tab-panel"></div>
						<div id="history-panel" class="tab-panel hidden"></div>
					</div>
				</section>
			</div>
			<!-- Tips & Usage Section moved below main content -->
			<section class="w-full px-6 py-3 bg-gradient-to-r from-blue-50 to-blue-100 border-t border-gray-200 mt-8">
				<h2 class="text-md font-bold mb-1 flex items-center">Tips & Usage</h2>
				<ul class="list-disc ml-6 text-sm text-gray-700">
					<li>Click objects in the sidebar to view their structure and history.</li>
					<li>Use the search box to quickly find objects.</li>
					<li>Switch between <b>Final Structure</b> and <b>History</b> tabs for details.</li>
					<li>Copy migration details or SQL definitions using the copy button.</li>
					<li>Toggle dark/light mode for your comfort.</li>
					<li>Use the feedback button to share your thoughts!</li>
				</ul>
			</section>
		</main>
	</div>
	<button id="feedback-btn" onclick="window.open('https://github.com/oarkflow/migrate/issues', '_blank')">Feedback</button>
	<button id="back-to-top" onclick="window.scrollTo({top:0,behavior:'smooth'})">↑ Top</button>
	<!-- End main content -->
	<script>
		// Sidebar filter
		document.getElementById('object-search').addEventListener('input', function () {
			const val = this.value.toLowerCase();
			document.querySelectorAll('#object-list li').forEach(li => {
				const name = li.getAttribute('data-obj').toLowerCase();
				li.style.display = name.includes(val) ? '' : 'none';
			});
		});
		// Tabs
		function showTab(tab) {
			document.getElementById('tab-structure-btn').classList.toggle('bg-blue-50', tab === 'structure');
			document.getElementById('tab-structure-btn').classList.toggle('text-blue-700', tab === 'structure');
			document.getElementById('tab-structure-btn').classList.toggle('bg-white', tab !== 'structure');
			document.getElementById('tab-structure-btn').classList.toggle('text-gray-700', tab !== 'structure');
			document.getElementById('tab-history-btn').classList.toggle('bg-blue-50', tab === 'history');
			document.getElementById('tab-history-btn').classList.toggle('text-blue-700', tab === 'history');
			document.getElementById('tab-history-btn').classList.toggle('bg-white', tab !== 'history');
			document.getElementById('tab-history-btn').classList.toggle('text-gray-700', tab !== 'history');
			document.getElementById('structure-panel').classList.toggle('hidden', tab !== 'structure');
			document.getElementById('history-panel').classList.toggle('hidden', tab !== 'history');
		}
		document.getElementById('tab-structure-btn').addEventListener('click', function () { showTab('structure'); });
		document.getElementById('tab-history-btn').addEventListener('click', function () { showTab('history'); });
		// Object selection
		var reports = {};
`)
	// Prepare JS object for reports
	for _, obj := range allObjects {
		report := reports[obj.Name]
		structure := ""
		history := ""
		// Structure panel
		if report.Type == "table" {
			if report.FinalTable != nil && !report.Dropped {
				structure += `<table class="min-w-full border border-gray-200 mb-2 text-xs"><thead><tr><th class="border px-2 py-1 bg-gray-100">Column</th><th class="border px-2 py-1 bg-gray-100">Type</th><th class="border px-2 py-1 bg-gray-100">Flags</th></tr></thead><tbody>`
				for _, col := range report.FinalTable.Columns {
					flags := ""
					if col.PrimaryKey {
						flags += `<span class="bg-green-600 text-white px-1 py-0.5 rounded text-2xs mr-1">PK</span>`
					}
					if col.AutoIncrement {
						flags += `<span class="bg-blue-600 text-white px-1 py-0.5 rounded text-2xs mr-1">AI</span>`
					}
					if col.Unique {
						flags += `<span class="bg-purple-600 text-white px-1 py-0.5 rounded text-2xs mr-1">Unique</span>`
					}
					if col.Index {
						flags += `<span class="bg-orange-500 text-white px-1 py-0.5 rounded text-2xs mr-1">Index</span>`
					}
					if col.Nullable {
						flags += `<span class="bg-gray-500 text-white px-1 py-0.5 rounded text-2xs mr-1">Nullable</span>`
					}
					structure += `<tr><td class="border px-2 py-1">` + col.Name + `</td><td class="border px-2 py-1"><code>` + col.Type + `</code></td><td class="border px-2 py-1">` + flags + `</td></tr>`
				}
				structure += `</tbody></table>`
			} else {
				structure += `<b>Object does not exist (dropped).</b>`
			}
		}
		// History panel: show actions and structure after each migration
		for _, group := range report.History {
			history += `<div class="border border-gray-200 rounded shadow-sm mb-2"><div class="px-3 py-2 bg-gray-100"><span class="text-base font-semibold text-gray-800">` + group.MigrationName + `</span> <span class="text-xs text-gray-500 ml-2">` + group.Date.Format("2006-01-02 15:04:05") + `</span></div><div class="px-3 py-2 bg-white text-sm">`
			for _, action := range group.Actions {
				history += `<div class="mb-2"><span class="font-medium text-blue-600">` + action.Operation + `</span> `
				switch action.Operation {
				case "AddColumn":
					if action.Column != nil {
						history += `<b>` + action.Column.Name + `</b> <code>` + action.Column.Type + `</code>`
					}
				case "DropColumn":
					if action.DropColumn != nil {
						history += `Dropped column: <b>` + action.DropColumn.Name + `</b>`
					}
				case "RenameColumn":
					if action.RenameColumn != nil {
						history += `Renamed column: <b>` + action.RenameColumn.From + `</b> to <b>` + action.RenameColumn.To + `</b>`
					}
				case "CreateTable":
					if action.CreateTable != nil {
						history += `<b>Table created:</b>`
					}
				case "DropTable":
					history += `Table dropped`
				}
				history += `</div>`
			}
			// Show structure after migration
			history += `<div class="mt-2 border-t pt-2"><b>Structure after migration:</b><br>` + group.StructureAfter + `</div>`
			history += `</div></div>`
		}
		html.WriteString(`reports["` + obj.Name + `"] = {structure: ` + "`" + structure + "`" + `, history: ` + "`" + history + "`" + `};`)
	}
	html.WriteString(`
		document.querySelectorAll('#object-list li').forEach(li => {
			li.addEventListener('click', function() {
				document.querySelectorAll('#object-list li').forEach(l => l.classList.remove('bg-blue-600', 'text-white'));
				li.classList.add('bg-blue-600', 'text-white');
				document.getElementById('structure-panel').innerHTML = reports[li.getAttribute('data-obj')].structure;
				document.getElementById('history-panel').innerHTML = reports[li.getAttribute('data-obj')].history;
			});
		});
		// Auto-select first object and default to "Final Structure" tab
		document.addEventListener('DOMContentLoaded', function() {
			var first = document.querySelector('#object-list li');
			if (first) first.click();
			showTab('structure');
		});
		window.addEventListener('scroll', function () {
			document.getElementById('back-to-top').classList.toggle('show', window.scrollY > 300);
		});
	</script>
	<footer class="w-full bg-white border-t border-gray-200 py-2 px-6 flex items-center justify-between text-xs text-gray-500">
		<div>
			&copy; 2025 Migration Dashboard. <a href="https://github.com/oarkflow/migrate" class="underline hover:text-blue-600">GitHub</a>
		</div>
		<div>
			<a href="#" class="underline hover:text-blue-600">Help</a> | <a href="#" class="underline hover:text-blue-600">Feedback</a>
		</div>
	</footer>
</body>
</html>
`)
	return html.String(), nil
}

// Helper for fallback stats
func countTotalMigrations(reports map[string]ObjectReport) int {
	m := make(map[string]struct{})
	for _, r := range reports {
		for _, h := range r.History {
			m[h.MigrationName] = struct{}{}
		}
	}
	return len(m)
}

// Helper functions for structure HTML
func generateTableHTML(table *CreateTable) string {
	if table == nil {
		return "<b>Object does not exist (dropped).</b>"
	}
	var b strings.Builder
	b.WriteString(`<table class="min-w-full border border-gray-200 mb-2 text-xs"><thead><tr><th class="border px-2 py-1 bg-gray-100">Column</th><th class="border px-2 py-1 bg-gray-100">Type</th><th class="border px-2 py-1 bg-gray-100">Flags</th></tr></thead><tbody>`)
	for _, col := range table.Columns {
		flags := ""
		if col.PrimaryKey {
			flags += `<span class="bg-green-600 text-white px-1 py-0.5 rounded text-2xs mr-1">PK</span>`
		}
		if col.AutoIncrement {
			flags += `<span class="bg-blue-600 text-white px-1 py-0.5 rounded text-2xs mr-1">AI</span>`
		}
		if col.Unique {
			flags += `<span class="bg-purple-600 text-white px-1 py-0.5 rounded text-2xs mr-1">Unique</span>`
		}
		if col.Index {
			flags += `<span class="bg-orange-500 text-white px-1 py-0.5 rounded text-2xs mr-1">Index</span>`
		}
		if col.Nullable {
			flags += `<span class="bg-gray-500 text-white px-1 py-0.5 rounded text-2xs mr-1">Nullable</span>`
		}
		b.WriteString(`<tr><td class="border px-2 py-1">` + col.Name + `</td><td class="border px-2 py-1"><code>` + col.Type + `</code></td><td class="border px-2 py-1">` + flags + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	if len(table.PrimaryKey) > 0 {
		b.WriteString(`<div class="mt-2"><b>Primary Key:</b> <span class="bg-green-600 text-white px-1 py-0.5 rounded text-2xs ml-1">` + strings.Join(table.PrimaryKey, ", ") + `</span></div>`)
	}
	return b.String()
}
func generateViewHTML(view *CreateView) string {
	if view == nil {
		return "<b>Object does not exist (dropped).</b>"
	}
	return `<b>Name:</b> ` + view.Name + `<br><b>Definition:</b><pre class="bg-gray-100 rounded p-2 text-xs mt-1">` + view.Definition + `</pre>`
}
func generateFunctionHTML(fn *CreateFunction) string {
	if fn == nil {
		return "<b>Object does not exist (dropped).</b>"
	}
	return `<b>Name:</b> ` + fn.Name + `<br><b>Definition:</b><pre class="bg-gray-100 rounded p-2 text-xs mt-1">` + fn.Definition + `</pre>`
}
func generateProcedureHTML(proc *CreateProcedure) string {
	if proc == nil {
		return "<b>Object does not exist (dropped).</b>"
	}
	return `<b>Name:</b> ` + proc.Name + `<br><b>Definition:</b><pre class="bg-gray-100 rounded p-2 text-xs mt-1">` + proc.Definition + `</pre>`
}
func generateTriggerHTML(trig *CreateTrigger) string {
	if trig == nil {
		return "<b>Object does not exist (dropped).</b>"
	}
	return `<b>Name:</b> ` + trig.Name + `<br><b>Definition:</b><pre class="bg-gray-100 rounded p-2 text-xs mt-1">` + trig.Definition + `</pre>`
}
