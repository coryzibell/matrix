package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/output"
)

// SecurityCategory represents a type of security-relevant finding
type SecurityCategory int

const (
	CategoryAuth SecurityCategory = iota
	CategoryAuthz
	CategorySecrets
	CategoryTrust
	CategoryCrypto
)

func (c SecurityCategory) String() string {
	switch c {
	case CategoryAuth:
		return "authentication"
	case CategoryAuthz:
		return "authorization"
	case CategorySecrets:
		return "secrets"
	case CategoryTrust:
		return "boundaries"
	case CategoryCrypto:
		return "crypto"
	default:
		return "unknown"
	}
}

func (c SecurityCategory) Title() string {
	switch c {
	case CategoryAuth:
		return "AUTHENTICATION POINTS"
	case CategoryAuthz:
		return "AUTHORIZATION BOUNDARIES"
	case CategorySecrets:
		return "SECRET LOCATIONS"
	case CategoryTrust:
		return "TRUST BOUNDARIES"
	case CategoryCrypto:
		return "CRYPTOGRAPHIC OPERATIONS"
	default:
		return "UNKNOWN"
	}
}

func (c SecurityCategory) Icon() string {
	switch c {
	case CategoryAuth:
		return "📍"
	case CategoryAuthz:
		return "🛡️"
	case CategorySecrets:
		return "⚠️"
	case CategoryTrust:
		return "🚪"
	case CategoryCrypto:
		return "🔐"
	default:
		return "•"
	}
}

// VaultKey represents a security-relevant code location
type VaultKey struct {
	Category    SecurityCategory
	FilePath    string
	Line        int
	Pattern     string
	Context     string
	Description string
}

// VaultKeysConfig holds scan configuration
type VaultKeysConfig struct {
	TargetPath string
	Focus      string // auth, secrets, crypto, boundaries, authz
	OutputJSON bool
}

// runVaultKeys implements the vault-keys command
func runVaultKeys() error {
	config := parseVKFlags()

	// Resolve target path
	absPath, err := filepath.Abs(config.TargetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Scan for vault keys
	keys, filesScanned := scanVaultKeys(absPath, config.Focus)

	// Output results
	if config.OutputJSON {
		outputVKJSON(keys, absPath, filesScanned)
	} else {
		outputVKText(keys, absPath, filesScanned)
	}

	return nil
}

// parseVKFlags parses command-line flags for vault-keys
func parseVKFlags() VaultKeysConfig {
	config := VaultKeysConfig{
		TargetPath: ".",
		Focus:      "",
		OutputJSON: false,
	}

	args := os.Args[2:] // Skip "matrix" and "vault-keys"

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--focus" && i+1 < len(args):
			i++
			focusInput := args[i]
			// Map user input to category strings
			switch focusInput {
			case "auth", "authentication":
				config.Focus = "authentication"
			case "authz", "authorization":
				config.Focus = "authorization"
			case "secrets":
				config.Focus = "secrets"
			case "boundaries":
				config.Focus = "boundaries"
			case "crypto":
				config.Focus = "crypto"
			default:
				config.Focus = focusInput
			}
		case arg == "--json":
			config.OutputJSON = true
		case !strings.HasPrefix(arg, "-"):
			config.TargetPath = arg
		}
	}

	return config
}

// scanVaultKeys scans directory for security-relevant patterns
func scanVaultKeys(rootPath string, focus string) ([]VaultKey, int) {
	var keys []VaultKey
	filesScanned := 0

	// Define search patterns
	patterns := buildPatternSet()

	// Walk directory
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && shouldSkipVKDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip non-code files
		if !isVKCodeFile(path) {
			return nil
		}

		// Skip large files
		if info.Size() > 5*1024*1024 {
			return nil
		}

		filesScanned++

		// Scan file
		fileKeys := scanFileForPatterns(rootPath, path, patterns, focus)
		keys = append(keys, fileKeys...)

		return nil
	})

	// Sort by category, then file, then line
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Category != keys[j].Category {
			return keys[i].Category < keys[j].Category
		}
		if keys[i].FilePath != keys[j].FilePath {
			return keys[i].FilePath < keys[j].FilePath
		}
		return keys[i].Line < keys[j].Line
	})

	return keys, filesScanned
}

