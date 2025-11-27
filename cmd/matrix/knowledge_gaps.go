package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// GapType represents category of knowledge gap
type GapType string

const (
	GapQuestion   GapType = "Question"
	GapTodo       GapType = "Documentation TODO"
	GapComplexity GapType = "High Complexity"
)

// Gap represents a detected knowledge gap
type Gap struct {
	Type     GapType
	FilePath string
	Identity string
	LineNum  int
	Quote    string
}

// GapGroup groups gaps by type
type GapGroup struct {
	Type GapType
	Gaps []Gap
}

// FileGaps groups gaps by file for detailed output
type FileGaps struct {
	FilePath string
	Identity string
	Gaps     []Gap
}

// runKnowledgeGaps implements the knowledge-gaps command
func runKnowledgeGaps() error {
	// Parse flags
	flags := flag.NewFlagSet("knowledge-gaps", flag.ExitOnError)
	showQuestions := flags.Bool("questions", false, "Show only questions")
	showTodos := flags.Bool("todos", false, "Show only documentation TODOs")
	showComplexity := flags.Bool("complexity", false, "Show only high-complexity areas")
	detailed := flags.Bool("detailed", false, "Include context around findings")
	filterIdentity := flags.String("identity", "", "Filter to specific identity")

	flags.Parse(os.Args[2:])

	// Determine which types to show
	showAll := !*showQuestions && !*showTodos && !*showComplexity
	showTypes := make(map[GapType]bool)
	if showAll || *showQuestions {
		showTypes[GapQuestion] = true
	}
	if showAll || *showTodos {
		showTypes[GapTodo] = true
	}
	if showAll || *showComplexity {
		showTypes[GapComplexity] = true
	}

	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if RAM exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		fmt.Println("🌾 No RAM found at ~/.claude/ram/ - nothing to scan yet")
		return nil
	}

	// Scan RAM directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("🌾 RAM exists but no markdown files found yet")
		return nil
	}

	// Filter by identity if requested
	if *filterIdentity != "" {
		normalizedFilter := strings.ToLower(strings.TrimSpace(*filterIdentity))
		if !identity.IsValid(normalizedFilter) {
			return fmt.Errorf("invalid identity: %s", *filterIdentity)
		}

		var filtered []ram.File
		for _, f := range files {
			if f.Identity == normalizedFilter {
				filtered = append(filtered, f)
			}
		}
		files = filtered

		if len(files) == 0 {
			fmt.Printf("No files found for identity: %s\n", normalizedFilter)
			return nil
		}
	}

	output.Success("🔍 Knowledge Gaps Report")
	fmt.Println("")
	if *filterIdentity != "" {
		fmt.Printf("Filtering to identity: %s\n", *filterIdentity)
		fmt.Println("")
	}
	fmt.Println("Scanning for unanswered questions and missing documentation...")
	fmt.Println("")

	// Scan all files for gaps
	var allGaps []Gap
	for _, file := range files {
		gaps := detectKnowledgeGaps(file)
		allGaps = append(allGaps, gaps...)
	}

	// Filter gaps by requested types
	var filteredGaps []Gap
	for _, gap := range allGaps {
		if showTypes[gap.Type] {
			filteredGaps = append(filteredGaps, gap)
		}
	}

	if len(filteredGaps) == 0 {
		fmt.Println("✨ No knowledge gaps detected - documentation is complete")
		return nil
	}

	// Display results
	if *detailed {
		displayDetailedGaps(filteredGaps, showTypes)
	} else {
		displayGroupedGaps(filteredGaps, showTypes)
	}

	fmt.Println("")
	displayGapSummary(filteredGaps, len(files))

	return nil
}

// detectKnowledgeGaps scans a file for knowledge gaps
func detectKnowledgeGaps(file ram.File) []Gap {
	var gaps []Gap
	lines := strings.Split(file.Content, "\n")

	// Create relative path for display
	homeDir, _ := os.UserHomeDir()
	relativePath := strings.Replace(file.Path, homeDir, "~", 1)

	for lineNum, line := range lines {
		lineLower := strings.ToLower(line)
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and markdown headers
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// Check for questions
		if matchesPattern(lineLower, questionPatterns()) {
			gaps = append(gaps, Gap{
				Type:     GapQuestion,
				FilePath: relativePath,
				Identity: file.Identity,
				LineNum:  lineNum + 1,
				Quote:    trimmedLine,
			})
			continue
		}

		// Check for documentation TODOs
		if matchesPattern(lineLower, todoPatterns()) {
			gaps = append(gaps, Gap{
				Type:     GapTodo,
				FilePath: relativePath,
				Identity: file.Identity,
				LineNum:  lineNum + 1,
				Quote:    trimmedLine,
			})
			continue
		}

		// Check for complexity markers
		if matchesPattern(lineLower, complexityPatterns()) {
			gaps = append(gaps, Gap{
				Type:     GapComplexity,
				FilePath: relativePath,
				Identity: file.Identity,
				LineNum:  lineNum + 1,
				Quote:    trimmedLine,
			})
			continue
		}
	}

	return gaps
}

