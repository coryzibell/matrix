package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/coryzibell/matrix/internal/output"
)

// RequirementLevel represents MUST/SHOULD/MAY
type RequirementLevel string

const (
	LevelMust   RequirementLevel = "MUST"
	LevelShould RequirementLevel = "SHOULD"
	LevelMay    RequirementLevel = "MAY"
)

// RequirementStatus represents verification status
type RequirementStatus string

const (
	StatusSatisfied RequirementStatus = "SATISFIED"
	StatusPartial   RequirementStatus = "PARTIAL"
	StatusMissing   RequirementStatus = "MISSING"
	StatusManual    RequirementStatus = "MANUAL"
)

// Spec represents a formal specification
type Spec struct {
	Spec struct {
		Name       string `json:"name"`
		Identifier string `json:"identifier"`
		URL        string `json:"url"`
	} `json:"spec"`
	Requirements []Requirement `json:"requirements"`
}

// Requirement represents a single spec requirement
type Requirement struct {
	ID           string   `json:"id"`
	Section      string   `json:"section"`
	Level        string   `json:"level"`
	Text         string   `json:"text"`
	Verification struct {
		Type     string   `json:"type"`
		Patterns []string `json:"patterns"`
	} `json:"verification"`
}

// VerificationResult holds verification outcome
type VerificationResult struct {
	Requirement Requirement
	Status      RequirementStatus
	Matches     []Match
}

// Match represents a code match for a requirement
type Match struct {
	FilePath string
	Line     int
	Context  string
}

// SpecVerifyConfig holds command configuration
type SpecVerifyConfig struct {
	Subcommand string
	SpecName   string
	TargetPath string
	OutputJSON bool
}

// runSpecVerify implements the spec-verify command
func runSpecVerify() error {
	config := parseSVFlags()

	switch config.Subcommand {
	case "list":
		return listSpecs()
	case "verify":
		return verifySpec(config)
	case "report":
		return reportSpec(config)
	default:
		printSVUsage()
		return nil
	}
}

// parseSVFlags parses command-line flags for spec-verify
func parseSVFlags() SpecVerifyConfig {
	config := SpecVerifyConfig{
		Subcommand: "",
		SpecName:   "",
		TargetPath: ".",
		OutputJSON: false,
	}

	args := os.Args[2:] // Skip "matrix" and "spec-verify"

	if len(args) == 0 {
		return config
	}

	// First arg is subcommand
	config.Subcommand = args[0]

	// Parse remaining args
	for i := 1; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--json":
			config.OutputJSON = true
		case arg == "--format" && i+1 < len(args):
			i++
			if args[i] == "json" {
				config.OutputJSON = true
			}
		case config.SpecName == "":
			config.SpecName = arg
		case config.TargetPath == ".":
			config.TargetPath = arg
		}
	}

	return config
}

// printSVUsage prints usage information
func printSVUsage() {
	fmt.Println("Usage: matrix spec-verify <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  list                    List available specs")
	fmt.Println("  verify <spec> <path>    Verify codebase against spec")
	fmt.Println("  report <spec> <path>    Generate detailed compliance report")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --json                  Output in JSON format")
	fmt.Println("  --format json           Output in JSON format")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  matrix spec-verify list")
	fmt.Println("  matrix spec-verify verify oauth2 ~/project")
	fmt.Println("  matrix spec-verify report oauth2 . --json")
}

// listSpecs lists available spec files
func listSpecs() error {
	specsDir := getSpecsDir()

	// Check if specs directory exists
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		fmt.Println("No specs directory found.")
		fmt.Printf("Create specs at: %s\n", specsDir)
		return nil
	}

	// List spec files
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return fmt.Errorf("failed to read specs directory: %w", err)
	}

	specs := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			specName := strings.TrimSuffix(entry.Name(), ".json")
			specs = append(specs, specName)
		}
	}

	if len(specs) == 0 {
		fmt.Println("No specs found.")
		fmt.Printf("Create specs at: %s\n", specsDir)
		return nil
	}

	output.Success(fmt.Sprintf("Available Specs (%d)", len(specs)))
	fmt.Println()
	for _, spec := range specs {
		fmt.Printf("  - %s\n", spec)
	}

	return nil
}

