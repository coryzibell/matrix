package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
)

// Template types
const (
	TemplateImpl     = "impl"
	TemplateDebug    = "debug"
	TemplateDesign   = "design"
	TemplateResearch = "research"
)

// runGardenSeeds implements the garden-seeds command
func runGardenSeeds() error {
	// Parse flags
	fs := flag.NewFlagSet("garden-seeds", flag.ExitOnError)
	typeFlag := fs.String("type", "impl", "Template type: impl, debug, design, research")
	identityFlag := fs.String("identity", "neo", "Identity RAM directory to create file in")
	listFlag := fs.Bool("list-templates", false, "List available templates and exit")

	// Parse remaining args (after "garden-seeds")
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	// Handle list templates flag
	if *listFlag {
		listTemplates()
		return nil
	}

	// Get title from remaining args
	if fs.NArg() == 0 {
		return fmt.Errorf("missing title argument\nUsage: matrix garden-seeds \"title\" [--type impl|debug|design|research] [--identity <name>]")
	}

	title := fs.Arg(0)

	// Validate identity
	if !identity.IsValid(*identityFlag) {
		return fmt.Errorf("invalid identity: %s", *identityFlag)
	}

	// Validate template type
	if !isValidTemplate(*typeFlag) {
		return fmt.Errorf("invalid template type: %s (valid: impl, debug, design, research)", *typeFlag)
	}

	// Get identity RAM path
	ramPath, err := identity.RAMPath(*identityFlag)
	if err != nil {
		return fmt.Errorf("failed to get RAM path: %w", err)
	}

	// Ensure RAM directory exists
	if err := os.MkdirAll(ramPath, 0755); err != nil {
		return fmt.Errorf("failed to create RAM directory: %w", err)
	}

	// Slugify title for filename
	slug := slugify(title)
	filename := slug + ".md"
	filePath := filepath.Join(ramPath, filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("file already exists: %s", filePath)
	}

	// Find related files
	relatedFiles := findRelatedFiles(ramPath, title, slug)

	// Generate content from template
	content := generateTemplate(*typeFlag, title, *identityFlag, relatedFiles)

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Output success message
	output.Success("🌱 Seed planted")
	fmt.Println("")
	fmt.Printf("Created: %s\n", filePath)
	fmt.Printf("Type: %s\n", *typeFlag)
	fmt.Printf("Identity: %s\n", *identityFlag)

	if len(relatedFiles) > 0 {
		fmt.Println("")
		fmt.Println("Related files suggested:")
		for _, rel := range relatedFiles {
			fmt.Printf("  - %s\n", rel)
		}
	}

	fmt.Println("")
	fmt.Println("Open the file to fill in the template sections.")

	return nil
}

// listTemplates displays available template types
func listTemplates() {
	output.Success("🌱 Available Templates")
	fmt.Println("")

	templates := []struct {
		Name        string
		Description string
		Sections    []string
	}{
		{
			Name:        "impl",
			Description: "Technical implementation and learnings",
			Sections:    []string{"Problem", "Solution", "Key Learnings", "Files Changed", "Next Steps"},
		},
		{
			Name:        "debug",
			Description: "Debugging session and incident resolution",
			Sections:    []string{"Symptoms", "Root Cause", "Fix", "Verification", "Prevention"},
		},
		{
			Name:        "design",
			Description: "Design and architecture decisions",
			Sections:    []string{"Context", "Constraints", "Design", "Alternatives", "Assertions"},
		},
		{
			Name:        "research",
			Description: "Research and exploration findings",
			Sections:    []string{"Question", "Findings", "Sources", "Open Questions", "Recommendations"},
		},
	}

	for _, tmpl := range templates {
		fmt.Printf("%s\n", output.Yellow+tmpl.Name+output.Reset)
		fmt.Printf("  %s\n", tmpl.Description)
		fmt.Printf("  Sections: %s\n", strings.Join(tmpl.Sections, ", "))
		fmt.Println("")
	}
}

// isValidTemplate checks if the template type is valid
func isValidTemplate(t string) bool {
	valid := map[string]bool{
		TemplateImpl:     true,
		TemplateDebug:    true,
		TemplateDesign:   true,
		TemplateResearch: true,
	}
	return valid[t]
}

