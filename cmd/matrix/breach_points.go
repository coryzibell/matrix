package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// Severity levels for security findings
type Severity int

const (
	SeverityLow Severity = iota + 1
	SeverityMedium
	SeverityHigh
)

func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "LOW"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityHigh:
		return "HIGH"
	default:
		return "UNKNOWN"
	}
}

func (s Severity) Color() string {
	switch s {
	case SeverityLow:
		return output.Cyan
	case SeverityMedium:
		return output.Yellow
	case SeverityHigh:
		return "\033[31m" // Red
	default:
		return output.Reset
	}
}

// Finding represents a security issue discovered
type Finding struct {
	Severity       Severity
	Category       string // credentials, permissions, injection, staleness
	FilePath       string
	Line           int
	Description    string
	MatchedContent string
	Recommendation string
}

// ScanConfig holds configuration for the breach-points scan
type ScanConfig struct {
	TargetPath      string
	ScanCredentials bool
	ScanPermissions bool
	ScanInjection   bool
	ScanStaleness   bool
	StaleDays       int
	OutputJSON      bool
	FailOnLevel     Severity
}

// runBreachPoints implements the breach-points command
func runBreachPoints() error {
	config := parseBPFlags()

	// Default scan mode: all if no specific scan is requested
	if !config.ScanCredentials && !config.ScanPermissions && !config.ScanInjection && !config.ScanStaleness {
		config.ScanCredentials = true
		config.ScanPermissions = true
		config.ScanInjection = true
		config.ScanStaleness = true
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

	// Run scans
	findings := []Finding{}

	if config.ScanCredentials {
		credFindings := scanCredentials(absPath)
		findings = append(findings, credFindings...)
	}

	if config.ScanPermissions {
		permFindings := scanPermissions(absPath)
		findings = append(findings, permFindings...)
	}

	if config.ScanInjection {
		injFindings := scanInjection(absPath)
		findings = append(findings, injFindings...)
	}

	if config.ScanStaleness {
		staleFindings := scanStaleness(absPath, config.StaleDays)
		findings = append(findings, staleFindings...)
	}

	// Output results
	if config.OutputJSON {
		outputBPJSON(findings)
	} else {
		outputText(findings, absPath)
	}

	// Determine exit code
	exitCode := determineExitCode(findings, config.FailOnLevel)
	if exitCode > 0 {
		os.Exit(exitCode)
	}

	return nil
}

// parseBPFlags parses command-line flags for breach-points
func parseBPFlags() ScanConfig {
	config := ScanConfig{
		TargetPath:  "",
		StaleDays:   90,
		FailOnLevel: 0,
	}

	// Default RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err == nil {
		config.TargetPath = ramDir
	} else {
		config.TargetPath = "."
	}

	args := os.Args[2:] // Skip "matrix" and "breach-points"

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--scan" && i+1 < len(args):
			i++
			scanType := args[i]
			switch scanType {
			case "credentials":
				config.ScanCredentials = true
			case "permissions":
				config.ScanPermissions = true
			case "injection":
				config.ScanInjection = true
			case "staleness":
				config.ScanStaleness = true
			}

		case arg == "--all":
			config.ScanCredentials = true
			config.ScanPermissions = true
			config.ScanInjection = true
			config.ScanStaleness = true

		case arg == "--path" && i+1 < len(args):
			i++
			config.TargetPath = args[i]

		case arg == "--days" && i+1 < len(args):
			i++
			days, err := strconv.Atoi(args[i])
			if err == nil && days > 0 {
				config.StaleDays = days
			}

		case arg == "--format" && i+1 < len(args):
			i++
			if args[i] == "json" {
				config.OutputJSON = true
			}

		case arg == "--fail-on" && i+1 < len(args):
			i++
			level := strings.ToLower(args[i])
			switch level {
			case "low":
				config.FailOnLevel = SeverityLow
			case "medium":
				config.FailOnLevel = SeverityMedium
			case "high":
				config.FailOnLevel = SeverityHigh
			}
		}
	}

	return config
}

