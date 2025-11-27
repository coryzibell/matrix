package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/output"
)

// ToolchainInfo represents an installed toolchain
type ToolchainInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Manager     string `json:"manager"`     // how it's installed
	Path        string `json:"path"`        // where the binary is
	Available   bool   `json:"available"`   // was it detected
	CheckedAt   string `json:"checked_at"`
}

// PackageManifest represents a package manifest file
type PackageManifest struct {
	Path         string       `json:"path"`
	Type         string       `json:"type"`        // cargo, npm, go, pip
	Dependencies []Dependency `json:"dependencies"`
	DevDeps      []Dependency `json:"dev_dependencies,omitempty"`
	TotalCount   int          `json:"total_count"`
}

// EcosystemSummary summarizes a package ecosystem
type EcosystemSummary struct {
	Ecosystem     string `json:"ecosystem"`
	DirectDeps    int    `json:"direct_deps"`
	DevDeps       int    `json:"dev_deps,omitempty"`
	ManifestCount int    `json:"manifest_count"`
}

// DependencyMapOutput contains the complete scan results
type DependencyMapOutput struct {
	ScannedAt   time.Time          `json:"scanned_at"`
	ScanPath    string             `json:"scan_path"`
	Toolchains  []ToolchainInfo    `json:"toolchains"`
	Manifests   []PackageManifest  `json:"manifests"`
	Ecosystems  []EcosystemSummary `json:"ecosystems"`
}

// runDependencyMap implements the dependency-map command
func runDependencyMap() error {
	fs := flag.NewFlagSet("dependency-map", flag.ExitOnError)
	subCmd := ""

	// Handle subcommands
	if len(os.Args) > 2 {
		subCmd = os.Args[2]
	}

	switch subCmd {
	case "scan":
		return runDependencyScan(fs)
	case "toolchains":
		return runToolchainsCheck()
	case "report":
		return runDependencyReport()
	case "":
		return runDependencyReport()
	default:
		return fmt.Errorf("unknown subcommand: %s (valid: scan, toolchains, report)", subCmd)
	}
}

// runDependencyScan scans for package ecosystems
func runDependencyScan(fs *flag.FlagSet) error {
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	// Parse flags
	if len(os.Args) > 3 {
		fs.Parse(os.Args[3:])
	}

	// Get target path
	targetPath := "."
	if fs.NArg() > 0 {
		targetPath = fs.Arg(0)
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	output.Success("🔧 Dependency Scanner")
	fmt.Println("")
	fmt.Printf("Scanning: %s\n", absPath)
	fmt.Println("")

	// Scan for manifests
	manifests := scanForManifests(absPath)

	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(manifests)
	}

	// Human-readable output
	if len(manifests) == 0 {
		fmt.Println("No package manifests found.")
		return nil
	}

	output.Header("Package Manifests Found")
	fmt.Println("")

	for _, m := range manifests {
		relPath, _ := filepath.Rel(absPath, m.Path)
		fmt.Printf("  %s (%d dependencies)\n", output.Yellow+relPath+output.Reset, m.TotalCount)

		// Show top 5 deps
		limit := 5
		if len(m.Dependencies) < limit {
			limit = len(m.Dependencies)
		}
		for i := 0; i < limit; i++ {
			dep := m.Dependencies[i]
			fmt.Printf("    - %s %s\n", dep.Name, dep.Version)
		}
		if len(m.Dependencies) > 5 {
			fmt.Printf("    ... and %d more\n", len(m.Dependencies)-5)
		}
		fmt.Println("")
	}

	return nil
}