// verifySpec verifies codebase against a spec
func verifySpec(config SpecVerifyConfig) error {
	if config.SpecName == "" {
		return fmt.Errorf("spec name required")
	}

	// Load spec
	spec, err := loadSpec(config.SpecName)
	if err != nil {
		return err
	}

	// Resolve target path
	absPath, err := filepath.Abs(config.TargetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Verify requirements
	results := verifyRequirements(spec, absPath)

	// Output results
	if config.OutputJSON {
		outputSVJSON(spec, results)
	} else {
		outputVerifyText(spec, results, absPath)
	}

	return nil
}

// reportSpec generates detailed compliance report
func reportSpec(config SpecVerifyConfig) error {
	// For MVP, report is the same as verify with more detail
	return verifySpec(config)
}

// getSpecsDir returns the specs directory path
func getSpecsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".claude/ram/lock/specs"
	}
	return filepath.Join(homeDir, ".claude", "ram", "lock", "specs")
}

// loadSpec loads a spec file
func loadSpec(specName string) (*Spec, error) {
	specsDir := getSpecsDir()
	specPath := filepath.Join(specsDir, specName+".json")

	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file %s: %w", specPath, err)
	}

	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	return &spec, nil
}

// verifyRequirements verifies all requirements against codebase
func verifyRequirements(spec *Spec, targetPath string) []VerificationResult {
	var results []VerificationResult

	for _, req := range spec.Requirements {
		result := verifyRequirement(req, targetPath)
		results = append(results, result)
	}

	return results
}

// verifyRequirement verifies a single requirement
func verifyRequirement(req Requirement, targetPath string) VerificationResult {
	result := VerificationResult{
		Requirement: req,
		Status:      StatusMissing,
		Matches:     []Match{},
	}

	// Handle manual verification
	if req.Verification.Type == "manual" {
		result.Status = StatusManual
		return result
	}

	// Compile patterns
	var regexes []*regexp.Regexp
	for _, pattern := range req.Verification.Patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		regexes = append(regexes, re)
	}

	if len(regexes) == 0 {
		result.Status = StatusManual
		return result
	}

	// Scan codebase
	matches := scanCodebase(targetPath, regexes)
	result.Matches = matches

	// Determine status
	if len(matches) > 0 {
		result.Status = StatusSatisfied
	} else {
		result.Status = StatusMissing
	}

	return result
}

// scanCodebase scans for pattern matches
func scanCodebase(rootPath string, patterns []*regexp.Regexp) []Match {
	var matches []Match

	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && shouldSkipSVDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip non-code files
		if !isSVCodeFile(path) {
			return nil
		}

		// Skip large files
		if info.Size() > 5*1024*1024 {
			return nil
		}

		// Scan file
		fileMatches := scanFile(rootPath, path, patterns)
		matches = append(matches, fileMatches...)

		return nil
	})

	return matches
}

// scanFile scans a single file for patterns
func scanFile(rootPath, filePath string, patterns []*regexp.Regexp) []Match {
	var matches []Match

	file, err := os.Open(filePath)
	if err != nil {
		return matches
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check each pattern
		for _, pattern := range patterns {
			if pattern.MatchString(line) {
				relPath, _ := filepath.Rel(rootPath, filePath)
				matches = append(matches, Match{
					FilePath: relPath,
					Line:     lineNum,
					Context:  strings.TrimSpace(line),
				})
				// Only match once per line
				break
			}
		}
	}

	return matches
}

// shouldSkipSVDir returns true if directory should be skipped
func shouldSkipSVDir(name string) bool {
	skipDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"target":       true,
		"build":        true,
		"dist":         true,
		"__pycache__":  true,
		"venv":         true,
		".venv":        true,
	}
	return skipDirs[name]
}

// isSVCodeFile returns true if file extension indicates code
func isSVCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExts := map[string]bool{
		".go": true, ".rs": true, ".js": true, ".ts": true, ".py": true,
		".java": true, ".c": true, ".cpp": true, ".cs": true, ".rb": true,
		".php": true, ".sh": true, ".bash": true, ".jsx": true, ".tsx": true,
		".kt": true, ".swift": true, ".scala": true, ".clj": true,
	}
	return codeExts[ext]
}

