package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// IncidentData represents extracted incident information
type IncidentData struct {
	Title       string
	FilePath    string
	Timestamp   time.Time
	Status      string
	RootCauses  []RootCause
	Fixes       []Fix
	Insights    []string
	Tests       *TestResults
}

// RootCause represents a single root cause
type RootCause struct {
	Issue    string
	Location string
	Detail   string
}

// Fix represents a code fix
type Fix struct {
	File     string
	Lines    string
	Function string
}

// TestResults represents before/after test results
type TestResults struct {
	Before int
	After  int
	Fixed  int
}

// runIncidentTrace implements the incident-trace command
func runIncidentTrace() error {
	// Parse flags
	jsonFlag := false
	neoFlag := false
	allFlag := false
	pattern := ""
	filePath := ""

	// Simple flag parsing
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--json" {
			jsonFlag = true
		} else if arg == "--neo" {
			neoFlag = true
		} else if arg == "--all" {
			allFlag = true
		} else if strings.HasPrefix(arg, "--pattern=") {
			pattern = strings.TrimPrefix(arg, "--pattern=")
		} else if !strings.HasPrefix(arg, "--") {
			filePath = arg
		}
	}

	// Validate flag combinations
	if allFlag && filePath != "" {
		return fmt.Errorf("cannot use --all with a specific file path")
	}

	if !allFlag && filePath == "" {
		return fmt.Errorf("must specify either --all or a file path")
	}

	// Get Trinity's RAM path
	trinityPath, err := identity.RAMPath("trinity")
	if err != nil {
		return fmt.Errorf("failed to get Trinity's RAM path: %w", err)
	}

	var incidents []IncidentData

	if allFlag {
		// Scan all markdown files directly in Trinity's directory
		dirEntries, err := os.ReadDir(trinityPath)
		if err != nil {
			return fmt.Errorf("failed to read Trinity's RAM directory: %w", err)
		}

		for _, entry := range dirEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			filePath := filepath.Join(trinityPath, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			file := ram.File{
				Path:     filePath,
				Identity: "trinity",
				Name:     strings.TrimSuffix(entry.Name(), ".md"),
				Content:  string(content),
			}

			// Skip non-incident files
			if !isIncidentFile(file.Content) {
				continue
			}

			// Apply pattern filter if specified
			if pattern != "" && !strings.Contains(strings.ToLower(file.Content), strings.ToLower(pattern)) {
				continue
			}

			incident := extractIncidentData(file)
			incidents = append(incidents, incident)
		}

		// Sort by timestamp
		sort.Slice(incidents, func(i, j int) bool {
			return incidents[i].Timestamp.After(incidents[j].Timestamp)
		})

	} else {
		// Process single file
		expandedPath := expandPath(filePath)
		content, err := os.ReadFile(expandedPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", expandedPath, err)
		}

		file := ram.File{
			Path:     expandedPath,
			Identity: "trinity",
			Content:  string(content),
		}

		if !isIncidentFile(file.Content) {
			return fmt.Errorf("file does not appear to be an incident report")
		}

		incidents = append(incidents, extractIncidentData(file))
	}

	if len(incidents) == 0 {
		fmt.Println("No incidents found")
		return nil
	}

	// Output based on flags
	if jsonFlag {
		return outputIncidentJSON(incidents)
	} else if neoFlag {
		return outputNeoSummary(incidents)
	} else if pattern != "" && allFlag {
		return outputPatternAnalysis(incidents, pattern)
	} else {
		return outputHumanReadable(incidents)
	}
}

// isIncidentFile checks if content looks like an incident report
func isIncidentFile(content string) bool {
	lower := strings.ToLower(content)
	// Look for incident markers
	markers := []string{
		"bug",
		"root cause",
		"problem:",
		"files modified",
		"result:",
		"fixed:",
	}

	count := 0
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			count++
		}
	}

	return count >= 2 // At least 2 markers
}

// extractIncidentData parses an incident file and extracts structured data
func extractIncidentData(file ram.File) IncidentData {
	incident := IncidentData{
		FilePath:   file.Path,
		Status:     "resolved",
		RootCauses: []RootCause{},
		Fixes:      []Fix{},
		Insights:   []string{},
	}

	lines := strings.Split(file.Content, "\n")

	// Extract title from first # header
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			incident.Title = strings.TrimPrefix(trimmed, "# ")
			break
		}
	}

	// Try to get timestamp from file modification time
	if info, err := os.Stat(file.Path); err == nil {
		incident.Timestamp = info.ModTime()
	}

	// Extract root causes
	incident.RootCauses = extractRootCauses(lines)

	// Extract fixes
	incident.Fixes = extractFixes(lines)

	// Extract insights
	incident.Insights = extractInsights(lines)

	// Extract test results
	incident.Tests = extractTestResults(lines)

	return incident
}

