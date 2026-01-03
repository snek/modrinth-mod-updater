package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateSHA1(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// echo -n "hello world" | sha1sum
	// 2aae6c35c94fcfb415dbe95f408b9ce91ee846ed
	expected := "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed"

	hash, err := calculateSHA1(filePath)
	if err != nil {
		t.Fatalf("calculateSHA1 failed: %v", err)
	}

	if hash != expected {
		t.Errorf("calculateSHA1() = %s, want %s", hash, expected)
	}
}

func TestCalculateSHA1FileNotFound(t *testing.T) {
	_, err := calculateSHA1("non-existent-file")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}
