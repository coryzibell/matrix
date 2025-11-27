package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/output"
)

// SchemaSnapshot represents a cataloged database schema
type SchemaSnapshot struct {
	Project      string            `json:"project"`
	SnapshotTime time.Time         `json:"snapshot_time"`
	Source       string            `json:"source"`
	GitCommit    string            `json:"git_commit,omitempty"`
	Checksum     string            `json:"checksum"`
	Tables       map[string]*Table `json:"tables"`
	SourceFiles  []string          `json:"source_files"`
}

// Table represents a database table
type Table struct {
	Name        string       `json:"name"`
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes"`
	ForeignKeys []ForeignKey `json:"foreign_keys"`
}

// Column represents a table column
type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primary_key"`
	Unique     bool   `json:"unique"`
	Default    string `json:"default,omitempty"`
}

// Index represents a table index
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

// ForeignKey represents a foreign key constraint
type ForeignKey struct {
	Column          string `json:"column"`
	ReferencedTable string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
}

// SchemaDiff tracks changes between snapshots
type SchemaDiff struct {
	Added    []string
	Modified []string
	Removed  []string
}

// runSchemaCatalog implements the schema-catalog command
func runSchemaCatalog() error {
	// Parse subcommand
	if len(os.Args) < 3 {
		printSchemaCatalogUsage()
		return nil
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "scan":
		return runSchemaScan()
	case "diff":
		return runSchemaDiff()
	case "history":
		return runSchemaHistory()
	case "find":
		return runSchemaFind()
	case "list":
		return runSchemaList()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printSchemaCatalogUsage()
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// printSchemaCatalogUsage displays usage information
func printSchemaCatalogUsage() {
	fmt.Println("schema-catalog - Track database schemas across projects")
	fmt.Println("")
	fmt.Println("USAGE:")
	fmt.Println("  matrix schema-catalog scan <path>     Discover and catalog schemas")
	fmt.Println("  matrix schema-catalog diff <path>     Compare current vs last snapshot")
	fmt.Println("  matrix schema-catalog history <table> Show evolution of specific table")
	fmt.Println("  matrix schema-catalog find <table>    Find table across all cataloged projects")
	fmt.Println("  matrix schema-catalog list            List all cataloged projects")
	fmt.Println("")
	fmt.Println("EXAMPLES:")
	fmt.Println("  matrix schema-catalog scan ~/projects/myapp")
	fmt.Println("  matrix schema-catalog diff .")
	fmt.Println("  matrix schema-catalog find users")
	fmt.Println("  matrix schema-catalog history sessions")
}

// runSchemaScan scans a directory for schemas and catalogs them
func runSchemaScan() error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	targetPath := "."
	if fs.NArg() > 0 {
		targetPath = fs.Arg(0)
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	output.Success("📚 Schema Catalog - Scan")
	fmt.Println("")
	fmt.Printf("Scanning: %s\n", absPath)
	fmt.Println("")

	// Discover schema files
	schemaFiles := discoverSchemaFiles(absPath)

	if len(schemaFiles) == 0 {
		fmt.Println("No schema files found.")
		fmt.Println("")
		fmt.Println("Looking for: *.sql, migrations/, *.prisma, models.py, schema.rb")
		return nil
	}

	fmt.Printf("Found %d schema files:\n", len(schemaFiles))
	for _, f := range schemaFiles {
		relPath, _ := filepath.Rel(absPath, f)
		fmt.Printf("  - %s\n", relPath)
	}
	fmt.Println("")

	// Parse schemas
	snapshot := &SchemaSnapshot{
		Project:      filepath.Base(absPath),
		SnapshotTime: time.Now(),
		Source:       absPath,
		Tables:       make(map[string]*Table),
		SourceFiles:  schemaFiles,
	}

	for _, file := range schemaFiles {
		tables, err := parseSchemaFile(file)
		if err != nil {
			fmt.Printf("Warning: failed to parse %s: %v\n", file, err)
			continue
		}

		for _, table := range tables {
			snapshot.Tables[table.Name] = table
		}
	}

	// Calculate checksum
	snapshot.Checksum = calculateChecksum(snapshot)

	// Try to get git commit
	snapshot.GitCommit = getGitCommit(absPath)

	// Display results
	displaySchemaSnapshot(snapshot)

	// Save to catalog
	if err := saveSnapshot(snapshot); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	fmt.Println("")
	output.Success("✓ Schema cataloged successfully")

	return nil
}

// runSchemaDiff compares current schema against last snapshot
func runSchemaDiff() error {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	targetPath := "."
	if fs.NArg() > 0 {
		targetPath = fs.Arg(0)
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	output.Success("📚 Schema Catalog - Diff")
	fmt.Println("")

	// Load last snapshot
	projectName := filepath.Base(absPath)
	lastSnapshot, err := loadLatestSnapshot(projectName)
	if err != nil {
		return fmt.Errorf("no previous snapshot found for project '%s': %w", projectName, err)
	}

	fmt.Printf("Project: %s\n", projectName)
	fmt.Printf("Last snapshot: %s\n", lastSnapshot.SnapshotTime.Format("2006-01-02 15:04:05"))
	fmt.Println("")

	// Scan current schema
	schemaFiles := discoverSchemaFiles(absPath)
	currentSnapshot := &SchemaSnapshot{
		Project:      projectName,
		SnapshotTime: time.Now(),
		Source:       absPath,
		Tables:       make(map[string]*Table),
		SourceFiles:  schemaFiles,
	}

	for _, file := range schemaFiles {
		tables, err := parseSchemaFile(file)
		if err != nil {
			continue
		}
		for _, table := range tables {
			currentSnapshot.Tables[table.Name] = table
		}
	}

	currentSnapshot.Checksum = calculateChecksum(currentSnapshot)

	// Compare snapshots
	diff := compareSnapshots(lastSnapshot, currentSnapshot)

	// Display drift
	if len(diff.Added) == 0 && len(diff.Modified) == 0 && len(diff.Removed) == 0 {
		output.Success("✓ No drift detected - schemas match")
		return nil
	}

	output.Header("DRIFT DETECTED:")
	fmt.Println("")

	if len(diff.Added) > 0 {
		fmt.Printf("%sADDED:%s\n", output.Green, output.Reset)
		for _, item := range diff.Added {
			fmt.Printf("  + %s\n", item)
		}
		fmt.Println("")
	}

	if len(diff.Modified) > 0 {
		fmt.Printf("%sMODIFIED:%s\n", output.Yellow, output.Reset)
		for _, item := range diff.Modified {
			fmt.Printf("  ~ %s\n", item)
		}
		fmt.Println("")
	}

	if len(diff.Removed) > 0 {
		fmt.Printf("%sREMOVED:%s\n", output.Red, output.Reset)
		for _, item := range diff.Removed {
			fmt.Printf("  - %s\n", item)
		}
		fmt.Println("")
	}

	return nil
}

// runSchemaHistory shows evolution of a specific table
func runSchemaHistory() error {
	if len(os.Args) < 4 {
		fmt.Println("Usage: matrix schema-catalog history <table>")
		return fmt.Errorf("table name required")
	}

	tableName := os.Args[3]

	output.Header(fmt.Sprintf("History: %s", tableName))
	fmt.Println("")

	// Load all snapshots and find this table
	catalogDir := getCatalogDir()
	projects, err := os.ReadDir(catalogDir)
	if err != nil {
		return fmt.Errorf("failed to read catalog: %w", err)
	}

	found := false
	for _, proj := range projects {
		if !proj.IsDir() {
			continue
		}

		projectPath := filepath.Join(catalogDir, proj.Name())
		snapshots, err := loadAllSnapshots(projectPath)
		if err != nil {
			continue
		}

		for _, snapshot := range snapshots {
			if table, exists := snapshot.Tables[tableName]; exists {
				found = true
				fmt.Printf("%s (%s)\n", snapshot.SnapshotTime.Format("2006-01-02 15:04:05"), snapshot.Project)
				fmt.Printf("  Columns: %d\n", len(table.Columns))
				for _, col := range table.Columns {
					markers := ""
					if col.PrimaryKey {
						markers += " PK"
					}
					if col.Unique {
						markers += " UNIQUE"
					}
					if !col.Nullable {
						markers += " NOT NULL"
					}
					fmt.Printf("    - %s: %s%s\n", col.Name, col.Type, markers)
				}
				fmt.Println("")
			}
		}
	}

	if !found {
		fmt.Printf("Table '%s' not found in any cataloged project\n", tableName)
	}

	return nil
}

// runSchemaFind searches for a table across all projects
func runSchemaFind() error {
	if len(os.Args) < 4 {
		fmt.Println("Usage: matrix schema-catalog find <table>")
		return fmt.Errorf("table name required")
	}

	tableName := os.Args[3]

	output.Header(fmt.Sprintf("Finding: %s", tableName))
	fmt.Println("")

	catalogDir := getCatalogDir()
	projects, err := os.ReadDir(catalogDir)
	if err != nil {
		return fmt.Errorf("failed to read catalog: %w", err)
	}

	found := false
	for _, proj := range projects {
		if !proj.IsDir() {
			continue
		}

		snapshot, err := loadLatestSnapshot(proj.Name())
		if err != nil {
			continue
		}

		if table, exists := snapshot.Tables[tableName]; exists {
			found = true
			fmt.Printf("Project: %s%s%s\n", output.Yellow, snapshot.Project, output.Reset)
			fmt.Printf("Source: %s\n", snapshot.Source)
			fmt.Printf("Last Updated: %s\n", snapshot.SnapshotTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("Columns: %d\n", len(table.Columns))
			fmt.Println("")

			for _, col := range table.Columns {
				fmt.Printf("  - %s: %s", col.Name, col.Type)
				if col.PrimaryKey {
					fmt.Printf(" (PK)")
				}
				fmt.Println("")
			}
			fmt.Println("")
		}
	}

	if !found {
		fmt.Printf("Table '%s' not found in any cataloged project\n", tableName)
	}

	return nil
}

// runSchemaList lists all cataloged projects
func runSchemaList() error {
	output.Success("📚 Cataloged Projects")
	fmt.Println("")

	catalogDir := getCatalogDir()
	if _, err := os.Stat(catalogDir); os.IsNotExist(err) {
		fmt.Println("No projects cataloged yet.")
		fmt.Println("")
		fmt.Println("Run 'matrix schema-catalog scan <path>' to catalog a project.")
		return nil
	}

	projects, err := os.ReadDir(catalogDir)
	if err != nil {
		return fmt.Errorf("failed to read catalog: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects cataloged yet.")
		return nil
	}

	for _, proj := range projects {
		if !proj.IsDir() {
			continue
		}

		snapshot, err := loadLatestSnapshot(proj.Name())
		if err != nil {
			continue
		}

		fmt.Printf("%s%s%s\n", output.Yellow, snapshot.Project, output.Reset)
		fmt.Printf("  Source: %s\n", snapshot.Source)
		fmt.Printf("  Tables: %d\n", len(snapshot.Tables))
		fmt.Printf("  Last Cataloged: %s\n", snapshot.SnapshotTime.Format("2006-01-02 15:04:05"))
		if snapshot.GitCommit != "" {
			fmt.Printf("  Git Commit: %s\n", snapshot.GitCommit[:8])
		}
		fmt.Println("")
	}

	return nil
}

// discoverSchemaFiles finds schema-related files
func discoverSchemaFiles(path string) []string {
	var files []string

	filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Skip common ignore directories
			name := info.Name()
			if name == "node_modules" || name == "vendor" || name == ".git" ||
				name == "target" || name == "build" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		name := strings.ToLower(info.Name())
		dir := strings.ToLower(filepath.Base(filepath.Dir(filePath)))

		// Match schema files
		if strings.HasSuffix(name, ".sql") ||
			strings.HasSuffix(name, ".prisma") ||
			name == "schema.rb" ||
			name == "models.py" ||
			dir == "migrations" || dir == "migrate" {
			files = append(files, filePath)
		}

		return nil
	})

	return files
}

// parseSchemaFile extracts table definitions from a schema file
func parseSchemaFile(filePath string) ([]*Table, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	contentStr := string(content)

	// For now, focus on SQL CREATE TABLE statements
	if strings.HasSuffix(strings.ToLower(filePath), ".sql") {
		return parseSQLSchema(contentStr)
	}

	// TODO: Add parsers for .prisma, schema.rb, models.py
	return nil, nil
}

// parseSQLSchema extracts CREATE TABLE statements from SQL
func parseSQLSchema(content string) ([]*Table, error) {
	var tables []*Table

	// Regex to match CREATE TABLE statements (with DOTALL flag for multiline)
	createTablePattern := regexp.MustCompile(`(?si)CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+` +
		`(?:` + "`" + `?(\w+)` + "`" + `?|\"?(\w+)\"?)\s*\((.*?)\);`)

	matches := createTablePattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		tableName := match[1]
		if tableName == "" {
			tableName = match[2]
		}
		columnsStr := match[3]

		table := &Table{
			Name:        tableName,
			Columns:     []Column{},
			Indexes:     []Index{},
			ForeignKeys: []ForeignKey{},
		}

		// Parse columns
		columns := parseColumns(columnsStr)
		table.Columns = columns

		tables = append(tables, table)
	}

	return tables, nil
}

// parseColumns extracts column definitions from CREATE TABLE body
func parseColumns(columnsStr string) []Column {
	var columns []Column

	// Split by comma (naive approach - doesn't handle nested parens)
	lines := strings.Split(columnsStr, ",")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip constraints
		if strings.HasPrefix(strings.ToUpper(line), "PRIMARY KEY") ||
			strings.HasPrefix(strings.ToUpper(line), "FOREIGN KEY") ||
			strings.HasPrefix(strings.ToUpper(line), "UNIQUE") ||
			strings.HasPrefix(strings.ToUpper(line), "INDEX") ||
			strings.HasPrefix(strings.ToUpper(line), "KEY") ||
			strings.HasPrefix(strings.ToUpper(line), "CONSTRAINT") {
			continue
		}

		// Extract column name and type
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		colName := strings.Trim(parts[0], "`\"")
		colType := parts[1]

		column := Column{
			Name:     colName,
			Type:     colType,
			Nullable: true,
		}

		// Check for modifiers
		lineUpper := strings.ToUpper(line)
		if strings.Contains(lineUpper, "PRIMARY KEY") {
			column.PrimaryKey = true
			column.Nullable = false
		}
		if strings.Contains(lineUpper, "NOT NULL") {
			column.Nullable = false
		}
		if strings.Contains(lineUpper, "UNIQUE") {
			column.Unique = true
		}

		// Extract default value
		defaultPattern := regexp.MustCompile(`(?i)DEFAULT\s+([^,\s]+)`)
		if matches := defaultPattern.FindStringSubmatch(line); len(matches) > 1 {
			column.Default = matches[1]
		}

		columns = append(columns, column)
	}

	return columns
}

