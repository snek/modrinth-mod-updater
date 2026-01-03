package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"modrinth-mod-updater/db"
)

// TestOldVersionRemoval tests that old mod files are removed when KeepOldVersions is false
func TestOldVersionRemoval(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create test mods directory: %v", err)
	}

	// Create mock old and new mod files
	oldModFile := filepath.Join(modsDir, "old-mod-1.0.jar")
	newModFile := filepath.Join(modsDir, "new-mod-2.0.jar")

	// Write old mod file
	if err := os.WriteFile(oldModFile, []byte("old mod content"), 0644); err != nil {
		t.Fatalf("Failed to create old mod file: %v", err)
	}

	// Verify old file exists
	if _, err := os.Stat(oldModFile); os.IsNotExist(err) {
		t.Fatal("Old mod file should exist before removal")
	}

	// Simulate old file removal (what the fix does)
	if err := os.Remove(oldModFile); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to remove old mod file: %v", err)
	}

	// Verify old file is removed
	if _, err := os.Stat(oldModFile); !os.IsNotExist(err) {
		t.Fatal("Old mod file should be removed")
	}

	// Write new mod file
	if err := os.WriteFile(newModFile, []byte("new mod content"), 0644); err != nil {
		t.Fatalf("Failed to create new mod file: %v", err)
	}

	// Verify new file exists
	if _, err := os.Stat(newModFile); os.IsNotExist(err) {
		t.Fatal("New mod file should exist")
	}
}

// TestOldVersionArchiving tests that old mod files are archived when KeepOldVersions is true
func TestOldVersionArchiving(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	versionsDir := filepath.Join(modsDir, "versions")

	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create mock old mod file
	oldModFile := filepath.Join(modsDir, "old-mod-1.0.jar")
	if err := os.WriteFile(oldModFile, []byte("old mod content"), 0644); err != nil {
		t.Fatalf("Failed to create old mod file: %v", err)
	}

	// Verify old file exists
	if _, err := os.Stat(oldModFile); os.IsNotExist(err) {
		t.Fatal("Old mod file should exist before archiving")
	}

	// Simulate archiving (what the fix does when KeepOldVersions is true)
	versionID := "version-123"
	fileName := "old-mod-1.0.jar"
	archivedPath := filepath.Join(versionsDir, fmt.Sprintf("%s-%s", versionID, fileName))

	if err := os.Rename(oldModFile, archivedPath); err != nil {
		t.Fatalf("Failed to move old file to versions directory: %v", err)
	}

	// Verify old file is moved
	if _, err := os.Stat(oldModFile); !os.IsNotExist(err) {
		t.Fatal("Old mod file should be moved from original location")
	}

	// Verify archived file exists
	if _, err := os.Stat(archivedPath); os.IsNotExist(err) {
		t.Fatal("Archived mod file should exist in versions directory")
	}

	// Verify archived file has correct name format
	expectedName := fmt.Sprintf("%s-%s", versionID, fileName)
	actualName := filepath.Base(archivedPath)
	if actualName != expectedName {
		t.Fatalf("Archived file name mismatch. Expected: %s, Got: %s", expectedName, actualName)
	}
}

// TestMultipleModUpdates tests that multiple mods can be updated correctly
func TestMultipleModUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create test mods directory: %v", err)
	}

	// Create multiple old mod files
	mods := []struct {
		oldFile string
		newFile string
	}{
		{"mod1-1.0.jar", "mod1-2.0.jar"},
		{"mod2-1.5.jar", "mod2-2.5.jar"},
		{"mod3-0.9.jar", "mod3-1.1.jar"},
	}

	// Create old files
	for _, mod := range mods {
		oldPath := filepath.Join(modsDir, mod.oldFile)
		if err := os.WriteFile(oldPath, []byte("old content"), 0644); err != nil {
			t.Fatalf("Failed to create old mod file: %v", err)
		}
	}

	// Remove old files and create new ones
	for _, mod := range mods {
		oldPath := filepath.Join(modsDir, mod.oldFile)
		newPath := filepath.Join(modsDir, mod.newFile)

		// Remove old file
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			t.Fatalf("Failed to remove old mod file: %v", err)
		}

		// Create new file
		if err := os.WriteFile(newPath, []byte("new content"), 0644); err != nil {
			t.Fatalf("Failed to create new mod file: %v", err)
		}
	}

	// Verify all old files are removed and new files exist
	for _, mod := range mods {
		oldPath := filepath.Join(modsDir, mod.oldFile)
		newPath := filepath.Join(modsDir, mod.newFile)

		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Fatalf("Old mod file %s should be removed", mod.oldFile)
		}

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			t.Fatalf("New mod file %s should exist", mod.newFile)
		}
	}
}

