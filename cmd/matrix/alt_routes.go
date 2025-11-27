package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/ram"
)

// AccessibilityIssue represents a potential accessibility barrier
type AccessibilityIssue struct {
	File        string
	LineNumber  int
	Type        string
	Description string
}

// runAltRoutes implements the alt-routes command
func runAltRoutes() error {
	if len(os.Args) < 3 {
		printAltRoutesUsage()
		return nil
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "audit":
		return auditAccessibility()
	case "strip":
		return stripANSI()
	case "search":
		return searchRAM()
	case "list":
		return listIdentitiesPlain()
	default:
		fmt.Fprintf(os.Stderr, "Unknown alt-routes subcommand: %s\n", subcommand)
		printAltRoutesUsage()
		os.Exit(1)
	}

	return nil
}

func printAltRoutesUsage() {
	fmt.Println("alt-routes - Accessibility audit and alternative output formats")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  matrix alt-routes audit")
	fmt.Println("  matrix alt-routes strip < input.txt")
	fmt.Println("  matrix alt-routes search <term>")
	fmt.Println("  matrix alt-routes list")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Println("  audit    Audit matrix commands for accessibility issues")
	fmt.Println("  strip    Read stdin, strip ANSI codes, output plain text")
	fmt.Println("  search   Search RAM files for term (plain text)")
	fmt.Println("  list     List identities with connection counts (plain text)")
}

// auditAccessibility scans matrix command files for accessibility issues
func auditAccessibility() error {
	// Find all .go command files
	cmdDir := "/home/w3surf/work/personal/code/matrix/cmd/matrix"
	files, err := filepath.Glob(filepath.Join(cmdDir, "*.go"))
	if err != nil {
		return fmt.Errorf("failed to find command files: %w", err)
	}

	var issues []AccessibilityIssue
	var accessibleFiles []string

	// Patterns to detect accessibility issues
	colorPattern := regexp.MustCompile(`(?:output\.(Green|Cyan|Yellow|Red|Dim)|"\033\[)`)
	noColorPattern := regexp.MustCompile(`NoColor|--no-color|--plain`)

	for _, filePath := range files {
		// Skip main.go and alt_routes.go itself
		base := filepath.Base(filePath)
		if base == "main.go" || base == "alt_routes.go" {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		fileContent := string(content)
		lines := strings.Split(fileContent, "\n")

		hasColors := false
		hasNoColorSupport := false
		fileIssues := []AccessibilityIssue{}

		// Check each line
		for i, line := range lines {
			lineNum := i + 1

			// Check for color usage
			if colorPattern.MatchString(line) {
				hasColors = true
			}

			// Check for no-color support
			if noColorPattern.MatchString(line) {
				hasNoColorSupport = true
			}

			// Check for ASCII art or visual formatting
			if strings.Contains(line, "├") || strings.Contains(line, "└") ||
				strings.Contains(line, "─") || strings.Contains(line, "│") ||
				strings.Contains(line, "→") || strings.Contains(line, "🌱") ||
				strings.Contains(line, "🌿") {

				// Check if there's also plain text alternative in same context
				hasPlainAlternative := false
				// Look ahead a few lines for plain text mode
				for j := i; j < i+10 && j < len(lines); j++ {
					if strings.Contains(lines[j], "--plain") || strings.Contains(lines[j], "NoColor") {
						hasPlainAlternative = true
						break
					}
				}

				if !hasPlainAlternative {
					fileIssues = append(fileIssues, AccessibilityIssue{
						File:        base,
						LineNumber:  lineNum,
						Type:        "visual-formatting",
						Description: "Uses visual formatting without plain text alternative",
					})
				}
			}
		}

		// Check if colors used without NoColor support
		if hasColors && !hasNoColorSupport {
			fileIssues = append(fileIssues, AccessibilityIssue{
				File:        base,
				LineNumber:  0,
				Type:        "no-color-flag",
				Description: "Uses ANSI colors without --no-color flag support",
			})
		}

		if len(fileIssues) > 0 {
			issues = append(issues, fileIssues...)
		} else if hasColors && hasNoColorSupport {
			accessibleFiles = append(accessibleFiles, base)
		}
	}

	// Print audit report
	fmt.Println("WHEELCHAIR Accessibility Audit")
	fmt.Println("")
	fmt.Printf("Commands Audited: %d\n", len(files)-2) // Exclude main.go and alt_routes.go
	fmt.Println("")

	if len(issues) > 0 {
		fmt.Println("ISSUES FOUND:")
		fmt.Println("")

		// Group issues by file
		issuesByFile := make(map[string][]AccessibilityIssue)
		for _, issue := range issues {
			issuesByFile[issue.File] = append(issuesByFile[issue.File], issue)
		}

		// Sort files for consistent output
		files := make([]string, 0, len(issuesByFile))
		for file := range issuesByFile {
			files = append(files, file)
		}
		sort.Strings(files)

		for _, file := range files {
			fmt.Printf("  %s\n", file)
			for _, issue := range issuesByFile[file] {
				if issue.LineNumber > 0 {
					fmt.Printf("    WARNING (line %d): %s\n", issue.LineNumber, issue.Description)
				} else {
					fmt.Printf("    WARNING: %s\n", issue.Description)
				}
			}
			fmt.Println("")
		}
	}

	if len(accessibleFiles) > 0 {
		fmt.Println("ACCESSIBLE:")
		fmt.Println("")
		for _, file := range accessibleFiles {
			fmt.Printf("  %s\n", file)
			fmt.Println("    CHECK MARK Provides structured output with color support")
			fmt.Println("")
		}
	}

	// Count unique files with issues
	uniqueFiles := make(map[string]bool)
	for _, issue := range issues {
		uniqueFiles[issue.File] = true
	}

	fmt.Printf("Summary: %d issues across %d commands\n", len(issues), len(uniqueFiles))

	return nil
}

// stripANSI reads from stdin, strips ANSI escape sequences, writes to stdout
func stripANSI() error {
	// ANSI escape sequence pattern
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		cleaned := ansiPattern.ReplaceAllString(line, "")
		fmt.Println(cleaned)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("error reading stdin: %w", err)
	}

	return nil
}

