package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// Assertion represents a structural claim extracted from architectural docs
type Assertion struct {
	Description string        // Human-readable claim
	VerifyCmd   string        // Command to verify (empty if manual/unknown)
	Status      AssertionStatus
	Violations  []string      // File:line references where assertion fails
	SourceFile  string        // Which design doc this came from
	SourceLine  int           // Line number in design doc
}

// AssertionStatus tracks whether assertion holds
type AssertionStatus int

const (
	StatusUnknown AssertionStatus = iota
	StatusBalanced
	StatusUnbalanced
)

// ProjectReport summarizes balance check results for a project
type ProjectReport struct {
	ProjectPath string
	Balanced    []Assertion
	Unbalanced  []Assertion
	Unknown     []Assertion
	Score       float64 // Percentage of verifiable assertions that hold
}

// runBalanceChecker implements the balance-checker command
func runBalanceChecker() error {
	// Parse command-line arguments
	args := os.Args[2:] // Skip "matrix" and "balance-checker"

	var targetPath string
	checkAll := false
	threshold := 0.0

	for _, arg := range args {
		if arg == "--all" {
			checkAll = true
		} else if strings.HasPrefix(arg, "--threshold=") {
			val := strings.TrimPrefix(arg, "--threshold=")
			t, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf("invalid threshold value: %s", val)
			}
			threshold = t
		} else if !strings.HasPrefix(arg, "--") {
			targetPath = arg
		}
	}

	// Get RAM directory where architect stores design docs
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	architectDir := filepath.Join(ramDir, "architect")

	// Check if architect directory exists
	if _, err := os.Stat(architectDir); os.IsNotExist(err) {
		fmt.Println("No architectural documents found at ~/.claude/ram/architect/")
		fmt.Println("")
		fmt.Println("Balance checker needs design documents to verify against.")
		return nil
	}

	// Scan architect's markdown files for assertions
	assertions, err := extractAssertions(architectDir)
	if err != nil {
		return fmt.Errorf("failed to extract assertions: %w", err)
	}

	if len(assertions) == 0 {
		fmt.Println("No verifiable assertions found in architectural documents.")
		fmt.Println("")
		fmt.Println("Use MUST, MUST NOT, SHALL, SHALL NOT keywords or [verify: command] directives.")
		return nil
	}

	// Determine target projects
	var targets []string
	if checkAll {
		// Find all known project roots
		homeDir, _ := os.UserHomeDir()
		targets = append(targets,
			filepath.Join(homeDir, "source", "repos"),
			filepath.Join(homeDir, "Work", "Personal"),
		)
	} else if targetPath != "" {
		targets = []string{targetPath}
	} else {
		// Default: current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		targets = []string{cwd}
	}

	// Run balance check on target(s)
	reports := make([]ProjectReport, 0)

	for _, target := range targets {
		// Expand tilde
		if strings.HasPrefix(target, "~") {
			homeDir, _ := os.UserHomeDir()
			target = strings.Replace(target, "~", homeDir, 1)
		}

		// Verify target exists
		if _, err := os.Stat(target); os.IsNotExist(err) {
			continue
		}

		report := checkBalance(target, assertions)
		reports = append(reports, report)
	}

	// Display results
	for _, report := range reports {
		displayBalanceReport(report)
		fmt.Println("")
	}

	// Check threshold
	if threshold > 0 {
		for _, report := range reports {
			if report.Score < threshold {
				return fmt.Errorf("balance score %.1f%% below threshold %.1f%%",
					report.Score, threshold)
			}
		}
	}

	return nil
}