// Pattern matching functions
func questionPatterns() []*regexp.Regexp {
	patterns := []string{
		`\?`,                            // Lines with question marks
		`\bhow does\b`,                  // "how does"
		`\bwhy does\b`,                  // "why does"
		`\bhow to\b`,                    // "how to"
		`\bwhat is\b`,                   // "what is"
		`\bunclear\b`,                   // "unclear"
		`\bconfused\b`,                  // "confused"
		`\bnot sure\b`,                  // "not sure"
		`\bdon't understand\b`,          // "don't understand"
		`\bwhat happens\b`,              // "what happens"
		`\bwhy would\b`,                 // "why would"
		`\bshould we\b.*\?`,             // "should we...?"
		`\bcan we\b.*\?`,                // "can we...?"
		`\bis it\b.*\?`,                 // "is it...?"
	}

	return compilePatterns(patterns)
}

func todoPatterns() []*regexp.Regexp {
	patterns := []string{
		`\btodo:.*\b(doc|explain|describe|document|write)\b`,  // TODO with doc keywords
		`\btodo:.*\bdocumentation\b`,                           // TODO: documentation
		`\btodo:.*\brunbook\b`,                                 // TODO: runbook
		`\btodo:.*\bguide\b`,                                   // TODO: guide
		`\bneed to document\b`,                                 // "need to document"
		`\bmissing documentation\b`,                            // "missing documentation"
		`\bundocumented\b`,                                     // "undocumented"
		`\bneeds explanation\b`,                                // "needs explanation"
		`\bshould document\b`,                                  // "should document"
		`\bwrite up\b`,                                         // "write up"
		`\bcapture this\b`,                                     // "capture this"
	}

	return compilePatterns(patterns)
}

func complexityPatterns() []*regexp.Regexp {
	patterns := []string{
		`\bcomplex\b`,                                          // "complex"
		`\bintricate\b`,                                        // "intricate"
		`\btricky\b`,                                           // "tricky"
		`\bsubtle\b`,                                           // "subtle"
		`\bedge case\b`,                                        // "edge case"
		`\bcorner case\b`,                                      // "corner case"
		`\bnuanced\b`,                                          // "nuanced"
		`\bdelicate\b`,                                         // "delicate"
		`\bconvoluted\b`,                                       // "convoluted"
		`\bnon-obvious\b`,                                      // "non-obvious"
		`\bnon-trivial\b`,                                      // "non-trivial"
		`\bcomplicated\b`,                                      // "complicated"
		`\bhard to\b`,                                          // "hard to"
		`\bdifficult to\b`,                                     // "difficult to"
		`\bmany moving parts\b`,                                // "many moving parts"
		`\bwip\b`,                                              // "WIP"
		`\bdraft\b`,                                            // "draft"
	}

	return compilePatterns(patterns)
}

// displayGroupedGaps displays gaps grouped by type
func displayGroupedGaps(gaps []Gap, showTypes map[GapType]bool) {
	groups := groupGapsByType(gaps)

	typeOrder := []GapType{GapQuestion, GapTodo, GapComplexity}

	for _, gapType := range typeOrder {
		if !showTypes[gapType] {
			continue
		}

		for _, group := range groups {
			if group.Type != gapType {
				continue
			}

			displayGapGroup(group)
			fmt.Println("")
		}
	}
}

// displayDetailedGaps displays gaps grouped by file with context
func displayDetailedGaps(gaps []Gap, showTypes map[GapType]bool) {
	fileGapsMap := make(map[string]*FileGaps)

	for _, gap := range gaps {
		key := gap.FilePath
		if _, exists := fileGapsMap[key]; !exists {
			fileGapsMap[key] = &FileGaps{
				FilePath: gap.FilePath,
				Identity: gap.Identity,
				Gaps:     []Gap{},
			}
		}
		fileGapsMap[key].Gaps = append(fileGapsMap[key].Gaps, gap)
	}

	// Convert to sorted slice
	var fileGapsList []*FileGaps
	for _, fg := range fileGapsMap {
		fileGapsList = append(fileGapsList, fg)
	}
	sort.Slice(fileGapsList, func(i, j int) bool {
		return fileGapsList[i].FilePath < fileGapsList[j].FilePath
	})

	// Display by type
	typeOrder := []GapType{GapQuestion, GapTodo, GapComplexity}
	colorMap := map[GapType]string{
		GapQuestion:   output.Yellow,
		GapTodo:       output.Cyan,
		GapComplexity: output.Red,
	}
	titleMap := map[GapType]string{
		GapQuestion:   "Questions Needing Answers",
		GapTodo:       "Documentation TODOs",
		GapComplexity: "High-Complexity Areas",
	}

	for _, gapType := range typeOrder {
		if !showTypes[gapType] {
			continue
		}

		// Count gaps of this type
		count := 0
		for _, fg := range fileGapsList {
			for _, gap := range fg.Gaps {
				if gap.Type == gapType {
					count++
				}
			}
		}

		if count == 0 {
			continue
		}

		fmt.Println(strings.Repeat("━", 70))
		fmt.Println(colorMap[gapType] + titleMap[gapType] + output.Reset)
		fmt.Println(strings.Repeat("━", 70))
		fmt.Println("")

		for _, fg := range fileGapsList {
			// Count gaps of this type in this file
			typeGaps := []Gap{}
			for _, gap := range fg.Gaps {
				if gap.Type == gapType {
					typeGaps = append(typeGaps, gap)
				}
			}

			if len(typeGaps) == 0 {
				continue
			}

			fmt.Printf("  %s (%d %s)\n",
				fg.FilePath,
				len(typeGaps),
				strings.ToLower(string(gapType)))

			for _, gap := range typeGaps {
				quote := gap.Quote
				if len(quote) > 100 {
					quote = quote[:97] + "..."
				}
				fmt.Printf("    → %s\n", quote)
			}
			fmt.Println("")
		}
	}
}

