package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"modrinth-mod-updater/db"
	"modrinth-mod-updater/logger"
	"modrinth-mod-updater/ui"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// rollbackCmd represents the rollback command
var rollbackCmd = &cobra.Command{
	Use:   "rollback [projectSlug]",
	Short: "Rollback a mod to its previous version",
	Long: `Rollback a mod to its previous version.
Example: modrinth-mod-updater rollback sodium

This will remove the current version of the mod and
replace it with the most recent previous version.`,
	Args: cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		projectSlug := args[0]
		rollbackMod(projectSlug)
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}

// rollbackMod handles the rollback process for a specific mod
func rollbackMod(projectSlug string) {
	// Find the current mod
	var currentMod db.Mod
	result := db.DB.Where("project_slug = ?", projectSlug).First(&currentMod)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			logger.Log.Warnw("Mod not found in database", zap.Error(result.Error))
			return
		}
		logger.Log.Fatalw("Failed to query database", zap.Error(result.Error))
	}

	log := logger.Log.With(zap.String("mod_title", ui.Colorize(currentMod.Title, currentMod.Color)))

	log.Infow("Attempting rollback")

	// Find the most recent previous version
	var previousVersion db.ModVersion
	result = db.DB.Where("project_slug = ?", projectSlug).Order("created_at DESC").First(&previousVersion)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			log.Fatalw("No previous versions found for mod", zap.String("mod", projectSlug))
		}
		log.Fatalw("Failed to query version history", zap.Error(result.Error))
	}

	// Check if the archive file still exists
	if previousVersion.ArchivePath == "" {
		log.Fatalw("Previous version for mod has no archive path", zap.String("mod", projectSlug))
	}

	if _, err := os.Stat(previousVersion.ArchivePath); errors.Is(err, os.ErrNotExist) {
		log.Fatalw("Archive file not found", zap.String("archive_path", previousVersion.ArchivePath))
	}

	// Get the mod directory
	modsDir := filepath.Dir(currentMod.InstallPath)

	// Delete the current file
	log.Infow(ui.Colorize("Removing current version", currentMod.Color), zap.String("file", currentMod.InstallPath))
	if err := os.Remove(currentMod.InstallPath); err != nil && !os.IsNotExist(err) {
		log.Warnw("Failed to remove current version", zap.String("file", currentMod.InstallPath), zap.Error(err))
	}

	// Copy the previous version to the mods directory
	targetPath := filepath.Join(modsDir, previousVersion.FileName)

	log.Infow(ui.Colorize("Restoring previous version", currentMod.Color),
		zap.String("file", previousVersion.FileName),
		zap.String("version", previousVersion.VersionID),
	)

	// Read the source file
	sourceBytes, err := os.ReadFile(previousVersion.ArchivePath)
	if err != nil {
		log.Fatalw("Failed to read archive file", zap.String("file", previousVersion.ArchivePath), zap.Error(err))
	}

	// Write to the destination
	if err := os.WriteFile(targetPath, sourceBytes, 0644); err != nil {
		log.Fatalw("Failed to write file", zap.String("file", targetPath), zap.Error(err))
	}

	// Update the current mod record in the database
	currentMod.VersionID = previousVersion.VersionID
	currentMod.FileName = previousVersion.FileName
	currentMod.InstallPath = targetPath

	if err := db.DB.Save(&currentMod).Error; err != nil {
		log.Fatalw("Failed to update database record", zap.Error(err))
	}

	// Delete the rollback record
	if err := db.DB.Delete(&previousVersion).Error; err != nil {
		log.Warnw("Failed to delete history record",
			zap.String("version", previousVersion.VersionID),
			zap.Error(err),
		)
	}

	log.Infow(ui.Colorize("Rollback successful", currentMod.Color),
		zap.String("restored_version_id", currentMod.VersionID),
		zap.String("restored_file", currentMod.FileName),
	)

	fmt.Printf("Successfully rolled back %s to version %s\n", projectSlug, previousVersion.VersionID)
}
