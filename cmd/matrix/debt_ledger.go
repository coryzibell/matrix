package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/output"
)

// DebtMarker represents a technical debt marker found in code
type DebtMarker struct {
	File        string
	Line        int
	Type        string // TODO, FIXME, XXX, HACK, NOTE, OPTIMIZE, DEPRECATED
	Content     string // The actual comment text
	Severity    DebtSeverity
	Context     []string // Surrounding lines for context
}

// DebtSeverity classifies debt by priority
type DebtSeverity int

const (
	SeverityMinor DebtSeverity = iota
	SeverityImportant
	SeverityCritical
)

// DebtReport summarizes technical debt across a codebase
type DebtReport struct {
	ScanPath string
	Markers  []DebtMarker
	Critical []DebtMarker
	Important []DebtMarker
	Minor    []DebtMarker
	TotalFiles int
}

// runDebtLedger implements the debt-ledger command
func runDebtLedger() error {
	// Parse flags
	fs := flag.NewFlagSet("debt-ledger", flag.ExitOnError)
	createTasks := fs.Bool("create-tasks", false, "Create remediation task files in RAM")
	severityFilter := fs.String("severity", "", "Filter by severity: critical, important, minor")

	// Parse remaining args (after "debt-ledger")
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	// Get target path (default to current directory)
	targetPath := "."
	if fs.NArg() > 0 {
		targetPath = fs.Arg(0)
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Validate severity filter
	if *severityFilter != "" {
		validSeverity := map[string]bool{"critical": true, "important": true, "minor": true}
		if !validSeverity[*severityFilter] {
			return fmt.Errorf("invalid severity: %s (valid: critical, important, minor)", *severityFilter)
		}
	}

	// Run debt scan
	output.Success("🔧 Technical Debt Ledger")
	fmt.Println("")
	fmt.Printf("Scanning: %s\n", absPath)
	fmt.Println("")

	// Scan for debt markers
	report, err := scanDebt(absPath)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Display report
	displayDebtReport(report, *severityFilter)

	// Optionally create task files
	if *createTasks {
		if err := createTaskFiles(report); err != nil {
			return fmt.Errorf("failed to create task files: %w", err)
		}
		fmt.Println("")
		output.Success("Task files created in ~/.claude/ram/ramakandra/debt-tasks/")
	}

	return nil
}

// scanDebt walks the directory tree and finds all debt markers
func scanDebt(path string) (*DebtReport, error) {
	report := &DebtReport{
		ScanPath: path,
		Markers:  []DebtMarker{},
	}

	// Debt marker patterns
	patterns := map[string]*regexp.Regexp{
		"TODO":       regexp.MustCompile(`(?i)//\s*TODO:?\s*(.*)|#\s*TODO:?\s*(.*)|/\*\s*TODO:?\s*(.*)\*/`),
		"FIXME":      regexp.MustCompile(`(?i)//\s*FIXME:?\s*(.*)|#\s*FIXME:?\s*(.*)|/\*\s*FIXME:?\s*(.*)\*/`),
		"XXX":        regexp.MustCompile(`(?i)//\s*XXX:?\s*(.*)|#\s*XXX:?\s*(.*)|/\*\s*XXX:?\s*(.*)\*/`),
		"HACK":       regexp.MustCompile(`(?i)//\s*HACK:?\s*(.*)|#\s*HACK:?\s*(.*)|/\*\s*HACK:?\s*(.*)\*/`),
		"NOTE":       regexp.MustCompile(`(?i)//\s*NOTE:?\s*(.*)|#\s*NOTE:?\s*(.*)|/\*\s*NOTE:?\s*(.*)\*/`),
		"OPTIMIZE":   regexp.MustCompile(`(?i)//\s*OPTIMIZE:?\s*(.*)|#\s*OPTIMIZE:?\s*(.*)|/\*\s*OPTIMIZE:?\s*(.*)\*/`),
		"DEPRECATED": regexp.MustCompile(`(?i)//\s*DEPRECATED:?\s*(.*)|#\s*DEPRECATED:?\s*(.*)|/\*\s*DEPRECATED:?\s*(.*)\*/`),
	}

	// Walk the directory tree
	err := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't read
		}

		// Skip common ignore patterns
		if shouldSkipPath(filePath, fileInfo) {
			if fileInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !fileInfo.IsDir() {
			report.TotalFiles++

			// Only scan text files
			ext := strings.ToLower(filepath.Ext(filePath))
			if !isCodeFile(ext) {
				return nil
			}

			// Read file content
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil // Skip files we can't read
			}

			// Scan for debt markers
			relPath, _ := filepath.Rel(path, filePath)
			lines := strings.Split(string(content), "\n")

			for lineNum, line := range lines {
				for markerType, pattern := range patterns {
					if pattern.MatchString(line) {
						// Extract comment content
						matches := pattern.FindStringSubmatch(line)
						commentText := ""
						for i := 1; i < len(matches); i++ {
							if matches[i] != "" {
								commentText = strings.TrimSpace(matches[i])
								break
							}
						}

						// Get surrounding context (3 lines before and after)
						context := extractContext(lines, lineNum, 3)

						marker := DebtMarker{
							File:     relPath,
							Line:     lineNum + 1,
							Type:     markerType,
							Content:  commentText,
							Severity: classifySeverity(markerType),
							Context:  context,
						}

						report.Markers = append(report.Markers, marker)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Organize by severity
	for _, marker := range report.Markers {
		switch marker.Severity {
		case SeverityCritical:
			report.Critical = append(report.Critical, marker)
		case SeverityImportant:
			report.Important = append(report.Important, marker)
		case SeverityMinor:
			report.Minor = append(report.Minor, marker)
		}
	}

	// Sort each category by file then line
	sortMarkers(report.Critical)
	sortMarkers(report.Important)
	sortMarkers(report.Minor)

	return report, nil
}

// shouldSkipPath returns true if the file/directory should be skipped
func shouldSkipPath(path string, info os.FileInfo) bool {
	name := info.Name()

	// Skip hidden files/dirs
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}

	// Skip common build/dependency directories
	skipDirs := map[string]bool{
		"node_modules": true,
		"target":       true,
		"build":        true,
		"dist":         true,
		"vendor":       true,
		"__pycache__":  true,
		".git":         true,
		"out":          true,
		"bin":          true,
	}

	if info.IsDir() && skipDirs[name] {
		return true
	}

	return false
}

// isCodeFile returns true if the extension is likely a code file
func isCodeFile(ext string) bool {
	codeExts := map[string]bool{
		".go": true, ".rs": true, ".js": true, ".ts": true, ".py": true,
		".java": true, ".c": true, ".cpp": true, ".cs": true, ".rb": true,
		".php": true, ".sh": true, ".bash": true, ".md": true, ".txt": true,
		".yml": true, ".yaml": true, ".toml": true, ".tsx": true, ".jsx": true,
		".h": true, ".hpp": true, ".vue": true, ".svelte": true,
	}
	return codeExts[ext]
}

// extractContext gets surrounding lines for context
func extractContext(lines []string, lineNum, contextLines int) []string {
	start := lineNum - contextLines
	if start < 0 {
		start = 0
	}

	end := lineNum + contextLines + 1
	if end > len(lines) {
		end = len(lines)
	}

	context := make([]string, 0)
	for i := start; i < end; i++ {
		context = append(context, lines[i])
	}

	return context
}

// classifySeverity assigns severity based on marker type
func classifySeverity(markerType string) DebtSeverity {
	switch strings.ToUpper(markerType) {
	case "FIXME", "XXX":
		return SeverityCritical
	case "TODO", "OPTIMIZE", "DEPRECATED":
		return SeverityImportant
	case "HACK", "NOTE":
		return SeverityMinor
	default:
		return SeverityMinor
	}
}

// sortMarkers sorts markers by file then line number
func sortMarkers(markers []DebtMarker) {
	sort.Slice(markers, func(i, j int) bool {
		if markers[i].File == markers[j].File {
			return markers[i].Line < markers[j].Line
		}
		return markers[i].File < markers[j].File
	})
}

// displayDebtReport outputs the debt report
func displayDebtReport(report *DebtReport, severityFilter string) {
	totalMarkers := len(report.Markers)
	uniqueFiles := countUniqueFiles(report.Markers)

	fmt.Printf("Found: %d markers across %d files\n", totalMarkers, uniqueFiles)
	fmt.Println("")

	// Summary by severity
	output.Header("By Severity")
	fmt.Println("")
	fmt.Printf("  🔴 Critical (FIXME, XXX):       %d\n", len(report.Critical))
	fmt.Printf("  🟡 Important (TODO, OPTIMIZE):  %d\n", len(report.Important))
	fmt.Printf("  🟢 Minor (HACK, NOTE):          %d\n", len(report.Minor))
	fmt.Println("")

	// Display debt items based on filter
	if severityFilter == "" || severityFilter == "critical" {
		displayMarkerSection("Critical", report.Critical, "🔴")
	}

	if severityFilter == "" || severityFilter == "important" {
		displayMarkerSection("Important", report.Important, "🟡")
	}

	if severityFilter == "" || severityFilter == "minor" {
		displayMarkerSection("Minor", report.Minor, "🟢")
	}
}

// displayMarkerSection displays a section of debt markers
func displayMarkerSection(title string, markers []DebtMarker, emoji string) {
	if len(markers) == 0 {
		return
	}

	output.Header(fmt.Sprintf("%s %s Debt Items", emoji, title))
	fmt.Println("")

	// Show up to 10 markers per section
	displayLimit := 10
	for i, marker := range markers {
		if i >= displayLimit {
			remaining := len(markers) - displayLimit
			fmt.Printf("  ... and %d more\n", remaining)
			break
		}

		fmt.Printf("  %s:%d\n", marker.File, marker.Line)
		fmt.Printf("    %s: %s\n", marker.Type, marker.Content)
		fmt.Printf("    Severity: %s\n", severityToString(marker.Severity))
		fmt.Println("")
	}
}

// countUniqueFiles counts unique files in markers
func countUniqueFiles(markers []DebtMarker) int {
	files := make(map[string]bool)
	for _, marker := range markers {
		files[marker.File] = true
	}
	return len(files)
}

// severityToString converts severity enum to string
func severityToString(severity DebtSeverity) string {
	switch severity {
	case SeverityCritical:
		return "critical"
	case SeverityImportant:
		return "important"
	case SeverityMinor:
		return "minor"
	default:
		return "unknown"
	}
}

// createTaskFiles generates remediation task files in Ramakandra's RAM directory
func createTaskFiles(report *DebtReport) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	taskDir := filepath.Join(homeDir, ".claude", "ram", "ramakandra", "debt-tasks")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("failed to create task directory: %w", err)
	}

	// Create task files for critical and important items
	taskMarkers := append(report.Critical, report.Important...)

	for i, marker := range taskMarkers {
		// Limit to 20 task files
		if i >= 20 {
			break
		}

		// Generate filename
		filename := fmt.Sprintf("%s-%s-line%d.md",
			strings.ReplaceAll(marker.File, "/", "-"),
			strings.ToLower(marker.Type),
			marker.Line)
		filename = strings.ReplaceAll(filename, " ", "-")

		taskPath := filepath.Join(taskDir, filename)

		// Generate task content
		taskContent := generateTaskContent(marker, report.ScanPath)

		// Write task file
		if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
			return fmt.Errorf("failed to write task file: %w", err)
		}
	}

	return nil
}

