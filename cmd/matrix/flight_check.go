package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// DeploymentStatus represents the current deployment state
type DeploymentStatus string

const (
	StatusReady    DeploymentStatus = "ready"
	StatusInFlight DeploymentStatus = "in-flight"
	StatusGrounded DeploymentStatus = "grounded"
	StatusShipped  DeploymentStatus = "shipped"
)

// DeploymentItem represents a deployment artifact with its status
type DeploymentItem struct {
	Name       string           // Project name
	Status     DeploymentStatus // Current status
	Identity   string           // Owner identity
	FilePath   string           // Path to deployment file
	BuiltDate  time.Time        // When it was built
	TestStatus string           // passing, failing, pending, n/a
	CIStatus   string           // passing, failing, pending, n/a
	Blocker    string           // Blocker description if grounded
	NeedsWho   string           // Which identity is needed to unblock
	ShippedDate time.Time       // When it was deployed
}

// FlightCheckReport contains all deployment items grouped by status
type FlightCheckReport struct {
	Ready    []DeploymentItem
	InFlight []DeploymentItem
	Grounded []DeploymentItem
	Shipped  []DeploymentItem
}

// runFlightCheck implements the flight-check command
func runFlightCheck() error {
	// Parse flags
	fs := flag.NewFlagSet("flight-check", flag.ExitOnError)
	readyFlag := fs.Bool("ready", false, "Show only ready-to-ship items")
	groundedFlag := fs.Bool("grounded", false, "Show only grounded items")
	historyFlag := fs.Bool("history", false, "Show only shipped items")
	jsonFlag := fs.Bool("json", false, "Output as JSON")

	// Parse remaining args (after "flight-check")
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		if *jsonFlag {
			emptyReport := FlightCheckReport{}
			outputFlightJSON(emptyReport)
			return nil
		}
		fmt.Println("🚀 No RAM directory found - no deployments tracked yet")
		return nil
	}

	// Scan RAM directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		if *jsonFlag {
			emptyReport := FlightCheckReport{}
			outputFlightJSON(emptyReport)
			return nil
		}
		fmt.Println("🚀 Garden exists but no deployment artifacts found yet")
		return nil
	}

	// Parse deployment items
	items := parseDeploymentItems(files)

	// Group by status
	report := groupByStatus(items)

	// Apply filters
	if *readyFlag {
		report = FlightCheckReport{Ready: report.Ready}
	} else if *groundedFlag {
		report = FlightCheckReport{Grounded: report.Grounded}
	} else if *historyFlag {
		report = FlightCheckReport{Shipped: report.Shipped}
	}

	// Output
	if *jsonFlag {
		outputFlightJSON(report)
	} else {
		displayFlightReport(report)
	}

	return nil
}

// parseDeploymentItems scans files for deployment artifacts
func parseDeploymentItems(files []ram.File) []DeploymentItem {
	var items []DeploymentItem

	for _, file := range files {
		// Check if file matches deployment patterns
		if !isDeploymentFile(file) {
			continue
		}

		item := extractDeploymentData(file)
		if item.Name != "" {
			items = append(items, item)
		}
	}

	return items
}

// isDeploymentFile checks if a file is a deployment artifact
func isDeploymentFile(file ram.File) bool {
	nameLower := strings.ToLower(file.Name)

	// Check filename patterns
	if strings.Contains(nameLower, "deployment") ||
		strings.Contains(nameLower, "deploy") ||
		strings.Contains(nameLower, "ship") {
		return true
	}

	// Check content patterns
	contentLower := strings.ToLower(file.Content)
	deploymentKeywords := []string{
		"deployment status",
		"ship checklist",
		"ready to ship",
		"deployment complete",
		"ci:",
		"tests:",
		"blocker:",
	}

	for _, keyword := range deploymentKeywords {
		if strings.Contains(contentLower, keyword) {
			return true
		}
	}

	return false
}

// extractDeploymentData parses deployment information from a file
func extractDeploymentData(file ram.File) DeploymentItem {
	item := DeploymentItem{
		Name:       inferProjectName(file),
		Identity:   file.Identity,
		FilePath:   file.Path,
		TestStatus: "n/a",
		CIStatus:   "n/a",
	}

	lines := strings.Split(file.Content, "\n")
	contentLower := strings.ToLower(file.Content)

	// Parse frontmatter if present
	if parseFrontmatter(&item, lines) {
		// Frontmatter takes precedence
	}

	// Parse content markers
	parseContentMarkers(&item, lines, contentLower)

	// Determine status
	item.Status = determineStatus(item)

	return item
}