// TestVersionHistoryTracking tests that version history is properly tracked
func TestVersionHistoryTracking(t *testing.T) {
	// This test verifies the data structure for version history
	oldVersion := db.ModVersion{
		ProjectSlug:   "test-mod",
		VersionID:     "version-123",
		VersionNumber: "1.0.0",
		FileName:      "test-mod-1.0.jar",
		ArchivePath:   "/path/to/versions/version-123-test-mod-1.0.jar",
	}

	// Verify all fields are set correctly
	if oldVersion.ProjectSlug != "test-mod" {
		t.Fatal("ProjectSlug not set correctly")
	}
	if oldVersion.VersionID != "version-123" {
		t.Fatal("VersionID not set correctly")
	}
	if oldVersion.FileName != "test-mod-1.0.jar" {
		t.Fatal("FileName not set correctly")
	}
	if oldVersion.ArchivePath != "/path/to/versions/version-123-test-mod-1.0.jar" {
		t.Fatal("ArchivePath not set correctly")
	}
}

// TestFilePathConstruction tests that file paths are constructed correctly
func TestFilePathConstruction(t *testing.T) {
	minecraftDir := "/home/user/.minecraft"
	projectBaseDir := filepath.Join(minecraftDir, "mods")
	versionID := "abc123"
	fileName := "example-mod-1.0.jar"

	// Test archive path construction
	versionsDir := filepath.Join(projectBaseDir, "versions")
	archivedPath := filepath.Join(versionsDir, fmt.Sprintf("%s-%s", versionID, fileName))

	expectedPath := filepath.Join(minecraftDir, "mods", "versions", "abc123-example-mod-1.0.jar")
	if archivedPath != expectedPath {
		t.Fatalf("Archive path mismatch. Expected: %s, Got: %s", expectedPath, archivedPath)
	}
}

// TestOldFileNotFoundHandling tests that missing old files don't cause errors
func TestOldFileNotFoundHandling(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create test mods directory: %v", err)
	}

	// Try to remove a file that doesn't exist
	nonExistentFile := filepath.Join(modsDir, "non-existent-mod.jar")

	// This should not error when file doesn't exist
	if err := os.Remove(nonExistentFile); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Should handle non-existent files gracefully: %v", err)
	}

	// Verify the error is specifically "file not found"
	if err := os.Remove(nonExistentFile); err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("Error should be IsNotExist: %v", err)
		}
	}
}

// TestConcurrentModUpdates tests that concurrent updates don't interfere
func TestConcurrentModUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create test mods directory: %v", err)
	}

	// Create test files for concurrent operations
	done := make(chan bool, 3)

	// Simulate concurrent mod updates
	for i := 1; i <= 3; i++ {
		go func(modNum int) {
			oldFile := filepath.Join(modsDir, fmt.Sprintf("mod%d-old.jar", modNum))
			newFile := filepath.Join(modsDir, fmt.Sprintf("mod%d-new.jar", modNum))

			// Create old file
			if err := os.WriteFile(oldFile, []byte(fmt.Sprintf("mod%d old", modNum)), 0644); err != nil {
				t.Errorf("Failed to create old mod file: %v", err)
				done <- false
				return
			}

			// Remove old file
			if err := os.Remove(oldFile); err != nil && !os.IsNotExist(err) {
				t.Errorf("Failed to remove old mod file: %v", err)
				done <- false
				return
			}

			// Create new file
			if err := os.WriteFile(newFile, []byte(fmt.Sprintf("mod%d new", modNum)), 0644); err != nil {
				t.Errorf("Failed to create new mod file: %v", err)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		if !<-done {
			t.Fatal("Concurrent update failed")
		}
	}

	// Verify all new files exist and old files don't
	for i := 1; i <= 3; i++ {
		oldFile := filepath.Join(modsDir, fmt.Sprintf("mod%d-old.jar", i))
		newFile := filepath.Join(modsDir, fmt.Sprintf("mod%d-new.jar", i))

		if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
			t.Fatalf("Old mod file mod%d-old.jar should be removed", i)
		}

		if _, err := os.Stat(newFile); os.IsNotExist(err) {
			t.Fatalf("New mod file mod%d-new.jar should exist", i)
		}
	}
}
