package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
)

// Crossroads represents a decision point record
type Crossroads struct {
	FilePath   string
	Context    string
	Date       string
	RecordedBy string
	Paths      []string
	Chosen     string
	Reasoning  string
}

// runCrossroads implements the crossroads command
func runCrossroads() error {
	if len(os.Args) < 3 {
		printCrossroadsUsage()
		return nil
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "record":
		return recordCrossroads()
	case "search":
		return searchCrossroads()
	case "list":
		return listCrossroads()
	case "patterns":
		return showPatterns()
	default:
		fmt.Fprintf(os.Stderr, "Unknown crossroads subcommand: %s\n", subcommand)
		printCrossroadsUsage()
		os.Exit(1)
	}

	return nil
}

func printCrossroadsUsage() {
	fmt.Println("crossroads - Capture decision points and paths not taken")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  matrix crossroads record --context=\"...\" --paths=\"1. X, 2. Y\" --chosen=\"1\" --because=\"...\"")
	fmt.Println("  matrix crossroads search <keyword>")
	fmt.Println("  matrix crossroads list")
	fmt.Println("  matrix crossroads patterns")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Println("  record    Record a new decision point")
	fmt.Println("  search    Search past crossroads by keyword")
	fmt.Println("  list      Show all recorded crossroads")
	fmt.Println("  patterns  Show recurring themes across decisions")
}

func recordCrossroads() error {
	// Parse flags
	var context, pathsStr, chosen, because string

	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]

		if strings.HasPrefix(arg, "--context=") {
			context = strings.TrimPrefix(arg, "--context=")
		} else if strings.HasPrefix(arg, "--paths=") {
			pathsStr = strings.TrimPrefix(arg, "--paths=")
		} else if strings.HasPrefix(arg, "--chosen=") {
			chosen = strings.TrimPrefix(arg, "--chosen=")
		} else if strings.HasPrefix(arg, "--because=") {
			because = strings.TrimPrefix(arg, "--because=")
		}
	}

	// Validate required fields
	if context == "" || pathsStr == "" {
		return fmt.Errorf("--context and --paths are required")
	}

	// Parse paths (split on numbered list pattern)
	paths := parsePaths(pathsStr)
	if len(paths) == 0 {
		return fmt.Errorf("could not parse paths - use format: '1. Option A, 2. Option B'")
	}

	// Determine which identity is recording (default to oracle)
	recordedBy := "oracle"

	// Get crossroads directory
	oraclePath, err := identity.RAMPath("oracle")
	if err != nil {
		return fmt.Errorf("failed to get oracle RAM path: %w", err)
	}

	crossroadsDir := filepath.Join(oraclePath, "crossroads")

	// Create directory if needed
	if err := os.MkdirAll(crossroadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create crossroads directory: %w", err)
	}

	// Generate filename
	dateStr := time.Now().Format("2006-01-02")
	slug := slugify(context)
	filename := fmt.Sprintf("%s-%s.md", slug, dateStr)
	filePath := filepath.Join(crossroadsDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("crossroads already recorded today with similar context: %s", filename)
	}

	// Build markdown content
	content := buildCrossroadsMarkdown(context, dateStr, recordedBy, paths, chosen, because)

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write crossroads file: %w", err)
	}

	// Display success
	homeDir, _ := os.UserHomeDir()
	relativePath := strings.Replace(filePath, homeDir, "~", 1)

	output.Success("✨ Crossroads recorded")
	fmt.Println("")
	fmt.Printf("Saved to: %s\n", output.Yellow+relativePath+output.Reset)
	fmt.Println("")
	fmt.Println(output.Cyan + "Context:" + output.Reset)
	fmt.Printf("  %s\n", context)
	fmt.Println("")
	fmt.Println(output.Cyan + "Paths considered:" + output.Reset)
	for i, path := range paths {
		prefix := "  "
		if chosen != "" && fmt.Sprintf("%d", i+1) == chosen {
			prefix = output.Green + "→ " + output.Reset
		}
		fmt.Printf("%s%d. %s\n", prefix, i+1, path)
	}
	if because != "" {
		fmt.Println("")
		fmt.Println(output.Cyan + "Reasoning:" + output.Reset)
		fmt.Printf("  %s\n", because)
	}

	return nil
}