// generateTaskContent creates markdown content for a task file
func generateTaskContent(marker DebtMarker, scanPath string) string {
	var sb strings.Builder

	// Title
	sb.WriteString(fmt.Sprintf("# %s: %s\n\n", marker.Type, marker.Content))

	// Metadata
	sb.WriteString("**Category:** Technical Debt\n")
	sb.WriteString(fmt.Sprintf("**Severity:** %s\n", severityToString(marker.Severity)))
	sb.WriteString(fmt.Sprintf("**File:** %s:%d\n", marker.File, marker.Line))
	sb.WriteString(fmt.Sprintf("**Project:** %s\n\n", scanPath))

	// Original Marker
	sb.WriteString("## Original Marker\n\n")
	sb.WriteString("```\n")
	sb.WriteString(fmt.Sprintf("%s: %s\n", marker.Type, marker.Content))
	sb.WriteString("```\n\n")

	// Context
	if len(marker.Context) > 0 {
		sb.WriteString("## Context\n\n")
		sb.WriteString("```\n")
		for _, line := range marker.Context {
			sb.WriteString(line + "\n")
		}
		sb.WriteString("```\n\n")
	}

	// Suggested Remediation
	sb.WriteString("## Suggested Remediation\n\n")
	sb.WriteString("1. Review the marked code and understand why the debt marker was added\n")
	sb.WriteString("2. Assess the impact and effort required to resolve\n")
	sb.WriteString("3. Implement a proper solution\n")
	sb.WriteString("4. Write or update tests if applicable\n")
	sb.WriteString("5. Remove the debt marker\n\n")

	// Handoff suggestions
	sb.WriteString("## Handoff\n\n")
	switch marker.Severity {
	case SeverityCritical:
		sb.WriteString("- **Smith** for complex refactoring\n")
		sb.WriteString("- **Trinity** if this represents a bug or crash risk\n")
		sb.WriteString("- **Deus** to verify with tests after resolution\n")
	case SeverityImportant:
		sb.WriteString("- **Smith** for implementation\n")
		sb.WriteString("- **Morpheus** if documentation updates needed\n")
	case SeverityMinor:
		sb.WriteString("- **Fellas** for quick fixes across multiple files\n")
		sb.WriteString("- **Morpheus** for documentation improvements\n")
	}

	return sb.String()
}
