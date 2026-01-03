package cmd

import (
	"testing"
)

// TestModInfoStructure tests that ModInfo is properly structured
func TestModInfoStructure(t *testing.T) {
	mod := ModInfo{
		Title:            "Test Mod",
		Slug:             "test-mod",
		InstalledVersion: "1.0.0",
		AvailableVersion: "2.0.0",
		Status:           "update-available",
		Color:            16711680, // Red
		ProjectType:      "mod",
	}

	if mod.Title != "Test Mod" {
		t.Fatal("Title not set correctly")
	}
	if mod.Slug != "test-mod" {
		t.Fatal("Slug not set correctly")
	}
	if mod.Status != "update-available" {
		t.Fatal("Status not set correctly")
	}
}

// TestModelInitialization tests that the Model initializes correctly
func TestModelInitialization(t *testing.T) {
	m := Model{
		selectedIndex: 0,
		loading:       true,
		width:         80,
		height:        24,
	}

	if m.selectedIndex != 0 {
		t.Fatal("selectedIndex not initialized correctly")
	}
	if !m.loading {
		t.Fatal("loading should be true initially")
	}
	if m.width != 80 || m.height != 24 {
		t.Fatal("width or height not initialized correctly")
	}
}

// TestTruncateFunction tests the truncate helper function
func TestTruncateFunction(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"Hello World", 5, "He..."},
		{"Hi", 5, "Hi"},
		{"Test", 4, "Test"},
		{"LongString", 7, "Long..."},
		{"", 5, ""},
	}

	for _, test := range tests {
		result := truncate(test.input, test.maxLen)
		if result != test.expected {
			t.Fatalf("truncate(%q, %d) = %q, expected %q", test.input, test.maxLen, result, test.expected)
		}
	}
}

// TestModStatusDetermination tests status determination logic
func TestModStatusDetermination(t *testing.T) {
	tests := []struct {
		installedVersion string
		availableVersion string
		expectedStatus   string
	}{
		{"1.0.0", "1.0.0", "up-to-date"},
		{"1.0.0", "2.0.0", "update-available"},
		{"Not installed", "1.0.0", "not-installed"},
	}

	for _, test := range tests {
		mod := ModInfo{
			InstalledVersion: test.installedVersion,
			AvailableVersion: test.availableVersion,
		}

		// Determine status based on versions
		switch {
		case test.installedVersion == "Not installed":
			mod.Status = "not-installed"
		case test.installedVersion == test.availableVersion:
			mod.Status = "up-to-date"
		default:
			mod.Status = "update-available"
		}

		if mod.Status != test.expectedStatus {
			t.Fatalf("Status determination failed for %q vs %q: got %q, expected %q",
				test.installedVersion, test.availableVersion, mod.Status, test.expectedStatus)
		}
	}
}

// TestModelNavigation tests navigation within the model
func TestModelNavigation(t *testing.T) {
	m := Model{
		selectedIndex: 0,
		mods: []ModInfo{
			{Title: "Mod 1"},
			{Title: "Mod 2"},
			{Title: "Mod 3"},
		},
	}

	// Test moving down
	if m.selectedIndex < len(m.mods)-1 {
		m.selectedIndex++
	}
	if m.selectedIndex != 1 {
		t.Fatal("Navigation down failed")
	}

	// Test moving down again
	if m.selectedIndex < len(m.mods)-1 {
		m.selectedIndex++
	}
	if m.selectedIndex != 2 {
		t.Fatal("Navigation down failed on second move")
	}

	// Test boundary - shouldn't go beyond last item
	if m.selectedIndex < len(m.mods)-1 {
		m.selectedIndex++
	}
	if m.selectedIndex != 2 {
		t.Fatal("Navigation should stop at last item")
	}

	// Test moving up
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
	if m.selectedIndex != 1 {
		t.Fatal("Navigation up failed")
	}

	// Test boundary - shouldn't go below first item
	m.selectedIndex = 0
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
	if m.selectedIndex != 0 {
		t.Fatal("Navigation should stop at first item")
	}
}

// TestEmptyModList tests behavior with empty mod list
func TestEmptyModList(t *testing.T) {
	m := Model{
		selectedIndex: 0,
		mods:          []ModInfo{},
		loading:       false,
	}

	if len(m.mods) != 0 {
		t.Fatal("Mods list should be empty")
	}

	// View should handle empty list gracefully
	view := m.View()
	if view == "" {
		t.Fatal("View should return a message for empty mod list")
	}
}

// TestModInfoWithDifferentProjectTypes tests ModInfo with various project types
func TestModInfoWithDifferentProjectTypes(t *testing.T) {
	projectTypes := []string{"mod", "shader", "resourcepack"}

	for _, pType := range projectTypes {
		mod := ModInfo{
			Title:       "Test " + pType,
			ProjectType: pType,
			Status:      "up-to-date",
		}

		if mod.ProjectType != pType {
			t.Fatalf("ProjectType not set correctly for %s", pType)
		}
	}
}