// searchRAM searches all RAM files for a term
func searchRAM() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("search requires a term argument")
	}

	term := strings.ToLower(os.Args[3])

	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		fmt.Println("No RAM directory found")
		return nil
	}

	// Scan RAM files
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No files found in RAM directory")
		return nil
	}

	// Search for term
	type Match struct {
		File       ram.File
		LineNumber int
		Line       string
	}

	var matches []Match

	for _, file := range files {
		lines := strings.Split(file.Content, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), term) {
				matches = append(matches, Match{
					File:       file,
					LineNumber: i + 1,
					Line:       strings.TrimSpace(line),
				})
			}
		}
	}

	// Display results
	if len(matches) == 0 {
		fmt.Printf("No matches found for '%s'\n", term)
		return nil
	}

	fmt.Printf("Search Results: %d matches for '%s'\n", len(matches), term)
	fmt.Println("")

	currentFile := ""
	for _, match := range matches {
		homeDir, _ := os.UserHomeDir()
		relativePath := strings.Replace(match.File.Path, homeDir, "~", 1)

		if relativePath != currentFile {
			if currentFile != "" {
				fmt.Println("")
			}
			fmt.Printf("%s\n", relativePath)
			currentFile = relativePath
		}

		fmt.Printf("  Line %d: %s\n", match.LineNumber, match.Line)
	}

	return nil
}

// listIdentitiesPlain lists identities with connection counts (plain text)
func listIdentitiesPlain() error {
	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		fmt.Println("No RAM directory found")
		return nil
	}

	// Scan RAM directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No files found in RAM directory")
		return nil
	}

	// Track connections (reuse logic from garden-paths)
	identityMentions := make(map[string]int)
	allIdentities := identity.All()

	for _, file := range files {
		mentions := findIdentityMentions(file.Content, file.Identity, allIdentities)
		for _, mention := range mentions {
			identityMentions[mention]++
		}
	}

	// Convert to sorted list
	type IdentityInfo struct {
		Name  string
		Count int
	}

	var identityList []IdentityInfo
	for _, id := range allIdentities {
		count := identityMentions[id]
		identityList = append(identityList, IdentityInfo{Name: id, Count: count})
	}

	// Sort by count descending, then by name
	sort.Slice(identityList, func(i, j int) bool {
		if identityList[i].Count != identityList[j].Count {
			return identityList[i].Count > identityList[j].Count
		}
		return identityList[i].Name < identityList[j].Name
	})

	// Display plain text list
	fmt.Println("Identity Connections")
	fmt.Println("")
	fmt.Println("Identity          | Connections")
	fmt.Println("------------------+------------")

	for _, info := range identityList {
		fmt.Printf("%-17s | %d\n", info.Name, info.Count)
	}

	fmt.Println("")
	fmt.Printf("Total: %d identities\n", len(identityList))

	return nil
}
