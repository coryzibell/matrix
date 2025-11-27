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

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// FileReference represents a cross-identity file reference
type FileReference struct {
	SourceFile   string // File containing the reference
	SourceID     string // Identity owning the source file
	TargetPath   string // Referenced path
	TargetID     string // Identity targeted (if determinable)
	LineNumber   int    // Line where reference appears
}

// ArtifactStats tracks how many times a file is referenced
type ArtifactStats struct {
	Path       string   // File path
	Identity   string   // Owning identity
	ImportCount int     // Number of references
	Importers  []string // Identities that import it
}

// TransactionStats tracks produce/consume volume per identity
type TransactionStats struct {
	Identity      string
	ProducesCount int // Files this identity creates
	ConsumesCount int // References to other identities' files
}

// DependencyEdge represents an identity-to-identity dependency
type DependencyEdge struct {
	From  string // Source identity
	To    string // Target identity
	Count int    // Number of references
	Via   string // Example file creating the dependency
}

// ContractLedgerReport contains full ledger analysis
type ContractLedgerReport struct {
	Dependencies  []DependencyEdge
	HotArtifacts  []ArtifactStats
	Transactions  []TransactionStats
	TotalFiles    int
	TotalRefs     int
}

// runContractLedger implements the contract-ledger command
func runContractLedger() error {
	// Parse flags
	fs := flag.NewFlagSet("contract-ledger", flag.ExitOnError)
	graphFlag := fs.Bool("graph", false, "Show only dependency graph")
	artifactsFlag := fs.Bool("artifacts", false, "Show only hot artifacts")
	volumeFlag := fs.Bool("volume", false, "Show only transaction volume")
	jsonFlag := fs.Bool("json", false, "Output as JSON")

	// Parse remaining args (after "contract-ledger")
	if len(os.Args) > 2 {
		fs.Parse(os.Args[2:])
	}

	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		if *jsonFlag {
			emptyReport := ContractLedgerReport{}
			outputContractJSON(emptyReport)
			return nil
		}
		fmt.Println("📜 No ledger found - ~/.claude/ram/ does not exist")
		return nil
	}

	// Scan RAM directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		if *jsonFlag {
			emptyReport := ContractLedgerReport{}
			outputContractJSON(emptyReport)
			return nil
		}
		fmt.Println("📜 Ledger empty - no markdown files in ~/.claude/ram/")
		return nil
	}

	// Scan cache directory for transaction tracking
	cacheDir := filepath.Join(ramDir, "cache")
	var cacheFiles []ram.File
	if stat, err := os.Stat(cacheDir); err == nil && stat.IsDir() {
		// Temporarily scan cache as identity "cache"
		cacheFiles, _ = scanCacheDir(cacheDir)
	}

	// Extract file references
	refs := extractFileReferences(files, ramDir)

	// Build report
	report := buildContractReport(refs, files, cacheFiles)

	// Output
	if *jsonFlag {
		outputContractJSON(report)
	} else {
		displayContractReport(report, *graphFlag, *artifactsFlag, *volumeFlag)
	}

	return nil
}

// extractFileReferences finds all cross-identity file references
func extractFileReferences(files []ram.File, ramDir string) []FileReference {
	var refs []FileReference

	// Patterns to match file paths
	ramPathPattern := regexp.MustCompile(`~?/?\.claude/ram/([a-z]+)/([^\s\)\"]+)`)
	absPathPattern := regexp.MustCompile(regexp.QuoteMeta(ramDir) + `/([a-z]+)/([^\s\)\"]+)`)

	for _, file := range files {
		lines := strings.Split(file.Content, "\n")

		for lineNum, line := range lines {
			// Check for RAM path references (~/.claude/ram/...)
			matches := ramPathPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				targetID := match[1]
				targetFile := match[2]

				// Skip self-references
				if targetID == file.Identity {
					continue
				}

				// Skip if not a valid identity
				if !identity.IsValid(targetID) {
					continue
				}

				refs = append(refs, FileReference{
					SourceFile: file.Path,
					SourceID:   file.Identity,
					TargetPath: filepath.Join(ramDir, targetID, targetFile),
					TargetID:   targetID,
					LineNumber: lineNum + 1,
				})
			}

			// Check for absolute path references
			absMatches := absPathPattern.FindAllStringSubmatch(line, -1)
			for _, match := range absMatches {
				targetID := match[1]
				targetFile := match[2]

				// Skip self-references
				if targetID == file.Identity {
					continue
				}

				// Skip if not a valid identity
				if !identity.IsValid(targetID) {
					continue
				}

				refs = append(refs, FileReference{
					SourceFile: file.Path,
					SourceID:   file.Identity,
					TargetPath: filepath.Join(ramDir, targetID, targetFile),
					TargetID:   targetID,
					LineNumber: lineNum + 1,
				})
			}
		}
	}

	return refs
}

