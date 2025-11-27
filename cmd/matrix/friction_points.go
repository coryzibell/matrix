package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
)

// FrictionPoint represents a UX review item
type FrictionPoint struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Owner        string    `json:"owner"`
	Priority     string    `json:"priority"`
	Status       string    `json:"status"`
	ReviewedDate string    `json:"reviewed_date,omitempty"`
	Feedback     string    `json:"feedback,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	Resolved     bool      `json:"resolved"`
	Approved     bool      `json:"approved"`
	ApprovalNote string    `json:"approval_note,omitempty"`
	QueuedDate   string    `json:"queued_date"`
}

// FrictionData represents the storage file structure
type FrictionData struct {
	Entries []FrictionPoint `json:"entries"`
}

// runFrictionPoints implements the friction-points command
func runFrictionPoints() error {
	if len(os.Args) < 3 {
		printFrictionPointsUsage()
		return nil
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "queue":
		return queueFrictionPoint()
	case "list":
		return listFrictionPoints()
	case "review":
		return reviewFrictionPoint()
	case "tag":
		return tagFrictionPoint()
	case "patterns":
		return showFrictionPatterns()
	case "approve":
		return approveFrictionPoint()
	case "status":
		return showFrictionStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown friction-points subcommand: %s\n", subcommand)
		printFrictionPointsUsage()
		os.Exit(1)
	}

	return nil
}

func printFrictionPointsUsage() {
	fmt.Println("friction-points - Track UX review queue and feedback")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  matrix friction-points queue \"name\" --type=X --owner=Y --priority=low|medium|high")
	fmt.Println("  matrix friction-points list")
	fmt.Println("  matrix friction-points review \"name\" --status=needs-changes|approved --feedback=\"text\"")
	fmt.Println("  matrix friction-points tag \"name\" <tag>")
	fmt.Println("  matrix friction-points patterns")
	fmt.Println("  matrix friction-points approve \"name\" --note=\"text\"")
	fmt.Println("  matrix friction-points status \"name\"")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Println("  queue     Add item to UX review queue")
	fmt.Println("  list      Show review queue")
	fmt.Println("  review    Mark item as reviewed with feedback")
	fmt.Println("  tag       Add friction pattern tag to item")
	fmt.Println("  patterns  Show common friction patterns")
	fmt.Println("  approve   Approve item for shipping")
	fmt.Println("  status    Check item review status")
}

func queueFrictionPoint() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("queue requires a name argument")
	}

	name := os.Args[3]

	// Parse flags
	var itemType, owner, priority string

	for i := 4; i < len(os.Args); i++ {
		arg := os.Args[i]

		if strings.HasPrefix(arg, "--type=") {
			itemType = strings.TrimPrefix(arg, "--type=")
		} else if strings.HasPrefix(arg, "--owner=") {
			owner = strings.TrimPrefix(arg, "--owner=")
		} else if strings.HasPrefix(arg, "--priority=") {
			priority = strings.TrimPrefix(arg, "--priority=")
		}
	}

	// Validate required fields
	if itemType == "" {
		return fmt.Errorf("--type is required (e.g., cli-output, error-handling, documentation)")
	}

	if owner == "" {
		return fmt.Errorf("--owner is required (identity name)")
	}

	// Validate priority
	if priority == "" {
		priority = "medium"
	}

	validPriorities := map[string]bool{"low": true, "medium": true, "high": true}
	if !validPriorities[priority] {
		return fmt.Errorf("invalid priority: %s (valid: low, medium, high)", priority)
	}

	// Validate owner is a valid identity
	if !identity.IsValid(owner) {
		return fmt.Errorf("invalid identity: %s", owner)
	}

	// Load existing data
	data, err := loadFrictionData()
	if err != nil {
		return fmt.Errorf("failed to load friction data: %w", err)
	}

	// Check if item already exists
	for _, entry := range data.Entries {
		if entry.Name == name {
			return fmt.Errorf("friction point already exists: %s", name)
		}
	}

	// Create new friction point
	frictionPoint := FrictionPoint{
		Name:       name,
		Type:       itemType,
		Owner:      owner,
		Priority:   priority,
		Status:     "waiting",
		Resolved:   false,
		Approved:   false,
		QueuedDate: time.Now().Format("2006-01-02"),
	}

	// Add to data
	data.Entries = append(data.Entries, frictionPoint)

	// Save data
	if err := saveFrictionData(data); err != nil {
		return fmt.Errorf("failed to save friction data: %w", err)
	}

	// Display success
	output.Success("Added to UX review queue")
	fmt.Println("")
	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Type: %s\n", itemType)
	fmt.Printf("Owner: %s\n", owner)
	fmt.Printf("Priority: %s\n", priority)
	fmt.Printf("Status: waiting\n")

	return nil
}

func listFrictionPoints() error {
	data, err := loadFrictionData()
	if err != nil {
		return fmt.Errorf("failed to load friction data: %w", err)
	}

	if len(data.Entries) == 0 {
		fmt.Println("No friction points in review queue.")
		fmt.Println("")
		fmt.Println("Use 'matrix friction-points queue' to add items.")
		return nil
	}

	// Organize by status
	var waiting, inProgress, needsChanges, approved []FrictionPoint

	for _, entry := range data.Entries {
		switch entry.Status {
		case "waiting":
			waiting = append(waiting, entry)
		case "in-progress":
			inProgress = append(inProgress, entry)
		case "needs-changes":
			needsChanges = append(needsChanges, entry)
		case "approved":
			approved = append(approved, entry)
		}
	}

	// Sort each category by priority (high, medium, low)
	sortByPriority := func(entries []FrictionPoint) {
		sort.Slice(entries, func(i, j int) bool {
			priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
			return priorityOrder[entries[i].Priority] < priorityOrder[entries[j].Priority]
		})
	}

	sortByPriority(waiting)
	sortByPriority(inProgress)
	sortByPriority(needsChanges)
	sortByPriority(approved)

	// Display output
	output.Success("UX Review Queue")
	fmt.Println("")

	// Waiting Review section
	if len(waiting) > 0 {
		output.Header(fmt.Sprintf("Waiting Review: %d items", len(waiting)))
		fmt.Println("")
		for _, entry := range waiting {
			priorityColor := getPriorityColor(entry.Priority)
			fmt.Printf("  [%s%s%s] %s (%s, owner: %s)\n",
				priorityColor, entry.Priority, output.Reset,
				entry.Name, entry.Type, entry.Owner)
		}
		fmt.Println("")
	}

	// In Progress section
	if len(inProgress) > 0 {
		output.Header(fmt.Sprintf("In Progress: %d items", len(inProgress)))
		fmt.Println("")
		for _, entry := range inProgress {
			priorityColor := getPriorityColor(entry.Priority)
			fmt.Printf("  [%s%s%s] %s (%s, owner: %s)\n",
				priorityColor, entry.Priority, output.Reset,
				entry.Name, entry.Type, entry.Owner)
		}
		fmt.Println("")
	}

	// Needs Changes section
	if len(needsChanges) > 0 {
		output.Header(fmt.Sprintf("Needs Changes: %d items", len(needsChanges)))
		fmt.Println("")
		for _, entry := range needsChanges {
			priorityColor := getPriorityColor(entry.Priority)
			feedbackSnippet := truncate(entry.Feedback, 60)
			fmt.Printf("  [%s%s%s] %s - %s\n",
				priorityColor, entry.Priority, output.Reset,
				entry.Name, feedbackSnippet)
		}
		fmt.Println("")
	}

	// Approved section (just count)
	if len(approved) > 0 {
		output.Header(fmt.Sprintf("Approved: %d items", len(approved)))
		fmt.Println("")
	}

	// Show friction patterns
	patternCounts := countPatterns(data.Entries)
	if len(patternCounts) > 0 {
		output.Header("Top Friction Patterns:")
		fmt.Println("")

		// Sort patterns by count
		type patternCount struct {
			pattern string
			count   int
		}
		var patterns []patternCount
		for pattern, count := range patternCounts {
			patterns = append(patterns, patternCount{pattern, count})
		}
		sort.Slice(patterns, func(i, j int) bool {
			return patterns[i].count > patterns[j].count
		})

		// Show top 5
		limit := 5
		if len(patterns) < limit {
			limit = len(patterns)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  %s: %d\n", patterns[i].pattern, patterns[i].count)
		}
		fmt.Println("")
	}

	return nil
}

func reviewFrictionPoint() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("review requires a name argument")
	}

	name := os.Args[3]

	// Parse flags
	var status, feedback string

	for i := 4; i < len(os.Args); i++ {
		arg := os.Args[i]

		if strings.HasPrefix(arg, "--status=") {
			status = strings.TrimPrefix(arg, "--status=")
		} else if strings.HasPrefix(arg, "--feedback=") {
			feedback = strings.TrimPrefix(arg, "--feedback=")
		}
	}

	// Validate status
	validStatuses := map[string]bool{
		"waiting":       true,
		"in-progress":   true,
		"needs-changes": true,
		"approved":      true,
	}

	if status == "" {
		return fmt.Errorf("--status is required (waiting, in-progress, needs-changes, approved)")
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	// Load data
	data, err := loadFrictionData()
	if err != nil {
		return fmt.Errorf("failed to load friction data: %w", err)
	}

	// Find and update entry
	found := false
	for i := range data.Entries {
		if data.Entries[i].Name == name {
			data.Entries[i].Status = status
			data.Entries[i].ReviewedDate = time.Now().Format("2006-01-02")
			if feedback != "" {
				data.Entries[i].Feedback = feedback
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("friction point not found: %s", name)
	}

	// Save data
	if err := saveFrictionData(data); err != nil {
		return fmt.Errorf("failed to save friction data: %w", err)
	}

	// Display success
	output.Success("Review recorded")
	fmt.Println("")
	fmt.Printf("Item: %s\n", name)
	fmt.Printf("Status: %s\n", status)
	if feedback != "" {
		fmt.Printf("Feedback: %s\n", feedback)
	}

	return nil
}

func tagFrictionPoint() error {
	if len(os.Args) < 5 {
		return fmt.Errorf("tag requires name and tag arguments")
	}

	name := os.Args[3]
	tag := os.Args[4]

	// Load data
	data, err := loadFrictionData()
	if err != nil {
		return fmt.Errorf("failed to load friction data: %w", err)
	}

	// Find and update entry
	found := false
	for i := range data.Entries {
		if data.Entries[i].Name == name {
			// Check if tag already exists
			hasTag := false
			for _, existingTag := range data.Entries[i].Tags {
				if existingTag == tag {
					hasTag = true
					break
				}
			}

			if !hasTag {
				data.Entries[i].Tags = append(data.Entries[i].Tags, tag)
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("friction point not found: %s", name)
	}

	// Save data
	if err := saveFrictionData(data); err != nil {
		return fmt.Errorf("failed to save friction data: %w", err)
	}

	// Display success
	output.Success("Tag added")
	fmt.Println("")
	fmt.Printf("Item: %s\n", name)
	fmt.Printf("Tag: %s\n", tag)

	return nil
}

func showFrictionPatterns() error {
	data, err := loadFrictionData()
	if err != nil {
		return fmt.Errorf("failed to load friction data: %w", err)
	}

	if len(data.Entries) == 0 {
		fmt.Println("No friction points tracked yet.")
		return nil
	}

	// Count patterns
	patternCounts := countPatterns(data.Entries)

	if len(patternCounts) == 0 {
		fmt.Println("No patterns tagged yet.")
		fmt.Println("")
		fmt.Println("Use 'matrix friction-points tag' to add pattern tags.")
		return nil
	}

	// Sort by count
	type patternCount struct {
		pattern string
		count   int
	}
	var patterns []patternCount
	for pattern, count := range patternCounts {
		patterns = append(patterns, patternCount{pattern, count})
	}
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].count != patterns[j].count {
			return patterns[i].count > patterns[j].count
		}
		return patterns[i].pattern < patterns[j].pattern
	})

	// Display
	output.Success("Friction Patterns")
	fmt.Println("")

	for _, p := range patterns {
		fmt.Printf("  %s: %d\n", p.pattern, p.count)
	}

	return nil
}

func approveFrictionPoint() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("approve requires a name argument")
	}

	name := os.Args[3]

	// Parse flags
	var note string

	for i := 4; i < len(os.Args); i++ {
		arg := os.Args[i]

		if strings.HasPrefix(arg, "--note=") {
			note = strings.TrimPrefix(arg, "--note=")
		}
	}

	// Load data
	data, err := loadFrictionData()
	if err != nil {
		return fmt.Errorf("failed to load friction data: %w", err)
	}

	// Find and update entry
	found := false
	for i := range data.Entries {
		if data.Entries[i].Name == name {
			data.Entries[i].Approved = true
			data.Entries[i].Status = "approved"
			data.Entries[i].Resolved = true
			data.Entries[i].ReviewedDate = time.Now().Format("2006-01-02")
			if note != "" {
				data.Entries[i].ApprovalNote = note
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("friction point not found: %s", name)
	}

	// Save data
	if err := saveFrictionData(data); err != nil {
		return fmt.Errorf("failed to save friction data: %w", err)
	}

	// Display success
	output.Success("Approved for shipping")
	fmt.Println("")
	fmt.Printf("Item: %s\n", name)
	if note != "" {
		fmt.Printf("Note: %s\n", note)
	}

	return nil
}

func showFrictionStatus() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("status requires a name argument")
	}

	name := os.Args[3]

	// Load data
	data, err := loadFrictionData()
	if err != nil {
		return fmt.Errorf("failed to load friction data: %w", err)
	}

	// Find entry
	var entry *FrictionPoint
	for i := range data.Entries {
		if data.Entries[i].Name == name {
			entry = &data.Entries[i]
			break
		}
	}

	if entry == nil {
		return fmt.Errorf("friction point not found: %s", name)
	}

	// Display status
	output.Success("Friction Point Status")
	fmt.Println("")
	fmt.Printf("Name: %s\n", entry.Name)
	fmt.Printf("Type: %s\n", entry.Type)
	fmt.Printf("Owner: %s\n", entry.Owner)
	fmt.Printf("Priority: %s\n", entry.Priority)
	fmt.Printf("Status: %s\n", entry.Status)
	fmt.Printf("Queued: %s\n", entry.QueuedDate)

	if entry.ReviewedDate != "" {
		fmt.Printf("Reviewed: %s\n", entry.ReviewedDate)
	}

	if entry.Feedback != "" {
		fmt.Printf("Feedback: %s\n", entry.Feedback)
	}

	if len(entry.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(entry.Tags, ", "))
	}

	fmt.Printf("Resolved: %t\n", entry.Resolved)
	fmt.Printf("Approved: %t\n", entry.Approved)

	if entry.ApprovalNote != "" {
		fmt.Printf("Approval Note: %s\n", entry.ApprovalNote)
	}

	return nil
}

// Helper functions

func loadFrictionData() (*FrictionData, error) {
	// Get persephone RAM path
	persephonePath, err := identity.RAMPath("persephone")
	if err != nil {
		return nil, fmt.Errorf("failed to get persephone RAM path: %w", err)
	}

	// Create friction-points directory if needed
	frictionDir := filepath.Join(persephonePath, "friction-points")
	if err := os.MkdirAll(frictionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create friction-points directory: %w", err)
	}

	// Load entries.json
	entriesPath := filepath.Join(frictionDir, "entries.json")

	// Check if file exists
	if _, err := os.Stat(entriesPath); os.IsNotExist(err) {
		// Return empty data
		return &FrictionData{Entries: []FrictionPoint{}}, nil
	}

	// Read file
	content, err := os.ReadFile(entriesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read entries file: %w", err)
	}

	// Parse JSON
	var data FrictionData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &data, nil
}

func saveFrictionData(data *FrictionData) error {
	// Get persephone RAM path
	persephonePath, err := identity.RAMPath("persephone")
	if err != nil {
		return fmt.Errorf("failed to get persephone RAM path: %w", err)
	}

	// Create friction-points directory if needed
	frictionDir := filepath.Join(persephonePath, "friction-points")
	if err := os.MkdirAll(frictionDir, 0755); err != nil {
		return fmt.Errorf("failed to create friction-points directory: %w", err)
	}

	// Write entries.json
	entriesPath := filepath.Join(frictionDir, "entries.json")

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write file
	if err := os.WriteFile(entriesPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write entries file: %w", err)
	}

	return nil
}

func getPriorityColor(priority string) string {
	switch priority {
	case "high":
		return output.Red
	case "medium":
		return output.Yellow
	case "low":
		return output.Dim
	default:
		return ""
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func countPatterns(entries []FrictionPoint) map[string]int {
	counts := make(map[string]int)
	for _, entry := range entries {
		for _, tag := range entry.Tags {
			counts[tag]++
		}
	}
	return counts
}