// inferProjectName extracts project name from filename or content
func inferProjectName(file ram.File) string {
	name := file.Name

	// Remove common suffixes
	suffixes := []string{"-deployment", "-deploy", "-ship", "-implementation", "-status"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	// If still empty or generic, try to find project name in content
	if name == "" || name == "deployment" || name == "status" {
		// Look for "Project:" or "## Project" in first 10 lines
		lines := strings.Split(file.Content, "\n")
		limit := min(10, len(lines))
		for i := 0; i < limit; i++ {
			line := strings.TrimSpace(lines[i])
			if strings.HasPrefix(strings.ToLower(line), "project:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	return name
}

// parseFrontmatter extracts YAML frontmatter if present
func parseFrontmatter(item *DeploymentItem, lines []string) bool {
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}

	// Find closing ---
	endIdx := -1
	for i := 1; i < len(lines) && i < 50; i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return false
	}

	// Parse frontmatter fields
	for i := 1; i < endIdx; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)

		switch key {
		case "project":
			if value != "" {
				item.Name = value
			}
		case "status":
			// Already handled by determineStatus
		case "owner":
			if value != "" {
				item.Identity = value
			}
		case "built":
			if t := parseTimestamp(value); !t.IsZero() {
				item.BuiltDate = t
			}
		case "tests":
			item.TestStatus = normalizeTestStatus(value)
		case "ci":
			item.CIStatus = normalizeCIStatus(value)
		case "blocker":
			item.Blocker = value
		case "needs":
			item.NeedsWho = value
		case "deployed":
			if t := parseTimestamp(value); !t.IsZero() {
				item.ShippedDate = t
			}
		}
	}

	return true
}

// parseContentMarkers scans content for deployment status markers
func parseContentMarkers(item *DeploymentItem, lines []string, contentLower string) {
	// Test status patterns
	testPatterns := map[string]string{
		`tests?\s*(?:passing|passed|green|✓)`:    "passing",
		`tests?\s*(?:failing|failed|red|✗)`:      "failing",
		`tests?\s*(?:running|pending|in.?progress)`: "pending",
		`all\s+tests\s+(?:pass|green)`:           "passing",
		`\d+\s+tests?\s+failed`:                  "failing",
	}

	for pattern, status := range testPatterns {
		if matched, _ := regexp.MatchString(pattern, contentLower); matched {
			item.TestStatus = status
			break
		}
	}

	// CI status patterns
	ciPatterns := map[string]string{
		`ci\s*:?\s*(?:passing|passed|green|✓)`:    "passing",
		`ci\s*:?\s*(?:failing|failed|red|✗)`:      "failing",
		`ci\s*:?\s*(?:pending|running)`:           "pending",
		`pipeline\s+(?:green|passing)`:            "passing",
		`pipeline\s+(?:failed|failing)`:           "failing",
		`github\s+actions\s*:?\s*✓`:               "passing",
		`checks\s*:?\s*✗`:                         "failing",
	}

	for pattern, status := range ciPatterns {
		if matched, _ := regexp.MatchString(pattern, contentLower); matched {
			item.CIStatus = status
			break
		}
	}

	// Build date patterns
	buildPattern := regexp.MustCompile(`(?i)built?\s*:?\s*(.+)`)
	for _, line := range lines {
		if match := buildPattern.FindStringSubmatch(line); match != nil {
			if t := parseTimestamp(match[1]); !t.IsZero() {
				item.BuiltDate = t
				break
			}
		}
	}

	// Blocker patterns
	blockerPattern := regexp.MustCompile(`(?i)(?:blocker|blocked\s+by|waiting\s+for)\s*:?\s*(.+)`)
	for _, line := range lines {
		if match := blockerPattern.FindStringSubmatch(line); match != nil {
			item.Blocker = strings.TrimSpace(match[1])
			break
		}
	}

	// Needs patterns
	needsPattern := regexp.MustCompile(`(?i)needs?\s*:?\s*(\w+)`)
	for _, line := range lines {
		if match := needsPattern.FindStringSubmatch(line); match != nil {
			item.NeedsWho = strings.ToLower(strings.TrimSpace(match[1]))
			break
		}
	}

	// Shipped/Deployed patterns
	shippedPattern := regexp.MustCompile(`(?i)(?:deployed|shipped)(?:\s+(?:on|to|at))?\s*:?\s*(.+?)(?:\n|$)`)
	if match := shippedPattern.FindStringSubmatch(contentLower); match != nil {
		if t := parseTimestamp(match[1]); !t.IsZero() {
			item.ShippedDate = t
		}
	}

	// Merged pattern (PR merged indicates shipped)
	mergedPattern := regexp.MustCompile(`(?i)merged?\s*:?\s*(.+?)(?:\n|$)`)
	if match := mergedPattern.FindStringSubmatch(contentLower); match != nil {
		if t := parseTimestamp(match[1]); !t.IsZero() {
			item.ShippedDate = t
		}
	}

	// Check for deployment complete keywords
	deploymentCompleteKeywords := []string{
		"deployment complete",
		"rollout finished",
		"live as of",
		"deployed - pr",
		"status: merged",
		"merge method:",
		"pr merged",
		"deployment status: ✅",
		"deployment status**: ✅",
	}

	for _, keyword := range deploymentCompleteKeywords {
		if strings.Contains(contentLower, keyword) {
			// Mark as shipped if not already dated
			if item.ShippedDate.IsZero() {
				item.ShippedDate = time.Now()
			}
			break
		}
	}
}