// runToolchainsCheck checks for installed toolchains
func runToolchainsCheck() error {
	output.Success("🔧 Toolchain Detection")
	fmt.Println("")

	toolchains := detectToolchains()

	if len(toolchains) == 0 {
		fmt.Println("No toolchains detected.")
		return nil
	}

	output.Header("Installed Toolchains")
	fmt.Println("")

	for _, tc := range toolchains {
		if tc.Available {
			managerInfo := ""
			if tc.Manager != "" {
				managerInfo = fmt.Sprintf(" (%s)", tc.Manager)
			}
			fmt.Printf("  ✓ %s %s%s\n", tc.Name, output.Green+tc.Version+output.Reset, managerInfo)
			if tc.Path != "" {
				fmt.Printf("    %s\n", output.Dim+tc.Path+output.Reset)
			}
		} else {
			fmt.Printf("  ✗ %s (not found)\n", output.Dim+tc.Name+output.Reset)
		}
	}
	fmt.Println("")

	return nil
}

// runDependencyReport generates full dependency report
func runDependencyReport() error {
	output.Success("🔧 Dependency Map")
	fmt.Println("")

	// Detect toolchains
	toolchains := detectToolchains()

	// Scan current directory for manifests
	cwd, _ := os.Getwd()
	manifests := scanForManifests(cwd)

	// Calculate ecosystem summaries
	ecosystems := summarizeEcosystems(manifests)

	// Display results
	if len(toolchains) > 0 {
		output.Header("Toolchains Detected")
		fmt.Println("")

		for _, tc := range toolchains {
			if tc.Available {
				managerInfo := ""
				if tc.Manager != "" {
					managerInfo = fmt.Sprintf(" (%s)", output.Dim+tc.Manager+output.Reset+")")
				}
				fmt.Printf("  %s %s%s\n", tc.Name, tc.Version, managerInfo)
			}
		}
		fmt.Println("")
	}

	if len(manifests) > 0 {
		output.Header("Package Manifests")
		fmt.Println("")

		for _, m := range manifests {
			relPath, _ := filepath.Rel(cwd, m.Path)
			fmt.Printf("  %s (%d dependencies)\n", output.Yellow+relPath+output.Reset, m.TotalCount)

			// Show top 3 deps
			limit := 3
			if len(m.Dependencies) < limit {
				limit = len(m.Dependencies)
			}
			for i := 0; i < limit; i++ {
				dep := m.Dependencies[i]
				fmt.Printf("    - %s %s\n", dep.Name, dep.Version)
			}
			if len(m.Dependencies) > limit {
				fmt.Printf("    ... and %d more\n", len(m.Dependencies)-limit)
			}
			fmt.Println("")
		}
	}

	if len(ecosystems) > 0 {
		output.Header("Ecosystem Summary")
		fmt.Println("")

		for _, eco := range ecosystems {
			devInfo := ""
			if eco.DevDeps > 0 {
				devInfo = fmt.Sprintf(", %d dev", eco.DevDeps)
			}
			manifestInfo := ""
			if eco.ManifestCount > 1 {
				manifestInfo = fmt.Sprintf(" (%d manifests)", eco.ManifestCount)
			}
			fmt.Printf("  %s: %d direct deps%s%s\n", eco.Ecosystem, eco.DirectDeps, devInfo, manifestInfo)
		}
		fmt.Println("")
	}

	if len(toolchains) == 0 && len(manifests) == 0 {
		fmt.Println("No toolchains or package manifests detected.")
		fmt.Println("")
	}

	return nil
}

