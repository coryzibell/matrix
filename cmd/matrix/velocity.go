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

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// TaskMetadata represents parsed task information from RAM files
type TaskMetadata struct {
	Identity   string
	FilePath   string
	Status     string    // success, failure, partial
	Started    time.Time // Zero if not found
	Completed  time.Time // Zero if not found
	Duration   time.Duration
	HandoffTo  string // Identity handed off to
	LineNumber int
}

// VelocityStats tracks performance metrics for an identity
type VelocityStats struct {
	Identity       string
	TotalTasks     int
	SuccessCount   int
	FailureCount   int
	PartialCount   int
	SuccessRate    float64
	AvgDuration    time.Duration
	HandoffsGiven  int
	MostHandoffTo  string
}

// HandoffPair tracks handoff patterns between identities
type HandoffPair struct {
	From    string
	To      string
	Count   int
	Success int
	Failure int
}

// VelocityReport contains the full analysis
type VelocityReport struct {
	Stats           []VelocityStats
	Handoffs        []HandoffPair
	TotalTasks      int
	FileCount       int
	AnalysisPeriod  string
	HighPerformers  []VelocityStats
	Bottlenecks     []VelocityStats
}

// runVelocity implements the velocity command
func runVelocity() error {
	// Parse flags
	fs := flag.NewFlagSet("velocity", flag.ExitOnError)
	identityFlag := fs.String("identity", "", "Filter by specific identity")
	daysFlag := fs.Int("days", 0, "Only analyze last N days (0 = all time)")
	jsonFlag := fs.Bool("json", false, "Output as JSON")

	// Parse remaining args (after "velocity")
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	// Validate identity flag
	if *identityFlag != "" && !identity.IsValid(*identityFlag) {
		return fmt.Errorf("invalid identity: %s", *identityFlag)
	}

	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		if *jsonFlag {
			emptyReport := VelocityReport{}
			outputJSON(emptyReport)
			return nil
		}
		fmt.Println("🌾 No garden found at ~/.claude/ram/ - no velocity data yet")
		return nil
	}

	// Scan RAM directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		if *jsonFlag {
			emptyReport := VelocityReport{}
			outputJSON(emptyReport)
			return nil
		}
		fmt.Println("🌾 Garden exists but no markdown files found yet")
		return nil
	}

	// Filter by identity if specified
	if *identityFlag != "" {
		filtered := make([]ram.File, 0)
		for _, f := range files {
			if f.Identity == *identityFlag {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	// Parse tasks from files
	tasks := parseTaskMetadata(files)

	// Filter by days if specified
	if *daysFlag > 0 {
		cutoff := time.Now().AddDate(0, 0, -*daysFlag)
		filtered := make([]TaskMetadata, 0)
		for _, task := range tasks {
			if !task.Completed.IsZero() && task.Completed.After(cutoff) {
				filtered = append(filtered, task)
			} else if !task.Started.IsZero() && task.Started.After(cutoff) {
				filtered = append(filtered, task)
			}
		}
		tasks = filtered
	}

	// Generate report
	report := generateReport(tasks, files)

	if *daysFlag > 0 {
		report.AnalysisPeriod = fmt.Sprintf("Last %d days", *daysFlag)
	} else {
		report.AnalysisPeriod = "All time"
	}

	// Output
	if *jsonFlag {
		outputJSON(report)
	} else {
		displayReport(report)
	}

	return nil
}

// parseTaskMetadata extracts task data from RAM files
func parseTaskMetadata(files []ram.File) []TaskMetadata {
	var tasks []TaskMetadata

	// Regex patterns
	statusPattern := regexp.MustCompile(`(?i)\b(status|state):\s*(success|failure|partial|failed|succeeded|completed)`)
	handoffPattern := regexp.MustCompile(`(?i)\bhandoff(?:\s+to)?:\s*(\w+)`)

	for _, file := range files {
		lines := strings.Split(file.Content, "\n")

		for lineNum, line := range lines {
			// Check for status lines
			if statusMatch := statusPattern.FindStringSubmatch(line); statusMatch != nil {
				task := TaskMetadata{
					Identity:   file.Identity,
					FilePath:   file.Path,
					Status:     normalizeStatus(statusMatch[2]),
					LineNumber: lineNum + 1,
				}

				// Look for timestamps in surrounding lines (context window)
				task.Started, task.Completed = extractTimestamps(lines, lineNum)
				if !task.Started.IsZero() && !task.Completed.IsZero() {
					task.Duration = task.Completed.Sub(task.Started)
				}

				// Look for handoffs in surrounding lines
				for i := max(0, lineNum-3); i < min(len(lines), lineNum+3); i++ {
					if handoffMatch := handoffPattern.FindStringSubmatch(lines[i]); handoffMatch != nil {
						task.HandoffTo = strings.ToLower(handoffMatch[1])
						break
					}
				}

				tasks = append(tasks, task)
			}
		}
	}

	return tasks
}

// extractTimestamps looks for timestamp patterns near a status line
func extractTimestamps(lines []string, centerLine int) (started, completed time.Time) {
	// Search context window around status line
	start := max(0, centerLine-5)
	end := min(len(lines), centerLine+5)

	startPattern := regexp.MustCompile(`(?i)(?:started|start|began):\s*(.+)`)
	completePattern := regexp.MustCompile(`(?i)(?:completed|finished|done|end):\s*(.+)`)

	for i := start; i < end; i++ {
		line := lines[i]

		if startMatch := startPattern.FindStringSubmatch(line); startMatch != nil {
			if t := parseTimestamp(startMatch[1]); !t.IsZero() {
				started = t
			}
		}

		if completeMatch := completePattern.FindStringSubmatch(line); completeMatch != nil {
			if t := parseTimestamp(completeMatch[1]); !t.IsZero() {
				completed = t
			}
		}
	}

	return
}

// parseTimestamp attempts to parse various timestamp formats
func parseTimestamp(s string) time.Time {
	s = strings.TrimSpace(s)

	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"Jan 2 15:04:05 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	return time.Time{}
}

// normalizeStatus converts various status strings to canonical form
func normalizeStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "success", "succeeded", "completed":
		return "success"
	case "failure", "failed":
		return "failure"
	case "partial":
		return "partial"
	default:
		return s
	}
}