// scanCredentials searches for exposed credentials
func scanCredentials(rootPath string) []Finding {
	var findings []Finding

	// Credential patterns
	patterns := []struct {
		regex       *regexp.Regexp
		description string
		severity    Severity
	}{
		// High severity - obvious secrets
		{regexp.MustCompile(`(?i)(aws_access_key_id|AWS_ACCESS_KEY_ID)\s*[=:]\s*["']?([A-Z0-9]{20})["']?`), "AWS Access Key ID", SeverityHigh},
		{regexp.MustCompile(`(?i)(aws_secret_access_key|AWS_SECRET_ACCESS_KEY)\s*[=:]\s*["']?([A-Za-z0-9/+=]{40})["']?`), "AWS Secret Access Key", SeverityHigh},
		{regexp.MustCompile(`(?i)(github_token|GITHUB_TOKEN|GH_TOKEN)\s*[=:]\s*["']?(ghp_[A-Za-z0-9]{36})["']?`), "GitHub Personal Access Token", SeverityHigh},
		{regexp.MustCompile(`(?i)(github_token|GITHUB_TOKEN|GH_TOKEN)\s*[=:]\s*["']?(gho_[A-Za-z0-9]{36})["']?`), "GitHub OAuth Token", SeverityHigh},
		{regexp.MustCompile(`(?i)(private[_-]?key|PRIVATE[_-]?KEY)\s*[=:]\s*["']?(-+BEGIN\s+[A-Z\s]+PRIVATE\s+KEY-+)`), "Private Key", SeverityHigh},
		{regexp.MustCompile(`(?i)(sk_live_[A-Za-z0-9]{24,})`), "Stripe Live Secret Key", SeverityHigh},

		// Medium severity - potential secrets
		{regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*["']([^"'\s]{8,})["']`), "Hardcoded password", SeverityMedium},
		{regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[=:]\s*["']([^"'\s]{16,})["']`), "API Key", SeverityMedium},
		{regexp.MustCompile(`(?i)(secret|token)\s*[=:]\s*["']([A-Za-z0-9+/=]{32,})["']`), "Secret or Token", SeverityMedium},
		{regexp.MustCompile(`(?i)(database[_-]?url|db[_-]?url)\s*[=:]\s*["']?(postgres|mysql|mongodb)://[^"'\s]+["']?`), "Database URL with credentials", SeverityMedium},

		// JWT tokens
		{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`), "JWT Token", SeverityMedium},
	}

	// Walk directory
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || shouldSkipFile(path, info) {
			if info != nil && info.IsDir() && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only scan text files
		if !isBPTextFile(strings.ToLower(filepath.Ext(path))) {
			return nil
		}

		// Read file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Check each pattern
			for _, pattern := range patterns {
				if pattern.regex.MatchString(line) {
					relPath, _ := filepath.Rel(rootPath, path)
					findings = append(findings, Finding{
						Severity:       pattern.severity,
						Category:       "credentials",
						FilePath:       relPath,
						Line:           lineNum,
						Description:    pattern.description + " exposed",
						MatchedContent: sanitizeSecret(line),
						Recommendation: "Move to secure credential store (environment variables, secrets manager)",
					})
				}
			}
		}

		return nil
	})

	return findings
}

// scanPermissions checks for overly permissive files containing sensitive data
func scanPermissions(rootPath string) []Finding {
	var findings []Finding

	// Sensitive file patterns
	sensitivePatterns := []string{
		"password", "secret", "token", "key", "credential", "auth",
		"private", "confidential", ".env", "config",
	}

	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipFile(path, info) {
			return nil
		}

		// Check if filename suggests sensitive content
		filename := strings.ToLower(filepath.Base(path))
		isSensitive := false
		for _, pattern := range sensitivePatterns {
			if strings.Contains(filename, pattern) {
				isSensitive = true
				break
			}
		}

		if !isSensitive {
			return nil
		}

		// Check permissions
		mode := info.Mode()
		perm := mode.Perm()

		// Check if world-readable (others have read permission)
		if perm&0004 != 0 {
			relPath, _ := filepath.Rel(rootPath, path)
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       "permissions",
				FilePath:       relPath,
				Line:           0,
				Description:    fmt.Sprintf("Overly permissive file (%s)", mode.String()),
				MatchedContent: fmt.Sprintf("File permissions: %o", perm),
				Recommendation: "chmod 600 (owner read/write only)",
			})
		}

		// Check if group-readable on sensitive files
		if perm&0040 != 0 {
			relPath, _ := filepath.Rel(rootPath, path)
			findings = append(findings, Finding{
				Severity:       SeverityLow,
				Category:       "permissions",
				FilePath:       relPath,
				Line:           0,
				Description:    fmt.Sprintf("Group-readable sensitive file (%s)", mode.String()),
				MatchedContent: fmt.Sprintf("File permissions: %o", perm),
				Recommendation: "chmod 600 (owner read/write only)",
			})
		}

		return nil
	})

	return findings
}

// scanInjection checks shell scripts for injection vulnerabilities
func scanInjection(rootPath string) []Finding {
	var findings []Finding

	// Injection patterns
	patterns := []struct {
		regex       *regexp.Regexp
		description string
		severity    Severity
		recommendation string
	}{
		{
			regexp.MustCompile(`\beval\s+`),
			"Use of eval",
			SeverityHigh,
			"Avoid eval; use safer alternatives",
		},
		{
			regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*\s`),
			"Potentially unquoted variable",
			SeverityMedium,
			"Quote variables: \"$VAR\" to prevent word splitting",
		},
		{
			regexp.MustCompile(`\$\{[^}]+\}\s`),
			"Potentially unquoted parameter expansion",
			SeverityMedium,
			"Quote expansions: \"${VAR}\" to prevent injection",
		},
		{
			regexp.MustCompile(`\$\([^)]+\)\s`),
			"Potentially unquoted command substitution",
			SeverityMedium,
			"Quote command substitution: \"$(cmd)\" to prevent injection",
		},
		{
			regexp.MustCompile(`rm\s+-rf\s+\$`),
			"Dangerous rm -rf with variable",
			SeverityHigh,
			"Use absolute paths and validate variables before destructive operations",
		},
	}

	// Walk directory
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipFile(path, info) {
			return nil
		}

		// Only scan shell scripts
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".sh" && ext != ".bash" {
			return nil
		}

		// Read file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Skip comments and empty lines
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}

			// Check each pattern
			for _, pattern := range patterns {
				if pattern.regex.MatchString(line) {
					relPath, _ := filepath.Rel(rootPath, path)
					findings = append(findings, Finding{
						Severity:       pattern.severity,
						Category:       "injection",
						FilePath:       relPath,
						Line:           lineNum,
						Description:    pattern.description,
						MatchedContent: strings.TrimSpace(line),
						Recommendation: pattern.recommendation,
					})
				}
			}
		}

		return nil
	})

	return findings
}