func searchCrossroads() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("search requires a keyword argument")
	}

	keyword := strings.ToLower(os.Args[3])

	// Get crossroads directory
	oraclePath, err := identity.RAMPath("oracle")
	if err != nil {
		return fmt.Errorf("failed to get oracle RAM path: %w", err)
	}

	crossroadsDir := filepath.Join(oraclePath, "crossroads")

	// Check if directory exists
	if _, err := os.Stat(crossroadsDir); os.IsNotExist(err) {
		fmt.Println("No crossroads recorded yet.")
		fmt.Println("")
		fmt.Println("Use 'matrix crossroads record' to capture decision points.")
		return nil
	}

	// Read all crossroads files
	files, err := os.ReadDir(crossroadsDir)
	if err != nil {
		return fmt.Errorf("failed to read crossroads directory: %w", err)
	}

	// Search through files
	var matches []Crossroads

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(crossroadsDir, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Check if keyword matches
		if strings.Contains(strings.ToLower(string(content)), keyword) {
			cr := parseCrossroadsFile(filePath, string(content))
			matches = append(matches, cr)
		}
	}

	// Display results
	if len(matches) == 0 {
		fmt.Printf("No crossroads found matching '%s'\n", keyword)
		return nil
	}

	output.Success(fmt.Sprintf("🔍 Crossroads found (%d matches):", len(matches)))
	fmt.Println("")

	// Sort by date descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Date > matches[j].Date
	})

	for i, cr := range matches {
		if i > 0 {
			fmt.Println(strings.Repeat("─", 70))
			fmt.Println("")
		}

		fmt.Printf("%s. %s (%s)\n",
			output.Yellow+fmt.Sprintf("%d", i+1)+output.Reset,
			output.Cyan+cr.Context+output.Reset,
			cr.Date)

		if len(cr.Paths) > 0 {
			fmt.Println("   Paths:", strings.Join(cr.Paths, " | "))
		}

		if cr.Chosen != "" {
			fmt.Printf("   Chose: %s", output.Green+cr.Chosen+output.Reset)
			if cr.Reasoning != "" {
				fmt.Printf(" → \"%s\"", cr.Reasoning)
			}
			fmt.Println("")
		}

		if cr.RecordedBy != "" {
			fmt.Printf("   (by: %s)\n", cr.RecordedBy)
		}

		fmt.Println("")
	}

	return nil
}

func listCrossroads() error {
	// Get crossroads directory
	oraclePath, err := identity.RAMPath("oracle")
	if err != nil {
		return fmt.Errorf("failed to get oracle RAM path: %w", err)
	}

	crossroadsDir := filepath.Join(oraclePath, "crossroads")

	// Check if directory exists
	if _, err := os.Stat(crossroadsDir); os.IsNotExist(err) {
		fmt.Println("No crossroads recorded yet.")
		fmt.Println("")
		fmt.Println("Use 'matrix crossroads record' to capture decision points.")
		return nil
	}

	// Read all crossroads files
	files, err := os.ReadDir(crossroadsDir)
	if err != nil {
		return fmt.Errorf("failed to read crossroads directory: %w", err)
	}

	var allCrossroads []Crossroads

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(crossroadsDir, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		cr := parseCrossroadsFile(filePath, string(content))
		allCrossroads = append(allCrossroads, cr)
	}

	if len(allCrossroads) == 0 {
		fmt.Println("No crossroads recorded yet.")
		return nil
	}

	// Sort by date descending
	sort.Slice(allCrossroads, func(i, j int) bool {
		return allCrossroads[i].Date > allCrossroads[j].Date
	})

	output.Success(fmt.Sprintf("🗺️  All Crossroads (%d recorded):", len(allCrossroads)))
	fmt.Println("")

	for i, cr := range allCrossroads {
		fmt.Printf("%s %s\n",
			output.Yellow+cr.Date+output.Reset,
			cr.Context)

		if cr.Chosen != "" {
			fmt.Printf("    → %s", cr.Chosen)
			if cr.Reasoning != "" {
				fmt.Printf(" (%s)", cr.Reasoning)
			}
			fmt.Println("")
		}

		if i < len(allCrossroads)-1 {
			fmt.Println("")
		}
	}

	return nil
}