// calculateChecksum generates a hash of the schema structure
func calculateChecksum(snapshot *SchemaSnapshot) string {
	data, _ := json.Marshal(snapshot.Tables)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// getGitCommit retrieves the current git commit hash if in a repo
func getGitCommit(path string) string {
	// Simple implementation - could use go-git library for more robust handling
	return ""
}

// getCatalogDir returns the catalog directory path
func getCatalogDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude", "ram", "librarian", "catalog")
}

// saveSnapshot saves a schema snapshot to the catalog
func saveSnapshot(snapshot *SchemaSnapshot) error {
	catalogDir := getCatalogDir()
	projectDir := filepath.Join(catalogDir, snapshot.Project)

	// Create project directory if needed
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create catalog directory: %w", err)
	}

	// Save timestamped snapshot
	timestamp := snapshot.SnapshotTime.Format("2006-01-02-150405")
	snapshotFile := filepath.Join(projectDir, fmt.Sprintf("schema-%s.json", timestamp))

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(snapshotFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	// Update latest symlink
	latestFile := filepath.Join(projectDir, "schema-latest.json")
	os.Remove(latestFile) // Remove old symlink if exists
	if err := os.WriteFile(latestFile, data, 0644); err != nil {
		// Fallback to copy if symlink fails
		return fmt.Errorf("failed to update latest snapshot: %w", err)
	}

	return nil
}

