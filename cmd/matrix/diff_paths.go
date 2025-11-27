package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileAnalysis contains structural metrics for a single file
type FileAnalysis struct {
	Path         string
	Language     string
	Lines        int
	Classes      int
	Functions    int
	Methods      int
	Imports      int
	NestingDepth int
	IsAsync      bool
	HasState     bool
}

// PathComparison contains the full diff analysis
type PathComparison struct {
	PathA      FileAnalysis
	PathB      FileAnalysis
	Tradeoffs  TradeoffSummary
}

// TradeoffSummary provides decision guidance
type TradeoffSummary struct {
	ChooseAIf []string
	ChooseBIf []string
}

// runDiffPaths implements the diff-paths command
func runDiffPaths() error {
	args := os.Args[2:]

	// Parse flags
	dirMode := false
	jsonOutput := false
	var pathA, pathB string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			dirMode = true
		case "--json":
			jsonOutput = true
		default:
			if pathA == "" {
				pathA = args[i]
			} else if pathB == "" {
				pathB = args[i]
			}
		}
	}

	if pathA == "" || pathB == "" {
		return fmt.Errorf("usage: diff-paths [--dir] [--json] <path-a> <path-b>")
	}

	// Make paths absolute
	absA, err := filepath.Abs(pathA)
	if err != nil {
		return fmt.Errorf("failed to resolve path A: %w", err)
	}
	absB, err := filepath.Abs(pathB)
	if err != nil {
		return fmt.Errorf("failed to resolve path B: %w", err)
	}

	if dirMode {
		return fmt.Errorf("directory mode not yet implemented")
	}

	// Analyze both files
	analysisA, err := analyzeFile(absA)
	if err != nil {
		return fmt.Errorf("failed to analyze %s: %w", absA, err)
	}

	analysisB, err := analyzeFile(absB)
	if err != nil {
		return fmt.Errorf("failed to analyze %s: %w", absB, err)
	}

	// Generate tradeoffs
	tradeoffs := generateTradeoffs(analysisA, analysisB)

	comparison := PathComparison{
		PathA:     analysisA,
		PathB:     analysisB,
		Tradeoffs: tradeoffs,
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(comparison)
	}

	// Human-readable output
	printComparison(comparison)
	return nil
}

// analyzeFile performs static analysis on a single file
func analyzeFile(path string) (FileAnalysis, error) {
	analysis := FileAnalysis{
		Path: path,
	}

	// Detect language from extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".py":
		analysis.Language = "Python"
	case ".js":
		analysis.Language = "JavaScript"
	case ".ts":
		analysis.Language = "TypeScript"
	case ".go":
		analysis.Language = "Go"
	case ".rs":
		analysis.Language = "Rust"
	case ".java":
		analysis.Language = "Java"
	case ".cpp", ".cc", ".cxx":
		analysis.Language = "C++"
	case ".c":
		analysis.Language = "C"
	case ".rb":
		analysis.Language = "Ruby"
	case ".php":
		analysis.Language = "PHP"
	default:
		analysis.Language = "unknown"
	}

	// Read file
	file, err := os.Open(path)
	if err != nil {
		return analysis, err
	}
	defer file.Close()

	// Scan content
	scanner := bufio.NewScanner(file)
	lineCount := 0
	currentNesting := 0
	maxNesting := 0

	// Language-specific patterns
	classPattern := regexp.MustCompile(`^\s*class\s+\w+`)
	funcPattern := regexp.MustCompile(`^\s*(def|function|func|fn)\s+\w+`)
	methodPattern := regexp.MustCompile(`^\s+(def|public|private|protected)\s+\w+\s*\(`)
	importPattern := regexp.MustCompile(`^\s*(import|from|use|require|#include)`)
	asyncPattern := regexp.MustCompile(`\b(async|await|Promise|Future|Task)\b`)
	statePattern := regexp.MustCompile(`\b(self\.|this\.|@|var|let|const|mut)\b`)

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Count structural elements
		if classPattern.MatchString(line) {
			analysis.Classes++
		}
		if funcPattern.MatchString(line) {
			analysis.Functions++
		}
		if methodPattern.MatchString(line) {
			analysis.Methods++
		}
		if importPattern.MatchString(line) {
			analysis.Imports++
		}

		// Detect async patterns
		if asyncPattern.MatchString(line) {
			analysis.IsAsync = true
		}

		// Detect state patterns
		if statePattern.MatchString(line) {
			analysis.HasState = true
		}

		// Track nesting depth (simple brace counting)
		currentNesting += strings.Count(line, "{") - strings.Count(line, "}")
		if currentNesting > maxNesting {
			maxNesting = currentNesting
		}
	}

	analysis.Lines = lineCount
	analysis.NestingDepth = maxNesting

	if err := scanner.Err(); err != nil {
		return analysis, err
	}

	return analysis, nil
}

