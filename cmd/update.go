package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"modrinth-mod-updater/config"
	"modrinth-mod-updater/db"
	"modrinth-mod-updater/logger"
	"modrinth-mod-updater/modrinth"
	"modrinth-mod-updater/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Checks for and downloads updates for followed mods",
	Long: `Checks Modrinth for new compatible versions of followed mods
and downloads them to the 'mods' directory.`,
	Run: func(cmd *cobra.Command, _ []string) {
		logger.Log.Info("Running update command...")

		// Get the force flag value
		forceUpdate, _ := cmd.Flags().GetBool("force")

		// Run update with TUI
		p := tea.NewProgram(initialUpdateModel(forceUpdate))
		if _, err := p.Run(); err != nil {
			logger.Log.Fatalw("Failed to run update UI", zap.Error(err))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// Add flags for the update command
	updateCmd.Flags().BoolP("force", "f", false, "Force redownload of all mods regardless of version")
}

func runUpdate(forceUpdate bool, progressChan chan<- UpdateProgressMsg) {
	sendMsg := func(msg UpdateProgressMsg) {
		if progressChan != nil {
			progressChan <- msg
		}
	}

	sendMsg(UpdateProgressMsg{Type: "status", Message: "Loading configuration..."})

	cfg, client := bootstrap(".")

	sendMsg(UpdateProgressMsg{Type: "status", Message: "Fetching followed projects..."})
	followedProjects, err := client.GetFollowedProjects()
	if err != nil {
		logger.Log.Fatalw("Failed to get followed projects", zap.Error(err))
	}

	if len(followedProjects) == 0 {
		logger.Log.Info("No followed projects found.")
		sendMsg(UpdateProgressMsg{Type: "summary", Message: "No followed projects found."})
		return
	}

	logger.Log.Infof("Found %d followed projects. Checking for updates for Minecraft %s (%s)...",
		len(followedProjects), cfg.MinecraftVersion, cfg.MinecraftLoader)

	sendMsg(UpdateProgressMsg{Type: "status", Message: fmt.Sprintf("Checking %d projects...", len(followedProjects))})

	var downloadedCount atomic.Int64
	var updatedCount atomic.Int64
	var wg sync.WaitGroup

	for _, project := range followedProjects {
		if project.ProjectType != "mod" && project.ProjectType != "shader" && project.ProjectType != "resourcepack" {
			logger.Log.Infow("Skipping non-mod/shader/resourcepack project",
				zap.String("title", project.Title),
				zap.String("type", project.ProjectType),
			)
			continue
		}

		sendMsg(UpdateProgressMsg{Type: "check", ProjectName: project.Title, Color: project.Color})

		wg.Add(1)
		go func(p modrinth.Project) {
			defer wg.Done()
			processProject(p, &cfg, client, forceUpdate, sendMsg, &downloadedCount, &updatedCount)
		}(project)
	}

	wg.Wait()

	summary := fmt.Sprintf("Finished. Downloaded %d new mods, updated %d existing mods.", downloadedCount.Load(), updatedCount.Load())
	logger.Log.Info(summary)
	sendMsg(UpdateProgressMsg{Type: "summary", Message: summary})
}

func processProject(p modrinth.Project, cfg *config.Config, client *modrinth.Client, forceUpdate bool, sendMsg func(UpdateProgressMsg), downloadedCount, updatedCount *atomic.Int64) {
	goroutineLogger := logger.Log.With(zap.String("project_slug", p.Slug), zap.String("project_title", p.Title))
	goroutineLogger.Info(ui.Colorize("Checking project", p.Color))

	if !shouldProcessProject(p, cfg, goroutineLogger) {
		return
	}

	versions, err := client.GetProjectVersions(p.Slug, p.ProjectType, cfg.MinecraftVersion, cfg.MinecraftLoader)
	if err != nil {
		goroutineLogger.Errorw("Failed to get project versions", zap.Error(err))
		sendMsg(UpdateProgressMsg{Type: "error", ProjectName: p.Title, Message: "Failed to get versions"})
		return
	}

	if len(versions) == 0 {
		goroutineLogger.Info("  No compatible versions found.")
		return
	}

	latestVersion := versions[0]
	primaryFile := findPrimaryFile(latestVersion)
	if primaryFile == nil {
		goroutineLogger.Errorw("Latest version has no files at all!", zap.String("version_id", latestVersion.ID))
		sendMsg(UpdateProgressMsg{Type: "error", ProjectName: p.Title, Message: "No files found for version"})
		return
	}

	targetSubDir := getTargetSubDir(p.ProjectType)
	projectBaseDir := filepath.Join(cfg.MinecraftDir, targetSubDir)

	if cfg.KeepOldVersions {
		_ = os.MkdirAll(filepath.Join(projectBaseDir, "versions"), 0755)
	}

	var existingMod db.Mod
	result := db.DB.Where("project_slug = ?", p.Slug).First(&existingMod)

	if result.Error == nil {
		handleExistingMod(p, existingMod, latestVersion, primaryFile, projectBaseDir, cfg, client, forceUpdate, sendMsg, updatedCount, goroutineLogger)
	} else {
		handleNewMod(p, latestVersion, primaryFile, projectBaseDir, client, sendMsg, downloadedCount, goroutineLogger)
	}
}

func shouldProcessProject(p modrinth.Project, cfg *config.Config, goroutineLogger *zap.SugaredLogger) bool {
	if !projectSupportsInstallationType(p, cfg.MinecraftInstallationType) {
		goroutineLogger.Infow(ui.Colorize("Skipping project, incompatible with installation type", p.Color),
			zap.String("installation_type", cfg.MinecraftInstallationType),
			zap.String("client_side", p.ClientSide),
			zap.String("server_side", p.ServerSide),
		)
		return false
	}
	return true
}

func handleExistingMod(p modrinth.Project, existingMod db.Mod, latestVersion modrinth.Version, primaryFile *modrinth.File, projectBaseDir string, cfg *config.Config, client *modrinth.Client, forceUpdate bool, sendMsg func(UpdateProgressMsg), updatedCount *atomic.Int64, goroutineLogger *zap.SugaredLogger) {
	oldFilePath := filepath.Join(projectBaseDir, existingMod.FileName)
	fileMissing := false
	if _, err := os.Stat(oldFilePath); os.IsNotExist(err) {
		fileMissing = true
		goroutineLogger.Warnw("Mod file missing from disk", zap.String("path", oldFilePath))
	}

	if !forceUpdate && existingMod.VersionID == latestVersion.ID && !fileMissing {
		goroutineLogger.Infow(ui.Colorize("Mod is already up to date", existingMod.Color),
			zap.String("version", existingMod.VersionID),
		)
		return
	}

	switch {
	case fileMissing:
		goroutineLogger.Infow(ui.Colorize("File missing, re-downloading", existingMod.Color), zap.String("version", latestVersion.ID))
	case forceUpdate && existingMod.VersionID == latestVersion.ID:
		goroutineLogger.Infow(ui.Colorize("Force re-downloading mod", existingMod.Color), zap.String("version", latestVersion.ID))
	default:
		goroutineLogger.Infow(ui.Colorize("Update available", existingMod.Color),
			zap.String("current_version", existingMod.VersionID),
			zap.String("new_version", latestVersion.ID),
		)
	}

	sendMsg(UpdateProgressMsg{
		Type:        "download_start",
		ProjectName: p.Title,
		Version:     latestVersion.VersionNumber,
		Color:       p.Color,
	})

	if !fileMissing {
		archiveAndCleanupOld(existingMod, projectBaseDir, cfg, goroutineLogger)
	}

	downloadPath := filepath.Join(projectBaseDir, primaryFile.Filename)
	goroutineLogger.Infow(ui.Colorize("Downloading file...", p.Color), zap.String("file", primaryFile.Filename))
	if err := client.DownloadModFile(goroutineLogger, downloadPath, primaryFile.URL); err != nil {
		goroutineLogger.Errorw("Failed to download mod", zap.String("filename", primaryFile.Filename), zap.Error(err))
		sendMsg(UpdateProgressMsg{Type: "error", ProjectName: p.Title, Message: "Download failed"})
		return
	}

	updatedTime, _ := time.Parse(time.RFC3339Nano, p.Updated)
	existingMod.VersionID = latestVersion.ID
	existingMod.VersionNumber = latestVersion.VersionNumber
	existingMod.FileName = primaryFile.Filename
	existingMod.InstallPath = downloadPath
	existingMod.ProjectID = p.ID
	existingMod.Title = p.Title
	existingMod.IconURL = p.IconURL
	existingMod.Color = p.Color
	existingMod.Updated = updatedTime

	if err := db.DB.Save(&existingMod).Error; err != nil {
		goroutineLogger.Warnw("Failed to update database record", zap.Error(err))
	}
	updatedCount.Add(1)
	sendMsg(UpdateProgressMsg{Type: "download_success", ProjectName: p.Title, Version: latestVersion.VersionNumber})
}

func handleNewMod(p modrinth.Project, latestVersion modrinth.Version, primaryFile *modrinth.File, projectBaseDir string, client *modrinth.Client, sendMsg func(UpdateProgressMsg), downloadedCount *atomic.Int64, goroutineLogger *zap.SugaredLogger) {
	goroutineLogger.Infow(ui.Colorize("New project found - downloading", p.Color), zap.String("version", latestVersion.VersionNumber))

	sendMsg(UpdateProgressMsg{
		Type:        "download_start",
		ProjectName: p.Title,
		Version:     latestVersion.VersionNumber,
		Color:       p.Color,
	})

	downloadPath := filepath.Join(projectBaseDir, primaryFile.Filename)
	if err := client.DownloadModFile(goroutineLogger, downloadPath, primaryFile.URL); err != nil {
		goroutineLogger.Errorw("Failed to download file", zap.String("filename", primaryFile.Filename), zap.Error(err))
		sendMsg(UpdateProgressMsg{Type: "error", ProjectName: p.Title, Message: "Download failed"})
		return
	}

	updatedTime, _ := time.Parse(time.RFC3339Nano, p.Updated)
	newMod := db.Mod{
		ProjectSlug:   p.Slug,
		ProjectID:     p.ID,
		Title:         p.Title,
		IconURL:       p.IconURL,
		Color:         p.Color,
		Updated:       updatedTime,
		VersionID:     latestVersion.ID,
		VersionNumber: latestVersion.VersionNumber,
		FileName:      primaryFile.Filename,
		InstallPath:   downloadPath,
	}

	if err := db.DB.Create(&newMod).Error; err != nil {
		goroutineLogger.Warnw("Failed to save mod to database", zap.Error(err))
	}

	downloadedCount.Add(1)
	sendMsg(UpdateProgressMsg{Type: "download_success", ProjectName: p.Title, Version: latestVersion.VersionNumber})
}