// determineStatus infers deployment status from available data
func determineStatus(item DeploymentItem) DeploymentStatus {
	// Shipped takes highest priority
	if !item.ShippedDate.IsZero() {
		return StatusShipped
	}

	// Grounded if blocker present or tests/CI failing
	if item.Blocker != "" ||
		item.TestStatus == "failing" ||
		item.CIStatus == "failing" {
		return StatusGrounded
	}

	// Ready if tests and CI passing
	if item.TestStatus == "passing" && item.CIStatus == "passing" {
		return StatusReady
	}

	// In-flight if tests or CI pending/running
	if item.TestStatus == "pending" || item.CIStatus == "pending" {
		return StatusInFlight
	}

	// Default to in-flight if we have build date but unclear status
	if !item.BuiltDate.IsZero() {
		return StatusInFlight
	}

	// Otherwise grounded (needs attention)
	return StatusGrounded
}

// normalizeTestStatus converts various test status strings
func normalizeTestStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "passing", "passed", "green", "✓", "ok":
		return "passing"
	case "failing", "failed", "red", "✗", "error":
		return "failing"
	case "pending", "running", "in progress":
		return "pending"
	default:
		return "n/a"
	}
}

// normalizeCIStatus converts various CI status strings
func normalizeCIStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "passing", "passed", "green", "✓", "success":
		return "passing"
	case "failing", "failed", "red", "✗", "error":
		return "failing"
	case "pending", "running", "in progress":
		return "pending"
	default:
		return "n/a"
	}
}

// groupByStatus separates items by their deployment status
func groupByStatus(items []DeploymentItem) FlightCheckReport {
	report := FlightCheckReport{}

	for _, item := range items {
		switch item.Status {
		case StatusReady:
			report.Ready = append(report.Ready, item)
		case StatusInFlight:
			report.InFlight = append(report.InFlight, item)
		case StatusGrounded:
			report.Grounded = append(report.Grounded, item)
		case StatusShipped:
			report.Shipped = append(report.Shipped, item)
		}
	}

	// Sort each group
	sort.Slice(report.Ready, func(i, j int) bool {
		return report.Ready[i].Name < report.Ready[j].Name
	})
	sort.Slice(report.InFlight, func(i, j int) bool {
		return report.InFlight[i].Name < report.InFlight[j].Name
	})
	sort.Slice(report.Grounded, func(i, j int) bool {
		return report.Grounded[i].Name < report.Grounded[j].Name
	})
	sort.Slice(report.Shipped, func(i, j int) bool {
		return report.Shipped[j].ShippedDate.Before(report.Shipped[i].ShippedDate)
	})

	return report
}