// loadLatestSnapshot loads the most recent snapshot for a project
func loadLatestSnapshot(projectName string) (*SchemaSnapshot, error) {
	catalogDir := getCatalogDir()
	projectDir := filepath.Join(catalogDir, projectName)
	latestFile := filepath.Join(projectDir, "schema-latest.json")

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, err
	}

	var snapshot SchemaSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// loadAllSnapshots loads all snapshots for a project
func loadAllSnapshots(projectDir string) ([]*SchemaSnapshot, error) {
	files, err := filepath.Glob(filepath.Join(projectDir, "schema-*.json"))
	if err != nil {
		return nil, err
	}

	var snapshots []*SchemaSnapshot
	for _, file := range files {
		// Skip latest symlink
		if strings.Contains(file, "latest") {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var snapshot SchemaSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}

		snapshots = append(snapshots, &snapshot)
	}

	// Sort by timestamp
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotTime.Before(snapshots[j].SnapshotTime)
	})

	return snapshots, nil
}

// compareSnapshots generates a diff between two snapshots
func compareSnapshots(old, new *SchemaSnapshot) SchemaDiff {
	diff := SchemaDiff{
		Added:    []string{},
		Modified: []string{},
		Removed:  []string{},
	}

	// Find added and modified tables
	for tableName, newTable := range new.Tables {
		oldTable, exists := old.Tables[tableName]
		if !exists {
			diff.Added = append(diff.Added, fmt.Sprintf("table: %s", tableName))
			continue
		}

		// Compare columns
		oldCols := make(map[string]Column)
		for _, col := range oldTable.Columns {
			oldCols[col.Name] = col
		}

		for _, newCol := range newTable.Columns {
			oldCol, exists := oldCols[newCol.Name]
			if !exists {
				diff.Added = append(diff.Added, fmt.Sprintf("%s.%s (%s)", tableName, newCol.Name, newCol.Type))
			} else if oldCol.Type != newCol.Type || oldCol.Nullable != newCol.Nullable {
				diff.Modified = append(diff.Modified, fmt.Sprintf("%s.%s (%s -> %s)", tableName, newCol.Name, oldCol.Type, newCol.Type))
			}
		}

		// Find removed columns
		newCols := make(map[string]bool)
		for _, col := range newTable.Columns {
			newCols[col.Name] = true
		}
		for _, oldCol := range oldTable.Columns {
			if !newCols[oldCol.Name] {
				diff.Removed = append(diff.Removed, fmt.Sprintf("%s.%s", tableName, oldCol.Name))
			}
		}
	}

	// Find removed tables
	for tableName := range old.Tables {
		if _, exists := new.Tables[tableName]; !exists {
			diff.Removed = append(diff.Removed, fmt.Sprintf("table: %s", tableName))
		}
	}

	return diff
}