// extractRootCauses finds root cause information
func extractRootCauses(lines []string) []RootCause {
	var causes []RootCause

	for i, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))

		// Look for root cause patterns
		if strings.HasPrefix(lower, "**root cause:**") || strings.HasPrefix(lower, "root cause:") {
			detail := strings.TrimSpace(strings.TrimPrefix(lower, "**root cause:**"))
			detail = strings.TrimSpace(strings.TrimPrefix(detail, "root cause:"))

			// Look for location in nearby lines
			location := extractLocation(lines, i-5, i+5)

			causes = append(causes, RootCause{
				Issue:    extractIssue(lines, i-2, i),
				Location: location,
				Detail:   detail,
			})
		} else if strings.HasPrefix(lower, "**problem:**") || strings.HasPrefix(lower, "problem:") {
			detail := strings.TrimSpace(strings.TrimPrefix(lower, "**problem:**"))
			detail = strings.TrimSpace(strings.TrimPrefix(detail, "problem:"))

			location := extractLocation(lines, i-5, i+5)

			causes = append(causes, RootCause{
				Issue:    "Problem identified",
				Location: location,
				Detail:   detail,
			})
		}
	}

	return causes
}

// extractLocation searches for line number references
func extractLocation(lines []string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}

	// Pattern: (Line 123) or (Line 123-456)
	linePattern := regexp.MustCompile(`\(Line (\d+(?:-\d+)?)\)`)

	for i := start; i <= end; i++ {
		if match := linePattern.FindStringSubmatch(lines[i]); match != nil {
			return match[1]
		}
	}

	return ""
}

// extractIssue finds the issue description from previous lines
func extractIssue(lines []string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}

	for i := end; i >= start; i-- {
		trimmed := strings.TrimSpace(lines[i])
		// Look for ### headers or numbered list items
		if strings.HasPrefix(trimmed, "### ") {
			return strings.TrimPrefix(trimmed, "### ")
		}
		if strings.HasPrefix(trimmed, "## ") {
			return strings.TrimPrefix(trimmed, "## ")
		}
	}

	return "Issue"
}

// extractFixes finds file modifications
func extractFixes(lines []string) []Fix {
	var fixes []Fix

	inFilesSection := false
	currentFile := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for "Files Modified" section
		if strings.Contains(strings.ToLower(trimmed), "files modified") {
			inFilesSection = true
			continue
		}

		if inFilesSection {
			// End of section
			if strings.HasPrefix(trimmed, "##") && !strings.Contains(strings.ToLower(trimmed), "files modified") {
				break
			}

			// File path line
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 0 {
					path := strings.TrimPrefix(parts[0], "- ")
					path = strings.TrimPrefix(path, "* ")
					path = strings.TrimPrefix(path, "`")
					path = strings.TrimSuffix(path, "`")

					if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~") {
						currentFile = path
					}
				}

				// Extract details if present
				if len(parts) > 1 && currentFile != "" {
					detail := strings.TrimSpace(parts[1])
					functionName := extractFunctionName(detail)
					lineRange := extractLineRange(detail)

					fixes = append(fixes, Fix{
						File:     currentFile,
						Lines:    lineRange,
						Function: functionName,
					})
				}
			} else if currentFile != "" && strings.Contains(trimmed, "Line ") {
				// Continuation line with more details
				functionName := extractFunctionName(trimmed)
				lineRange := extractLineRange(trimmed)

				if lineRange != "" {
					fixes = append(fixes, Fix{
						File:     currentFile,
						Lines:    lineRange,
						Function: functionName,
					})
				}
			}
		}
	}

	return fixes
}

// extractFunctionName pulls function name from description
func extractFunctionName(text string) string {
	// Pattern: function_name() or `function_name()`
	funcPattern := regexp.MustCompile("`?([a-zA-Z_][a-zA-Z0-9_]*)\\(\\)`?")
	if match := funcPattern.FindStringSubmatch(text); match != nil {
		return match[1]
	}
	return ""
}

// extractLineRange pulls line numbers from text
func extractLineRange(text string) string {
	// Pattern: Line 123 or Line 123-456 or Lines 123-456
	linePattern := regexp.MustCompile(`Lines? (\d+(?:-\d+)?)`)
	if match := linePattern.FindStringSubmatch(text); match != nil {
		return match[1]
	}
	return ""
}