// buildContractReport generates the full contract ledger report
func buildContractReport(refs []FileReference, files []ram.File, cacheFiles []ram.File) ContractLedgerReport {
	// Build dependency graph
	depMap := make(map[string]map[string]*DependencyEdge) // from -> to -> edge

	for _, ref := range refs {
		if depMap[ref.SourceID] == nil {
			depMap[ref.SourceID] = make(map[string]*DependencyEdge)
		}

		if depMap[ref.SourceID][ref.TargetID] == nil {
			depMap[ref.SourceID][ref.TargetID] = &DependencyEdge{
				From: ref.SourceID,
				To:   ref.TargetID,
				Via:  filepath.Base(ref.SourceFile),
			}
		}

		depMap[ref.SourceID][ref.TargetID].Count++
	}

	// Flatten to slice
	var deps []DependencyEdge
	for _, targets := range depMap {
		for _, edge := range targets {
			deps = append(deps, *edge)
		}
	}
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Count != deps[j].Count {
			return deps[i].Count > deps[j].Count
		}
		return deps[i].From < deps[j].From
	})

	// Build hot artifacts (most-referenced files)
	artifactMap := make(map[string]*ArtifactStats) // target path -> stats
	importerSet := make(map[string]map[string]bool) // target path -> importer identity -> bool

	for _, ref := range refs {
		if artifactMap[ref.TargetPath] == nil {
			artifactMap[ref.TargetPath] = &ArtifactStats{
				Path:     ref.TargetPath,
				Identity: ref.TargetID,
			}
			importerSet[ref.TargetPath] = make(map[string]bool)
		}

		artifactMap[ref.TargetPath].ImportCount++
		importerSet[ref.TargetPath][ref.SourceID] = true
	}

	// Populate importers list
	for path, stats := range artifactMap {
		for importer := range importerSet[path] {
			stats.Importers = append(stats.Importers, importer)
		}
		sort.Strings(stats.Importers)
	}

	// Flatten and sort artifacts
	var artifacts []ArtifactStats
	for _, stats := range artifactMap {
		artifacts = append(artifacts, *stats)
	}
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].ImportCount > artifacts[j].ImportCount
	})

	// Build transaction volume (produce/consume per identity)
	transMap := make(map[string]*TransactionStats)

	// Count produces (files owned by identity)
	for _, file := range files {
		if transMap[file.Identity] == nil {
			transMap[file.Identity] = &TransactionStats{
				Identity: file.Identity,
			}
		}
		transMap[file.Identity].ProducesCount++
	}

	// Count cache produces
	for range cacheFiles {
		// Cache files count as produces for cache "identity"
		if transMap["cache"] == nil {
			transMap["cache"] = &TransactionStats{
				Identity: "cache",
			}
		}
		transMap["cache"].ProducesCount++
	}

	// Count consumes (references to other identities)
	for _, ref := range refs {
		if transMap[ref.SourceID] == nil {
			transMap[ref.SourceID] = &TransactionStats{
				Identity: ref.SourceID,
			}
		}
		transMap[ref.SourceID].ConsumesCount++
	}

	// Flatten and sort transactions
	var trans []TransactionStats
	for _, stats := range transMap {
		trans = append(trans, *stats)
	}
	sort.Slice(trans, func(i, j int) bool {
		totalI := trans[i].ProducesCount + trans[i].ConsumesCount
		totalJ := trans[j].ProducesCount + trans[j].ConsumesCount
		return totalI > totalJ
	})

	return ContractLedgerReport{
		Dependencies: deps,
		HotArtifacts: artifacts,
		Transactions: trans,
		TotalFiles:   len(files) + len(cacheFiles),
		TotalRefs:    len(refs),
	}
}

