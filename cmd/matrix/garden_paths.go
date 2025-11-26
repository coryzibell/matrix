package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/coryzibell/matrix/internal/identity"
	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// ConnectionInfo tracks which identities a file mentions
type ConnectionInfo struct {
	FilePath   string
	Mentions   []string
	MentionSet map[string]bool
}

// IdentityCount tracks how many files mention an identity
type IdentityCount struct {
	Identity string
	Count    int
}

// runGardenPaths implements the garden-paths command
func runGardenPaths() error {
	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		fmt.Println("🌾 No garden found at ~/.claude/ram/ - nothing to explore yet")
		fmt.Println("")
		fmt.Println("The garden will grow as identities write to their RAM directories.")
		return nil
	}

	// Scan RAM directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		return fmt.Errorf("failed to scan RAM directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("🌾 Garden exists but no markdown files found yet")
		return nil
	}

	output.Success("🌱 Garden Paths")
	fmt.Println("")
	fmt.Println("Scanning the matrix for connections...")
	fmt.Println("")

	// Track connections
	fileConnections := make(map[string]*ConnectionInfo)
	identityMentions := make(map[string]int)
	allIdentities := identity.All()

	// Scan each file for mentions
	for _, file := range files {
		mentions := findIdentityMentions(file.Content, file.Identity, allIdentities)

		if len(mentions) > 0 {
			// Create relative path for display
			homeDir, _ := os.UserHomeDir()
			relativePath := strings.Replace(file.Path, homeDir, "~", 1)

			// Build mention set for deduplication
			mentionSet := make(map[string]bool)
			for _, m := range mentions {
				mentionSet[m] = true
			}

			// Convert set back to sorted slice
			uniqueMentions := make([]string, 0, len(mentionSet))
			for m := range mentionSet {
				uniqueMentions = append(uniqueMentions, m)
			}
			sort.Strings(uniqueMentions)

			fileConnections[relativePath] = &ConnectionInfo{
				FilePath:   relativePath,
				Mentions:   uniqueMentions,
				MentionSet: mentionSet,
			}

			// Count mentions per identity
			for identity := range mentionSet {
				identityMentions[identity]++
			}
		}
	}

	// Display files with connections
	output.Header("Files with connections:")
	fmt.Println("")

	if len(fileConnections) == 0 {
		fmt.Println("No connections found yet. The garden is just beginning.")
	} else {
		// Sort file paths for consistent output
		sortedFiles := make([]string, 0, len(fileConnections))
		for path := range fileConnections {
			sortedFiles = append(sortedFiles, path)
		}
		sort.Strings(sortedFiles)

		for _, path := range sortedFiles {
			info := fileConnections[path]
			count := len(info.Mentions)

			fmt.Printf("%s (%d connections)\n", output.Yellow+path+output.Reset, count)
			fmt.Printf("  → %s\n", strings.Join(info.Mentions, " "))
			fmt.Println("")
		}
	}

	// Display most-mentioned identities
	if len(identityMentions) > 0 {
		fmt.Println("")
		output.Header("Most connected identities:")
		fmt.Println("")

		// Convert to slice for sorting
		counts := make([]IdentityCount, 0, len(identityMentions))
		for identity, count := range identityMentions {
			counts = append(counts, IdentityCount{Identity: identity, Count: count})
		}

		// Sort by count descending
		sort.Slice(counts, func(i, j int) bool {
			if counts[i].Count != counts[j].Count {
				return counts[i].Count > counts[j].Count
			}
			return counts[i].Identity < counts[j].Identity
		})

		// Display top 10
		limit := 10
		if len(counts) < limit {
			limit = len(counts)
		}

		for i := 0; i < limit; i++ {
			fmt.Printf("  %s (mentioned in %d files)\n", counts[i].Identity, counts[i].Count)
		}
	}

	fmt.Println("")
	output.Success("✨ The garden grows through collaboration")

	return nil
}

// findIdentityMentions searches content for mentions of other identities
// excluding self-references. Returns slice of mentioned identities.
func findIdentityMentions(content string, selfIdentity string, allIdentities []string) []string {
	var mentions []string
	contentLower := strings.ToLower(content)

	for _, other := range allIdentities {
		// Skip self-references
		if other == selfIdentity {
			continue
		}

		// Use word boundary regex for case-insensitive match
		pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(other))
		re := regexp.MustCompile(`(?i)` + pattern)

		if re.MatchString(contentLower) {
			mentions = append(mentions, other)
		}
	}

	return mentions
}