// extractInsights finds key learnings
func extractInsights(lines []string) []string {
	var insights []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Look for insight markers
		for _, marker := range []string{"**key learning:**", "key learning:", "**lesson:**", "lesson:", "**insight:**", "insight:"} {
			if strings.HasPrefix(lower, marker) {
				insight := strings.TrimSpace(strings.TrimPrefix(lower, marker))
				if insight != "" {
					insights = append(insights, insight)
				}
			}
		}
	}

	return insights
}

// extractTestResults finds before/after test counts
func extractTestResults(lines []string) *TestResults {
	for _, line := range lines {
		lower := strings.ToLower(line)

		// Pattern: "8 failing → 8 passing (103/103 total)"
		failToPassPattern := regexp.MustCompile(`(\d+)\s+failing\s*→\s*(\d+)\s+passing\s*\((\d+)/(\d+)`)
		if match := failToPassPattern.FindStringSubmatch(lower); match != nil {
			failing := 0
			passing := 0
			total := 0
			fmt.Sscanf(match[1], "%d", &failing)
			fmt.Sscanf(match[3], "%d", &passing)
			fmt.Sscanf(match[4], "%d", &total)

			return &TestResults{
				Before: total - failing,
				After:  total,
				Fixed:  failing,
			}
		}

		// Pattern: "103/103 passing"
		allPassPattern := regexp.MustCompile(`(\d+)/(\d+)\s+passing`)
		if match := allPassPattern.FindStringSubmatch(lower); match != nil {
			total := 0
			fmt.Sscanf(match[2], "%d", &total)

			return &TestResults{
				Before: 0,
				After:  total,
				Fixed:  0,
			}
		}
	}

	return nil
}

// outputHumanReadable outputs incident data in human-readable format
func outputHumanReadable(incidents []IncidentData) error {
	for i, incident := range incidents {
		if i > 0 {
			fmt.Println()
			fmt.Println(strings.Repeat("─", 70))
			fmt.Println()
		}

		output.Success(fmt.Sprintf("INCIDENT: %s", incident.Title))
		fmt.Println()
		output.Item("DATE", incident.Timestamp.Format("2006-01-02"))
		output.Item("STATUS", incident.Status)
		fmt.Println()

		if len(incident.RootCauses) > 0 {
			output.Header("ROOT CAUSES:")
			for i, cause := range incident.RootCauses {
				location := ""
				if cause.Location != "" {
					location = fmt.Sprintf(" (line %s)", cause.Location)
				}
				fmt.Printf("  %d. %s%s\n", i+1, cause.Detail, location)
			}
			fmt.Println()
		}

		if len(incident.Fixes) > 0 {
			output.Header("FIXES:")
			for _, fix := range incident.Fixes {
				fmt.Printf("  %s\n", fix.File)
				if fix.Lines != "" && fix.Function != "" {
					fmt.Printf("    Lines %s: %s()\n", fix.Lines, fix.Function)
				} else if fix.Lines != "" {
					fmt.Printf("    Lines %s\n", fix.Lines)
				} else if fix.Function != "" {
					fmt.Printf("    Function: %s()\n", fix.Function)
				}
			}
			fmt.Println()
		}

		if len(incident.Insights) > 0 {
			output.Header("INSIGHTS:")
			for _, insight := range incident.Insights {
				fmt.Printf("  - %s\n", insight)
			}
			fmt.Println()
		}

		if incident.Tests != nil {
			output.Header("TESTS:")
			if incident.Tests.Fixed > 0 {
				fmt.Printf("  %d failing → %d passing (%d/%d total)\n",
					incident.Tests.Fixed,
					incident.Tests.Fixed,
					incident.Tests.After,
					incident.Tests.After)
			} else {
				fmt.Printf("  %d/%d passing\n", incident.Tests.After, incident.Tests.After)
			}
		}
	}

	return nil
}

