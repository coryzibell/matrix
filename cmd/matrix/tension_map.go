package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// TensionType represents a category of tension
type TensionType string

const (
	TensionConflict  TensionType = "Conflicting Statement"
	TensionBoundary  TensionType = "Boundary Dispute"
	TensionProtocol  TensionType = "Protocol Concern"
	TensionGap       TensionType = "Capability Gap"
)

// Tension represents a detected tension in the RAM
type Tension struct {
	Type      TensionType
	FilePath  string
	Identity  string
	LineNum   int
	Quote     string
}

// TensionGroup groups tensions by type
type TensionGroup struct {
	Type     TensionType
	Tensions []Tension
}

// runTensionMap implements the tension-map command
func runTensionMap() error {
	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		fmt.Println("🌾 No garden found at ~/.claude/ram/ - nothing to scan yet")
		fmt.Println("")
		fmt.Println("The garden will grow as identities write to their RAM directories.")
		return nil
	}

	// Scan RAM directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("🌾 Garden exists but no markdown files found yet")
		return nil
	}

	output.Success("🔥 Tension Map - Conflicts Across the Garden")
	fmt.Println("")
	fmt.Println("Scanning for tensions...")
	fmt.Println("")

	// Scan all files for tensions
	var allTensions []Tension

	for _, file := range files {
		tensions := detectTensions(file)
		allTensions = append(allTensions, tensions...)
	}

	// Group by type
	groupedTensions := groupTensionsByType(allTensions)

	// Display results
	if len(allTensions) == 0 {
		fmt.Println("✨ No tensions detected - the garden is harmonious")
		return nil
	}

	// Display each group
	for _, group := range groupedTensions {
		displayTensionGroup(group)
		fmt.Println("")
	}

	// Summary
	displaySummary(groupedTensions, len(files))

	return nil
}

// detectTensions scans a file for tension patterns
func detectTensions(file ram.File) []Tension {
	var tensions []Tension
	lines := strings.Split(file.Content, "\n")

	// Create relative path for display
	homeDir, _ := os.UserHomeDir()
	relativePath := strings.Replace(file.Path, homeDir, "~", 1)

	for lineNum, line := range lines {
		lineLower := strings.ToLower(line)

		// Skip empty lines and pure markdown formatting
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		// Check for conflict patterns
		if matchesPattern(lineLower, conflictPatterns()) {
			tensions = append(tensions, Tension{
				Type:     TensionConflict,
				FilePath: relativePath,
				Identity: file.Identity,
				LineNum:  lineNum + 1,
				Quote:    strings.TrimSpace(line),
			})
			continue
		}

		// Check for boundary dispute patterns
		if matchesPattern(lineLower, boundaryPatterns()) {
			tensions = append(tensions, Tension{
				Type:     TensionBoundary,
				FilePath: relativePath,
				Identity: file.Identity,
				LineNum:  lineNum + 1,
				Quote:    strings.TrimSpace(line),
			})
			continue
		}

		// Check for protocol concern patterns
		if matchesPattern(lineLower, protocolPatterns()) {
			tensions = append(tensions, Tension{
				Type:     TensionProtocol,
				FilePath: relativePath,
				Identity: file.Identity,
				LineNum:  lineNum + 1,
				Quote:    strings.TrimSpace(line),
			})
			continue
		}

		// Check for capability gap patterns
		if matchesPattern(lineLower, gapPatterns()) {
			tensions = append(tensions, Tension{
				Type:     TensionGap,
				FilePath: relativePath,
				Identity: file.Identity,
				LineNum:  lineNum + 1,
				Quote:    strings.TrimSpace(line),
			})
			continue
		}
	}

	return tensions
}

// Pattern matching functions
func conflictPatterns() []*regexp.Regexp {
	patterns := []string{
		`\bbut\b.*\b(disagree|conflict|tension|wrong|incorrect|incompatible)`,
		`\bhowever\b.*\b(disagree|conflict|tension|wrong|incompatible)`,
		`\b(disagree|conflict|tension)\b.*\bwith\b`,
		`\b(this|that)\s+(conflicts?|disagrees?|tensions?)\b`,
		`\bcontradicts?\b`,
		`\bincompatible\s+with\b`,
		`\bconflicting\s+(statements?|perspectives?|requirements?)\b`,
	}

	return compilePatterns(patterns)
}

