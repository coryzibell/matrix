package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/output"
)

// HarvestResult contains discovered data patterns
type HarvestResult struct {
	FileTypes       map[string]int
	NamingPatterns  NamingConventions
	CommonSchemas   []SchemaPattern
	APIPatterns     []APIPattern
	ScanPath        string
	TotalFilesScanned int
}

// NamingConventions tracks field naming patterns
type NamingConventions struct {
	SnakeCaseCount int
	CamelCaseCount int
	TimestampFields map[string]int
	IDFormats      map[string]int
	BooleanPrefixes map[string]int
}

// SchemaPattern represents a discovered schema structure
type SchemaPattern struct {
	Name      string
	Fields    []FieldPattern
	Locations []string
}

// FieldPattern represents a common field
type FieldPattern struct {
	Name string
	Type string
}

// APIPattern represents discovered API conventions
type APIPattern struct {
	Pattern string
	Examples []string
}

// runDataHarvest implements the data-harvest command
func runDataHarvest() error {
	// Parse subcommand
	if len(os.Args) < 3 {
		printDataHarvestUsage()
		return nil
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "scan":
		return runHarvestScan()
	case "patterns":
		return runHarvestPatterns()
	case "schemas":
		return runHarvestSchemas()
	case "report":
		return runHarvestReport()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printDataHarvestUsage()
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// printDataHarvestUsage displays usage information
func printDataHarvestUsage() {
	fmt.Println("data-harvest - Scan RAM for data patterns to build better fixtures")
	fmt.Println("")
	fmt.Println("USAGE:")
	fmt.Println("  matrix data-harvest scan [path]     Scan for data patterns (default: ~/.claude/ram/)")
	fmt.Println("  matrix data-harvest patterns        Show discovered naming/type patterns")
	fmt.Println("  matrix data-harvest schemas         List discovered schema structures")
	fmt.Println("  matrix data-harvest report          Full harvest report")
	fmt.Println("")
	fmt.Println("EXAMPLES:")
	fmt.Println("  matrix data-harvest scan")
	fmt.Println("  matrix data-harvest scan ~/projects/myapp")
	fmt.Println("  matrix data-harvest patterns")
	fmt.Println("  matrix data-harvest report")
}

// runHarvestScan scans a directory for data patterns
func runHarvestScan() error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	// Default to ~/.claude/ram/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	targetPath := filepath.Join(homeDir, ".claude", "ram")
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

	output.Success("🌾 Data Harvest - Scan")
	fmt.Println("")
	fmt.Printf("Scanning: %s\n", absPath)
	fmt.Println("")

	// Perform the harvest
	result, err := harvestDataPatterns(absPath)
	if err != nil {
		return fmt.Errorf("harvest failed: %w", err)
	}

	// Display results
	displayHarvestResults(result)

	// Save results to Mouse's working directory
	if err := saveHarvestResults(result); err != nil {
		fmt.Printf("Warning: failed to save harvest results: %v\n", err)
	} else {
		fmt.Println("")
		output.Success("✓ Harvest data saved to ~/.claude/ram/mouse/harvest/")
	}

	return nil
}

// runHarvestPatterns shows discovered naming patterns
func runHarvestPatterns() error {
	result, err := loadHarvestResults()
	if err != nil {
		return fmt.Errorf("no harvest data found. Run 'matrix data-harvest scan' first: %w", err)
	}

	output.Success("🔍 Discovered Naming Patterns")
	fmt.Println("")

	displayNamingPatterns(result.NamingPatterns)

	return nil
}

// runHarvestSchemas lists discovered schemas
func runHarvestSchemas() error {
	result, err := loadHarvestResults()
	if err != nil {
		return fmt.Errorf("no harvest data found. Run 'matrix data-harvest scan' first: %w", err)
	}

	output.Success("📋 Discovered Schemas")
	fmt.Println("")

	for _, schema := range result.CommonSchemas {
		fmt.Printf("%s%s%s (found in %d locations)\n", output.Yellow, schema.Name, output.Reset, len(schema.Locations))
		fmt.Println("  Fields:")
		for _, field := range schema.Fields {
			fmt.Printf("    - %s: %s\n", field.Name, field.Type)
		}
		fmt.Println("")
	}

	if len(result.CommonSchemas) == 0 {
		fmt.Println("No common schemas discovered yet.")
	}

	return nil
}

// runHarvestReport generates full harvest report
func runHarvestReport() error {
	result, err := loadHarvestResults()
	if err != nil {
		return fmt.Errorf("no harvest data found. Run 'matrix data-harvest scan' first: %w", err)
	}

	displayHarvestReport(result)

	return nil
}

// harvestDataPatterns scans directory and extracts patterns
func harvestDataPatterns(path string) (*HarvestResult, error) {
	result := &HarvestResult{
		FileTypes:      make(map[string]int),
		NamingPatterns: NamingConventions{
			TimestampFields: make(map[string]int),
			IDFormats:       make(map[string]int),
			BooleanPrefixes: make(map[string]int),
		},
		CommonSchemas: []SchemaPattern{},
		APIPatterns:   []APIPattern{},
		ScanPath:      path,
	}

	// Track schemas by name
	schemaMap := make(map[string]*SchemaPattern)

	// Walk directory tree
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Count file extensions
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext != "" {
			result.FileTypes[ext]++
		}

		// Analyze relevant file types
		if ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".sql" {
			result.TotalFilesScanned++
			analyzeDataFile(filePath, ext, result, schemaMap)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert schema map to slice
	for _, schema := range schemaMap {
		result.CommonSchemas = append(result.CommonSchemas, *schema)
	}

	// Sort schemas by frequency
	sort.Slice(result.CommonSchemas, func(i, j int) bool {
		return len(result.CommonSchemas[i].Locations) > len(result.CommonSchemas[j].Locations)
	})

	return result, nil
}

// analyzeDataFile extracts patterns from a data file
func analyzeDataFile(filePath, ext string, result *HarvestResult, schemaMap map[string]*SchemaPattern) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	contentStr := string(content)

	switch ext {
	case ".json":
		analyzeJSON(contentStr, filePath, result, schemaMap)
	case ".yaml", ".yml":
		analyzeYAML(contentStr, filePath, result)
	case ".sql":
		analyzeSQL(contentStr, filePath, result, schemaMap)
	}
}

// analyzeJSON extracts patterns from JSON files
func analyzeJSON(content, filePath string, result *HarvestResult, schemaMap map[string]*SchemaPattern) {
	var data interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return
	}

	// Extract field patterns
	fields := extractFieldsFromJSON(data)
	analyzeFields(fields, result)

	// Try to infer schema from structure
	if obj, ok := data.(map[string]interface{}); ok {
		inferSchemaFromObject(obj, filePath, schemaMap)
	} else if arr, ok := data.([]interface{}); ok && len(arr) > 0 {
		if obj, ok := arr[0].(map[string]interface{}); ok {
			inferSchemaFromObject(obj, filePath, schemaMap)
		}
	}

	// Look for API patterns
	extractAPIPatterns(content, result)
}