// detectToolchains probes for installed toolchains
func detectToolchains() []ToolchainInfo {
	checks := []struct {
		name       string
		command    string
		args       []string
		versionRe  *regexp.Regexp
		managers   []string // possible managers, in order of preference
	}{
		{
			name:      "rust",
			command:   "rustc",
			args:      []string{"--version"},
			versionRe: regexp.MustCompile(`rustc (\d+\.\d+\.\d+)`),
			managers:  []string{"rustup"},
		},
		{
			name:      "cargo",
			command:   "cargo",
			args:      []string{"--version"},
			versionRe: regexp.MustCompile(`cargo (\d+\.\d+\.\d+)`),
			managers:  []string{"rustup"},
		},
		{
			name:      "node",
			command:   "node",
			args:      []string{"--version"},
			versionRe: regexp.MustCompile(`v?(\d+\.\d+\.\d+)`),
			managers:  []string{"aqua", "nvm", "asdf"},
		},
		{
			name:      "npm",
			command:   "npm",
			args:      []string{"--version"},
			versionRe: regexp.MustCompile(`(\d+\.\d+\.\d+)`),
			managers:  []string{"node"},
		},
		{
			name:      "go",
			command:   "go",
			args:      []string{"version"},
			versionRe: regexp.MustCompile(`go(\d+\.\d+\.\d+)`),
			managers:  []string{"aqua", "asdf", "system"},
		},
		{
			name:      "python",
			command:   "python3",
			args:      []string{"--version"},
			versionRe: regexp.MustCompile(`Python (\d+\.\d+\.\d+)`),
			managers:  []string{"pyenv", "asdf", "system"},
		},
		{
			name:      "pip",
			command:   "pip3",
			args:      []string{"--version"},
			versionRe: regexp.MustCompile(`pip (\d+\.\d+\.\d+)`),
			managers:  []string{"python"},
		},
	}

	var toolchains []ToolchainInfo

	for _, check := range checks {
		tc := ToolchainInfo{
			Name:      check.name,
			Available: false,
			CheckedAt: time.Now().Format(time.RFC3339),
		}

		// Try to run the command
		cmd := exec.Command(check.command, check.args...)
		output, err := cmd.CombinedOutput()

		if err == nil {
			tc.Available = true

			// Extract version
			if matches := check.versionRe.FindStringSubmatch(string(output)); len(matches) > 1 {
				tc.Version = matches[1]
			} else {
				tc.Version = strings.TrimSpace(string(output))
			}

			// Find binary path
			pathCmd := exec.Command("which", check.command)
			if pathOutput, err := pathCmd.Output(); err == nil {
				tc.Path = strings.TrimSpace(string(pathOutput))
			}

			// Detect manager
			tc.Manager = detectManager(tc.Path, check.managers)
		}

		toolchains = append(toolchains, tc)
	}

	return toolchains
}

// detectManager tries to determine which manager installed a tool
func detectManager(path string, possibleManagers []string) string {
	if path == "" {
		return "unknown"
	}

	for _, manager := range possibleManagers {
		if strings.Contains(path, manager) {
			return manager
		}
	}

	// Check for common patterns
	if strings.Contains(path, "/.cargo/") {
		return "cargo"
	}
	if strings.Contains(path, "/.rustup/") {
		return "rustup"
	}
	if strings.Contains(path, "/.asdf/") {
		return "asdf"
	}
	if strings.Contains(path, "/.nvm/") {
		return "nvm"
	}
	if strings.Contains(path, "/.pyenv/") {
		return "pyenv"
	}
	if strings.Contains(path, "/usr/bin") || strings.Contains(path, "/usr/local/bin") {
		return "system"
	}

	return "unknown"
}

// scanForManifests finds package manifest files
func scanForManifests(rootPath string) []PackageManifest {
	var manifests []PackageManifest

	// Known manifest files
	manifestChecks := map[string]string{
		"Cargo.toml":       "cargo",
		"package.json":     "npm",
		"go.mod":           "go",
		"requirements.txt": "pip",
		"Pipfile":          "pipenv",
		"pyproject.toml":   "poetry",
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip common ignore directories
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == "target" || name == "vendor" ||
			   name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this is a known manifest file
		basename := filepath.Base(path)
		if manifestType, ok := manifestChecks[basename]; ok {
			manifest := PackageManifest{
				Path: path,
				Type: manifestType,
			}

			// Parse dependencies based on type
			content, err := os.ReadFile(path)
			if err == nil {
				switch manifestType {
				case "cargo":
					manifest.Dependencies = parseDepsFromToml(string(content), path)
				case "npm":
					manifest.Dependencies, manifest.DevDeps = parseDepsFromPackageJSON(string(content), path)
				case "go":
					manifest.Dependencies = parseDepsFromGoMod(string(content), path)
				case "pip", "pipenv", "poetry":
					manifest.Dependencies = parseDepsFromPython(string(content), path, manifestType)
				}
			}

			manifest.TotalCount = len(manifest.Dependencies) + len(manifest.DevDeps)
			manifests = append(manifests, manifest)
		}

		return nil
	})

	if err != nil {
		// Silently ignore walk errors
		return manifests
	}

	return manifests
}