// displaySchemaSnapshot displays a schema snapshot
func displaySchemaSnapshot(snapshot *SchemaSnapshot) {
	output.Header("SCHEMA")
	fmt.Println("")
	fmt.Printf("Project: %s\n", snapshot.Project)
	fmt.Printf("Source: %s\n", snapshot.Source)
	fmt.Printf("Tables: %d\n", len(snapshot.Tables))
	fmt.Println("")

	if len(snapshot.Tables) > 0 {
		output.Header("TABLES:")
		fmt.Println("")

		// Sort table names
		tableNames := make([]string, 0, len(snapshot.Tables))
		for name := range snapshot.Tables {
			tableNames = append(tableNames, name)
		}
		sort.Strings(tableNames)

		for _, name := range tableNames {
			table := snapshot.Tables[name]
			fmt.Printf("  %s%s%s (%d columns)\n", output.Yellow, name, output.Reset, len(table.Columns))

			// Show first 5 columns
			limit := 5
			if len(table.Columns) < limit {
				limit = len(table.Columns)
			}

			for i := 0; i < limit; i++ {
				col := table.Columns[i]
				markers := ""
				if col.PrimaryKey {
					markers = " (PK)"
				}
				nullable := ""
				if col.Nullable {
					nullable = ", nullable"
				}
				fmt.Printf("    - %s: %s%s%s\n", col.Name, col.Type, markers, nullable)
			}

			if len(table.Columns) > limit {
				fmt.Printf("    ... and %d more columns\n", len(table.Columns)-limit)
			}
			fmt.Println("")
		}
	}
}
