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

	"github.com/coryzibell/matrix/internal/output"
)

// EntryType represents the type of compatibility entry
type EntryType string

const (
	EntryTypeCompatibility EntryType = "compatibility"
	EntryTypeBreak         EntryType = "break"
	EntryTypePattern       EntryType = "pattern"
)

// PhaseShiftEntry represents a single compatibility/pattern/break record
type PhaseShiftEntry struct {
	Type      EntryType `json:"type"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Note      string    `json:"note"`
	Timestamp string    `json:"timestamp"`
}

// PhaseShiftData contains all recorded entries
type PhaseShiftData struct {
	Entries []PhaseShiftEntry `json:"entries"`
}

// VersionSpec represents a parsed version specification (e.g., "python:3.9")
type VersionSpec struct {
	Language string
	Version  string
}

// runPhaseShift implements the phase-shift command
func runPhaseShift() error {
	if len(os.Args) < 3 {
		printPhaseShiftHelp()
		return nil
	}

	subCmd := os.Args[2]

	switch subCmd {
	case "record":
		return runPhaseShiftRecord()
	case "break":
		return runPhaseShiftBreak()
	case "pattern":
		return runPhaseShiftPattern()
	case "check":
		return runPhaseShiftCheck()
	case "patterns":
		return runPhaseShiftPatterns()
	case "breaks":
		return runPhaseShiftBreaks()
	case "list":
		return runPhaseShiftList()
	case "--help", "-h", "help":
		printPhaseShiftHelp()
		return nil
	default:
		return fmt.Errorf("unknown subcommand: %s", subCmd)
	}
}

func printPhaseShiftHelp() {
	fmt.Println("🔄 Phase Shift - Cross-language compatibility tracker")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  matrix phase-shift record <from> <to> <note>    Record compatibility pair")
	fmt.Println("  matrix phase-shift break <from> <to> <note>     Record breaking change")
	fmt.Println("  matrix phase-shift pattern <from> <to> <note>   Record translation pattern")
	fmt.Println("  matrix phase-shift check <from> <to>            Check compatibility")
	fmt.Println("  matrix phase-shift patterns <lang1> <lang2>     List patterns for language pair")
	fmt.Println("  matrix phase-shift breaks <from> <to>           Show breaking changes")
	fmt.Println("  matrix phase-shift list                         List all entries")
	fmt.Println("")
	fmt.Println("Version specs: language:version (e.g., python:3.9, rust:1.70)")
}

// runPhaseShiftRecord records a compatibility pair
func runPhaseShiftRecord() error {
	if len(os.Args) < 6 {
		return fmt.Errorf("usage: phase-shift record <from> <to> <note>")
	}

	from := os.Args[3]
	to := os.Args[4]
	note := strings.Join(os.Args[5:], " ")

	return addEntry(EntryTypeCompatibility, from, to, note)
}

// runPhaseShiftBreak records a breaking change
func runPhaseShiftBreak() error {
	if len(os.Args) < 6 {
		return fmt.Errorf("usage: phase-shift break <from> <to> <note>")
	}

	from := os.Args[3]
	to := os.Args[4]
	note := strings.Join(os.Args[5:], " ")

	return addEntry(EntryTypeBreak, from, to, note)
}

// runPhaseShiftPattern records a translation pattern
func runPhaseShiftPattern() error {
	if len(os.Args) < 6 {
		return fmt.Errorf("usage: phase-shift pattern <from> <to> <note>")
	}

	from := os.Args[3]
	to := os.Args[4]
	note := strings.Join(os.Args[5:], " ")

	return addEntry(EntryTypePattern, from, to, note)
}

// runPhaseShiftCheck checks compatibility between versions
func runPhaseShiftCheck() error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: phase-shift check <from> <to>")
	}

	from := os.Args[3]
	to := os.Args[4]

	data, err := loadPhaseShiftData()
	if err != nil {
		return err
	}

	// Find all relevant entries
	var compatEntries []PhaseShiftEntry
	var breakEntries []PhaseShiftEntry
	var patternEntries []PhaseShiftEntry

	for _, entry := range data.Entries {
		if matchesSpec(entry, from, to) {
			switch entry.Type {
			case EntryTypeCompatibility:
				compatEntries = append(compatEntries, entry)
			case EntryTypeBreak:
				breakEntries = append(breakEntries, entry)
			case EntryTypePattern:
				patternEntries = append(patternEntries, entry)
			}
		}
	}

	output.Success("🔄 Phase Shift")
	fmt.Println("")
	fmt.Printf("Checking: %s → %s\n", output.Cyan+from+output.Reset, output.Cyan+to+output.Reset)
	fmt.Println("")

	if len(compatEntries) > 0 {
		fmt.Println("✓ COMPATIBILITY:")
		for _, e := range compatEntries {
			fmt.Printf("  %s → %s\n", e.From, e.To)
			fmt.Printf("    %s\n", e.Note)
		}
		fmt.Println("")
	}

	if len(breakEntries) > 0 {
		fmt.Println("⚠ BREAKS:")
		for _, e := range breakEntries {
			fmt.Printf("  %s → %s\n", e.From, e.To)
			fmt.Printf("    %s\n", e.Note)
		}
		fmt.Println("")
	}

	if len(patternEntries) > 0 {
		fmt.Println("📋 PATTERNS:")
		for _, e := range patternEntries {
			fmt.Printf("  %s → %s\n", e.From, e.To)
			fmt.Printf("    %s\n", e.Note)
		}
		fmt.Println("")
	}

	if len(compatEntries) == 0 && len(breakEntries) == 0 && len(patternEntries) == 0 {
		fmt.Println("No data recorded for this pair.")
	}

	return nil
}

// runPhaseShiftPatterns lists patterns for a language pair
func runPhaseShiftPatterns() error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: phase-shift patterns <lang1> <lang2>")
	}

	lang1 := os.Args[3]
	lang2 := os.Args[4]

	data, err := loadPhaseShiftData()
	if err != nil {
		return err
	}

	// Find all patterns for this language pair
	var patterns []PhaseShiftEntry
	for _, entry := range data.Entries {
		if entry.Type == EntryTypePattern {
			fromSpec := parseVersionSpec(entry.From)
			toSpec := parseVersionSpec(entry.To)

			if (fromSpec.Language == lang1 && toSpec.Language == lang2) ||
				(fromSpec.Language == lang2 && toSpec.Language == lang1) {
				patterns = append(patterns, entry)
			}
		}
	}

	output.Success("🔄 Phase Shift")
	fmt.Println("")
	fmt.Printf("PATTERNS: %s ↔ %s\n", output.Cyan+lang1+output.Reset, output.Cyan+lang2+output.Reset)
	fmt.Println("")

	if len(patterns) == 0 {
		fmt.Println("No patterns recorded for this language pair.")
		return nil
	}

	for _, p := range patterns {
		fmt.Printf("  %s → %s\n", p.From, p.To)
		fmt.Printf("    %s\n", p.Note)
		fmt.Println("")
	}

	return nil
}

// runPhaseShiftBreaks shows breaking changes in upgrade path
func runPhaseShiftBreaks() error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: phase-shift breaks <from> <to>")
	}

	from := os.Args[3]
	to := os.Args[4]

	data, err := loadPhaseShiftData()
	if err != nil {
		return err
	}

	// Find all breaks
	var breaks []PhaseShiftEntry
	for _, entry := range data.Entries {
		if entry.Type == EntryTypeBreak && matchesSpec(entry, from, to) {
			breaks = append(breaks, entry)
		}
	}

	output.Success("🔄 Phase Shift")
	fmt.Println("")
	fmt.Printf("BREAKS: %s → %s\n", output.Cyan+from+output.Reset, output.Cyan+to+output.Reset)
	fmt.Println("")

	if len(breaks) == 0 {
		fmt.Println("No breaking changes recorded for this upgrade path.")
		return nil
	}

	for _, b := range breaks {
		fmt.Printf("  ⚠ %s → %s\n", b.From, b.To)
		fmt.Printf("    %s\n", b.Note)
		fmt.Println("")
	}

	return nil
}

// runPhaseShiftList lists all recorded entries
func runPhaseShiftList() error {
	fs := flag.NewFlagSet("phase-shift-list", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "Output as JSON")
	typeFilter := fs.String("type", "", "Filter by type (compatibility, break, pattern)")

	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	data, err := loadPhaseShiftData()
	if err != nil {
		return err
	}

	// Filter by type if specified
	var entries []PhaseShiftEntry
	for _, entry := range data.Entries {
		if *typeFilter == "" || string(entry.Type) == *typeFilter {
			entries = append(entries, entry)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	if *jsonFlag {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	}

	output.Success("🔄 Phase Shift")
	fmt.Println("")
	fmt.Printf("Total entries: %d\n", len(entries))
	fmt.Println("")

	if len(entries) == 0 {
		fmt.Println("No entries recorded yet.")
		return nil
	}

	// Group by type
	compatEntries := filterByType(entries, EntryTypeCompatibility)
	breakEntries := filterByType(entries, EntryTypeBreak)
	patternEntries := filterByType(entries, EntryTypePattern)

	if len(compatEntries) > 0 {
		fmt.Println("═══ COMPATIBILITY ═══")
		fmt.Println("")
		for _, e := range compatEntries {
			fmt.Printf("  %s → %s\n", output.Yellow+e.From+output.Reset, output.Cyan+e.To+output.Reset)
			fmt.Printf("    %s\n", e.Note)
			fmt.Printf("    %s\n", output.Dim+e.Timestamp+output.Reset)
			fmt.Println("")
		}
	}

	if len(breakEntries) > 0 {
		fmt.Println("═══ BREAKS ═══")
		fmt.Println("")
		for _, e := range breakEntries {
			fmt.Printf("  %s → %s\n", output.Yellow+e.From+output.Reset, output.Red+e.To+output.Reset)
			fmt.Printf("    ⚠ %s\n", e.Note)
			fmt.Printf("    %s\n", output.Dim+e.Timestamp+output.Reset)
			fmt.Println("")
		}
	}

	if len(patternEntries) > 0 {
		fmt.Println("═══ PATTERNS ═══")
		fmt.Println("")
		for _, e := range patternEntries {
			fmt.Printf("  %s → %s\n", output.Yellow+e.From+output.Reset, output.Cyan+e.To+output.Reset)
			fmt.Printf("    %s\n", e.Note)
			fmt.Printf("    %s\n", output.Dim+e.Timestamp+output.Reset)
			fmt.Println("")
		}
	}

	return nil
}

// addEntry adds a new entry to the data file
func addEntry(entryType EntryType, from, to, note string) error {
	data, err := loadPhaseShiftData()
	if err != nil {
		return err
	}

	entry := PhaseShiftEntry{
		Type:      entryType,
		From:      from,
		To:        to,
		Note:      note,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	data.Entries = append(data.Entries, entry)

	if err := savePhaseShiftData(data); err != nil {
		return err
	}

	output.Success(fmt.Sprintf("✓ Recorded %s: %s → %s", entryType, from, to))
	return nil
}

// loadPhaseShiftData loads the data file
func loadPhaseShiftData() (*PhaseShiftData, error) {
	dataPath, err := getDataPath()
	if err != nil {
		return nil, err
	}

	// Create directory if it doesn't exist
	dataDir := filepath.Dir(dataPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// If file doesn't exist, return empty data
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return &PhaseShiftData{Entries: []PhaseShiftEntry{}}, nil
	}

	// Read file
	content, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read data file: %w", err)
	}

	// Parse JSON
	var data PhaseShiftData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse data file: %w", err)
	}

	return &data, nil
}

// savePhaseShiftData saves the data file
func savePhaseShiftData(data *PhaseShiftData) error {
	dataPath, err := getDataPath()
	if err != nil {
		return err
	}

	// Marshal to JSON
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Write file
	if err := os.WriteFile(dataPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write data file: %w", err)
	}

	return nil
}

// getDataPath returns the path to the data file
func getDataPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".claude", "ram", "twins", "compatibility", "entries.json"), nil
}

// parseVersionSpec parses a version specification (e.g., "python:3.9")
func parseVersionSpec(spec string) VersionSpec {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) == 2 {
		return VersionSpec{
			Language: parts[0],
			Version:  parts[1],
		}
	}
	return VersionSpec{
		Language: spec,
		Version:  "",
	}
}

// matchesSpec checks if an entry matches the given from/to specs
func matchesSpec(entry PhaseShiftEntry, from, to string) bool {
	// Exact match
	if entry.From == from && entry.To == to {
		return true
	}

	// Language-only match
	fromSpec := parseVersionSpec(from)
	toSpec := parseVersionSpec(to)
	entryFromSpec := parseVersionSpec(entry.From)
	entryToSpec := parseVersionSpec(entry.To)

	// If no version specified in query, match on language only
	if fromSpec.Version == "" && toSpec.Version == "" {
		return entryFromSpec.Language == fromSpec.Language &&
			entryToSpec.Language == toSpec.Language
	}

	// If version specified in query, require exact match
	return entry.From == from && entry.To == to
}

// filterByType filters entries by type
func filterByType(entries []PhaseShiftEntry, entryType EntryType) []PhaseShiftEntry {
	var filtered []PhaseShiftEntry
	for _, e := range entries {
		if e.Type == entryType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