func showPatterns() error {
	// Get crossroads directory
	oraclePath, err := identity.RAMPath("oracle")
	if err != nil {
		return fmt.Errorf("failed to get oracle RAM path: %w", err)
	}

	crossroadsDir := filepath.Join(oraclePath, "crossroads")

	// Check if directory exists
	if _, err := os.Stat(crossroadsDir); os.IsNotExist(err) {
		fmt.Println("No crossroads recorded yet.")
		fmt.Println("")
		fmt.Println("Use 'matrix crossroads record' to capture decision points.")
		return nil
	}

	// Read all crossroads
	files, err := os.ReadDir(crossroadsDir)
	if err != nil {
		return fmt.Errorf("failed to read crossroads directory: %w", err)
	}

	var allCrossroads []Crossroads
	keywordCounts := make(map[string]int)
	pathCounts := make(map[string]int)

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(crossroadsDir, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		cr := parseCrossroadsFile(filePath, string(content))
		allCrossroads = append(allCrossroads, cr)

		// Count keywords in context
		words := extractKeywords(cr.Context)
		for _, word := range words {
			keywordCounts[word]++
		}

		// Count paths considered
		for _, path := range cr.Paths {
			cleanPath := strings.TrimSpace(path)
			if cleanPath != "" {
				pathCounts[cleanPath]++
			}
		}
	}

	if len(allCrossroads) == 0 {
		fmt.Println("No crossroads recorded yet.")
		return nil
	}

	output.Success(fmt.Sprintf("📊 Patterns Across %d Crossroads:", len(allCrossroads)))
	fmt.Println("")

	// Most common contexts
	output.Header("Recurring Themes:")
	fmt.Println("")

	type keywordCount struct {
		keyword string
		count   int
	}

	var keywords []keywordCount
	for k, v := range keywordCounts {
		if v > 1 { // Only show recurring themes
			keywords = append(keywords, keywordCount{k, v})
		}
	}

	sort.Slice(keywords, func(i, j int) bool {
		if keywords[i].count != keywords[j].count {
			return keywords[i].count > keywords[j].count
		}
		return keywords[i].keyword < keywords[j].keyword
	})

	if len(keywords) == 0 {
		fmt.Println("  Not enough data yet - record more crossroads to see patterns")
	} else {
		limit := 10
		if len(keywords) < limit {
			limit = len(keywords)
		}

		for i := 0; i < limit; i++ {
			fmt.Printf("  %s (appears in %d crossroads)\n",
				keywords[i].keyword,
				keywords[i].count)
		}
	}

	fmt.Println("")

	// Most considered paths
	output.Header("Frequently Considered Paths:")
	fmt.Println("")

	type pathCount struct {
		path  string
		count int
	}

	var paths []pathCount
	for p, c := range pathCounts {
		if c > 1 {
			paths = append(paths, pathCount{p, c})
		}
	}

	sort.Slice(paths, func(i, j int) bool {
		if paths[i].count != paths[j].count {
			return paths[i].count > paths[j].count
		}
		return paths[i].path < paths[j].path
	})

	if len(paths) == 0 {
		fmt.Println("  No recurring paths yet")
	} else {
		limit := 8
		if len(paths) < limit {
			limit = len(paths)
		}

		for i := 0; i < limit; i++ {
			fmt.Printf("  %s (considered %d times)\n",
				paths[i].path,
				paths[i].count)
		}
	}

	fmt.Println("")
	output.Success("✨ The paths reveal the garden's shape")

	return nil
}

// Helper functions