// Note: slugify function is defined in crossroads.go

// findRelatedFiles searches for related files in the same identity's RAM
func findRelatedFiles(ramPath, title, slug string) []string {
	var related []string

	// Extract keywords from title for matching
	keywords := extractKeywords(title)

	// Read files in RAM directory
	entries, err := os.ReadDir(ramPath)
	if err != nil {
		return related
	}

	type scoredFile struct {
		path  string
		score int
	}

	var scored []scoredFile

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Skip the file we're about to create
		if entry.Name() == slug+".md" {
			continue
		}

		// Score based on keyword matches
		fileName := strings.ToLower(entry.Name())
		score := 0
		for _, kw := range keywords {
			if strings.Contains(fileName, kw) {
				score++
			}
		}

		if score > 0 {
			scored = append(scored, scoredFile{
				path:  filepath.Join(ramPath, entry.Name()),
				score: score,
			})
		}
	}

	// Sort by score descending
	if len(scored) > 1 {
		for i := 0; i < len(scored)-1; i++ {
			for j := i + 1; j < len(scored); j++ {
				if scored[j].score > scored[i].score {
					scored[i], scored[j] = scored[j], scored[i]
				}
			}
		}
	}

	// Take top 3
	limit := 3
	if len(scored) < limit {
		limit = len(scored)
	}

	for i := 0; i < limit; i++ {
		related = append(related, scored[i].path)
	}

	return related
}

// Note: extractKeywords function is defined in crossroads.go

// generateTemplate creates content based on template type
func generateTemplate(templateType, title, identityName string, relatedFiles []string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Metadata
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().Format("2006-01-02")))

	switch templateType {
	case TemplateImpl:
		sb.WriteString("**Status:** draft\n")
	case TemplateDebug:
		sb.WriteString("**Status:** investigating\n")
	case TemplateDesign:
		sb.WriteString("**Status:** proposal\n")
	case TemplateResearch:
		sb.WriteString("**Status:** ongoing\n")
	}

	// Related files
	if len(relatedFiles) > 0 {
		sb.WriteString("\n**Related:**\n")
		for _, rel := range relatedFiles {
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", filepath.Base(rel), rel))
		}
	}

	sb.WriteString("\n---\n\n")

	// Template-specific sections
	switch templateType {
	case TemplateImpl:
		sb.WriteString(templateImplContent())
	case TemplateDebug:
		sb.WriteString(templateDebugContent())
	case TemplateDesign:
		sb.WriteString(templateDesignContent())
	case TemplateResearch:
		sb.WriteString(templateResearchContent())
	}

	// Footer
	sb.WriteString("\n---\n\n")
	sb.WriteString(fmt.Sprintf("[Identity: %s | Model: sonnet | Status: in-progress]\n", identityName))

	return sb.String()
}

// Template content functions
func templateImplContent() string {
	return `## Problem

[What were we trying to solve?]

## Solution

[What did we build/fix/discover?]

## Key Learnings

- [Bullet points for quick scanning]

## Files Changed

- [List files modified]

## Next Steps

- [ ] [Checkboxes for follow-up tasks]
`
}

func templateDebugContent() string {
	return `## Symptoms

[What broke? How did we notice?]

## Root Cause

[The actual underlying issue]

## Fix

[What we changed to resolve it]

## Verification

[How we know it's fixed]

## Prevention

[What stops this class of bug in the future?]
`
}

func templateDesignContent() string {
	return `## Context

[Why are we designing this? What's the background?]

## Constraints

[What limits our options? Technical, resource, or business constraints]

## Design

[The actual design - structure, patterns, decisions]

## Alternatives Considered

[What we didn't choose and why]

## Structural Assertions

[Key properties that must hold]

- [ ] [Assertion 1]
  [verify: command to check]

- [ ] [Assertion 2]
  [verify: command to check]
`
}

func templateResearchContent() string {
	return `## Question

[What are we trying to learn?]

## Findings

[What we discovered through research]

## Sources

[Where we looked - docs, articles, code examples]

- [Source 1]
- [Source 2]

## Open Questions

[What we still don't know]

- [Question 1]
- [Question 2]

## Recommendations

[Where to go next based on findings]
`
}
