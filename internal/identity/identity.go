package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// All known identities in the matrix system
var identities = []string{
	"neo",
	"smith",
	"fellas",
	"seraph",
	"kid",
	"tank",
	"trinity",
	"morpheus",
	"oracle",
	"architect",
	"cypher",
	"niobe",
	"keymaker",
	"merovingian",
	"librarian",
	"twins",
	"trainman",
	"deus",
	"hamann",
	"spoon",
	"sati",
	"ramakandra",
	"persephone",
	"lock",
	"zee",
	"mouse",
	"apoc",
	"switch",
}

// All returns all identity names
func All() []string {
	result := make([]string, len(identities))
	copy(result, identities)
	return result
}

// IsValid checks if a name is a valid identity
func IsValid(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, id := range identities {
		if id == normalized {
			return true
		}
	}
	return false
}

// RAMPath returns the expanded path to an identity's RAM directory
// Returns ~/.claude/ram/{name} expanded to absolute path
func RAMPath(name string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if !IsValid(normalized) {
		return "", fmt.Errorf("invalid identity: %s", name)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".claude", "ram", normalized), nil
}