// generateReport computes velocity statistics
func generateReport(tasks []TaskMetadata, files []ram.File) VelocityReport {
	// Build stats per identity
	identityStats := make(map[string]*VelocityStats)
	handoffCounts := make(map[string]map[string]int) // from -> to -> count
	handoffSuccess := make(map[string]map[string]int)

	for _, task := range tasks {
		// Initialize stats if needed
		if _, exists := identityStats[task.Identity]; !exists {
			identityStats[task.Identity] = &VelocityStats{
				Identity: task.Identity,
			}
		}

		stats := identityStats[task.Identity]
		stats.TotalTasks++

		// Count by status
		switch task.Status {
		case "success":
			stats.SuccessCount++
		case "failure":
			stats.FailureCount++
		case "partial":
			stats.PartialCount++
		}

		// Track duration
		if task.Duration > 0 {
			stats.AvgDuration = (stats.AvgDuration*time.Duration(stats.TotalTasks-1) + task.Duration) / time.Duration(stats.TotalTasks)
		}

		// Track handoffs
		if task.HandoffTo != "" && identity.IsValid(task.HandoffTo) {
			stats.HandoffsGiven++

			if handoffCounts[task.Identity] == nil {
				handoffCounts[task.Identity] = make(map[string]int)
				handoffSuccess[task.Identity] = make(map[string]int)
			}
			handoffCounts[task.Identity][task.HandoffTo]++
			if task.Status == "success" {
				handoffSuccess[task.Identity][task.HandoffTo]++
			}
		}
	}

	// Calculate success rates and most common handoff
	for _, stats := range identityStats {
		if stats.TotalTasks > 0 {
			stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalTasks) * 100
		}

		// Find most common handoff target
		if counts := handoffCounts[stats.Identity]; counts != nil {
			maxCount := 0
			for target, count := range counts {
				if count > maxCount {
					maxCount = count
					stats.MostHandoffTo = target
				}
			}
		}
	}

	// Convert to sorted slice
	statsList := make([]VelocityStats, 0, len(identityStats))
	for _, stats := range identityStats {
		statsList = append(statsList, *stats)
	}
	sort.Slice(statsList, func(i, j int) bool {
		return statsList[i].TotalTasks > statsList[j].TotalTasks
	})

	// Build handoff pairs
	var handoffPairs []HandoffPair
	for from, targets := range handoffCounts {
		for to, count := range targets {
			pair := HandoffPair{
				From:    from,
				To:      to,
				Count:   count,
				Success: handoffSuccess[from][to],
				Failure: count - handoffSuccess[from][to],
			}
			handoffPairs = append(handoffPairs, pair)
		}
	}
	sort.Slice(handoffPairs, func(i, j int) bool {
		return handoffPairs[i].Count > handoffPairs[j].Count
	})

	// Identify high performers (top 3 by success rate with >3 tasks)
	highPerformers := make([]VelocityStats, 0)
	for _, stats := range statsList {
		if stats.TotalTasks >= 3 {
			highPerformers = append(highPerformers, stats)
		}
	}
	sort.Slice(highPerformers, func(i, j int) bool {
		return highPerformers[i].SuccessRate > highPerformers[j].SuccessRate
	})
	if len(highPerformers) > 3 {
		highPerformers = highPerformers[:3]
	}

	// Identify bottlenecks (high failure rate with >2 tasks)
	bottlenecks := make([]VelocityStats, 0)
	for _, stats := range statsList {
		if stats.TotalTasks >= 2 && stats.FailureCount > 0 {
			bottlenecks = append(bottlenecks, stats)
		}
	}
	sort.Slice(bottlenecks, func(i, j int) bool {
		return bottlenecks[i].FailureCount > bottlenecks[j].FailureCount
	})
	if len(bottlenecks) > 3 {
		bottlenecks = bottlenecks[:3]
	}

	return VelocityReport{
		Stats:          statsList,
		Handoffs:       handoffPairs,
		TotalTasks:     len(tasks),
		FileCount:      len(files),
		HighPerformers: highPerformers,
		Bottlenecks:    bottlenecks,
	}
}

