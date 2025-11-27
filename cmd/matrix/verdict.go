package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
)

// VerdictEntry represents a single test result or benchmark
type VerdictEntry struct {
	ID        string    `json:"id"`        // unique identifier
	Type      string    `json:"type"`      // "test" or "benchmark"
	Identity  string    `json:"identity"`  // who ran it
	Component string    `json:"component"` // what was tested
	Test      string    `json:"test"`      // test name (for tests)
	Metric    string    `json:"metric"`    // metric name (for benchmarks)
	Result    string    `json:"result"`    // "pass" or "fail" (for tests)
	Value     float64   `json:"value"`     // metric value (for benchmarks)
	Duration  float64   `json:"duration"`  // duration in seconds (for tests)
	Timestamp time.Time `json:"timestamp"`
}

// VerdictBaseline represents a performance baseline
type VerdictBaseline struct {
	Component string  `json:"component"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	SetAt     time.Time `json:"set_at"`
	SetBy     string  `json:"set_by"`
}

// VerdictData is the full storage structure
type VerdictData struct {
	Entries   []VerdictEntry   `json:"entries"`
	Baselines []VerdictBaseline `json:"baselines"`
}

// VerdictSummary aggregates verdict data for reporting
type VerdictSummary struct {
	Component    string
	TotalTests   int
	PassCount    int
	FailCount    int
	SuccessRate  float64
	AvgDuration  float64
	LastRun      time.Time
	Trend        string // "↑", "↓", "→" (improving, declining, stable)
	ConsecutivePass int
}

// runVerdict implements the verdict command
func runVerdict() error {
	if len(os.Args) < 3 {
		printVerdictUsage()
		return nil
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "record":
		return runVerdictRecord()
	case "bench":
		return runVerdictBench()
	case "check":
		return runVerdictCheck()
	case "report":
		return runVerdictReport()
	case "baseline":
		return runVerdictBaseline()
	case "list":
		return runVerdictList()
	default:
		return fmt.Errorf("unknown verdict subcommand: %s", subcommand)
	}
}

// runVerdictRecord records a test result
func runVerdictRecord() error {
	fs := flag.NewFlagSet("verdict record", flag.ExitOnError)
	identityFlag := fs.String("identity", "", "Identity that ran the test")
	componentFlag := fs.String("component", "", "Component being tested")
	testFlag := fs.String("test", "", "Test name")
	resultFlag := fs.String("result", "", "Result: pass or fail")
	durationFlag := fs.Float64("duration", 0, "Test duration in seconds")

	// Parse remaining args (after "verdict record")
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	// Validate required flags
	if *identityFlag == "" || *componentFlag == "" || *testFlag == "" || *resultFlag == "" {
		return fmt.Errorf("required flags: --identity, --component, --test, --result")
	}

	if !identity.IsValid(*identityFlag) {
		return fmt.Errorf("invalid identity: %s", *identityFlag)
	}

	result := strings.ToLower(*resultFlag)
	if result != "pass" && result != "fail" {
		return fmt.Errorf("result must be 'pass' or 'fail', got: %s", *resultFlag)
	}

	// Load existing data
	data, err := loadVerdictData()
	if err != nil {
		return err
	}

	// Create entry
	entry := VerdictEntry{
		ID:        fmt.Sprintf("%s-%s-%d", *componentFlag, *testFlag, time.Now().Unix()),
		Type:      "test",
		Identity:  *identityFlag,
		Component: *componentFlag,
		Test:      *testFlag,
		Result:    result,
		Duration:  *durationFlag,
		Timestamp: time.Now(),
	}

	// Add to data
	data.Entries = append(data.Entries, entry)

	// Save
	if err := saveVerdictData(data); err != nil {
		return err
	}

	// Display result
	output.Success("⚖️ VERDICT RECORDED")
	fmt.Println("")
	fmt.Printf("Component: %s\n", entry.Component)
	fmt.Printf("Test: %s\n", entry.Test)
	fmt.Printf("Result: %s\n", strings.ToUpper(entry.Result))
	if entry.Duration > 0 {
		fmt.Printf("Duration: %.2fs\n", entry.Duration)
	}
	fmt.Printf("Identity: %s\n", entry.Identity)
	fmt.Printf("Timestamp: %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))

	return nil
}

// runVerdictBench records a benchmark result
func runVerdictBench() error {
	fs := flag.NewFlagSet("verdict bench", flag.ExitOnError)
	identityFlag := fs.String("identity", "", "Identity that ran the benchmark")
	componentFlag := fs.String("component", "", "Component being benchmarked")
	metricFlag := fs.String("metric", "", "Metric name")
	valueFlag := fs.Float64("value", 0, "Metric value")

	// Parse remaining args (after "verdict bench")
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	// Validate required flags
	if *identityFlag == "" || *componentFlag == "" || *metricFlag == "" {
		return fmt.Errorf("required flags: --identity, --component, --metric, --value")
	}

	if !identity.IsValid(*identityFlag) {
		return fmt.Errorf("invalid identity: %s", *identityFlag)
	}

	// Load existing data
	data, err := loadVerdictData()
	if err != nil {
		return err
	}

	// Create entry
	entry := VerdictEntry{
		ID:        fmt.Sprintf("%s-%s-%d", *componentFlag, *metricFlag, time.Now().Unix()),
		Type:      "benchmark",
		Identity:  *identityFlag,
		Component: *componentFlag,
		Metric:    *metricFlag,
		Value:     *valueFlag,
		Timestamp: time.Now(),
	}

	// Add to data
	data.Entries = append(data.Entries, entry)

	// Save
	if err := saveVerdictData(data); err != nil {
		return err
	}

	// Check against baseline
	baseline := findBaseline(data, *componentFlag, *metricFlag)

	output.Success("⚖️ BENCHMARK RECORDED")
	fmt.Println("")
	fmt.Printf("Component: %s\n", entry.Component)
	fmt.Printf("Metric: %s\n", entry.Metric)
	fmt.Printf("Value: %.2f\n", entry.Value)
	if baseline != nil {
		percentChange := ((entry.Value - baseline.Value) / baseline.Value) * 100
		fmt.Printf("Baseline: %.2f (%+.1f%%)\n", baseline.Value, percentChange)
	}
	fmt.Printf("Identity: %s\n", entry.Identity)
	fmt.Printf("Timestamp: %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))

	return nil
}

// runVerdictCheck checks for regressions
func runVerdictCheck() error {
	fs := flag.NewFlagSet("verdict check", flag.ExitOnError)
	componentFlag := fs.String("component", "", "Component to check")
	thresholdFlag := fs.Float64("threshold", 10.0, "Regression threshold percentage (default: 10%)")

	// Parse remaining args (after "verdict check")
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	if *componentFlag == "" {
		return fmt.Errorf("required flag: --component")
	}

	// Load existing data
	data, err := loadVerdictData()
	if err != nil {
		return err
	}

	// Get benchmarks for component
	var benchmarks []VerdictEntry
	for _, entry := range data.Entries {
		if entry.Type == "benchmark" && entry.Component == *componentFlag {
			benchmarks = append(benchmarks, entry)
		}
	}

	if len(benchmarks) == 0 {
		fmt.Printf("No benchmark data for component: %s\n", *componentFlag)
		return nil
	}

	// Check each metric
	regressions := make(map[string]struct {
		current  float64
		baseline float64
		percent  float64
	})

	for _, bench := range benchmarks {
		baseline := findBaseline(data, bench.Component, bench.Metric)
		if baseline != nil {
			percentChange := ((bench.Value - baseline.Value) / baseline.Value) * 100
			// Negative change is regression (assuming lower is better)
			if percentChange < -*thresholdFlag {
				regressions[bench.Metric] = struct {
					current  float64
					baseline float64
					percent  float64
				}{bench.Value, baseline.Value, percentChange}
			}
		}
	}

	if len(regressions) > 0 {
		output.Header("⚠️ REGRESSIONS DETECTED")
		fmt.Println("")
		fmt.Printf("Component: %s\n", *componentFlag)
		fmt.Printf("Threshold: %.1f%%\n", *thresholdFlag)
		fmt.Println("")
		for metric, data := range regressions {
			fmt.Printf("Metric: %s\n", output.Yellow+metric+output.Reset)
			fmt.Printf("  Current: %.2f\n", data.current)
			fmt.Printf("  Baseline: %.2f\n", data.baseline)
			fmt.Printf("  Change: %s%.1f%%%s\n", output.Red, data.percent, output.Reset)
			fmt.Println("")
		}
		return nil
	}

	output.Success("✓ No regressions detected")
	fmt.Printf("Component: %s (threshold: %.1f%%)\n", *componentFlag, *thresholdFlag)

	return nil
}

// runVerdictReport generates a verdict report
func runVerdictReport() error {
	fs := flag.NewFlagSet("verdict report", flag.ExitOnError)
	identityFlag := fs.String("identity", "", "Filter by identity")
	componentFlag := fs.String("component", "", "Filter by component")

	// Parse remaining args (after "verdict report")
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	// Validate identity flag
	if *identityFlag != "" && !identity.IsValid(*identityFlag) {
		return fmt.Errorf("invalid identity: %s", *identityFlag)
	}

	// Load existing data
	data, err := loadVerdictData()
	if err != nil {
		return err
	}

	if len(data.Entries) == 0 {
		fmt.Println("No verdict data recorded yet")
		return nil
	}

	// Filter entries
	var filtered []VerdictEntry
	for _, entry := range data.Entries {
		if *identityFlag != "" && entry.Identity != *identityFlag {
			continue
		}
		if *componentFlag != "" && entry.Component != *componentFlag {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Generate summaries per component
	summaries := generateSummaries(filtered)

	// Display report
	output.Success("⚖️ VERDICT REPORT")
	fmt.Println("")
	fmt.Printf("Total Entries: %d\n", len(filtered))
	fmt.Println("")

	for _, summary := range summaries {
		fmt.Printf("Component: %s\n", output.Yellow+summary.Component+output.Reset)
		fmt.Printf("  Tests: %d (Pass: %d, Fail: %d)\n", summary.TotalTests, summary.PassCount, summary.FailCount)
		fmt.Printf("  Success Rate: %.1f%%\n", summary.SuccessRate)
		if summary.AvgDuration > 0 {
			fmt.Printf("  Avg Duration: %.2fs\n", summary.AvgDuration)
		}
		if !summary.LastRun.IsZero() {
			fmt.Printf("  Last Run: %s\n", summary.LastRun.Format("2006-01-02 15:04:05"))
		}
		if summary.ConsecutivePass > 0 {
			fmt.Printf("  Trend: %s (%d consecutive passes)\n", summary.Trend, summary.ConsecutivePass)
		}
		fmt.Println("")
	}

	return nil
}

// runVerdictBaseline sets a performance baseline
func runVerdictBaseline() error {
	fs := flag.NewFlagSet("verdict baseline", flag.ExitOnError)
	componentFlag := fs.String("component", "", "Component name")
	metricFlag := fs.String("metric", "", "Metric name")
	valueFlag := fs.Float64("value", 0, "Baseline value")
	identityFlag := fs.String("identity", "", "Identity setting baseline")

	// Parse remaining args (after "verdict baseline")
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	// Validate required flags
	if *componentFlag == "" || *metricFlag == "" || *identityFlag == "" {
		return fmt.Errorf("required flags: --component, --metric, --value, --identity")
	}

	if !identity.IsValid(*identityFlag) {
		return fmt.Errorf("invalid identity: %s", *identityFlag)
	}

	// Load existing data
	data, err := loadVerdictData()
	if err != nil {
		return err
	}

	// Create or update baseline
	baseline := VerdictBaseline{
		Component: *componentFlag,
		Metric:    *metricFlag,
		Value:     *valueFlag,
		SetAt:     time.Now(),
		SetBy:     *identityFlag,
	}

	// Remove existing baseline for this component/metric
	newBaselines := []VerdictBaseline{}
	for _, b := range data.Baselines {
		if b.Component != *componentFlag || b.Metric != *metricFlag {
			newBaselines = append(newBaselines, b)
		}
	}
	newBaselines = append(newBaselines, baseline)
	data.Baselines = newBaselines

	// Save
	if err := saveVerdictData(data); err != nil {
		return err
	}

	output.Success("⚖️ BASELINE SET")
	fmt.Println("")
	fmt.Printf("Component: %s\n", baseline.Component)
	fmt.Printf("Metric: %s\n", baseline.Metric)
	fmt.Printf("Value: %.2f\n", baseline.Value)
	fmt.Printf("Set By: %s\n", baseline.SetBy)
	fmt.Printf("Set At: %s\n", baseline.SetAt.Format("2006-01-02 15:04:05"))

	return nil
}

// runVerdictList lists all verdicts
func runVerdictList() error {
	// Load existing data
	data, err := loadVerdictData()
	if err != nil {
		return err
	}

	if len(data.Entries) == 0 {
		fmt.Println("No verdicts recorded yet")
		return nil
	}

	output.Success("⚖️ VERDICT LIST")
	fmt.Println("")

	// Sort by timestamp (newest first)
	entries := make([]VerdictEntry, len(data.Entries))
	copy(entries, data.Entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// Display up to 50 most recent
	limit := 50
	if len(entries) < limit {
		limit = len(entries)
	}

	for i := 0; i < limit; i++ {
		entry := entries[i]
		if entry.Type == "test" {
			fmt.Printf("[%s] %s/%s: %s",
				entry.Timestamp.Format("2006-01-02 15:04"),
				entry.Component,
				entry.Test,
				strings.ToUpper(entry.Result))
			if entry.Duration > 0 {
				fmt.Printf(" (%.2fs)", entry.Duration)
			}
			fmt.Printf(" - %s\n", entry.Identity)
		} else if entry.Type == "benchmark" {
			fmt.Printf("[%s] %s/%s: %.2f - %s\n",
				entry.Timestamp.Format("2006-01-02 15:04"),
				entry.Component,
				entry.Metric,
				entry.Value,
				entry.Identity)
		}
	}

	if len(entries) > limit {
		fmt.Printf("\n... and %d more\n", len(entries)-limit)
	}

	return nil
}

// Helper functions

func loadVerdictData() (*VerdictData, error) {
	verdictPath, err := getVerdictPath()
	if err != nil {
		return nil, err
	}

	// If file doesn't exist, return empty data
	if _, err := os.Stat(verdictPath); os.IsNotExist(err) {
		return &VerdictData{
			Entries:   []VerdictEntry{},
			Baselines: []VerdictBaseline{},
		}, nil
	}

	// Read file
	content, err := os.ReadFile(verdictPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read verdict data: %w", err)
	}

	var data VerdictData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse verdict data: %w", err)
	}

	return &data, nil
}

func saveVerdictData(data *VerdictData) error {
	verdictPath, err := getVerdictPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(verdictPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create verdict directory: %w", err)
	}

	// Marshal to JSON
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal verdict data: %w", err)
	}

	// Write file
	if err := os.WriteFile(verdictPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write verdict data: %w", err)
	}

	return nil
}

func getVerdictPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "ram", "deus", "verdicts", "entries.json"), nil
}

func findBaseline(data *VerdictData, component, metric string) *VerdictBaseline {
	for _, baseline := range data.Baselines {
		if baseline.Component == component && baseline.Metric == metric {
			return &baseline
		}
	}
	return nil
}

func generateSummaries(entries []VerdictEntry) []VerdictSummary {
	// Group by component
	byComponent := make(map[string][]VerdictEntry)
	for _, entry := range entries {
		if entry.Type == "test" { // Only process tests for summaries
			byComponent[entry.Component] = append(byComponent[entry.Component], entry)
		}
	}

	// Generate summaries
	var summaries []VerdictSummary
	for component, componentEntries := range byComponent {
		summary := VerdictSummary{
			Component: component,
		}

		// Sort by timestamp
		sort.Slice(componentEntries, func(i, j int) bool {
			return componentEntries[i].Timestamp.Before(componentEntries[j].Timestamp)
		})

		totalDuration := 0.0
		consecutivePass := 0
		lastWasPass := false

		for _, entry := range componentEntries {
			summary.TotalTests++
			if entry.Result == "pass" {
				summary.PassCount++
				if lastWasPass {
					consecutivePass++
				} else {
					consecutivePass = 1
					lastWasPass = true
				}
			} else {
				summary.FailCount++
				lastWasPass = false
			}
			totalDuration += entry.Duration
			summary.LastRun = entry.Timestamp
		}

		if summary.TotalTests > 0 {
			summary.SuccessRate = (float64(summary.PassCount) / float64(summary.TotalTests)) * 100
			summary.AvgDuration = totalDuration / float64(summary.TotalTests)
		}

		summary.ConsecutivePass = consecutivePass

		// Determine trend
		if consecutivePass >= 3 {
			summary.Trend = "↑"
		} else if summary.FailCount > summary.PassCount {
			summary.Trend = "↓"
		} else {
			summary.Trend = "→"
		}

		summaries = append(summaries, summary)
	}

	// Sort by component name
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Component < summaries[j].Component
	})

	return summaries
}

func printVerdictUsage() {
	fmt.Println("verdict - Track test results and performance metrics")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Println("  record      Record a test result")
	fmt.Println("  bench       Record a benchmark result")
	fmt.Println("  check       Check for regressions")
	fmt.Println("  report      Generate verdict report")
	fmt.Println("  baseline    Set a performance baseline")
	fmt.Println("  list        List all verdicts")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  matrix verdict record --identity smith --component auth --test login --result pass --duration 2.3")
	fmt.Println("  matrix verdict bench --identity smith --component parser --metric \"ops/sec\" --value 1000")
	fmt.Println("  matrix verdict check --component parser --threshold 10")
	fmt.Println("  matrix verdict baseline --component parser --metric \"ops/sec\" --value 1000 --identity deus")
	fmt.Println("  matrix verdict report --component auth")
	fmt.Println("  matrix verdict list")
}
