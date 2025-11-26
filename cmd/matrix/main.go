package main

import (
	"fmt"
	"os"
)

func main() {
	// Simple command routing without cobra for now
	if len(os.Args) < 2 {
		fmt.Println("matrix v0.0.1")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  garden-paths    Discover connections in the matrix garden")
		fmt.Println("  tension-map     Surface conflicts and tensions across RAM")
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "garden-paths":
		if err := runGardenPaths(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "tension-map":
		if err := runTensionMap(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "--help", "-h", "help":
		fmt.Println("matrix v0.0.1")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  garden-paths    Discover connections in the matrix garden")
		fmt.Println("  tension-map     Surface conflicts and tensions across RAM")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Println("Run 'matrix help' for usage")
		os.Exit(1)
	}
}
