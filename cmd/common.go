package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"modrinth-mod-updater/config"
	"modrinth-mod-updater/db"
	"modrinth-mod-updater/logger"
	"modrinth-mod-updater/modrinth"

	"go.uber.org/zap"
)

// bootstrap handles shared initialization logic for commands.
func bootstrap(path string) (config.Config, *modrinth.Client) {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		logger.Log.Fatalw("Failed to load configuration", zap.Error(err))
	}

	db.InitDatabase(cfg.DatabasePath)
	logger.Log.Infow("Database initialized", zap.String("path", cfg.DatabasePath))

	if cfg.ModrinthAPIKey == "" {
		logger.Log.Fatal("Error: MODRINTH_API_KEY must be set.")
	}
	if cfg.MinecraftVersion == "" || cfg.MinecraftLoader == "" {
		logger.Log.Fatal("Error: MINECRAFT_VERSION and MINECRAFT_LOADER must be set.")
	}

	client, err := modrinth.NewClient(cfg)
	if err != nil {
		logger.Log.Fatalw("Failed to create Modrinth client", zap.Error(err))
	}

	if err := importInstalledMods(client, cfg.MinecraftDir); err != nil {
		logger.Log.Warnw("Failed to import installed mods", zap.Error(err))
	}

	return cfg, client
}

// getTargetSubDir returns the appropriate subdirectory for a project type.
func getTargetSubDir(projectType string) string {
	switch projectType {
	case "mod":
		return "mods"
	case "shader":
		return "shaderpacks"
	case "resourcepack":
		return "resourcepacks"
	default:
		return "mods"
	}
}

// findPrimaryFile locates the primary file in a Modrinth version, or the first file if no primary is marked.
func findPrimaryFile(v modrinth.Version) *modrinth.File {
	for i := range v.Files {
		if v.Files[i].Primary {
			return &v.Files[i]
		}
	}
	if len(v.Files) > 0 {
		return &v.Files[0]
	}
	return nil
}

// projectSupportsInstallationType checks if a project supports the configured installation type.
func projectSupportsInstallationType(p modrinth.Project, installationType string) bool {
	switch strings.ToLower(installationType) {
	case "client":
		return p.ClientSide == "required" || p.ClientSide == "optional"
	case "server":
		return p.ServerSide == "required" || p.ServerSide == "optional"
	case "both":
		return true // Install everything
	default:
		return true // If installation type is unknown/unspecified, assume support
	}
}

// archiveAndCleanupOld handles moving old mod versions to the archive or deleting them.
func archiveAndCleanupOld(existingMod db.Mod, projectBaseDir string, cfg *config.Config, goroutineLogger *zap.SugaredLogger) {
	oldFilePath := filepath.Join(projectBaseDir, existingMod.FileName)
	archivePath := ""

	if cfg.KeepOldVersions {
		versionsDir := filepath.Join(projectBaseDir, "versions")
		// Ensure versions directory exists
		_ = os.MkdirAll(versionsDir, 0755)

		newPathInVersions := filepath.Join(versionsDir, fmt.Sprintf("%s-%s", existingMod.VersionID, existingMod.FileName))
		if err := os.Rename(oldFilePath, newPathInVersions); err == nil {
			archivePath = newPathInVersions
		} else if !os.IsNotExist(err) {
			goroutineLogger.Warnw("Failed to archive old mod version", zap.String("file", existingMod.FileName), zap.Error(err))
		}
	} else {
		if err := os.Remove(oldFilePath); err != nil && !os.IsNotExist(err) {
			goroutineLogger.Warnw("Failed to remove old mod version", zap.String("file", existingMod.FileName), zap.Error(err))
		}
	}

	// Record the old version in history
	if err := db.DB.Create(&db.ModVersion{
		ProjectSlug:   existingMod.ProjectSlug,
		VersionID:     existingMod.VersionID,
		VersionNumber: existingMod.VersionNumber,
		FileName:      existingMod.FileName,
		ArchivePath:   archivePath,
	}).Error; err != nil {
		goroutineLogger.Warnw("Failed to save mod version history to database", zap.Error(err))
	}
}