// displayGapGroup displays a group of gaps
func displayGapGroup(group GapGroup) {
	colorMap := map[GapType]string{
		GapQuestion:   output.Yellow,
		GapTodo:       output.Cyan,
		GapComplexity: output.Red,
	}
	titleMap := map[GapType]string{
		GapQuestion:   "Questions Needing Answers",
		GapTodo:       "Documentation TODOs",
		GapComplexity: "High-Complexity Areas",
	}

	fmt.Println(strings.Repeat("━", 70))
	fmt.Println(colorMap[group.Type] + titleMap[group.Type] + output.Reset)
	fmt.Println(strings.Repeat("━", 70))
	fmt.Println("")

	// Group by file
	fileGapsMap := make(map[string][]Gap)
	for _, gap := range group.Gaps {
		fileGapsMap[gap.FilePath] = append(fileGapsMap[gap.FilePath], gap)
	}

	// Sort file paths
	var filePaths []string
	for path := range fileGapsMap {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	// Display each file's gaps
	for _, path := range filePaths {
		gaps := fileGapsMap[path]
		fmt.Printf("  %s (%d)\n", path, len(gaps))

		// Show first 3 gaps from this file
		limit := len(gaps)
		if limit > 3 {
			limit = 3
		}

		for i := 0; i < limit; i++ {
			gap := gaps[i]
			quote := gap.Quote
			if len(quote) > 100 {
				quote = quote[:97] + "..."
			}
			fmt.Printf("    → %s\n", quote)
		}

		if len(gaps) > 3 {
			fmt.Printf("    ... and %d more\n", len(gaps)-3)
		}
		fmt.Println("")
	}
}

// groupGapsByType groups gaps by their type
func groupGapsByType(gaps []Gap) []GapGroup {
	groups := make(map[GapType][]Gap)

	for _, g := range gaps {
		groups[g.Type] = append(groups[g.Type], g)
	}

	// Convert to sorted slice
	var result []GapGroup
	typeOrder := []GapType{GapQuestion, GapTodo, GapComplexity}

	for _, gType := range typeOrder {
		if gaps, ok := groups[gType]; ok && len(gaps) > 0 {
			result = append(result, GapGroup{
				Type: gType,
				Gaps: gaps,
			})
		}
	}

	return result
}

// displayGapSummary displays summary statistics
func displayGapSummary(gaps []Gap, filesScanned int) {
	fmt.Println(strings.Repeat("━", 70))
	output.Header("SUMMARY")
	fmt.Println(strings.Repeat("━", 70))
	fmt.Println("")

	// Count by type
	typeCounts := make(map[GapType]int)
	for _, gap := range gaps {
		typeCounts[gap.Type]++
	}

	if count, ok := typeCounts[GapQuestion]; ok && count > 0 {
		fmt.Printf("  - %d unanswered questions\n", count)
	}
	if count, ok := typeCounts[GapTodo]; ok && count > 0 {
		fmt.Printf("  - %d documentation TODOs\n", count)
	}
	if count, ok := typeCounts[GapComplexity]; ok && count > 0 {
		fmt.Printf("  - %d high-complexity areas\n", count)
	}

	fmt.Println("")

	// Count affected identities
	identitySet := make(map[string]bool)
	for _, gap := range gaps {
		identitySet[gap.Identity] = true
	}

	identities := make([]string, 0, len(identitySet))
	for id := range identitySet {
		identities = append(identities, id)
	}
	sort.Strings(identities)

	fmt.Printf("Affected Identities: %d\n", len(identities))
	if len(identities) > 0 {
		fmt.Printf("  %s\n", strings.Join(identities, ", "))
	}
	fmt.Println("")

	fmt.Printf("Files Scanned: %d markdown files\n", filesScanned)
	fmt.Println("")

	output.Success("🔍 Knowledge gaps surfaced - ready for documentation")
}