// displayFlightReport outputs the flight check report to stdout
func displayFlightReport(report FlightCheckReport) {
	output.Success("🚀 Flight Check - " + time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("")

	// Ready to ship
	if len(report.Ready) > 0 {
		fmt.Println(strings.Repeat("━", 70))
		output.Header(fmt.Sprintf("  READY TO SHIP (%d)", len(report.Ready)))
		fmt.Println(strings.Repeat("━", 70))
		fmt.Println("")

		for _, item := range report.Ready {
			fmt.Printf("  ✓ %s\n", output.Green+item.Name+output.Reset)

			// Build status line
			statusParts := []string{}
			if !item.BuiltDate.IsZero() {
				statusParts = append(statusParts, fmt.Sprintf("Built: %s", formatDate(item.BuiltDate)))
			}
			if item.TestStatus != "n/a" {
				statusParts = append(statusParts, fmt.Sprintf("Tests: %s", formatStatusSymbol(item.TestStatus)))
			}
			if item.CIStatus != "n/a" {
				statusParts = append(statusParts, fmt.Sprintf("CI: %s", formatStatusSymbol(item.CIStatus)))
			}

			if len(statusParts) > 0 {
				fmt.Printf("    %s\n", strings.Join(statusParts, " | "))
			}
			fmt.Printf("    Owner: %s\n", output.Yellow+item.Identity+output.Reset)
			fmt.Println("")
		}
	}

	// In flight
	if len(report.InFlight) > 0 {
		fmt.Println(strings.Repeat("━", 70))
		output.Header(fmt.Sprintf("  IN FLIGHT (%d)", len(report.InFlight)))
		fmt.Println(strings.Repeat("━", 70))
		fmt.Println("")

		for _, item := range report.InFlight {
			fmt.Printf("  ⟳ %s\n", output.Yellow+item.Name+output.Reset)

			statusParts := []string{}
			if !item.BuiltDate.IsZero() {
				statusParts = append(statusParts, fmt.Sprintf("Built: %s", formatDate(item.BuiltDate)))
			}
			if item.TestStatus != "n/a" {
				statusParts = append(statusParts, fmt.Sprintf("Tests: %s", item.TestStatus))
			}
			if item.CIStatus != "n/a" {
				statusParts = append(statusParts, fmt.Sprintf("CI: %s", item.CIStatus))
			}

			if len(statusParts) > 0 {
				fmt.Printf("    %s\n", strings.Join(statusParts, " | "))
			}
			fmt.Printf("    Owner: %s\n", output.Yellow+item.Identity+output.Reset)
			fmt.Println("")
		}
	}

	// Grounded
	if len(report.Grounded) > 0 {
		fmt.Println(strings.Repeat("━", 70))
		output.Header(fmt.Sprintf("  GROUNDED (%d)", len(report.Grounded)))
		fmt.Println(strings.Repeat("━", 70))
		fmt.Println("")

		for _, item := range report.Grounded {
			symbol := "✗"
			if item.Blocker == "" && item.TestStatus != "failing" && item.CIStatus != "failing" {
				symbol = "⚠"
			}

			fmt.Printf("  %s %s\n", symbol, item.Name)

			statusParts := []string{}
			if !item.BuiltDate.IsZero() {
				statusParts = append(statusParts, fmt.Sprintf("Built: %s", formatDate(item.BuiltDate)))
			} else {
				statusParts = append(statusParts, "Built: never")
			}
			if item.TestStatus != "n/a" {
				statusParts = append(statusParts, fmt.Sprintf("Tests: %s", formatStatusSymbol(item.TestStatus)))
			}
			if item.CIStatus != "n/a" {
				statusParts = append(statusParts, fmt.Sprintf("CI: %s", formatStatusSymbol(item.CIStatus)))
			}

			if len(statusParts) > 0 {
				fmt.Printf("    %s\n", strings.Join(statusParts, " | "))
			}
			fmt.Printf("    Owner: %s\n", output.Yellow+item.Identity+output.Reset)

			if item.Blocker != "" {
				fmt.Printf("    Blocker: %s\n", item.Blocker)
			}
			if item.NeedsWho != "" {
				fmt.Printf("    Needs: %s\n", item.NeedsWho)
			}
			fmt.Println("")
		}
	}

	// Shipped
	if len(report.Shipped) > 0 {
		fmt.Println(strings.Repeat("━", 70))
		output.Header(fmt.Sprintf("  SHIPPED (%d)", len(report.Shipped)))
		fmt.Println(strings.Repeat("━", 70))
		fmt.Println("")

		for _, item := range report.Shipped {
			deployedStr := ""
			if !item.ShippedDate.IsZero() {
				deployedStr = fmt.Sprintf(" (deployed %s)", formatDate(item.ShippedDate))
			}
			fmt.Printf("  ✓ %s%s\n", item.Name, deployedStr)
		}
		fmt.Println("")
	}

	fmt.Println(strings.Repeat("━", 70))
	fmt.Println("")

	// Summary
	if len(report.Ready) > 0 {
		output.Success(fmt.Sprintf("Flight path clear. %d ready to ship on your mark.", len(report.Ready)))
	} else if len(report.Grounded) > 0 {
		fmt.Printf("Ground control: %d items need attention.\n", len(report.Grounded))
	} else if len(report.InFlight) > 0 {
		fmt.Printf("%d items in flight.\n", len(report.InFlight))
	} else {
		fmt.Println("No active deployments.")
	}
}

// outputFlightJSON outputs the report as JSON
func outputFlightJSON(report FlightCheckReport) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(report)
}

// formatDate formats a date for display
func formatDate(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// formatStatusSymbol converts status to symbol
func formatStatusSymbol(status string) string {
	switch status {
	case "passing":
		return "✓"
	case "failing":
		return "✗"
	case "pending":
		return "⟳"
	default:
		return "n/a"
	}
}