// analyzeYAML extracts patterns from YAML files
func analyzeYAML(content, filePath string, result *HarvestResult) {
	// Simple field extraction using regex
	fieldPattern := regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:`, )

	for _, line := range strings.Split(content, "\n") {
		if matches := fieldPattern.FindStringSubmatch(line); len(matches) > 1 {
			fieldName := matches[1]
			analyzeFieldName(fieldName, result)
		}
	}
}

// analyzeSQL extracts patterns from SQL files
func analyzeSQL(content, filePath string, result *HarvestResult, schemaMap map[string]*SchemaPattern) {
	// Extract CREATE TABLE statements
	createTablePattern := regexp.MustCompile(`(?si)CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+` +
		"`" + `?(\w+)` + "`" + `?\s*\((.*?)\)`)

	matches := createTablePattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		tableName := match[1]
		columnsStr := match[2]

		schema := getOrCreateSchema(tableName, filePath, schemaMap)

		// Parse columns
		columnPattern := regexp.MustCompile(`(\w+)\s+(\w+(?:\([^)]+\))?)`)
		columnMatches := columnPattern.FindAllStringSubmatch(columnsStr, -1)

		for _, colMatch := range columnMatches {
			if len(colMatch) < 3 {
				continue
			}

			fieldName := colMatch[1]
			fieldType := colMatch[2]

			// Skip SQL keywords
			if isSQLKeyword(fieldName) {
				continue
			}

			schema.Fields = append(schema.Fields, FieldPattern{
				Name: fieldName,
				Type: fieldType,
			})

			analyzeFieldName(fieldName, result)
		}
	}
}

// extractFieldsFromJSON recursively extracts field names from JSON data
func extractFieldsFromJSON(data interface{}) []string {
	var fields []string

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			fields = append(fields, key)
			fields = append(fields, extractFieldsFromJSON(value)...)
		}
	case []interface{}:
		for _, item := range v {
			fields = append(fields, extractFieldsFromJSON(item)...)
		}
	}

	return fields
}

// analyzeFields analyzes a list of field names for patterns
func analyzeFields(fields []string, result *HarvestResult) {
	for _, field := range fields {
		analyzeFieldName(field, result)
	}
}

// analyzeFieldName analyzes a single field name for patterns
func analyzeFieldName(field string, result *HarvestResult) {
	// Count snake_case vs camelCase
	if strings.Contains(field, "_") {
		result.NamingPatterns.SnakeCaseCount++
	} else if len(field) > 0 && field[0] >= 'a' && field[0] <= 'z' {
		// Check if it has uppercase letters (camelCase)
		for _, c := range field[1:] {
			if c >= 'A' && c <= 'Z' {
				result.NamingPatterns.CamelCaseCount++
				break
			}
		}
	}

	// Analyze timestamp patterns
	lowerField := strings.ToLower(field)
	if strings.Contains(lowerField, "created") || strings.Contains(lowerField, "updated") ||
		strings.Contains(lowerField, "timestamp") || strings.HasSuffix(lowerField, "_at") {
		result.NamingPatterns.TimestampFields[field]++
	}

	// Analyze ID patterns
	if strings.Contains(lowerField, "id") || strings.HasSuffix(lowerField, "_id") {
		result.NamingPatterns.IDFormats[field]++
	}

	// Analyze boolean prefixes
	if strings.HasPrefix(lowerField, "is_") || strings.HasPrefix(lowerField, "has_") ||
		strings.HasPrefix(lowerField, "can_") || strings.HasPrefix(lowerField, "should_") {
		prefix := strings.Split(lowerField, "_")[0]
		result.NamingPatterns.BooleanPrefixes[prefix]++
	}
}

// inferSchemaFromObject infers schema from JSON object
func inferSchemaFromObject(obj map[string]interface{}, filePath string, schemaMap map[string]*SchemaPattern) {
	// Try to infer schema name from common patterns
	schemaName := "Unknown"

	// Check for common identifier fields
	if _, hasID := obj["id"]; hasID {
		if name, hasName := obj["name"]; hasName {
			if nameStr, ok := name.(string); ok {
				schemaName = nameStr
			}
		}
	}

	// Try to infer from field patterns
	if _, hasEmail := obj["email"]; hasEmail {
		schemaName = "Users"
	} else if _, hasPrice := obj["price"]; hasPrice {
		schemaName = "Products"
	}

	if schemaName != "Unknown" {
		schema := getOrCreateSchema(schemaName, filePath, schemaMap)

		for key, value := range obj {
			fieldType := inferTypeFromValue(value)
			// Only add if not already present
			found := false
			for _, f := range schema.Fields {
				if f.Name == key {
					found = true
					break
				}
			}
			if !found {
				schema.Fields = append(schema.Fields, FieldPattern{
					Name: key,
					Type: fieldType,
				})
			}
		}
	}
}

// inferTypeFromValue infers type from JSON value
func inferTypeFromValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		// Check for UUID pattern
		if matched, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, v); matched {
			return "uuid"
		}
		// Check for timestamp
		if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}`, v); matched {
			return "timestamp"
		}
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	default:
		return "unknown"
	}
}

