package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/output"
)

// ProjectInfo contains reconnaissance data about a codebase
type ProjectInfo struct {
	Path           string
	Language       string
	Framework      string
	BuildSystem    string
	TotalFiles     int
	CodeFiles      int
	TestFiles      int
	EntryPoints    []EntryPoint
	Architecture   ArchitectureInfo
	Dependencies   []Dependency
	Documentation  DocInfo
	HealthIndicators HealthInfo
	ScanType       string
	Timestamp      time.Time
}

// EntryPoint represents a key file in the codebase
type EntryPoint struct {
	Path        string
	Type        string // main, test, config
	Description string
}

// ArchitectureInfo describes the structural patterns
type ArchitectureInfo struct {
	Pattern     string   // layered, mvc, microservices, monolith
	Directories []string // key directories found
	KeyModules  []ModuleInfo
}

// ModuleInfo describes a module or component
type ModuleInfo struct {
	Path      string
	FileCount int
}

// Dependency represents an external dependency
type Dependency struct {
	Name    string
	Version string
	Source  string // which file it came from
}

// DocInfo tracks documentation availability
type DocInfo struct {
	HasReadme      bool
	ReadmeLines    int
	HasDocsDir     bool
	InlineComments int // percentage
	Examples       bool
}

// HealthInfo tracks code health indicators
type HealthInfo struct {
	TODOs           []CodeMarker
	FIXMEs          []CodeMarker
	SecurityConcerns []CodeMarker
	DeadCodeSignals []string
}

// CodeMarker represents a comment marker with location
type CodeMarker struct {
	File    string
	Line    int
	Content string
}