// PatternDef defines a pattern to search for
type PatternDef struct {
	Regex       *regexp.Regexp
	Category    SecurityCategory
	Pattern     string
	Description string
}

// buildPatternSet creates the pattern definitions
func buildPatternSet() []PatternDef {
	return []PatternDef{
		// Authentication patterns
		{regexp.MustCompile(`(?i)\b(jwt|jsonwebtoken|bearer|authenticate|login|logout|signin|signout)\b`), CategoryAuth, "JWT/OAuth", "Authentication keyword"},
		{regexp.MustCompile(`(?i)\b(session|passport|bcrypt|argon2|pbkdf2)\b`), CategoryAuth, "Auth library", "Authentication library usage"},
		{regexp.MustCompile(`(?i)(\.isAuthenticated|@RequiresAuth|requireAuth|ensureAuth)`), CategoryAuth, "Auth check", "Authentication check"},
		{regexp.MustCompile(`(?i)(token\.verify|verifyToken|validateToken|checkToken)`), CategoryAuth, "Token validation", "Token verification"},

		// Authorization patterns
		{regexp.MustCompile(`(?i)\b(hasRole|checkRole|requireRole|isAdmin)\b`), CategoryAuthz, "Role check", "Role-based authorization"},
		{regexp.MustCompile(`(?i)\b(authorize|permission|can|cannot|ability)\b`), CategoryAuthz, "Permission check", "Permission verification"},
		{regexp.MustCompile(`(?i)\b(acl|rbac|accessControl)\b`), CategoryAuthz, "Access control", "Access control system"},
		{regexp.MustCompile(`(?i)(authMiddleware|requireAdmin|checkPermission)`), CategoryAuthz, "Auth middleware", "Authorization middleware"},

		// Secret patterns
		{regexp.MustCompile(`(?i)(process\.env|os\.getenv|ENV\[|System\.getenv)`), CategorySecrets, "Env variable", "Environment variable access"},
		{regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret[_-]?key|secretkey|private[_-]?key|privatekey)`), CategorySecrets, "API key", "API key or secret reference"},
		{regexp.MustCompile(`(?i)(password|passwd|credential|token).*=.*["'][^"']{8,}["']`), CategorySecrets, "Hardcoded secret", "Potential hardcoded credential"},
		{regexp.MustCompile(`(?i)(\.env|secrets\.yaml|credentials\.json|config\.json)`), CategorySecrets, "Config file", "Configuration file reference"},

		// Trust boundary patterns
		{regexp.MustCompile(`(@app\.route|@router\.|router\.get|router\.post|@PostMapping|@GetMapping|\[HttpGet\]|\[HttpPost\])`), CategoryTrust, "API route", "API endpoint definition"},
		{regexp.MustCompile(`\b(fetch|axios|http\.get|http\.post|requests\.get|requests\.post|HttpClient)\b`), CategoryTrust, "External call", "External API call"},
		{regexp.MustCompile(`(?i)(database|db\.connect|mongodb|postgres|mysql|redis)`), CategoryTrust, "Database", "Database connection"},
		{regexp.MustCompile(`(?i)(cors|crossOrigin|allowOrigin)`), CategoryTrust, "CORS", "Cross-origin configuration"},

		// Cryptographic patterns
		{regexp.MustCompile(`\b(encrypt|decrypt|cipher|decipher)\b`), CategoryCrypto, "Encryption", "Encryption/decryption operation"},
		{regexp.MustCompile(`\b(hash|sha256|sha512|md5|blake2)\b`), CategoryCrypto, "Hashing", "Cryptographic hashing"},
		{regexp.MustCompile(`\b(sign|verify|signature)\b`), CategoryCrypto, "Signing", "Digital signature operation"},
		{regexp.MustCompile(`\b(crypto|sodium|openssl|aes|rsa)\b`), CategoryCrypto, "Crypto library", "Cryptographic library usage"},
	}
}

// scanFileForPatterns scans a file for security patterns
func scanFileForPatterns(rootPath, filePath string, patterns []PatternDef, focus string) []VaultKey {
	var keys []VaultKey

	file, err := os.Open(filePath)
	if err != nil {
		return keys
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check each pattern
		for _, pattern := range patterns {
			// Apply focus filter
			if focus != "" && pattern.Category.String() != focus {
				continue
			}

			if pattern.Regex.MatchString(line) {
				relPath, _ := filepath.Rel(rootPath, filePath)

				keys = append(keys, VaultKey{
					Category:    pattern.Category,
					FilePath:    relPath,
					Line:        lineNum,
					Pattern:     pattern.Pattern,
					Description: pattern.Description,
					Context:     strings.TrimSpace(line),
				})

				// Only match once per line
				break
			}
		}
	}

	return keys
}

// shouldSkipVKDir returns true if directory should be skipped
func shouldSkipVKDir(name string) bool {
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

// isVKCodeFile returns true if file extension indicates code
func isVKCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExts := map[string]bool{
		".go": true, ".rs": true, ".js": true, ".ts": true, ".py": true,
		".java": true, ".c": true, ".cpp": true, ".cs": true, ".rb": true,
		".php": true, ".sh": true, ".bash": true, ".jsx": true, ".tsx": true,
		".kt": true, ".swift": true, ".scala": true, ".clj": true,
	}
	return codeExts[ext]
}

// outputVKText outputs vault keys in human-readable format
func outputVKText(keys []VaultKey, targetPath string, filesScanned int) {
	fmt.Println()
	output.Success("🔑 Vault Keys Report")
	fmt.Printf("Repository: %s\n", targetPath)
	fmt.Printf("Scanned: %d files\n", filesScanned)
	fmt.Println()

	if len(keys) == 0 {
		fmt.Println("No security-relevant patterns detected.")
		return
	}

	// Group by category
	byCategory := make(map[SecurityCategory][]VaultKey)
	for _, key := range keys {
		byCategory[key.Category] = append(byCategory[key.Category], key)
	}

	// Output each category
	categories := []SecurityCategory{CategoryAuth, CategoryAuthz, CategorySecrets, CategoryTrust, CategoryCrypto}
	for _, cat := range categories {
		items := byCategory[cat]
		if len(items) == 0 {
			continue
		}

		fmt.Printf("═══ %s (%d) ═══\n\n", cat.Title(), len(items))

		for _, key := range items {
			fmt.Printf("%s %s:%d\n", cat.Icon(), key.FilePath, key.Line)
			fmt.Printf("   Pattern: %s\n", key.Pattern)

			// Truncate long context lines
			context := key.Context
			if len(context) > 80 {
				context = context[:77] + "..."
			}
			fmt.Printf("   Context: %s\n", context)
			fmt.Println()
		}
	}

	// Summary
	fmt.Println("SUMMARY:")
	fmt.Printf("Total: %d", len(keys))
	for _, cat := range categories {
		count := len(byCategory[cat])
		if count > 0 {
			fmt.Printf(" | %s: %d", cat.String(), count)
		}
	}
	fmt.Println()
}

// outputVKJSON outputs vault keys in JSON format
func outputVKJSON(keys []VaultKey, targetPath string, filesScanned int) {
	fmt.Println("{")
	fmt.Printf("  \"repository\": \"%s\",\n", escapeVKJSON(targetPath))
	fmt.Printf("  \"files_scanned\": %d,\n", filesScanned)
	fmt.Printf("  \"total_findings\": %d,\n", len(keys))
	fmt.Println("  \"findings\": [")

	for i, key := range keys {
		comma := ","
		if i == len(keys)-1 {
			comma = ""
		}

		fmt.Println("    {")
		fmt.Printf("      \"category\": \"%s\",\n", key.Category.String())
		fmt.Printf("      \"file\": \"%s\",\n", escapeVKJSON(key.FilePath))
		fmt.Printf("      \"line\": %d,\n", key.Line)
		fmt.Printf("      \"pattern\": \"%s\",\n", escapeVKJSON(key.Pattern))
		fmt.Printf("      \"description\": \"%s\",\n", escapeVKJSON(key.Description))
		fmt.Printf("      \"context\": \"%s\"\n", escapeVKJSON(key.Context))
		fmt.Printf("    }%s\n", comma)
	}

	fmt.Println("  ]")
	fmt.Println("}")
}

// escapeVKJSON escapes strings for JSON output
func escapeVKJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