// scanStaleness finds old files that may contain sensitive data
func scanStaleness(rootPath string, staleDays int) []Finding {
	var findings []Finding

	threshold := time.Now().AddDate(0, 0, -staleDays)

	// Sensitive patterns
	sensitivePatterns := []string{
		"password", "secret", "token", "key", "credential",
		"debug", "trace", "log",
	}

	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipFile(path, info) {
			return nil
		}

		// Check if file is old
		if info.ModTime().After(threshold) {
			return nil
		}

		// Check if file might contain sensitive data
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := strings.ToLower(string(content))
		hasSensitive := false
		for _, pattern := range sensitivePatterns {
			if strings.Contains(contentStr, pattern) {
				hasSensitive = true
				break
			}
		}

		if hasSensitive {
			relPath, _ := filepath.Rel(rootPath, path)
			daysSinceModified := int(time.Since(info.ModTime()).Hours() / 24)

			findings = append(findings, Finding{
				Severity:       SeverityLow,
				Category:       "staleness",
				FilePath:       relPath,
				Line:           0,
				Description:    fmt.Sprintf("Stale file with sensitive content (%d days old)", daysSinceModified),
				MatchedContent: fmt.Sprintf("Last modified: %s", info.ModTime().Format("2006-01-02")),
				Recommendation: "Review and archive/delete if no longer needed",
			})
		}

		return nil
	})

	return findings
}

// shouldSkipDir returns true if directory should be skipped
func shouldSkipDir(name string) bool {
	skipDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"target":       true,
		"build":        true,
		"dist":         true,
		"__pycache__":  true,
	}
	return skipDirs[name] || strings.HasPrefix(name, ".")
}

// shouldSkipFile returns true if file should be skipped
func shouldSkipFile(path string, info os.FileInfo) bool {
	// Skip large files (> 10MB)
	if info.Size() > 10*1024*1024 {
		return true
	}

	// Skip hidden files
	if strings.HasPrefix(info.Name(), ".") {
		return true
	}

	return false
}