// runRecon implements the recon command
func runRecon() error {
	// Parse flags
	fs := flag.NewFlagSet("recon", flag.ExitOnError)
	quickFlag := fs.Bool("quick", false, "Fast overview, skip deep analysis")
	focusFlag := fs.String("focus", "", "Focus on specific aspect: security, architecture, docs")

	// Parse remaining args (after "recon")
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	// Get target path (default to current directory)
	targetPath := "."
	if fs.NArg() > 0 {
		targetPath = fs.Arg(0)
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Validate focus flag
	if *focusFlag != "" {
		validFocus := map[string]bool{"security": true, "architecture": true, "docs": true}
		if !validFocus[*focusFlag] {
			return fmt.Errorf("invalid focus option: %s (valid: security, architecture, docs)", *focusFlag)
		}
	}

	// Run reconnaissance
	output.Success("🔍 Reconnaissance Scanner")
	fmt.Println("")
	fmt.Printf("Target: %s\n", absPath)

	scanType := "full"
	if *quickFlag {
		scanType = "quick"
	}
	if *focusFlag != "" {
		scanType = fmt.Sprintf("focused (%s)", *focusFlag)
	}
	fmt.Printf("Scan Type: %s\n", scanType)
	fmt.Println("")
	fmt.Println("Scanning...")
	fmt.Println("")

	// Scan the target
	info, err := scanDirectory(absPath, *quickFlag, *focusFlag)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Display report
	displayReconReport(info, *focusFlag)

	return nil
}

// scanDirectory performs the reconnaissance scan
func scanDirectory(path string, quick bool, focus string) (*ProjectInfo, error) {
	info := &ProjectInfo{
		Path:      path,
		ScanType:  "full",
		Timestamp: time.Now(),
	}

	if quick {
		info.ScanType = "quick"
	}

	// Track file types
	fileExtensions := make(map[string]int)
	var allFiles []string

	// Walk the directory tree
	err := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't read
		}

		// Skip common ignore patterns
		if shouldSkip(filePath, fileInfo) {
			if fileInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !fileInfo.IsDir() {
			info.TotalFiles++
			allFiles = append(allFiles, filePath)

			// Track extensions
			ext := strings.ToLower(filepath.Ext(filePath))
			if ext != "" {
				fileExtensions[ext]++
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Detect language from file extensions
	info.Language = detectLanguage(fileExtensions)
	info.CodeFiles = countCodeFiles(fileExtensions)

	// Detect framework and build system
	info.Framework, info.BuildSystem = detectProjectType(path)

	// Find entry points
	info.EntryPoints = findEntryPoints(path, allFiles, info.Language)

	// Analyze architecture (unless quick mode)
	if !quick || focus == "architecture" {
		info.Architecture = analyzeArchitecture(path, allFiles, info.Language)
	}

	// Find dependencies
	if focus == "" || focus == "security" {
		info.Dependencies = findDependencies(path)
	}

	// Analyze documentation
	if !quick || focus == "docs" {
		info.Documentation = analyzeDocumentation(path, allFiles)
	}

	// Health indicators
	if !quick || focus == "security" {
		info.HealthIndicators = analyzeHealth(path, allFiles, quick, focus)
	}

	return info, nil
}

// shouldSkip returns true if the file/directory should be skipped
func shouldSkip(path string, info os.FileInfo) bool {
	name := info.Name()

	// Skip hidden files/dirs
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}

	// Skip common build/dependency directories
	skipDirs := map[string]bool{
		"node_modules": true,
		"target":       true,
		"build":        true,
		"dist":         true,
		"vendor":       true,
		"__pycache__":  true,
		".git":         true,
		".svn":         true,
		".hg":          true,
	}

	if info.IsDir() && skipDirs[name] {
		return true
	}

	// Skip binary files by extension
	skipExts := map[string]bool{
		".exe":  true,
		".dll":  true,
		".so":   true,
		".dylib": true,
		".o":    true,
		".a":    true,
		".bin":  true,
		".pdf":  true,
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".mp4":  true,
		".avi":  true,
		".zip":  true,
		".tar":  true,
		".gz":   true,
	}

	ext := strings.ToLower(filepath.Ext(name))
	return skipExts[ext]
}

// detectLanguage determines the primary language from file extensions
func detectLanguage(extensions map[string]int) string {
	// Map extensions to languages
	languageMap := map[string]string{
		".go":   "Go",
		".rs":   "Rust",
		".js":   "JavaScript",
		".ts":   "TypeScript",
		".py":   "Python",
		".java": "Java",
		".c":    "C",
		".cpp":  "C++",
		".cs":   "C#",
		".rb":   "Ruby",
		".php":  "PHP",
		".swift": "Swift",
		".kt":   "Kotlin",
		".sh":   "Shell",
		".bash": "Bash",
	}

	// Count by language
	languageCounts := make(map[string]int)
	for ext, count := range extensions {
		if lang, exists := languageMap[ext]; exists {
			languageCounts[lang] += count
		}
	}

	// Find most common
	maxCount := 0
	primaryLang := "Unknown"
	for lang, count := range languageCounts {
		if count > maxCount {
			maxCount = count
			primaryLang = lang
		}
	}

	return primaryLang
}

// countCodeFiles counts files likely to be source code
func countCodeFiles(extensions map[string]int) int {
	codeExts := map[string]bool{
		".go": true, ".rs": true, ".js": true, ".ts": true, ".py": true,
		".java": true, ".c": true, ".cpp": true, ".cs": true, ".rb": true,
		".php": true, ".swift": true, ".kt": true, ".sh": true, ".bash": true,
	}

	count := 0
	for ext, fileCount := range extensions {
		if codeExts[ext] {
			count += fileCount
		}
	}
	return count
}

// detectProjectType detects framework and build system
func detectProjectType(path string) (framework, buildSystem string) {
	framework = "None detected"
	buildSystem = "None detected"

	// Check for known files
	checks := map[string]struct {
		Framework   string
		BuildSystem string
	}{
		"package.json":    {"Node.js/npm", "npm"},
		"Cargo.toml":      {"Rust", "Cargo"},
		"go.mod":          {"Go modules", "go build"},
		"requirements.txt": {"Python", "pip"},
		"Pipfile":         {"Python/pipenv", "pipenv"},
		"pyproject.toml":  {"Python", "poetry/setuptools"},
		"pom.xml":         {"Maven", "Maven"},
		"build.gradle":    {"Gradle", "Gradle"},
		"Makefile":        {"", "Make"},
		"CMakeLists.txt":  {"CMake", "CMake"},
		"Gemfile":         {"Ruby/Bundler", "Bundler"},
		"composer.json":   {"PHP/Composer", "Composer"},
	}

	for file, info := range checks {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			if info.Framework != "" {
				framework = info.Framework
			}
			if info.BuildSystem != "" {
				buildSystem = info.BuildSystem
			}
			break
		}
	}

	return
}

// findEntryPoints locates key files in the codebase
func findEntryPoints(basePath string, files []string, language string) []EntryPoint {
	var entryPoints []EntryPoint

	for _, filePath := range files {
		relPath, _ := filepath.Rel(basePath, filePath)
		name := filepath.Base(filePath)
		nameLower := strings.ToLower(name)

		// Detect entry point types
		if nameLower == "main.go" || nameLower == "main.rs" || nameLower == "main.py" ||
			nameLower == "main.java" || nameLower == "main.c" || nameLower == "main.cpp" {
			entryPoints = append(entryPoints, EntryPoint{
				Path:        relPath,
				Type:        "main",
				Description: "Main executable entry point",
			})
		} else if nameLower == "lib.rs" {
			entryPoints = append(entryPoints, EntryPoint{
				Path:        relPath,
				Type:        "library",
				Description: "Library root",
			})
		} else if strings.HasPrefix(nameLower, "test") && strings.HasSuffix(nameLower, language) {
			entryPoints = append(entryPoints, EntryPoint{
				Path:        relPath,
				Type:        "test",
				Description: "Test suite",
			})
		} else if nameLower == "package.json" || nameLower == "cargo.toml" ||
			nameLower == "go.mod" || nameLower == "requirements.txt" {
			entryPoints = append(entryPoints, EntryPoint{
				Path:        relPath,
				Type:        "config",
				Description: "Project configuration",
			})
		}
	}

	return entryPoints
}

// analyzeArchitecture detects structural patterns
func analyzeArchitecture(basePath string, files []string, language string) ArchitectureInfo {
	arch := ArchitectureInfo{
		Pattern:     "Unknown",
		Directories: []string{},
		KeyModules:  []ModuleInfo{},
	}

	// Count files per directory
	dirCounts := make(map[string]int)
	for _, filePath := range files {
		dir := filepath.Dir(filePath)
		relDir, _ := filepath.Rel(basePath, dir)
		if relDir != "." {
			dirCounts[relDir]++
		}
	}

	// Identify key directories
	keyDirs := []string{"src", "lib", "pkg", "internal", "cmd", "tests", "test",
		"handlers", "services", "models", "controllers", "views", "routes", "api"}

	foundDirs := make(map[string]bool)
	for dir := range dirCounts {
		for _, key := range keyDirs {
			if strings.Contains(strings.ToLower(dir), key) {
				foundDirs[key] = true
				arch.Directories = append(arch.Directories, dir)
			}
		}
	}

	// Sort directories for consistent output
	sort.Strings(arch.Directories)

	// Detect pattern based on directories
	if foundDirs["handlers"] && foundDirs["services"] {
		arch.Pattern = "Layered (handlers → services)"
	} else if foundDirs["controllers"] && foundDirs["models"] && foundDirs["views"] {
		arch.Pattern = "MVC (Model-View-Controller)"
	} else if foundDirs["api"] || foundDirs["routes"] {
		arch.Pattern = "API-focused"
	} else if foundDirs["src"] || foundDirs["lib"] {
		arch.Pattern = "Standard library structure"
	} else {
		arch.Pattern = "Flat/Simple structure"
	}

	// Build key modules list (top directories by file count)
	type dirCount struct {
		path  string
		count int
	}
	var sortedDirs []dirCount
	for dir, count := range dirCounts {
		if count >= 2 { // Only include directories with 2+ files
			sortedDirs = append(sortedDirs, dirCount{dir, count})
		}
	}
	sort.Slice(sortedDirs, func(i, j int) bool {
		return sortedDirs[i].count > sortedDirs[j].count
	})

	// Take top 5
	limit := 5
	if len(sortedDirs) < limit {
		limit = len(sortedDirs)
	}
	for i := 0; i < limit; i++ {
		arch.KeyModules = append(arch.KeyModules, ModuleInfo{
			Path:      sortedDirs[i].path,
			FileCount: sortedDirs[i].count,
		})
	}

	return arch
}

// findDependencies extracts dependencies from known files
func findDependencies(path string) []Dependency {
	var deps []Dependency

	// Check package.json
	packageJSON := filepath.Join(path, "package.json")
	if content, err := os.ReadFile(packageJSON); err == nil {
		deps = append(deps, parseDepsFromJSON(string(content), "package.json")...)
	}

	// Check Cargo.toml
	cargoToml := filepath.Join(path, "Cargo.toml")
	if content, err := os.ReadFile(cargoToml); err == nil {
		deps = append(deps, parseDepsFromToml(string(content), "Cargo.toml")...)
	}

	// Check go.mod
	goMod := filepath.Join(path, "go.mod")
	if content, err := os.ReadFile(goMod); err == nil {
		deps = append(deps, parseDepsFromGoMod(string(content), "go.mod")...)
	}

	return deps
}

// parseDepsFromJSON extracts dependencies from package.json
func parseDepsFromJSON(content, source string) []Dependency {
	var deps []Dependency

	// Simple regex-based parsing (good enough for recon)
	depPattern := regexp.MustCompile(`"([^"]+)":\s*"([^"]+)"`)
	inDeps := false

	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, `"dependencies"`) || strings.Contains(line, `"devDependencies"`) {
			inDeps = true
			continue
		}
		if inDeps && strings.Contains(line, "}") {
			inDeps = false
		}
		if inDeps {
			if matches := depPattern.FindStringSubmatch(line); len(matches) == 3 {
				deps = append(deps, Dependency{
					Name:    matches[1],
					Version: matches[2],
					Source:  source,
				})
			}
		}
	}

	return deps
}