// parseDepsFromPackageJSON extracts dependencies from package.json
func parseDepsFromPackageJSON(content, source string) ([]Dependency, []Dependency) {
	var deps []Dependency
	var devDeps []Dependency

	depPattern := regexp.MustCompile(`"([^"]+)":\s*"([^"]+)"`)
	inDeps := false
	inDevDeps := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, `"dependencies"`) && strings.Contains(trimmed, `:`) {
			inDeps = true
			inDevDeps = false
			continue
		}
		if strings.Contains(trimmed, `"devDependencies"`) && strings.Contains(trimmed, `:`) {
			inDevDeps = true
			inDeps = false
			continue
		}
		if (inDeps || inDevDeps) && (trimmed == "}" || trimmed == "},") {
			inDeps = false
			inDevDeps = false
			continue
		}

		if inDeps || inDevDeps {
			if matches := depPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
				dep := Dependency{
					Name:    matches[1],
					Version: matches[2],
					Source:  source,
				}
				if inDeps {
					deps = append(deps, dep)
				} else {
					devDeps = append(devDeps, dep)
				}
			}
		}
	}

	return deps, devDeps
}

// parseDepsFromPython extracts dependencies from Python files
func parseDepsFromPython(content, source, manifestType string) []Dependency {
	var deps []Dependency

	if manifestType == "pip" {
		// requirements.txt format: package==version or package>=version
		linePattern := regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*([>=<~!]+)\s*([^\s#]+)`)

		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			if matches := linePattern.FindStringSubmatch(line); len(matches) >= 4 {
				deps = append(deps, Dependency{
					Name:    matches[1],
					Version: matches[2] + matches[3],
					Source:  source,
				})
			} else {
				// Just package name, no version
				parts := strings.Fields(line)
				if len(parts) > 0 && !strings.HasPrefix(parts[0], "#") {
					deps = append(deps, Dependency{
						Name:    parts[0],
						Version: "*",
						Source:  source,
					})
				}
			}
		}
	} else if manifestType == "poetry" {
		// Simple TOML parsing for [tool.poetry.dependencies]
		depPattern := regexp.MustCompile(`^([a-zA-Z0-9_-]+)\s*=\s*"([^"]+)"`)
		inDeps := false

		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)

			if line == "[tool.poetry.dependencies]" {
				inDeps = true
				continue
			}
			if strings.HasPrefix(line, "[") && line != "[tool.poetry.dependencies]" {
				inDeps = false
			}
			if inDeps && line != "" {
				if matches := depPattern.FindStringSubmatch(line); len(matches) == 3 {
					if matches[1] != "python" { // Skip python version specifier
						deps = append(deps, Dependency{
							Name:    matches[1],
							Version: matches[2],
							Source:  source,
						})
					}
				}
			}
		}
	}

	return deps
}

// summarizeEcosystems creates ecosystem summaries
func summarizeEcosystems(manifests []PackageManifest) []EcosystemSummary {
	ecosystemMap := make(map[string]*EcosystemSummary)

	for _, m := range manifests {
		if _, exists := ecosystemMap[m.Type]; !exists {
			ecosystemMap[m.Type] = &EcosystemSummary{
				Ecosystem: m.Type,
			}
		}

		eco := ecosystemMap[m.Type]
		eco.DirectDeps += len(m.Dependencies)
		eco.DevDeps += len(m.DevDeps)
		eco.ManifestCount++
	}

	// Convert to slice and sort
	var ecosystems []EcosystemSummary
	for _, eco := range ecosystemMap {
		ecosystems = append(ecosystems, *eco)
	}

	sort.Slice(ecosystems, func(i, j int) bool {
		return ecosystems[i].DirectDeps > ecosystems[j].DirectDeps
	})

	return ecosystems
}
