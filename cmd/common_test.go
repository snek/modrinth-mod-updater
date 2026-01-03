package cmd

import (
	"testing"

	"modrinth-mod-updater/modrinth"
)

func TestGetTargetSubDir(t *testing.T) {
	tests := []struct {
		projectType string
		expected    string
	}{
		{"mod", "mods"},
		{"shader", "shaderpacks"},
		{"resourcepack", "resourcepacks"},
		{"something-else", "mods"}, // default case
		{"", "mods"},               // empty case
	}

	for _, tt := range tests {
		t.Run(tt.projectType, func(t *testing.T) {
			result := getTargetSubDir(tt.projectType)
			if result != tt.expected {
				t.Errorf("getTargetSubDir(%q) = %q, want %q", tt.projectType, result, tt.expected)
			}
		})
	}
}

func TestFindPrimaryFile(t *testing.T) {
	t.Run("primary file exists", func(t *testing.T) {
		v := modrinth.Version{
			Files: []modrinth.File{
				{Filename: "secondary.jar", Primary: false},
				{Filename: "primary.jar", Primary: true},
				{Filename: "also-secondary.jar", Primary: false},
			},
		}
		result := findPrimaryFile(v)
		if result == nil || result.Filename != "primary.jar" {
			t.Errorf("findPrimaryFile() failed to find primary file")
		}
	})

	t.Run("no primary marked, returns first", func(t *testing.T) {
		v := modrinth.Version{
			Files: []modrinth.File{
				{Filename: "file1.jar", Primary: false},
				{Filename: "file2.jar", Primary: false},
			},
		}
		result := findPrimaryFile(v)
		if result == nil || result.Filename != "file1.jar" {
			t.Errorf("findPrimaryFile() failed to return first file when no primary marked")
		}
	})

	t.Run("empty files list", func(t *testing.T) {
		v := modrinth.Version{
			Files: []modrinth.File{},
		}
		result := findPrimaryFile(v)
		if result != nil {
			t.Errorf("findPrimaryFile() should return nil for empty files list")
		}
	})
}

func TestProjectSupportsInstallationType(t *testing.T) {
	tests := []struct {
		name             string
		project          modrinth.Project
		installationType string
		expected         bool
	}{
		{
			"client required on client",
			modrinth.Project{ClientSide: "required"},
			"client",
			true,
		},
		{
			"client optional on client",
			modrinth.Project{ClientSide: "optional"},
			"client",
			true,
		},
		{
			"client unsupported on client",
			modrinth.Project{ClientSide: "unsupported"},
			"client",
			false,
		},
		{
			"server required on server",
			modrinth.Project{ServerSide: "required"},
			"server",
			true,
		},
		{
			"server optional on server",
			modrinth.Project{ServerSide: "optional"},
			"server",
			true,
		},
		{
			"server unsupported on server",
			modrinth.Project{ServerSide: "unsupported"},
			"server",
			false,
		},
		{
			"unknown installation type",
			modrinth.Project{},
			"unknown",
			true,
		},
		{
			"both type allows client-only",
			modrinth.Project{ClientSide: "required", ServerSide: "unsupported"},
			"both",
			true,
		},
		{
			"both type allows server-only",
			modrinth.Project{ClientSide: "unsupported", ServerSide: "required"},
			"both",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := projectSupportsInstallationType(tt.project, tt.installationType)
			if result != tt.expected {
				t.Errorf("projectSupportsInstallationType() = %v, want %v", result, tt.expected)
			}
		})
	}
}