// generateTradeoffs infers decision guidance from metrics
func generateTradeoffs(a, b FileAnalysis) TradeoffSummary {
	summary := TradeoffSummary{
		ChooseAIf: []string{},
		ChooseBIf: []string{},
	}

	// Structure: classes vs functions
	if a.Classes > b.Classes {
		summary.ChooseAIf = append(summary.ChooseAIf, "You need structure and type safety")
		summary.ChooseBIf = append(summary.ChooseBIf, "You prefer simplicity and composition")
	} else if b.Classes > a.Classes {
		summary.ChooseBIf = append(summary.ChooseBIf, "You need structure and type safety")
		summary.ChooseAIf = append(summary.ChooseAIf, "You prefer simplicity and composition")
	}

	// Complexity: lines and nesting
	if a.Lines > int(float64(b.Lines)*1.3) {
		summary.ChooseBIf = append(summary.ChooseBIf, "You value conciseness")
		summary.ChooseAIf = append(summary.ChooseAIf, "You prefer explicit over implicit")
	} else if b.Lines > int(float64(a.Lines)*1.3) {
		summary.ChooseAIf = append(summary.ChooseAIf, "You value conciseness")
		summary.ChooseBIf = append(summary.ChooseBIf, "You prefer explicit over implicit")
	}

	// Dependencies
	if a.Imports > b.Imports {
		summary.ChooseBIf = append(summary.ChooseBIf, "You want minimal dependencies")
		summary.ChooseAIf = append(summary.ChooseAIf, "You're okay with external tools/libraries")
	} else if b.Imports > a.Imports {
		summary.ChooseAIf = append(summary.ChooseAIf, "You want minimal dependencies")
		summary.ChooseBIf = append(summary.ChooseBIf, "You're okay with external tools/libraries")
	}

	// Async patterns
	if a.IsAsync && !b.IsAsync {
		summary.ChooseAIf = append(summary.ChooseAIf, "You need concurrency/async behavior")
		summary.ChooseBIf = append(summary.ChooseBIf, "You prefer synchronous simplicity")
	} else if b.IsAsync && !a.IsAsync {
		summary.ChooseBIf = append(summary.ChooseBIf, "You need concurrency/async behavior")
		summary.ChooseAIf = append(summary.ChooseAIf, "You prefer synchronous simplicity")
	}

	// State management
	if a.HasState && !b.HasState {
		summary.ChooseAIf = append(summary.ChooseAIf, "You need stateful behavior")
		summary.ChooseBIf = append(summary.ChooseBIf, "You prefer pure functions/immutability")
	} else if b.HasState && !a.HasState {
		summary.ChooseBIf = append(summary.ChooseBIf, "You need stateful behavior")
		summary.ChooseAIf = append(summary.ChooseAIf, "You prefer pure functions/immutability")
	}

	// Testability heuristic (functions > classes for test simplicity)
	totalUnitsA := a.Classes + a.Functions + a.Methods
	totalUnitsB := b.Classes + b.Functions + b.Methods
	if a.Functions > a.Classes && b.Classes > b.Functions {
		summary.ChooseAIf = append(summary.ChooseAIf, "You prioritize testability")
	} else if b.Functions > b.Classes && a.Classes > a.Functions {
		summary.ChooseBIf = append(summary.ChooseBIf, "You prioritize testability")
	}

	// Fallback if no clear differences
	if len(summary.ChooseAIf) == 0 {
		summary.ChooseAIf = append(summary.ChooseAIf, "Minimal structural differences detected")
	}
	if len(summary.ChooseBIf) == 0 {
		summary.ChooseBIf = append(summary.ChooseBIf, "Minimal structural differences detected")
	}

	// Suppress unused variable warnings
	_ = totalUnitsA
	_ = totalUnitsB

	return summary
}

// printComparison outputs human-readable comparison
func printComparison(comp PathComparison) {
	fmt.Println("🔀 Path Divergence Analysis")
	fmt.Println()
	fmt.Println("Comparing:")
	fmt.Printf("  Path A: %s\n", comp.PathA.Path)
	fmt.Printf("  Path B: %s\n", comp.PathB.Path)
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// STRUCTURE
	fmt.Println("STRUCTURE")
	fmt.Printf("  A: %d classes, %d functions, %d methods\n",
		comp.PathA.Classes, comp.PathA.Functions, comp.PathA.Methods)
	fmt.Printf("  B: %d classes, %d functions, %d methods\n",
		comp.PathB.Classes, comp.PathB.Functions, comp.PathB.Methods)
	fmt.Println()

	// COMPLEXITY
	fmt.Println("COMPLEXITY")
	fmt.Printf("  A: %d lines, %d imports, nesting depth %d\n",
		comp.PathA.Lines, comp.PathA.Imports, comp.PathA.NestingDepth)
	fmt.Printf("  B: %d lines, %d imports, nesting depth %d\n",
		comp.PathB.Lines, comp.PathB.Imports, comp.PathB.NestingDepth)
	fmt.Println()

	// PATTERNS
	fmt.Println("PATTERNS")
	fmt.Printf("  A: async=%v, stateful=%v\n", comp.PathA.IsAsync, comp.PathA.HasState)
	fmt.Printf("  B: async=%v, stateful=%v\n", comp.PathB.IsAsync, comp.PathB.HasState)
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// TRADEOFFS
	fmt.Println("TRADEOFFS")
	fmt.Println()
	fmt.Println("Choose A if:")
	for _, reason := range comp.Tradeoffs.ChooseAIf {
		fmt.Printf("  - %s\n", reason)
	}
	fmt.Println()
	fmt.Println("Choose B if:")
	for _, reason := range comp.Tradeoffs.ChooseBIf {
		fmt.Printf("  - %s\n", reason)
	}
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