// outputIncidentJSON outputs incident data as JSON
func outputIncidentJSON(incidents []IncidentData) error {
	// Convert to JSON-friendly format
	type JSONIncident struct {
		Incident   string       `json:"incident"`
		Timestamp  string       `json:"timestamp"`
		Status     string       `json:"status"`
		RootCauses []RootCause  `json:"root_causes"`
		Fixes      []Fix        `json:"fixes"`
		Insights   []string     `json:"insights"`
		Tests      *TestResults `json:"tests,omitempty"`
	}

	var jsonIncidents []JSONIncident
	for _, incident := range incidents {
		jsonIncidents = append(jsonIncidents, JSONIncident{
			Incident:   incident.Title,
			Timestamp:  incident.Timestamp.Format(time.RFC3339),
			Status:     incident.Status,
			RootCauses: incident.RootCauses,
			Fixes:      incident.Fixes,
			Insights:   incident.Insights,
			Tests:      incident.Tests,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonIncidents)
}

// outputNeoSummary outputs one-paragraph handoff summary
func outputNeoSummary(incidents []IncidentData) error {
	for i, incident := range incidents {
		if i > 0 {
			fmt.Println()
			fmt.Println()
		}

		summary := fmt.Sprintf("%s on %s. ",
			incident.Title,
			incident.Timestamp.Format("2006-01-02"))

		if len(incident.RootCauses) > 0 {
			summary += "Root causes: "
			causeTexts := make([]string, len(incident.RootCauses))
			for i, cause := range incident.RootCauses {
				causeTexts[i] = cause.Detail
			}
			summary += strings.Join(causeTexts, "; ") + ". "
		}

		if len(incident.Fixes) > 0 {
			summary += "Fixed in "
			fixTexts := make([]string, len(incident.Fixes))
			for i, fix := range incident.Fixes {
				filename := filepath.Base(fix.File)
				if fix.Lines != "" {
					fixTexts[i] = fmt.Sprintf("%s lines %s", filename, fix.Lines)
				} else {
					fixTexts[i] = filename
				}
			}
			summary += strings.Join(fixTexts, " and ") + ". "
		}

		if incident.Tests != nil && incident.Tests.Fixed > 0 {
			summary += fmt.Sprintf("All %d failing tests now pass (%d/%d total). ",
				incident.Tests.Fixed,
				incident.Tests.After,
				incident.Tests.After)
		} else if incident.Tests != nil {
			summary += fmt.Sprintf("%d/%d tests passing. ", incident.Tests.After, incident.Tests.After)
		}

		if len(incident.Insights) > 0 {
			summary += "Key insight: " + incident.Insights[0] + "."
		}

		fmt.Println(summary)
	}

	return nil
}

// outputPatternAnalysis outputs pattern analysis across incidents
func outputPatternAnalysis(incidents []IncidentData, pattern string) error {
	output.Success(fmt.Sprintf("PATTERN ANALYSIS: %s (%d incidents)", pattern, len(incidents)))
	fmt.Println()

	// Aggregate common root causes
	causeFreq := make(map[string]int)
	for _, incident := range incidents {
		for _, cause := range incident.RootCauses {
			// Simplify cause to key phrases
			key := simplifyText(cause.Detail)
			causeFreq[key]++
		}
	}

	if len(causeFreq) > 0 {
		output.Header("COMMON ROOT CAUSES:")
		type causeCount struct {
			text  string
			count int
		}
		var causes []causeCount
		for text, count := range causeFreq {
			causes = append(causes, causeCount{text, count})
		}
		sort.Slice(causes, func(i, j int) bool {
			return causes[i].count > causes[j].count
		})

		for _, cc := range causes {
			if cc.count > 1 {
				fmt.Printf("  - %s (%d incidents)\n", cc.text, cc.count)
			}
		}
		fmt.Println()
	}

	// Aggregate affected files
	fileFreq := make(map[string]int)
	for _, incident := range incidents {
		for _, fix := range incident.Fixes {
			fileFreq[fix.File]++
		}
	}

	if len(fileFreq) > 0 {
		output.Header("AFFECTED FILES:")
		type fileCount struct {
			file  string
			count int
		}
		var files []fileCount
		for file, count := range fileFreq {
			files = append(files, fileCount{file, count})
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i].count > files[j].count
		})

		for _, fc := range files {
			fmt.Printf("  - %s (%d fixes)\n", fc.file, fc.count)
		}
		fmt.Println()
	}

	// Aggregate insights
	if len(incidents) > 0 {
		output.Header("INSIGHTS:")
		for _, incident := range incidents {
			for _, insight := range incident.Insights {
				fmt.Printf("  - %s\n", insight)
			}
		}
	}

	return nil
}

// simplifyText extracts key phrases from text
func simplifyText(text string) string {
	// Extract first meaningful phrase
	words := strings.Fields(strings.ToLower(text))
	if len(words) > 5 {
		return strings.Join(words[:5], " ")
	}
	return strings.ToLower(text)
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}