// extractAssertions scans architectural markdown files for MUST/SHALL assertions
func extractAssertions(architectDir string) ([]Assertion, error) {
	files, err := ram.ScanDir(filepath.Dir(architectDir))
	if err != nil {
		return nil, err
	}

	var assertions []Assertion

	// Regex patterns
	mustPattern := regexp.MustCompile(`(?i)\b(MUST|SHALL|MUST NOT|SHALL NOT)\b`)
	verifyPattern := regexp.MustCompile(`\[verify:\s*([^\]]+)\]`)

	for _, file := range files {
		// Only scan architect's files
		if file.Identity != "architect" {
			continue
		}

		lines := strings.Split(file.Content, "\n")

		for lineNum, line := range lines {
			// Check for MUST/SHALL keywords
			if mustPattern.MatchString(line) {
				assertion := Assertion{
					Description: strings.TrimSpace(line),
					Status:      StatusUnknown,
					SourceFile:  file.Path,
					SourceLine:  lineNum + 1,
				}

				// Look for explicit verify command
				if verifyMatch := verifyPattern.FindStringSubmatch(line); verifyMatch != nil {
					cmd := strings.TrimSpace(verifyMatch[1])
					if cmd != "manual" {
						assertion.VerifyCmd = cmd
					}
				} else {
					// Try to infer verification from assertion text
					assertion.VerifyCmd = inferVerifyCommand(line)
				}

				assertions = append(assertions, assertion)
			}
		}
	}

	return assertions, nil
}

// inferVerifyCommand attempts to construct a verification command from assertion text
func inferVerifyCommand(assertionText string) string {
	lower := strings.ToLower(assertionText)

	// Pattern: "X MUST NOT import Y"
	if strings.Contains(lower, "must not import") || strings.Contains(lower, "shall not import") {
		// Try to extract module names
		re := regexp.MustCompile(`(?i)(\w+/).*(?:must not|shall not)\s+import.*?(\w+/)`)
		if matches := re.FindStringSubmatch(lower); len(matches) >= 3 {
			source := strings.TrimSuffix(matches[1], "/")
			forbidden := strings.TrimSuffix(matches[2], "/")
			return fmt.Sprintf("! grep -r 'import.*%s' %s/", forbidden, source)
		}
	}

	// Pattern: "X SHALL have zero dependencies"
	if strings.Contains(lower, "zero dependencies") || strings.Contains(lower, "no dependencies") {
		re := regexp.MustCompile(`(?i)(\w+/).*(?:zero|no)\s+(?:external\s+)?dependencies`)
		if matches := re.FindStringSubmatch(lower); len(matches) >= 2 {
			module := strings.TrimSuffix(matches[1], "/")
			return fmt.Sprintf("[ ! -f %s/package.json ] && [ ! -f %s/Cargo.toml ] || grep -q '\"dependencies\".*{}' %s/package.json",
				module, module, module)
		}
	}

	// Cannot infer - requires manual verification
	return ""
}

// checkBalance verifies assertions against a project codebase
func checkBalance(projectPath string, assertions []Assertion) ProjectReport {
	report := ProjectReport{
		ProjectPath: projectPath,
		Balanced:    make([]Assertion, 0),
		Unbalanced:  make([]Assertion, 0),
		Unknown:     make([]Assertion, 0),
	}

	for _, assertion := range assertions {
		result := assertion // Copy

		if assertion.VerifyCmd == "" {
			// No verification method - mark unknown
			result.Status = StatusUnknown
			report.Unknown = append(report.Unknown, result)
			continue
		}

		// Execute verification command in project directory
		success, violations := executeVerification(projectPath, assertion.VerifyCmd)

		if success {
			result.Status = StatusBalanced
			report.Balanced = append(report.Balanced, result)
		} else {
			result.Status = StatusUnbalanced
			result.Violations = violations
			report.Unbalanced = append(report.Unbalanced, result)
		}
	}

	// Calculate balance score
	verifiable := len(report.Balanced) + len(report.Unbalanced)
	if verifiable > 0 {
		report.Score = float64(len(report.Balanced)) / float64(verifiable) * 100.0
	}

	return report
}

// executeVerification runs a verification command and returns success status + violations
func executeVerification(projectPath, cmdString string) (bool, []string) {
	// Parse command string
	parts := strings.Fields(cmdString)
	if len(parts) == 0 {
		return false, []string{"invalid command"}
	}

	// Handle negation (commands starting with !)
	expectFailure := false
	if parts[0] == "!" {
		expectFailure = true
		parts = parts[1:]
	}

	if len(parts) == 0 {
		return false, []string{"invalid command after !"}
	}

	// Create command
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = projectPath

	// Execute
	output, err := cmd.CombinedOutput()

	// Determine success based on expectation
	success := (err == nil) != expectFailure

	// Parse violations from output
	violations := parseViolations(string(output))

	return success, violations
}