// parseDepsFromToml extracts dependencies from Cargo.toml
func parseDepsFromToml(content, source string) []Dependency {
	var deps []Dependency

	depPattern := regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*=\s*"([^"]+)"`)
	inDeps := false

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if line == "[dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[") && line != "[dependencies]" {
			inDeps = false
		}
		if inDeps && line != "" {
			if matches := depPattern.FindStringSubmatch(line); len(matches) == 3 {
				deps = append(deps, Dependency{
					Name:    matches[1],
					Version: matches[2],
					Source:  source,
				})
			}
		}
	}

	return deps
}

// parseDepsFromGoMod extracts dependencies from go.mod
func parseDepsFromGoMod(content, source string) []Dependency {
	var deps []Dependency

	requirePattern := regexp.MustCompile(`^\s*([^\s]+)\s+v([^\s]+)`)
	inRequire := false

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "require") {
			inRequire = true
			// Handle single-line require
			if strings.Contains(line, ")") {
				inRequire = false
			}
			continue
		}
		if inRequire && strings.HasPrefix(line, ")") {
			inRequire = false
			continue
		}
		if inRequire || strings.HasPrefix(line, "require ") {
			if matches := requirePattern.FindStringSubmatch(line); len(matches) == 3 {
				deps = append(deps, Dependency{
					Name:    matches[1],
					Version: "v" + matches[2],
					Source:  source,
				})
			}
		}
	}

	return deps
}