// extractAPIPatterns looks for API endpoint patterns in content
func extractAPIPatterns(content string, result *HarvestResult) {
	// Look for REST endpoint patterns
	endpointPattern := regexp.MustCompile(`/api/v\d+/\w+`)
	matches := endpointPattern.FindAllString(content, -1)

	if len(matches) > 0 {
		// Check if we already have this pattern
		found := false
		for _, pattern := range result.APIPatterns {
			if pattern.Pattern == "REST: /api/v{N}/{resource}" {
				found = true
				break
			}
		}
		if !found {
			result.APIPatterns = append(result.APIPatterns, APIPattern{
				Pattern:  "REST: /api/v{N}/{resource}",
				Examples: unique(matches),
			})
		}
	}

	// Look for auth patterns
	if strings.Contains(content, "Bearer") || strings.Contains(content, "Authorization") {
		found := false
		for _, pattern := range result.APIPatterns {
			if pattern.Pattern == "Auth: Bearer tokens" {
				found = true
				break
			}
		}
		if !found {
			result.APIPatterns = append(result.APIPatterns, APIPattern{
				Pattern:  "Auth: Bearer tokens",
				Examples: []string{},
			})
		}
	}
}

// getOrCreateSchema gets or creates a schema in the map
func getOrCreateSchema(name, location string, schemaMap map[string]*SchemaPattern) *SchemaPattern {
	schema, exists := schemaMap[name]
	if !exists {
		schema = &SchemaPattern{
			Name:      name,
			Fields:    []FieldPattern{},
			Locations: []string{},
		}
		schemaMap[name] = schema
	}

	// Add location if not already present
	found := false
	for _, loc := range schema.Locations {
		if loc == location {
			found = true
			break
		}
	}
	if !found {
		schema.Locations = append(schema.Locations, location)
	}

	return schema
}