// displayReport outputs the velocity report to stdout
func displayReport(report VelocityReport) {
	output.Success("⚡ Task Velocity Report")
	fmt.Println("")
	fmt.Printf("Analysis Period: %s\n", report.AnalysisPeriod)
	fmt.Printf("Total Tasks: %d\n", report.TotalTasks)
	fmt.Printf("Files Scanned: %d markdown files\n", report.FileCount)
	fmt.Println("")

	// High Performers
	if len(report.HighPerformers) > 0 {
		output.Header("High Performers:")
		fmt.Println("")
		for _, stats := range report.HighPerformers {
			fmt.Printf("  %s - %d tasks, %.0f%% success",
				output.Yellow+stats.Identity+output.Reset,
				stats.TotalTasks,
				stats.SuccessRate)
			if stats.AvgDuration > 0 {
				fmt.Printf(", avg %s", formatDuration(stats.AvgDuration))
			}
			fmt.Println("")
		}
		fmt.Println("")
	}

	// All Identity Stats
	if len(report.Stats) > 0 {
		output.Header("Identity Velocity:")
		fmt.Println("")
		for _, stats := range report.Stats {
			fmt.Printf("  %s\n", output.Yellow+stats.Identity+output.Reset)
			fmt.Printf("    Tasks: %d (S:%d F:%d P:%d)\n",
				stats.TotalTasks,
				stats.SuccessCount,
				stats.FailureCount,
				stats.PartialCount)
			fmt.Printf("    Success Rate: %.1f%%\n", stats.SuccessRate)
			if stats.AvgDuration > 0 {
				fmt.Printf("    Avg Duration: %s\n", formatDuration(stats.AvgDuration))
			}
			if stats.MostHandoffTo != "" {
				fmt.Printf("    Most Handoffs To: %s (%d total)\n", stats.MostHandoffTo, stats.HandoffsGiven)
			}
			fmt.Println("")
		}
	}

	// Bottlenecks
	if len(report.Bottlenecks) > 0 {
		output.Header("Bottlenecks:")
		fmt.Println("")
		for _, stats := range report.Bottlenecks {
			fmt.Printf("  %s - %d failures in %d tasks (%.1f%% failure rate)\n",
				output.Yellow+stats.Identity+output.Reset,
				stats.FailureCount,
				stats.TotalTasks,
				float64(stats.FailureCount)/float64(stats.TotalTasks)*100)
		}
		fmt.Println("")
	}

	// Handoff Patterns
	if len(report.Handoffs) > 0 {
		output.Header("Top Handoff Patterns:")
		fmt.Println("")
		limit := min(5, len(report.Handoffs))
		for i := 0; i < limit; i++ {
			h := report.Handoffs[i]
			successRate := 0.0
			if h.Count > 0 {
				successRate = float64(h.Success) / float64(h.Count) * 100
			}
			fmt.Printf("  %s → %s (%d handoffs, %.0f%% success)\n",
				h.From, h.To, h.Count, successRate)
		}
		fmt.Println("")
	}

	output.Success("⚡ Analysis complete")
}

// outputJSON outputs the report as JSON
func outputJSON(report VelocityReport) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(report)
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