// analyzeDocumentation checks for documentation presence
func analyzeDocumentation(path string, files []string) DocInfo {
	info := DocInfo{}

	for _, filePath := range files {
		name := strings.ToLower(filepath.Base(filePath))

		// Check for README
		if strings.HasPrefix(name, "readme") {
			info.HasReadme = true
			if content, err := os.ReadFile(filePath); err == nil {
				info.ReadmeLines = len(strings.Split(string(content), "\n"))
			}
		}

		// Check for docs directory
		if strings.Contains(strings.ToLower(filePath), "docs/") ||
			strings.Contains(strings.ToLower(filePath), "documentation/") {
			info.HasDocsDir = true
		}

		// Check for examples
		if strings.Contains(strings.ToLower(filePath), "examples/") ||
			strings.Contains(strings.ToLower(filePath), "example") {
			info.Examples = true
		}
	}

	return info
}

// analyzeHealth finds code health indicators
func analyzeHealth(path string, files []string, quick bool, focus string) HealthInfo {
	health := HealthInfo{
		TODOs:           []CodeMarker{},
		FIXMEs:          []CodeMarker{},
		SecurityConcerns: []CodeMarker{},
		DeadCodeSignals: []string{},
	}

	// Patterns to search for
	todoPattern := regexp.MustCompile(`(?i)\bTODO\b:?\s*(.*)`)
	fixmePattern := regexp.MustCompile(`(?i)\b(FIXME|HACK|XXX)\b:?\s*(.*)`)
	securityPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)password\s*=\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)secret\s*=\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)api[_-]?key\s*=\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)hardcoded`),
	}

	// Limit files scanned in quick mode
	scanLimit := len(files)
	if quick && focus != "security" {
		scanLimit = 50
	}

	for i, filePath := range files {
		if i >= scanLimit {
			break
		}

		// Only scan text files
		ext := strings.ToLower(filepath.Ext(filePath))
		if !isTextFile(ext) {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		relPath, _ := filepath.Rel(path, filePath)
		lines := strings.Split(string(content), "\n")

		for lineNum, line := range lines {
			// TODO markers
			if !quick && len(health.TODOs) < 20 {
				if match := todoPattern.FindStringSubmatch(line); len(match) > 1 {
					health.TODOs = append(health.TODOs, CodeMarker{
						File:    relPath,
						Line:    lineNum + 1,
						Content: strings.TrimSpace(match[1]),
					})
				}
			}

			// FIXME markers
			if !quick && len(health.FIXMEs) < 20 {
				if match := fixmePattern.FindStringSubmatch(line); len(match) > 2 {
					health.FIXMEs = append(health.FIXMEs, CodeMarker{
						File:    relPath,
						Line:    lineNum + 1,
						Content: strings.TrimSpace(match[2]),
					})
				}
			}

			// Security concerns
			if (focus == "security" || focus == "") && len(health.SecurityConcerns) < 10 {
				for _, pattern := range securityPatterns {
					if pattern.MatchString(line) {
						health.SecurityConcerns = append(health.SecurityConcerns, CodeMarker{
							File:    relPath,
							Line:    lineNum + 1,
							Content: strings.TrimSpace(line),
						})
						break
					}
				}
			}
		}
	}

	return health
}

// isTextFile returns true if the extension is likely a text file
func isTextFile(ext string) bool {
	textExts := map[string]bool{
		".go": true, ".rs": true, ".js": true, ".ts": true, ".py": true,
		".java": true, ".c": true, ".cpp": true, ".cs": true, ".rb": true,
		".php": true, ".sh": true, ".bash": true, ".md": true, ".txt": true,
		".yml": true, ".yaml": true, ".json": true, ".toml": true, ".xml": true,
	}
	return textExts[ext]
}

// displayReconReport outputs the reconnaissance report
func displayReconReport(info *ProjectInfo, focus string) {
	output.Success("📋 Reconnaissance Report")
	fmt.Println("")

	fmt.Printf("Scanned: %s\n", info.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Location: %s\n", info.Path)
	fmt.Printf("Scan Type: %s\n", info.ScanType)
	fmt.Println("")

	// Overview section
	if focus == "" || focus == "architecture" {
		output.Header("Overview")
		fmt.Println("")
		output.Item("Language", info.Language)
		output.Item("Framework", info.Framework)
		output.Item("Build System", info.BuildSystem)
		output.Item("Total Files", fmt.Sprintf("%d", info.TotalFiles))
		output.Item("Code Files", fmt.Sprintf("%d", info.CodeFiles))
		fmt.Println("")
	}

	// Entry points
	if (focus == "" || focus == "architecture") && len(info.EntryPoints) > 0 {
		output.Header("Entry Points")
		fmt.Println("")
		for i, ep := range info.EntryPoints {
			if i >= 10 {
				break
			}
			fmt.Printf("  %s - %s (%s)\n", output.Yellow+ep.Path+output.Reset, ep.Description, ep.Type)
		}
		fmt.Println("")
	}

	// Architecture
	if focus == "" || focus == "architecture" {
		output.Header("Architecture")
		fmt.Println("")
		output.Item("Pattern", info.Architecture.Pattern)
		if len(info.Architecture.KeyModules) > 0 {
			fmt.Println("")
			fmt.Println("  Key Modules:")
			for _, mod := range info.Architecture.KeyModules {
				fmt.Printf("    %s (%d files)\n", mod.Path, mod.FileCount)
			}
		}
		fmt.Println("")
	}

	// Dependencies
	if (focus == "" || focus == "security") && len(info.Dependencies) > 0 {
		output.Header("Dependencies")
		fmt.Println("")
		fmt.Printf("  Found %d dependencies\n", len(info.Dependencies))

		// Group by source file
		bySource := make(map[string][]Dependency)
		for _, dep := range info.Dependencies {
			bySource[dep.Source] = append(bySource[dep.Source], dep)
		}

		for source, deps := range bySource {
			fmt.Printf("\n  %s:\n", source)
			limit := 5
			if len(deps) < limit {
				limit = len(deps)
			}
			for i := 0; i < limit; i++ {
				fmt.Printf("    - %s %s\n", deps[i].Name, deps[i].Version)
			}
			if len(deps) > 5 {
				fmt.Printf("    ... and %d more\n", len(deps)-5)
			}
		}
		fmt.Println("")
	}

	// Documentation
	if focus == "" || focus == "docs" {
		output.Header("Documentation")
		fmt.Println("")
		if info.Documentation.HasReadme {
			fmt.Printf("  ✓ README found (%d lines)\n", info.Documentation.ReadmeLines)
		} else {
			fmt.Println("  ✗ No README found")
		}
		if info.Documentation.HasDocsDir {
			fmt.Println("  ✓ Documentation directory found")
		}
		if info.Documentation.Examples {
			fmt.Println("  ✓ Examples found")
		}
		fmt.Println("")
	}

	// Health indicators
	if focus == "" || focus == "security" {
		output.Header("Health Indicators")
		fmt.Println("")

		if len(info.HealthIndicators.TODOs) > 0 {
			fmt.Printf("  TODOs: %d found\n", len(info.HealthIndicators.TODOs))
			for i, todo := range info.HealthIndicators.TODOs {
				if i >= 5 {
					fmt.Printf("  ... and %d more\n", len(info.HealthIndicators.TODOs)-5)
					break
				}
				fmt.Printf("    - %s:%d - %s\n", todo.File, todo.Line, todo.Content)
			}
			fmt.Println("")
		}

		if len(info.HealthIndicators.FIXMEs) > 0 {
			fmt.Printf("  FIXMEs: %d found\n", len(info.HealthIndicators.FIXMEs))
			for i, fixme := range info.HealthIndicators.FIXMEs {
				if i >= 5 {
					fmt.Printf("  ... and %d more\n", len(info.HealthIndicators.FIXMEs)-5)
					break
				}
				fmt.Printf("    - %s:%d - %s\n", fixme.File, fixme.Line, fixme.Content)
			}
			fmt.Println("")
		}

		if len(info.HealthIndicators.SecurityConcerns) > 0 {
			fmt.Printf("  ⚠ Security Concerns: %d found\n", len(info.HealthIndicators.SecurityConcerns))
			for i, concern := range info.HealthIndicators.SecurityConcerns {
				if i >= 5 {
					fmt.Printf("  ... and %d more\n", len(info.HealthIndicators.SecurityConcerns)-5)
					break
				}
				fmt.Printf("    - %s:%d\n", concern.File, concern.Line)
			}
			fmt.Println("")
		}

		if len(info.HealthIndicators.TODOs) == 0 &&
			len(info.HealthIndicators.FIXMEs) == 0 &&
			len(info.HealthIndicators.SecurityConcerns) == 0 {
			fmt.Println("  ✓ No major issues detected")
			fmt.Println("")
		}
	}

	output.Success("🔍 Reconnaissance complete")
}