// isSQLKeyword checks if a string is a SQL keyword
func isSQLKeyword(s string) bool {
	keywords := []string{"PRIMARY", "KEY", "FOREIGN", "UNIQUE", "NOT", "NULL", "DEFAULT",
		"INDEX", "CONSTRAINT", "REFERENCES", "CASCADE", "ON", "DELETE", "UPDATE"}
	upper := strings.ToUpper(s)
	for _, kw := range keywords {
		if upper == kw {
			return true
		}
	}
	return false
}

// unique returns unique strings from a slice
func unique(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// displayHarvestResults displays scan results
func displayHarvestResults(result *HarvestResult) {
	output.Success("🌾 Data Harvest Report")
	fmt.Println("")

	// File types
	output.Header("FILE TYPES:")
	fmt.Println("")

	// Sort by count
	type fileTypeCount struct {
		ext   string
		count int
	}
	var sortedTypes []fileTypeCount
	for ext, count := range result.FileTypes {
		sortedTypes = append(sortedTypes, fileTypeCount{ext, count})
	}
	sort.Slice(sortedTypes, func(i, j int) bool {
		return sortedTypes[i].count > sortedTypes[j].count
	})

	for _, ft := range sortedTypes {
		if ft.count > 0 {
			fmt.Printf("  %-10s %d files\n", ft.ext, ft.count)
		}
	}
	fmt.Println("")

	// Naming conventions
	output.Header("NAMING CONVENTIONS:")
	fmt.Println("")
	displayNamingPatterns(result.NamingPatterns)

	// Common schemas
	if len(result.CommonSchemas) > 0 {
		output.Header("COMMON SCHEMAS:")
		fmt.Println("")
		limit := 5
		if len(result.CommonSchemas) < limit {
			limit = len(result.CommonSchemas)
		}
		for i := 0; i < limit; i++ {
			schema := result.CommonSchemas[i]
			fmt.Printf("  %s (found in %d locations)\n", schema.Name, len(schema.Locations))
			fieldLimit := 5
			if len(schema.Fields) < fieldLimit {
				fieldLimit = len(schema.Fields)
			}
			for j := 0; j < fieldLimit; j++ {
				fmt.Printf("    - %s: %s\n", schema.Fields[j].Name, schema.Fields[j].Type)
			}
			if len(schema.Fields) > fieldLimit {
				fmt.Printf("    ... and %d more fields\n", len(schema.Fields)-fieldLimit)
			}
			fmt.Println("")
		}
	}

	// API patterns
	if len(result.APIPatterns) > 0 {
		output.Header("API PATTERNS:")
		fmt.Println("")
		for _, pattern := range result.APIPatterns {
			fmt.Printf("  %s\n", pattern.Pattern)
			if len(pattern.Examples) > 0 {
				fmt.Printf("    Examples: %s\n", strings.Join(pattern.Examples, ", "))
			}
		}
		fmt.Println("")
	}
}

// displayNamingPatterns displays naming convention patterns
func displayNamingPatterns(patterns NamingConventions) {
	total := patterns.SnakeCaseCount + patterns.CamelCaseCount
	if total > 0 {
		snakePercent := (patterns.SnakeCaseCount * 100) / total
		camelPercent := (patterns.CamelCaseCount * 100) / total
		fmt.Printf("  snake_case: %d%% (%d occurrences)\n", snakePercent, patterns.SnakeCaseCount)
		fmt.Printf("  camelCase:  %d%% (%d occurrences)\n", camelPercent, patterns.CamelCaseCount)
		fmt.Println("")
	}

	if len(patterns.TimestampFields) > 0 {
		fmt.Println("  Common timestamp fields:")
		sortedFields := sortMapByValue(patterns.TimestampFields)
		for i, field := range sortedFields {
			if i >= 5 {
				break
			}
			fmt.Printf("    - %s (%d times)\n", field, patterns.TimestampFields[field])
		}
		fmt.Println("")
	}

	if len(patterns.IDFormats) > 0 {
		fmt.Println("  Common ID fields:")
		sortedIDs := sortMapByValue(patterns.IDFormats)
		for i, field := range sortedIDs {
			if i >= 5 {
				break
			}
			fmt.Printf("    - %s (%d times)\n", field, patterns.IDFormats[field])
		}
		fmt.Println("")
	}
}

// displayHarvestReport displays full harvest report
func displayHarvestReport(result *HarvestResult) {
	output.Success("🌾 Data Harvest Report")
	fmt.Println("")

	fmt.Printf("Scanned: %s\n", result.ScanPath)
	fmt.Printf("Files Analyzed: %d\n", result.TotalFilesScanned)
	fmt.Println("")

	displayHarvestResults(result)

	fmt.Println("")
	output.Success("Ready to build training programs that taste like the real thing.")
}

// saveHarvestResults saves harvest data to Mouse's directory
func saveHarvestResults(result *HarvestResult) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	harvestDir := filepath.Join(homeDir, ".claude", "ram", "mouse", "harvest")
	if err := os.MkdirAll(harvestDir, 0755); err != nil {
		return err
	}

	// Save as JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	resultFile := filepath.Join(harvestDir, "latest-harvest.json")
	return os.WriteFile(resultFile, data, 0644)
}

// loadHarvestResults loads harvest data from Mouse's directory
func loadHarvestResults() (*HarvestResult, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	resultFile := filepath.Join(homeDir, ".claude", "ram", "mouse", "harvest", "latest-harvest.json")
	data, err := os.ReadFile(resultFile)
	if err != nil {
		return nil, err
	}

	var result HarvestResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// sortMapByValue sorts a map by values in descending order and returns keys
func sortMapByValue(m map[string]int) []string {
	type kv struct {
		key   string
		value int
	}

	var pairs []kv
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value > pairs[j].value
	})

	var keys []string
	for _, pair := range pairs {
		keys = append(keys, pair.key)
	}

	return keys
}