// parseViolations extracts file:line references from grep/command output
func parseViolations(output string) []string {
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	violations := make([]string, 0)

	// Pattern: filepath:linenumber:content or just filepath:content
	filePattern := regexp.MustCompile(`^([^:]+):(\d+)?:?(.*)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if matches := filePattern.FindStringSubmatch(line); matches != nil {
			violations = append(violations, line)
		}
	}

	if len(violations) == 0 && output != "" {
		// No structured output, return raw
		violations = []string{strings.TrimSpace(output)}
	}

	return violations
}

// displayBalanceReport prints a balance report with color-coded output
func displayBalanceReport(report ProjectReport) {
	// Header
	output.Header(fmt.Sprintf("Balance Report: %s", filepath.Base(report.ProjectPath)))
	fmt.Println("")

	// Balanced assertions
	if len(report.Balanced) > 0 {
		fmt.Printf("%sBALANCED%s (%d assertions)\n", output.Green, output.Reset, len(report.Balanced))

		// Show first 5, summarize rest
		displayLimit := 5
		for i, assertion := range report.Balanced {
			if i >= displayLimit {
				remaining := len(report.Balanced) - displayLimit
				fmt.Printf("  ... and %d more\n", remaining)
				break
			}

			desc := truncateDescription(assertion.Description, 80)
			fmt.Printf("  %s✓%s %s\n", output.Green, output.Reset, desc)
		}
		fmt.Println("")
	}

	// Unbalanced assertions
	if len(report.Unbalanced) > 0 {
		fmt.Printf("\033[31mUNBALANCED\033[0m (%d assertions)\n", len(report.Unbalanced))

		for _, assertion := range report.Unbalanced {
			desc := truncateDescription(assertion.Description, 80)
			fmt.Printf("  \033[31m✗\033[0m %s\n", desc)

			// Show violations
			if len(assertion.Violations) > 0 {
				violationLimit := 3
				for i, violation := range assertion.Violations {
					if i >= violationLimit {
						remaining := len(assertion.Violations) - violationLimit
						fmt.Printf("      ... and %d more violations\n", remaining)
						break
					}
					fmt.Printf("      Violation: %s\n", violation)
				}
			}
		}
		fmt.Println("")
	}

	// Unknown assertions
	if len(report.Unknown) > 0 {
		fmt.Printf("%sUNKNOWN%s (%d assertions)\n", output.Yellow, output.Reset, len(report.Unknown))

		// Show first 3, summarize rest
		displayLimit := 3
		for i, assertion := range report.Unknown {
			if i >= displayLimit {
				remaining := len(report.Unknown) - displayLimit
				fmt.Printf("  ... and %d more (manual review required)\n", remaining)
				break
			}

			desc := truncateDescription(assertion.Description, 80)
			fmt.Printf("  %s?%s %s\n", output.Yellow, output.Reset, desc)
		}
		fmt.Println("")
	}

	// Balance score
	verifiable := len(report.Balanced) + len(report.Unbalanced)

	if verifiable > 0 {
		scoreColor := output.Green
		if report.Score < 70 {
			scoreColor = "\033[31m" // Red
		} else if report.Score < 90 {
			scoreColor = output.Yellow
		}

		fmt.Printf("Balance Score: %s%.1f%%%s (%d/%d verifiable assertions hold)\n",
			scoreColor, report.Score, output.Reset,
			len(report.Balanced), verifiable)
	} else {
		fmt.Println("No verifiable assertions found.")
	}
}

// truncateDescription shortens description to fit display
func truncateDescription(desc string, maxLen int) string {
	// Remove markdown syntax and extra whitespace
	desc = strings.TrimSpace(desc)
	desc = regexp.MustCompile(`\[verify:.*?\]`).ReplaceAllString(desc, "")
	desc = regexp.MustCompile(`\s+`).ReplaceAllString(desc, " ")
	desc = strings.TrimSpace(desc)

	if len(desc) <= maxLen {
		return desc
	}

	return desc[:maxLen-3] + "..."
}
