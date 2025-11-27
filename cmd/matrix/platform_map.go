package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/output"
)

// PlatformCategory represents the compatibility level of a file
type PlatformCategory string

const (
	CrossPlatformVerified PlatformCategory = "cross-platform"
	PlatformSpecific      PlatformCategory = "platform-specific"
	UnknownCompatibility  PlatformCategory = "unknown"
	KnownIssues           PlatformCategory = "known-issues"
)

// FileCompatibility tracks platform compatibility information for a file
type FileCompatibility struct {
	FilePath    string           `json:"file_path"`
	Category    PlatformCategory `json:"category"`
	TestedOn    []string         `json:"tested_on,omitempty"`
	Breaks      []string         `json:"breaks,omitempty"`
	Mentions    []string         `json:"mentions,omitempty"`
	Patterns    []string         `json:"patterns,omitempty"`
	Description string           `json:"description,omitempty"`
}

// PlatformMapOutput contains the complete scan results
type PlatformMapOutput struct {
	CrossPlatform []FileCompatibility            `json:"cross_platform"`
	Specific      []FileCompatibility            `json:"platform_specific"`
	Unknown       []FileCompatibility            `json:"unknown"`
	Issues        []FileCompatibility            `json:"issues"`
	Stats         map[string]int                 `json:"platform_stats"`
	PatternCounts map[string]map[string][]string `json:"pattern_counts,omitempty"`
}

// Platform patterns to detect
var platformPatterns = map[string][]string{
	"win32":  {`\bwindows?\b`, `\bwin32\b`, `\bwsl\b`, `\bpowershell\b`, `\bcygwin\b`, `\bscoop\b`, `\.exe\b`, `\bwslpath\b`, `\bcygpath\b`},
	"linux":  {`\blinux\b`, `\bapt\b`, `\bapt-get\b`, `\byum\b`, `\bdnf\b`, `\bpacman\b`, `/usr/bin`, `/etc/`, `\bsystemd\b`},
	"darwin": {`\bdarwin\b`, `\bmacos\b`, `\bmac\b`, `\bhomebrew\b`, `\bbrew\b`, `/usr/local/`, `\blaunchd\b`},
}

// Package managers
var packageManagers = []string{
	"scoop", "homebrew", "brew", "apt", "apt-get", "yum", "dnf", "pacman", "aqua", "chocolatey", "winget",
}

// runPlatformMap implements the platform-map command
func runPlatformMap() error {
	fs := flag.NewFlagSet("platform-map", flag.ExitOnError)
	issuesOnly := fs.Bool("issues-only", false, "Show only problems")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	// Parse flags
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	// Get target path (default to ~/.claude/)
	targetPath := ""
	if fs.NArg() > 0 {
		targetPath = fs.Arg(0)
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		targetPath = filepath.Join(homeDir, ".claude")
	}

	// Expand ~ if present
	if strings.HasPrefix(targetPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		targetPath = strings.Replace(targetPath, "~", homeDir, 1)
	}

	// Check if target exists
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", targetPath)
		}
		return fmt.Errorf("failed to access path: %w", err)
	}

	// Scan the directory
	results, err := scanForPlatformCompatibility(targetPath)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Filter if issues-only
	if *issuesOnly {
		results.CrossPlatform = nil
		results.Unknown = nil
	}

	// Output results
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	}

	// Human-readable output
	printPlatformMap(results, *issuesOnly)
	return nil
}

