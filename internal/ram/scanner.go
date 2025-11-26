package ram

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// File represents a markdown file found in the RAM directory
type File struct {
	Path     string // Full absolute path to the file
	Identity string // Identity name (subdirectory name)
	Name     string // File name without extension
	Content  string // Raw file content
}

// DefaultRAMDir returns the default RAM directory path with ~ expanded
func DefaultRAMDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".claude", "ram"), nil
}

// ScanDir finds all .md files in the RAM directory subdirectories
// and returns a slice of File structs populated with their data.
// It scans one level deep (identity directories) and finds all .md files within.
func ScanDir(ramDir string) ([]File, error) {
	// Check if RAM directory exists
	if _, err := os.Stat(ramDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("RAM directory does not exist: %s", ramDir)
		}
		return nil, fmt.Errorf("failed to access RAM directory: %w", err)
	}

	var files []File

	// Read identity directories (first level)
	entries, err := os.ReadDir(ramDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read RAM directory %s: %w", ramDir, err)
	}

	// Iterate through identity directories
	for _, entry := range entries {
		// Skip non-directories
		if !entry.IsDir() {
			continue
		}

		identityName := entry.Name()
		identityPath := filepath.Join(ramDir, identityName)

		// Read all files in this identity directory
		err := filepath.WalkDir(identityPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				// Skip directories we can't read
				return nil
			}

			// Skip directories
			if d.IsDir() {
				return nil
			}

			// Only process .md files
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}

			// Read file content
			content, err := os.ReadFile(path)
			if err != nil {
				// Skip files we can't read
				return nil
			}

			// Extract name without extension
			fileName := d.Name()
			name := strings.TrimSuffix(fileName, filepath.Ext(fileName))

			// Create File struct
			file := File{
				Path:     path,
				Identity: identityName,
				Name:     name,
				Content:  string(content),
			}

			files = append(files, file)
			return nil
		})

		if err != nil {
			// Log error but continue with other identities
			continue
		}
	}

	return files, nil
}