func boundaryPatterns() []*regexp.Regexp {
	patterns := []string{
		`\b(should be|is|isn't|not)\s+(my|our)\s+(responsibility|role|domain|scope)`,
		`\b(overlaps?\s+with|unclear\s+whether|undefined\s+boundary)\b`,
		`\bboth\s+\w+\s+and\s+\w+\s+(handle|own|manage)`,
		`\b(whose\s+domain|who\s+owns|ownership\s+unclear)\b`,
		`\b(boundary|scope)\s+(dispute|unclear|undefined|fuzzy)`,
		`\bsits\s+between\b.*\band\b`,
		`\b(gap|overlap)\s+between\b`,
	}

	return compilePatterns(patterns)
}

func protocolPatterns() []*regexp.Regexp {
	patterns := []string{
		`\b(violates?|breaks?|doesn't\s+follow)\b.*\b(protocol|guideline|rule|instruction)`,
		`\b(protocol|guideline|rule)\s+(says|requires|demands)\b.*\bbut\b`,
		`\bcan't\s+follow\b.*\b(protocol|guideline|instruction)`,
		`\b(protocol|rule)\s+(conflict|violation|issue|problem)`,
		`\bbase.*says\b.*\bbut\b`,
		`\btold\s+not\s+to\b.*\bbut\b.*\b(need|require|must)`,
	}

	return compilePatterns(patterns)
}

func gapPatterns() []*regexp.Regexp {
	patterns := []string{
		`\b(missing|lacks?|no)\s+(capability|identity|function|tool|feature)`,
		`\b(nobody|no\s+identity|no\s+one)\s+(handles?|owns?|manages?)`,
		`\b(capability|feature|function)\s+gap\b`,
		`\bundefined\s+(capability|ownership|responsibility)`,
		`\bneeds?\s+new\s+(identity|capability|protocol)`,
		`\bwho\s+(handles?|owns?|does)\b.*\?`,
	}

	return compilePatterns(patterns)
}

func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

func matchesPattern(text string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// groupTensionsByType groups tensions by their type
func groupTensionsByType(tensions []Tension) []TensionGroup {
	groups := make(map[TensionType][]Tension)

	for _, t := range tensions {
		groups[t.Type] = append(groups[t.Type], t)
	}

	// Convert to sorted slice
	var result []TensionGroup
	typeOrder := []TensionType{TensionConflict, TensionBoundary, TensionProtocol, TensionGap}

	for _, ttype := range typeOrder {
		if tensions, ok := groups[ttype]; ok && len(tensions) > 0 {
			result = append(result, TensionGroup{
				Type:     ttype,
				Tensions: tensions,
			})
		}
	}

	return result
}

// displayTensionGroup displays a group of tensions
func displayTensionGroup(group TensionGroup) {
	fmt.Println(strings.Repeat("━", 70))
	output.Header(fmt.Sprintf("%s (%d found)", group.Type, len(group.Tensions)))
	fmt.Println(strings.Repeat("━", 70))
	fmt.Println("")

	// Show first 5 tensions in this group
	limit := len(group.Tensions)
	if limit > 5 {
		limit = 5
	}

	for i := 0; i < limit; i++ {
		t := group.Tensions[i]
		fmt.Printf("  [%s] %s:%d\n",
			output.Yellow+t.Identity+output.Reset,
			t.FilePath,
			t.LineNum)

		// Truncate long quotes
		quote := t.Quote
		if len(quote) > 120 {
			quote = quote[:117] + "..."
		}
		fmt.Printf("    \"%s\"\n", quote)
		fmt.Println("")
	}

	if len(group.Tensions) > 5 {
		fmt.Printf("  ... and %d more\n", len(group.Tensions)-5)
		fmt.Println("")
	}
}

// displaySummary displays summary statistics
func displaySummary(groups []TensionGroup, filesScanned int) {
	fmt.Println(strings.Repeat("━", 70))
	output.Header("SUMMARY")
	fmt.Println(strings.Repeat("━", 70))
	fmt.Println("")

	totalTensions := 0
	for _, g := range groups {
		totalTensions += len(g.Tensions)
	}

	fmt.Printf("Tensions Found: %d\n", totalTensions)
	fmt.Println("")

	fmt.Println("By Category:")
	for _, g := range groups {
		fmt.Printf("  - %s: %d\n", g.Type, len(g.Tensions))
	}
	fmt.Println("")

	// Count affected identities
	identitySet := make(map[string]bool)
	for _, g := range groups {
		for _, t := range g.Tensions {
			identitySet[t.Identity] = true
		}
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

	output.Success("🔥 Tensions surfaced - ready for synthesis")
}
