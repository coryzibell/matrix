package ram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultRAMDir(t *testing.T) {
	ramDir, err := DefaultRAMDir()
	if err != nil {
		t.Fatalf("DefaultRAMDir() failed: %v", err)
	}

	// Should contain .claude/ram
	if !filepath.IsAbs(ramDir) {
		t.Errorf("DefaultRAMDir() returned non-absolute path: %s", ramDir)
	}

	// Basic sanity check - path should be absolute
	homeDir, _ := os.UserHomeDir()
	expectedSuffix := filepath.Join(".claude", "ram")
	if !strings.HasPrefix(ramDir, homeDir) {
		t.Errorf("DefaultRAMDir() should start with home directory, got: %s", ramDir)
	}
	if !strings.HasSuffix(ramDir, expectedSuffix) {
		t.Errorf("DefaultRAMDir() should end with .claude/ram, got: %s", ramDir)
	}
}

func TestScanDir(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()

	// Create identity directories
	smithDir := filepath.Join(tmpDir, "smith")
	trinityDir := filepath.Join(tmpDir, "trinity")

	if err := os.MkdirAll(smithDir, 0755); err != nil {
		t.Fatalf("Failed to create smith directory: %v", err)
	}
	if err := os.MkdirAll(trinityDir, 0755); err != nil {
		t.Fatalf("Failed to create trinity directory: %v", err)
	}

	// Create test files
	smithFile1 := filepath.Join(smithDir, "test1.md")
	smithFile2 := filepath.Join(smithDir, "test2.md")
	trinityFile := filepath.Join(trinityDir, "debug.md")
	nonMdFile := filepath.Join(smithDir, "notes.txt")

	testContent := "# Test Content\nSome markdown here."

	if err := os.WriteFile(smithFile1, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(smithFile2, []byte("# Another file"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(trinityFile, []byte("# Trinity's notes"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(nonMdFile, []byte("Not markdown"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test scanning
	files, err := ScanDir(tmpDir)
	if err != nil {
		t.Fatalf("ScanDir() failed: %v", err)
	}

	// Should find 3 .md files (2 from smith, 1 from trinity)
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}

	// Verify file properties
	smithCount := 0
	trinityCount := 0

	for _, file := range files {
		// Check that path is set
		if file.Path == "" {
			t.Error("File path is empty")
		}

		// Check that content is read
		if file.Content == "" {
			t.Error("File content is empty")
		}

		// Check that name is extracted correctly (no extension)
		if filepath.Ext(file.Name) != "" {
			t.Errorf("File name should not have extension, got: %s", file.Name)
		}

		// Count by identity
		switch file.Identity {
		case "smith":
			smithCount++
		case "trinity":
			trinityCount++
		default:
			t.Errorf("Unexpected identity: %s", file.Identity)
		}
	}

	if smithCount != 2 {
		t.Errorf("Expected 2 smith files, got %d", smithCount)
	}
	if trinityCount != 1 {
		t.Errorf("Expected 1 trinity file, got %d", trinityCount)
	}
}

func TestScanDirNonExistent(t *testing.T) {
	_, err := ScanDir("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}
}

func TestScanDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := ScanDir(tmpDir)
	if err != nil {
		t.Fatalf("ScanDir() failed on empty directory: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files in empty directory, got %d", len(files))
	}
}