// outputVerifyText outputs verification results in text format
func outputVerifyText(spec *Spec, results []VerificationResult, targetPath string) {
	fmt.Println()
	fmt.Printf("📋 Spec Verification: %s\n", spec.Spec.Name)
	fmt.Println()

	// Count by level and status
	mustSatisfied := 0
	mustTotal := 0
	shouldSatisfied := 0
	shouldTotal := 0
	missingReqs := []VerificationResult{}
	satisfiedReqs := []VerificationResult{}

	for _, result := range results {
		level := RequirementLevel(result.Requirement.Level)

		if level == LevelMust {
			mustTotal++
			if result.Status == StatusSatisfied {
				mustSatisfied++
			}
		} else if level == LevelShould {
			shouldTotal++
			if result.Status == StatusSatisfied {
				shouldSatisfied++
			}
		}

		if result.Status == StatusMissing {
			missingReqs = append(missingReqs, result)
		} else if result.Status == StatusSatisfied {
			satisfiedReqs = append(satisfiedReqs, result)
		}
	}

	// Summary
	fmt.Println("Summary:")
	if mustTotal > 0 {
		fmt.Printf("  MUST: %d/%d satisfied (%.0f%%)\n",
			mustSatisfied, mustTotal, float64(mustSatisfied)/float64(mustTotal)*100)
	}
	if shouldTotal > 0 {
		fmt.Printf("  SHOULD: %d/%d satisfied (%.0f%%)\n",
			shouldSatisfied, shouldTotal, float64(shouldSatisfied)/float64(shouldTotal)*100)
	}
	fmt.Println()

	// Status
	compliant := mustSatisfied == mustTotal
	if compliant {
		output.Success("Status: COMPLIANT")
	} else {
		fmt.Printf("%sStatus: NON-COMPLIANT%s\n", output.Red, output.Reset)
	}
	fmt.Println()

	// Missing requirements
	if len(missingReqs) > 0 {
		fmt.Printf("%sMISSING Requirements:%s\n", output.Yellow, output.Reset)
		for _, result := range missingReqs {
			fmt.Printf("  [%s] %s: %s\n",
				result.Requirement.ID,
				result.Requirement.Level,
				result.Requirement.Text)
			fmt.Println("    - No matching patterns found")
			fmt.Println()
		}
	}

	// Satisfied requirements (show first 10)
	if len(satisfiedReqs) > 0 {
		fmt.Printf("%sSATISFIED Requirements:%s\n", output.Green, output.Reset)
		showCount := len(satisfiedReqs)
		if showCount > 10 {
			showCount = 10
		}
		for i := 0; i < showCount; i++ {
			result := satisfiedReqs[i]
			fmt.Printf("  [%s] %s: %s\n",
				result.Requirement.ID,
				result.Requirement.Level,
				result.Requirement.Text)
			if len(result.Matches) > 0 {
				match := result.Matches[0]
				fmt.Printf("    - Found in %s:%d\n", match.FilePath, match.Line)
			}
			fmt.Println()
		}
		if len(satisfiedReqs) > 10 {
			fmt.Printf("  ... and %d more\n\n", len(satisfiedReqs)-10)
		}
	}
}

// outputSVJSON outputs verification results in JSON format
func outputSVJSON(spec *Spec, results []VerificationResult) {
	fmt.Println("{")
	fmt.Printf("  \"spec\": \"%s\",\n", escapeSVJSON(spec.Spec.Name))
	fmt.Printf("  \"identifier\": \"%s\",\n", escapeSVJSON(spec.Spec.Identifier))
	fmt.Printf("  \"total_requirements\": %d,\n", len(results))

	// Count by status
	satisfied := 0
	missing := 0
	manual := 0
	for _, r := range results {
		switch r.Status {
		case StatusSatisfied:
			satisfied++
		case StatusMissing:
			missing++
		case StatusManual:
			manual++
		}
	}

	fmt.Printf("  \"satisfied\": %d,\n", satisfied)
	fmt.Printf("  \"missing\": %d,\n", missing)
	fmt.Printf("  \"manual\": %d,\n", manual)
	fmt.Println("  \"results\": [")

	for i, result := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}

		fmt.Println("    {")
		fmt.Printf("      \"id\": \"%s\",\n", escapeSVJSON(result.Requirement.ID))
		fmt.Printf("      \"level\": \"%s\",\n", escapeSVJSON(result.Requirement.Level))
		fmt.Printf("      \"text\": \"%s\",\n", escapeSVJSON(result.Requirement.Text))
		fmt.Printf("      \"status\": \"%s\",\n", result.Status)
		fmt.Printf("      \"matches\": %d\n", len(result.Matches))
		fmt.Printf("    }%s\n", comma)
	}

	fmt.Println("  ]")
	fmt.Println("}")
}

// escapeSVJSON escapes strings for JSON output
func escapeSVJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
