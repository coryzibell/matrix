package main

import (
	"fmt"
	"os"

	"github.com/coryzibell/matrix/internal/ram"
)

func main() {
	// Get default RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting RAM directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scanning RAM directory: %s\n\n", ramDir)

	// Scan the directory
	files, err := ram.ScanDir(ramDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d markdown files:\n\n", len(files))

	// Group by identity
	identityMap := make(map[string][]ram.File)
	for _, file := range files {
		identityMap[file.Identity] = append(identityMap[file.Identity], file)
	}

	// Display results grouped by identity
	for identity, identityFiles := range identityMap {
		fmt.Printf("  %s: %d files\n", identity, len(identityFiles))
		for _, file := range identityFiles {
			contentSize := len(file.Content)
			fmt.Printf("    - %s (%d bytes)\n", file.Name, contentSize)
		}
		fmt.Println()
	}
}
