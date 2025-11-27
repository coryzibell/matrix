package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coryzibell/matrix/internal/output"
	"github.com/coryzibell/matrix/internal/ram"
)

// runQuestion implements the question command
func runQuestion() error {
	// Parse flags
	var targetIdentity string
	var showContext bool

	args := os.Args[2:] // Skip command name
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--identity":
			if i+1 < len(args) {
				targetIdentity = args[i+1]
				i++ // Skip next arg
			}
		case "--context":
			showContext = true
		}
	}

	// Get RAM directory
	ramDir, err := ram.DefaultRAMDir()
	if err != nil {
		return fmt.Errorf("failed to get RAM directory: %w", err)
	}

	// Check if garden exists
	if _, err := os.Stat(ramDir); os.IsNotExist(err) {
		fmt.Println("The garden is empty. Nothing to question yet.")
		return nil
	}

	// Find all markdown files
	var files []string
	searchDir := ramDir
	if targetIdentity != "" {
		searchDir = filepath.Join(ramDir, targetIdentity)
		if _, err := os.Stat(searchDir); os.IsNotExist(err) {
			return fmt.Errorf("identity directory not found: %s", targetIdentity)
		}
	}

	err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No markdown files found. Nothing to question yet.")
		return nil
	}

	// Seed random and pick a file
	rand.Seed(time.Now().UnixNano())
	selectedFile := files[rand.Intn(len(files))]

	// Get home dir for display
	homeDir, _ := os.UserHomeDir()
	displayPath := strings.Replace(selectedFile, homeDir, "~", 1)

	// Output the question
	fmt.Println("🥄 Spoon's Question")
	fmt.Println("")
	fmt.Printf("File: %s\n", output.Yellow+displayPath+output.Reset)
	fmt.Println("")

	// Show context if requested
	if showContext {
		context, err := readFirstLines(selectedFile, 10)
		if err == nil && context != "" {
			fmt.Println(output.Dim + "Context:" + output.Reset)
			for _, line := range strings.Split(context, "\n") {
				fmt.Println(output.Dim + "  " + line + output.Reset)
			}
			fmt.Println("")
		}
	}

	fmt.Println("What assumption created this work?")
	fmt.Println("")
	fmt.Println("Consider:")
	fmt.Println("  • Could this problem not exist with different architecture?")
	fmt.Println("  • Is this solving a symptom instead of a cause?")
	fmt.Println("  • What would make this documentation unnecessary?")
	fmt.Println("")
	fmt.Println("Not to criticize, but to notice.")

	return nil
}

// readFirstLines reads the first N lines from a file
func readFirstLines(filePath string, n int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	count := 0

	for scanner.Scan() && count < n {
		line := scanner.Text()
		// Skip empty lines and markdown headers at start
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