// scanForPlatformCompatibility scans a directory tree for platform compatibility markers
func scanForPlatformCompatibility(rootPath string) (*PlatformMapOutput, error) {
	output := &PlatformMapOutput{
		CrossPlatform: []FileCompatibility{},
		Specific:      []FileCompatibility{},
		Unknown:       []FileCompatibility{},
		Issues:        []FileCompatibility{},
		Stats:         make(map[string]int),
		PatternCounts: make(map[string]map[string][]string),
	}

	// Initialize pattern counts
	for platform := range platformPatterns {
		output.PatternCounts[platform] = make(map[string][]string)
	}

	// Walk directory tree
	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip unreadable paths
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only scan text files
		if !isPlatformTextFile(d.Name()) {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		// Analyze file for platform markers
		compat := analyzeFileCompatibility(path, string(content))

		// Categorize
		switch compat.Category {
		case CrossPlatformVerified:
			output.CrossPlatform = append(output.CrossPlatform, compat)
		case PlatformSpecific:
			output.Specific = append(output.Specific, compat)
		case KnownIssues:
			output.Issues = append(output.Issues, compat)
		default:
			// Only add to unknown if it has some platform relevance
			if len(compat.Mentions) > 0 || len(compat.Patterns) > 0 {
				output.Unknown = append(output.Unknown, compat)
			}
		}

		// Update stats
		for _, platform := range compat.TestedOn {
			output.Stats[platform]++
		}
		for _, platform := range compat.Breaks {
			output.Stats[platform+"_breaks"]++
		}
		for _, platform := range compat.Mentions {
			output.Stats[platform+"_mentions"]++
		}

		// Track pattern usage
		for _, pattern := range compat.Patterns {
			for platform, patterns := range platformPatterns {
				for _, p := range patterns {
					re := regexp.MustCompile(`(?i)` + p)
					if re.MatchString(pattern) {
						output.PatternCounts[platform][pattern] = append(output.PatternCounts[platform][pattern], path)
						break
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return output, nil
}

// analyzeFileCompatibility examines a file for platform compatibility markers
func analyzeFileCompatibility(path, content string) FileCompatibility {
	compat := FileCompatibility{
		FilePath:    path,
		Category:    UnknownCompatibility,
		TestedOn:    []string{},
		Breaks:      []string{},
		Mentions:    []string{},
		Patterns:    []string{},
		Description: "",
	}

	// Create relative path for cleaner display
	homeDir, _ := os.UserHomeDir()
	compat.FilePath = strings.Replace(path, homeDir, "~", 1)

	lines := strings.Split(content, "\n")

	// Look for explicit markers
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// PLATFORM: marker
		if strings.Contains(line, "# PLATFORM:") || strings.Contains(line, "## PLATFORM:") {
			platforms := extractPlatformList(line)
			compat.Mentions = append(compat.Mentions, platforms...)
		}

		// TESTED: marker
		if strings.Contains(line, "# TESTED:") || strings.Contains(line, "## TESTED:") {
			platforms := extractPlatformList(line)
			compat.TestedOn = append(compat.TestedOn, platforms...)
		}

		// BREAKS: marker
		if strings.Contains(line, "# BREAKS:") || strings.Contains(line, "## BREAKS:") {
			platforms := extractPlatformList(line)
			compat.Breaks = append(compat.Breaks, platforms...)
		}
	}

	// Detect shebangs
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		shebang := lines[0]
		compat.Patterns = append(compat.Patterns, fmt.Sprintf("shebang: %s", shebang))

		// Bash/sh shebangs are generally cross-platform
		if strings.Contains(shebang, "/bin/bash") || strings.Contains(shebang, "/bin/sh") {
			if len(compat.TestedOn) == 0 && len(compat.Breaks) == 0 {
				compat.Description = "bash script"
			}
		}
	}

	// Look for platform checks in code
	contentLower := strings.ToLower(content)

	// uname checks
	if strings.Contains(contentLower, "uname") {
		compat.Patterns = append(compat.Patterns, "uname check")
	}

	// $OSTYPE checks
	if strings.Contains(content, "$OSTYPE") || strings.Contains(content, "${OSTYPE}") {
		compat.Patterns = append(compat.Patterns, "$OSTYPE check")
	}

	// Platform-specific paths
	if strings.Contains(content, "/usr/bin") || strings.Contains(content, "/etc/") {
		compat.Patterns = append(compat.Patterns, "unix paths")
		if !contains(compat.Mentions, "linux") && !contains(compat.Mentions, "darwin") {
			compat.Mentions = append(compat.Mentions, "linux/darwin")
		}
	}

	if strings.Contains(content, "C:\\") || strings.Contains(content, "%USERPROFILE%") {
		compat.Patterns = append(compat.Patterns, "windows paths")
		if !contains(compat.Mentions, "win32") {
			compat.Mentions = append(compat.Mentions, "win32")
		}
	}

	// Platform-specific commands
	if strings.Contains(contentLower, "wslpath") || strings.Contains(contentLower, "cygpath") {
		compat.Patterns = append(compat.Patterns, "path conversion tools")
		if !contains(compat.Mentions, "win32") {
			compat.Mentions = append(compat.Mentions, "win32")
		}
	}

	if strings.Contains(contentLower, "powershell") {
		compat.Patterns = append(compat.Patterns, "powershell")
		if !contains(compat.Mentions, "win32") {
			compat.Mentions = append(compat.Mentions, "win32")
		}
	}

	// Package managers
	for _, pm := range packageManagers {
		if strings.Contains(contentLower, pm) {
			compat.Patterns = append(compat.Patterns, fmt.Sprintf("package manager: %s", pm))

			// Infer platform
			if pm == "scoop" || pm == "chocolatey" || pm == "winget" {
				if !contains(compat.Mentions, "win32") {
					compat.Mentions = append(compat.Mentions, "win32")
				}
			} else if pm == "homebrew" || pm == "brew" {
				if !contains(compat.Mentions, "darwin") {
					compat.Mentions = append(compat.Mentions, "darwin")
				}
			} else if pm == "apt" || pm == "apt-get" || pm == "yum" || pm == "dnf" || pm == "pacman" {
				if !contains(compat.Mentions, "linux") {
					compat.Mentions = append(compat.Mentions, "linux")
				}
			}
		}
	}

	// Categorize based on findings
	if len(compat.Breaks) > 0 {
		compat.Category = KnownIssues
	} else if len(compat.TestedOn) >= 2 {
		compat.Category = CrossPlatformVerified
	} else if len(compat.Mentions) > 0 || len(compat.Patterns) > 0 {
		compat.Category = PlatformSpecific
	}

	// Deduplicate slices
	compat.TestedOn = deduplicate(compat.TestedOn)
	compat.Breaks = deduplicate(compat.Breaks)
	compat.Mentions = deduplicate(compat.Mentions)
	compat.Patterns = deduplicate(compat.Patterns)

	return compat
}

// extractPlatformList extracts comma-separated platforms from a marker line
func extractPlatformList(line string) []string {
	// Find the part after the colon
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return []string{}
	}

	// Split by comma and clean
	platformStr := strings.TrimSpace(parts[1])
	platforms := strings.Split(platformStr, ",")

	result := []string{}
	for _, p := range platforms {
		cleaned := strings.TrimSpace(p)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}

	return result
}

// isPlatformTextFile checks if a file is likely a text file based on extension
func isPlatformTextFile(filename string) bool {
	textExtensions := []string{
		".md", ".txt", ".sh", ".bash", ".zsh", ".fish",
		".py", ".rb", ".js", ".ts", ".go", ".rs", ".c", ".cpp", ".h",
		".yml", ".yaml", ".json", ".toml", ".xml",
		".ps1", ".bat", ".cmd",
		".conf", ".config", ".ini", ".env",
	}

	ext := strings.ToLower(filepath.Ext(filename))
	for _, valid := range textExtensions {
		if ext == valid {
			return true
		}
	}

	// Also check for extensionless files that might be scripts
	if ext == "" {
		return true
	}

	return false
}

// printPlatformMap prints human-readable output
func printPlatformMap(results *PlatformMapOutput, issuesOnly bool) {
	output.Success("🗺️  Platform Map")
	fmt.Println("")

	if !issuesOnly && len(results.CrossPlatform) > 0 {
		fmt.Println("✓ Cross-platform verified:")
		fmt.Println("")
		for _, f := range results.CrossPlatform {
			desc := ""
			if f.Description != "" {
				desc = fmt.Sprintf(" (%s)", f.Description)
			}
			fmt.Printf("  %s%s\n", output.Cyan+f.FilePath+output.Reset, desc)
			if len(f.TestedOn) > 0 {
				fmt.Printf("    Tested: %s\n", strings.Join(f.TestedOn, ", "))
			}
			if len(f.Patterns) > 0 {
				fmt.Printf("    Patterns: %s\n", output.Dim+strings.Join(f.Patterns, ", ")+output.Reset)
			}
			fmt.Println("")
		}
	}

	if len(results.Specific) > 0 {
		fmt.Println("⚠  Platform-specific:")
		fmt.Println("")
		for _, f := range results.Specific {
			fmt.Printf("  %s\n", output.Yellow+f.FilePath+output.Reset)
			if len(f.Mentions) > 0 {
				fmt.Printf("    Mentions: %s\n", strings.Join(f.Mentions, ", "))
			}
			if len(f.Patterns) > 0 {
				fmt.Printf("    Patterns: %s\n", output.Dim+strings.Join(f.Patterns, ", ")+output.Reset)
			}
			fmt.Println("")
		}
	}

	if len(results.Issues) > 0 {
		fmt.Println("✗ Known issues:")
		fmt.Println("")
		for _, f := range results.Issues {
			fmt.Printf("  %s\n", output.Red+f.FilePath+output.Reset)
			if len(f.Breaks) > 0 {
				fmt.Printf("    Breaks: %s\n", strings.Join(f.Breaks, ", "))
			}
			if len(f.Patterns) > 0 {
				fmt.Printf("    Patterns: %s\n", output.Dim+strings.Join(f.Patterns, ", ")+output.Reset)
			}
			fmt.Println("")
		}
	}

	if !issuesOnly && len(results.Unknown) > 0 {
		fmt.Println("? Unknown compatibility:")
		fmt.Println("")
		for _, f := range results.Unknown {
			fmt.Printf("  %s\n", output.Dim+f.FilePath+output.Reset)
			if len(f.Mentions) > 0 {
				fmt.Printf("    Mentions: %s\n", strings.Join(f.Mentions, ", "))
			}
			if len(f.Patterns) > 0 {
				fmt.Printf("    Patterns: %s\n", strings.Join(f.Patterns, ", ")+output.Reset)
			}
			fmt.Println("")
		}
	}

	// Print stats
	if len(results.Stats) > 0 {
		fmt.Println("Platform patterns found:")
		fmt.Println("")

		// Group stats by platform
		platforms := map[string]struct {
			count    int
			mentions int
			breaks   int
		}{}

		for key, value := range results.Stats {
			if strings.HasSuffix(key, "_mentions") {
				platform := strings.TrimSuffix(key, "_mentions")
				p := platforms[platform]
				p.mentions = value
				platforms[platform] = p
			} else if strings.HasSuffix(key, "_breaks") {
				platform := strings.TrimSuffix(key, "_breaks")
				p := platforms[platform]
				p.breaks = value
				platforms[platform] = p
			} else {
				p := platforms[key]
				p.count = value
				platforms[key] = p
			}
		}

		// Sort platform names
		names := make([]string, 0, len(platforms))
		for name := range platforms {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			p := platforms[name]
			parts := []string{}

			if p.count > 0 {
				parts = append(parts, fmt.Sprintf("tested: %d", p.count))
			}
			if p.mentions > 0 {
				parts = append(parts, fmt.Sprintf("mentioned: %d", p.mentions))
			}
			if p.breaks > 0 {
				parts = append(parts, fmt.Sprintf("%sbreaks: %d%s", output.Red, p.breaks, output.Reset))
			}

			if len(parts) > 0 {
				fmt.Printf("  %s: %s\n", name, strings.Join(parts, ", "))
			}
		}
		fmt.Println("")
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func deduplicate(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	sort.Strings(result)
	return result
}