func slugify(text string) string {
	// Convert to lowercase
	slug := strings.ToLower(text)

	// Replace spaces and special chars with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug = re.ReplaceAllString(slug, "-")

	// Trim hyphens from ends
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 60 {
		slug = slug[:60]
	}

	return slug
}

func parsePaths(pathsStr string) []string {
	// Split on comma or newline
	parts := strings.Split(pathsStr, ",")
	if len(parts) == 1 {
		parts = strings.Split(pathsStr, "\n")
	}

	var paths []string
	re := regexp.MustCompile(`^\d+\.\s*(.+)$`)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Try to extract numbered item (e.g., "1. Option A")
		matches := re.FindStringSubmatch(part)
		if len(matches) > 1 {
			paths = append(paths, matches[1])
		} else {
			// If not numbered, just use the text
			paths = append(paths, part)
		}
	}

	return paths
}

func buildCrossroadsMarkdown(context, date, recordedBy string, paths []string, chosen, reasoning string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Crossroads: %s\n\n", context))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", date))
	sb.WriteString(fmt.Sprintf("**Recorded by:** %s\n\n", recordedBy))

	sb.WriteString("## Paths Considered\n\n")
	for i, path := range paths {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, path))
	}
	sb.WriteString("\n")

	if chosen != "" {
		sb.WriteString("## Chosen Path\n\n")

		// Try to find which path was chosen
		chosenIdx := -1
		if _, err := fmt.Sscanf(chosen, "%d", &chosenIdx); err == nil && chosenIdx > 0 && chosenIdx <= len(paths) {
			sb.WriteString(fmt.Sprintf("**#%d: %s**\n\n", chosenIdx, paths[chosenIdx-1]))
		} else {
			sb.WriteString(fmt.Sprintf("**%s**\n\n", chosen))
		}

		if reasoning != "" {
			sb.WriteString(fmt.Sprintf("**Reasoning:** %s\n\n", reasoning))
		}
	}

	sb.WriteString("---\n")
	sb.WriteString("*\"You didn't come here to make the choice. You've already made it.\"*\n")

	return sb.String()
}

func parseCrossroadsFile(filePath, content string) Crossroads {
	cr := Crossroads{
		FilePath: filePath,
	}

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract title/context
		if strings.HasPrefix(line, "# Crossroads:") {
			cr.Context = strings.TrimSpace(strings.TrimPrefix(line, "# Crossroads:"))
		}

		// Extract date
		if strings.HasPrefix(line, "**Date:**") {
			cr.Date = strings.TrimSpace(strings.TrimPrefix(line, "**Date:**"))
		}

		// Extract recorded by
		if strings.HasPrefix(line, "**Recorded by:**") {
			cr.RecordedBy = strings.TrimSpace(strings.TrimPrefix(line, "**Recorded by:**"))
		}

		// Extract chosen path
		if strings.HasPrefix(line, "**#") && strings.Contains(line, ":**") {
			// Format: **#1: Path name**
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				cr.Chosen = strings.TrimSpace(strings.Trim(parts[1], "*"))
			}
		}

		// Extract reasoning
		if strings.HasPrefix(line, "**Reasoning:**") {
			cr.Reasoning = strings.TrimSpace(strings.TrimPrefix(line, "**Reasoning:**"))
		}

		// Extract paths (numbered list items)
		if match, _ := regexp.MatchString(`^\d+\.\s+\*\*`, line); match {
			re := regexp.MustCompile(`^\d+\.\s+\*\*(.+)\*\*`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				cr.Paths = append(cr.Paths, matches[1])
			}
		}
	}

	return cr
}

func extractKeywords(text string) []string {
	// Simple keyword extraction - split on spaces and filter
	words := strings.Fields(strings.ToLower(text))
	var keywords []string

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "was": true, "are": true, "were": true, "be": true,
		"this": true, "that": true, "these": true, "those": true,
	}

	for _, word := range words {
		// Clean word
		word = strings.Trim(word, ".,!?;:\"'")
		if len(word) < 3 {
			continue
		}
		if stopWords[word] {
			continue
		}
		keywords = append(keywords, word)
	}

	return keywords
}