// displayContractReport outputs the ledger to stdout
func displayContractReport(report ContractLedgerReport, graphOnly, artifactsOnly, volumeOnly bool) {
	// Default: show all sections
	showGraph := graphOnly || (!artifactsOnly && !volumeOnly)
	showArtifacts := artifactsOnly || (!graphOnly && !volumeOnly)
	showVolume := volumeOnly || (!graphOnly && !artifactsOnly)

	output.Success("📜 Contract Ledger")
	fmt.Println("")
	fmt.Printf("Total Files: %d\n", report.TotalFiles)
	fmt.Printf("Cross-Identity References: %d\n", report.TotalRefs)
	fmt.Println("")

	// Dependency Graph
	if showGraph {
		fmt.Println("═══ DEPENDENCY GRAPH ═══")
		fmt.Println("")

		if len(report.Dependencies) == 0 {
			fmt.Println("No cross-identity dependencies found.")
		} else {
			for _, dep := range report.Dependencies {
				homeDir, _ := os.UserHomeDir()
				viaFile := strings.Replace(dep.Via, homeDir, "~", 1)
				fmt.Printf("%s → %s (%d refs via %s)\n",
					output.Yellow+dep.From+output.Reset,
					output.Cyan+dep.To+output.Reset,
					dep.Count,
					viaFile)
			}
		}
		fmt.Println("")
	}

	// Hot Artifacts
	if showArtifacts {
		fmt.Println("═══ HOT ARTIFACTS ═══")
		fmt.Println("")
		fmt.Println("Most referenced files:")
		fmt.Println("")

		if len(report.HotArtifacts) == 0 {
			fmt.Println("No artifacts referenced yet.")
		} else {
			limit := 10
			if len(report.HotArtifacts) < limit {
				limit = len(report.HotArtifacts)
			}

			for i := 0; i < limit; i++ {
				art := report.HotArtifacts[i]
				homeDir, _ := os.UserHomeDir()
				displayPath := strings.Replace(art.Path, homeDir, "~", 1)

				fmt.Printf("  %d. %s (%d imports)\n",
					i+1,
					output.Yellow+displayPath+output.Reset,
					art.ImportCount)
				fmt.Printf("     Imported by: %s\n", strings.Join(art.Importers, ", "))
			}
		}
		fmt.Println("")
	}

	// Transaction Volume
	if showVolume {
		fmt.Println("═══ TRANSACTION VOLUME ═══")
		fmt.Println("")
		fmt.Println("Identity exchange volume:")
		fmt.Println("")

		if len(report.Transactions) == 0 {
			fmt.Println("No transactions recorded.")
		} else {
			for _, trans := range report.Transactions {
				total := trans.ProducesCount + trans.ConsumesCount
				fmt.Printf("  %s (total: %d)\n",
					output.Yellow+trans.Identity+output.Reset,
					total)
				fmt.Printf("    Produces: %d files\n", trans.ProducesCount)
				fmt.Printf("    Consumes: %d references\n", trans.ConsumesCount)
			}
		}
		fmt.Println("")
	}

	output.Success("📜 Ledger complete")
}

// outputContractJSON outputs the report as JSON
func outputContractJSON(report ContractLedgerReport) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(report)
}

// scanCacheDir scans the cache directory for files
func scanCacheDir(cacheDir string) ([]ram.File, error) {
	var files []ram.File

	err := filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if d.IsDir() {
			return nil
		}

		// Only process .md files
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		files = append(files, ram.File{
			Path:     path,
			Identity: "cache",
			Name:     strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
			Content:  string(content),
		})

		return nil
	})

	return files, err
}
