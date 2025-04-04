package db

import (
	"gorm.io/gorm"
	"time"
)

// Mod represents a downloaded mod in the database
type Mod struct {
	gorm.Model
	ProjectSlug string `gorm:"uniqueIndex"` // Modrinth Project Slug (unique identifier)
	ProjectID   string // Modrinth Project ID
	Title       string // Mod Title
	IconURL     string // Mod Icon URL
	Color       int    // Mod Color
	Updated     time.Time // Last time the mod was updated on Modrinth
	VersionID   string // Modrinth Version ID
	FileName    string // Downloaded file name
	InstallPath string // Path where the mod is currently installed
}

// ModVersion represents a historical version of a mod
type ModVersion struct {
	gorm.Model
	ProjectSlug   string // References Mod.ProjectSlug
	VersionID     string // Modrinth Version ID
	VersionNumber string // Human-readable version number
	FileName      string // Original file name
	ArchivePath   string // Path to the archived file (if kept)
}