// isBPTextFile returns true if extension is likely text
func isBPTextFile(ext string) bool {
	textExts := map[string]bool{
		".go": true, ".rs": true, ".js": true, ".ts": true, ".py": true,
		".java": true, ".c": true, ".cpp": true, ".cs": true, ".rb": true,
		".php": true, ".sh": true, ".bash": true, ".md": true, ".txt": true,
		".yml": true, ".yaml": true, ".json": true, ".toml": true, ".xml": true,
		".env": true, ".conf": true, ".config": true, ".ini": true,
	}
	return textExts[ext]
}

// sanitizeSecret redacts part of the secret for display
func sanitizeSecret(line string) string {
	// Simple redaction: show first and last few chars
	if len(line) > 50 {
		return line[:25] + "..." + line[len(line)-10:]
	}
	return line
}

// outputText outputs findings in human-readable format
func outputText(findings []Finding, targetPath string) {
	if len(findings) == 0 {
		output.Success("🔒 No breach points detected")
		fmt.Printf("Target: %s\n", targetPath)
		return
	}

	// Group by severity
	bySeverity := make(map[Severity][]Finding)
	for _, f := range findings {
		bySeverity[f.Severity] = append(bySeverity[f.Severity], f)
	}

	fmt.Printf("\n🚨 Breach Points Detected\n")
	fmt.Printf("Target: %s\n\n", targetPath)

	// Output in order: High, Medium, Low
	for _, sev := range []Severity{SeverityHigh, SeverityMedium, SeverityLow} {
		items := bySeverity[sev]
		if len(items) == 0 {
			continue
		}

		for _, finding := range items {
			color := finding.Severity.Color()
			fmt.Printf("%s[%s]%s %s\n", color, finding.Severity.String(), output.Reset, finding.Description)

			if finding.Line > 0 {
				fmt.Printf("  File: %s:%d\n", finding.FilePath, finding.Line)
			} else {
				fmt.Printf("  File: %s\n", finding.FilePath)
			}

			if finding.MatchedContent != "" {
				fmt.Printf("  Match: %s\n", finding.MatchedContent)
			}

			fmt.Printf("  %sRecommendation:%s %s\n", output.Yellow, output.Reset, finding.Recommendation)
			fmt.Println()
		}
	}

	// Summary
	fmt.Printf("Summary: %d findings (%d high, %d medium, %d low)\n",
		len(findings),
		len(bySeverity[SeverityHigh]),
		len(bySeverity[SeverityMedium]),
		len(bySeverity[SeverityLow]))
}

// outputBPJSON outputs findings in JSON format
func outputBPJSON(findings []Finding) {
	fmt.Println("[")
	for i, f := range findings {
		comma := ","
		if i == len(findings)-1 {
			comma = ""
		}

		fmt.Printf("  {\n")
		fmt.Printf("    \"severity\": \"%s\",\n", f.Severity.String())
		fmt.Printf("    \"category\": \"%s\",\n", f.Category)
		fmt.Printf("    \"file\": \"%s\",\n", escapeJSON(f.FilePath))

		if f.Line > 0 {
			fmt.Printf("    \"line\": %d,\n", f.Line)
		}

		fmt.Printf("    \"description\": \"%s\",\n", escapeJSON(f.Description))
		fmt.Printf("    \"matched_content\": \"%s\",\n", escapeJSON(f.MatchedContent))
		fmt.Printf("    \"recommendation\": \"%s\"\n", escapeJSON(f.Recommendation))
		fmt.Printf("  }%s\n", comma)
	}
	fmt.Println("]")
}

// escapeJSON escapes strings for JSON output
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// determineExitCode returns appropriate exit code based on findings
func determineExitCode(findings []Finding, failOnLevel Severity) int {
	if failOnLevel == 0 {
		return 0
	}

	maxSeverity := SeverityLow
	hasFindings := false

	for _, f := range findings {
		hasFindings = true
		if f.Severity > maxSeverity {
			maxSeverity = f.Severity
		}
	}

	if !hasFindings {
		return 0
	}

	// Exit with severity level if >= failOnLevel
	if maxSeverity >= failOnLevel {
		return int(maxSeverity)
	}

	return 0
}
