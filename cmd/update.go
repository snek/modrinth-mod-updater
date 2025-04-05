package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"modrinth-mod-updater/config"
	"modrinth-mod-updater/db"
	"modrinth-mod-updater/logger"
	"modrinth-mod-updater/modrinth"
	"modrinth-mod-updater/ui"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Checks for and downloads updates for followed mods",
	Long: `Checks Modrinth for new compatible versions of followed mods
and downloads them to the 'mods' directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Log.Info("Running update command...")
		
		// Get the force flag value
		forceUpdate, _ := cmd.Flags().GetBool("force")
		
		// Run update with force flag value
		runUpdate(forceUpdate)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// Add flags for the update command
	updateCmd.Flags().BoolP("force", "f", false, "Force redownload of all mods regardless of version")
}

// projectSupportsInstallationType checks if a project supports the configured installation type.
func projectSupportsInstallationType(p modrinth.Project, installationType string) bool {
	switch strings.ToLower(installationType) {
	case "client":
		return p.ClientSide == "required" || p.ClientSide == "optional"
	case "server":
		return p.ServerSide == "required" || p.ServerSide == "optional"
	default:
		return true // If installation type is unknown/unspecified, assume support
	}
}

func runUpdate(forceUpdate bool) {
	
	// Load configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		logger.Log.Fatalw("Failed to load configuration", zap.Error(err))
	}
	
	// Initialize database
	db.InitDatabase(cfg.DatabasePath)
	logger.Log.Infow("Database initialized", zap.String("path", cfg.DatabasePath))

	// Ensure target directories exist
	modsDir := filepath.Join(cfg.MinecraftDir, "mods")
	shaderpacksDir := filepath.Join(cfg.MinecraftDir, "shaderpacks")
	resourcepacksDir := filepath.Join(cfg.MinecraftDir, "resourcepacks")
	for _, dir := range []string{modsDir, shaderpacksDir, resourcepacksDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Log.Fatalw("Failed to create required directory",
				zap.String("directory", dir),
				zap.Error(err),
			)
		}
	}

	// Validate required config
	if cfg.ModrinthAPIKey == "" {
		logger.Log.Fatal("Error: MODRINTH_API_KEY must be set.")
	}
	if cfg.MinecraftVersion == "" || cfg.MinecraftLoader == "" {
		logger.Log.Fatal("Error: MINECRAFT_VERSION and MINECRAFT_LOADER must be set.")
	}

	// Create Modrinth client
	client, err := modrinth.NewClient(cfg)
	if err != nil {
		logger.Log.Fatalw("Failed to create Modrinth client", zap.Error(err))
	}

	logger.Log.Info("Fetching followed projects...")
	followedProjects, err := client.GetFollowedProjects()
	if err != nil {
		logger.Log.Fatalw("Failed to get followed projects", zap.Error(err))
	}

	if len(followedProjects) == 0 {
		logger.Log.Info("No followed projects found.")
		return // Exit function, not the whole program
	}

	logger.Log.Infof("Found %d followed projects. Checking for updates for Minecraft %s (%s)...",
		len(followedProjects), cfg.MinecraftVersion, cfg.MinecraftLoader)

	// Mods directory is now ensured by LoadConfig
	// modsDir := "mods" // Removed hardcoded dir
	// if err := os.MkdirAll(modsDir, 0755); err != nil { // Removed MkdirAll call
	// 	logger.Log.Fatalw("Failed to create mods directory",
	// 		zap.String("directory", modsDir),
	// 		zap.Error(err),
	// 	)
	// }

	var downloadedCount atomic.Int64 // Atomic counter for concurrent updates
	var updatedCount atomic.Int64   // Atomic counter for updated mods
	var wg sync.WaitGroup

	for _, project := range followedProjects {
		if project.ProjectType != "mod" && project.ProjectType != "shader" && project.ProjectType != "resourcepack" {
			logger.Log.Infow("Skipping non-mod/shader/resourcepack project",
				zap.String("title", project.Title),
				zap.String("type", project.ProjectType),
			)
			continue
		}

		// Use color if available
		projectName := project.Title
		if project.Color != 0 {
			projectName = ui.Colorize(project.Title, project.Color)
		}

		// Launch a goroutine for each project check and potential download
		wg.Add(1)
		go func(p modrinth.Project, coloredName string) {
			defer wg.Done()

			// Create a logger specific to this goroutine/project
			goroutineLogger := logger.Log.With(zap.String("project_slug", p.Slug), zap.String("project_title", p.Title)) // Use plain title for logger context

			goroutineLogger.Info(ui.Colorize("Checking project", p.Color))

			// --- Project Level Filtering ---
			if cfg.MinecraftInstallationType == "client" && p.ClientSide == "unsupported" {
				goroutineLogger.Infow(ui.Colorize("Skipping project: Unsupported on client", p.Color))
				return // Skip this project entirely
			}
			if cfg.MinecraftInstallationType == "server" && p.ServerSide == "unsupported" {
				goroutineLogger.Infow(ui.Colorize("Skipping project: Unsupported on server", p.Color))
				return // Skip this project entirely
			}
			// --- End Project Level Filtering ---

			// Pre-filter based on client/server side if applicable
			if !projectSupportsInstallationType(p, cfg.MinecraftInstallationType) {
				// Use Colorize here
				goroutineLogger.Infow(ui.Colorize("Skipping project, incompatible with installation type", p.Color),
					zap.String("installation_type", cfg.MinecraftInstallationType),
				)
				return
			}

			// Allow mods, shaders, and resource packs
			if p.ProjectType != "mod" && p.ProjectType != "shader" && p.ProjectType != "resourcepack" {
				goroutineLogger.Infow(ui.Colorize("Skipping unsupported project type", p.Color),
					zap.String("project_type", p.ProjectType),
				)
				return // Skip this project
			}

			// Fetch versions, passing the project type
			goroutineLogger.Infow(ui.Colorize("Fetching versions...", p.Color))
			versions, err := client.GetProjectVersions(p.Slug, p.ProjectType, cfg.MinecraftVersion, cfg.MinecraftLoader)
			if err != nil {
				goroutineLogger.Errorw("Failed to get project versions", zap.Error(err))
				return
			}

			if len(versions) == 0 {
				goroutineLogger.Info("  No compatible versions found.")
				return
			}

			// --- Revert Version Selection Logic ---
			// The API returns versions sorted by relevance/date, so the first one is usually the latest compatible.
			// Project-level filtering already handled the client/server side support.
			latestVersion := versions[0]
			goroutineLogger.Infow("Latest compatible version found", zap.String("version_id", latestVersion.ID), zap.String("version_number", latestVersion.VersionNumber))

			// Find the primary file for the latest version
			var primaryFile *modrinth.File
			for i := range latestVersion.Files {
				if latestVersion.Files[i].Primary {
					primaryFile = &latestVersion.Files[i]
					break
				}
			}

			if primaryFile == nil {
				goroutineLogger.Warnw("Latest version found, but it has no primary file!", zap.String("version_id", latestVersion.ID))
				// Attempt to use the first file if no primary file is marked?
				if len(latestVersion.Files) > 0 {
					goroutineLogger.Info("Attempting to use first file as fallback", zap.String("filename", latestVersion.Files[0].Filename))
					primaryFile = &latestVersion.Files[0]
				} else {
					goroutineLogger.Errorw("Latest version has no files at all!", zap.String("version_id", latestVersion.ID))
					return
				}
			}
			// --- End Revert Version Selection Logic ---

			// Determine target directory based on project type
			var targetSubDir string
			switch p.ProjectType {
			case "mod":
				targetSubDir = "mods"
			case "shader":
				targetSubDir = "shaderpacks"
			case "resourcepack":
				targetSubDir = "resourcepacks"
			default:
				goroutineLogger.Errorw("Unsupported project type for directory determination", zap.String("type", p.ProjectType))
				return // Or handle differently if other types might be valid in the future
			}
			projectBaseDir := filepath.Join(cfg.MinecraftDir, targetSubDir)

			// Create versions directory if keeping old versions, inside the specific target subdirectory
			if cfg.KeepOldVersions {
				versionsDir := filepath.Join(projectBaseDir, "versions")
				if err := os.MkdirAll(versionsDir, 0755); err != nil {
					// Log error but don't necessarily fail the entire update for this project?
					goroutineLogger.Warnw("Failed to ensure versions directory exists",
						zap.String("directory", versionsDir),
						zap.Error(err),
					)
				}
			}

			// Check if mod is already in database
			var existingMod db.Mod
			result := db.DB.Where("project_slug = ?", p.Slug).First(&existingMod)

			if result.Error == nil && !forceUpdate {
				// Mod exists in database, check if update needed
				if existingMod.VersionID != latestVersion.ID {
					goroutineLogger.Infow(ui.Colorize("Update available", existingMod.Color),
						zap.String("current_version", existingMod.VersionID),
						zap.String("new_version", latestVersion.ID),
					)

					// Save the current version to history before updating
					archivePath := ""
					old := existingMod // Make a copy of the existing record

					// Handle the old file - either delete or archive
					oldFileName := existingMod.FileName // Get the filename before potentially updating it
					// Construct full path to the old file using projectBaseDir
					oldFilePath := filepath.Join(projectBaseDir, oldFileName)
					versionsDir := filepath.Join(projectBaseDir, "versions")
					newPathInVersions := filepath.Join(versionsDir, fmt.Sprintf("%s-%s", existingMod.VersionID, oldFileName))

					// Attempt to move the old file
					if err := os.Rename(oldFilePath, newPathInVersions); err != nil {
						// Log unexpected error during move
						goroutineLogger.Warnw("Failed to move old file to versions directory",
							zap.String("from", oldFilePath),
							zap.String("to", newPathInVersions),
							zap.Error(err),
						)
					} else {
						goroutineLogger.Infow(ui.Colorize("Archived old mod version", existingMod.Color),
							zap.String("file", existingMod.FileName),
							zap.String("archive_path", newPathInVersions),
						)
						archivePath = newPathInVersions
					}

					// Save the previous version to history
					modVersion := db.ModVersion{
						ProjectSlug:   old.ProjectSlug,
						VersionID:     old.VersionID,
						VersionNumber: "", // We don't have this information readily available
						FileName:      old.FileName,
						ArchivePath:   archivePath,
					}

					if err := db.DB.Create(&modVersion).Error; err != nil {
						goroutineLogger.Warnw("Failed to save version history",
							zap.Error(err),
						)
					}

					// Construct full download path using projectBaseDir
					downloadPath := filepath.Join(projectBaseDir, primaryFile.Filename)
					goroutineLogger.Infow(ui.Colorize("Downloading update...", p.Color), zap.String("file", primaryFile.Filename))
					err = client.DownloadModFile(goroutineLogger, downloadPath, primaryFile.URL) // Pass downloadPath
					if err != nil {
						goroutineLogger.Errorw("Failed to download update",
							zap.String("filename", primaryFile.Filename),
							zap.Error(err),
						)
						return
					}

					goroutineLogger.Infow(ui.Colorize("Successfully downloaded file", existingMod.Color),
						zap.String("filename", primaryFile.Filename),
					)

					// Update database record
					// Parse the timestamp string
					updatedTimeUpdate, errUpdate := time.Parse(time.RFC3339Nano, p.Updated)
					if errUpdate != nil {
						goroutineLogger.Warnw("Failed to parse project updated timestamp (update)",
							zap.String("timestamp", p.Updated),
							zap.Error(errUpdate),
						)
						updatedTimeUpdate = time.Time{} // Use zero time on error
					}
					existingMod.VersionID = latestVersion.ID
					existingMod.FileName = primaryFile.Filename
					existingMod.InstallPath = downloadPath // Update InstallPath to the new path
					existingMod.ProjectID = p.ID
					existingMod.Title = p.Title
					existingMod.IconURL = p.IconURL
					existingMod.Color = p.Color
					existingMod.Updated = updatedTimeUpdate

					if err := db.DB.Save(&existingMod).Error; err != nil {
						goroutineLogger.Warnw("Failed to update database record",
							zap.Error(err),
						)
					}
					
					updatedCount.Add(1)
				} else if forceUpdate {
					goroutineLogger.Infow(ui.Colorize("Force updating mod", existingMod.Color),
						zap.String("current_version", existingMod.VersionID),
						zap.String("reinstalling_version", latestVersion.ID),
					)
					
					// Construct full download path using projectBaseDir
					downloadPath := filepath.Join(projectBaseDir, primaryFile.Filename)

					// Remove old file first
					// Construct full path to the old file using projectBaseDir
					oldFilePath := filepath.Join(projectBaseDir, existingMod.FileName)
					if err := os.Remove(oldFilePath); err != nil && !os.IsNotExist(err) {
						goroutineLogger.Warnw("Failed to remove old mod file before force download",
							zap.String("path", oldFilePath),
							zap.Error(err),
						)
					}
					// Download new file using the full path
					goroutineLogger.Infow(ui.Colorize("Downloading mod", existingMod.Color),
						zap.String("filename", primaryFile.Filename),
					)
					err = client.DownloadModFile(goroutineLogger, downloadPath, primaryFile.URL) // Pass downloadPath
					if err != nil {
						goroutineLogger.Errorw("Failed to download file",
							zap.String("filename", primaryFile.Filename),
							zap.Error(err),
						)
						return
					}

					goroutineLogger.Infow(ui.Colorize("Successfully downloaded file", existingMod.Color),
						zap.String("filename", primaryFile.Filename),
					)

					// Update database record if version ID changed or other project details changed
					// Parse the timestamp string
					updatedTimeForce, errForce := time.Parse(time.RFC3339Nano, p.Updated)
					if errForce != nil {
						goroutineLogger.Warnw("Failed to parse project updated timestamp (force)",
							zap.String("timestamp", p.Updated),
							zap.Error(errForce),
						)
						updatedTimeForce = time.Time{} // Use zero time on error
					}
					if existingMod.VersionID != latestVersion.ID ||
						existingMod.Title != p.Title ||
						existingMod.IconURL != p.IconURL ||
						existingMod.Color != p.Color ||
						!existingMod.Updated.Equal(updatedTimeForce) {
						existingMod.VersionID = latestVersion.ID
						existingMod.FileName = primaryFile.Filename
						existingMod.InstallPath = downloadPath // Update InstallPath to the new path
						existingMod.ProjectID = p.ID
						existingMod.Title = p.Title
						existingMod.IconURL = p.IconURL
						existingMod.Color = p.Color
						existingMod.Updated = updatedTimeForce

						if err := db.DB.Save(&existingMod).Error; err != nil {
							goroutineLogger.Warnw("Failed to update database record (force)",
								zap.Error(err),
							)
						}
					}
					
					updatedCount.Add(1)
				} else {
					goroutineLogger.Infow(ui.Colorize("Mod is already up to date", existingMod.Color),
						zap.String("version", existingMod.VersionID),
					)
				}
			} else {
				// Mod not in database or force update requested, download and add
				goroutineLogger.Infow(ui.Colorize("New mod - downloading", p.Color),
					zap.String("version", latestVersion.VersionNumber))

				// Construct full download path using projectBaseDir
				downloadPath := filepath.Join(projectBaseDir, primaryFile.Filename)

				// New mod: Download
				goroutineLogger.Infow(ui.Colorize("Downloading...", p.Color), zap.String("file", primaryFile.Filename))
				err = client.DownloadModFile(goroutineLogger, downloadPath, primaryFile.URL) // Pass downloadPath
				if err != nil {
					goroutineLogger.Errorw("Failed to download file",
						zap.String("filename", primaryFile.Filename),
						zap.Error(err),
					)
					return
				}

				goroutineLogger.Infow(ui.Colorize("Successfully downloaded file", p.Color),
					zap.String("filename", primaryFile.Filename),
				)

				// Add to database
				// Parse the timestamp string
				updatedTimeNew, errNew := time.Parse(time.RFC3339Nano, p.Updated)
				if errNew != nil {
					goroutineLogger.Warnw("Failed to parse project updated timestamp (new)",
						zap.String("timestamp", p.Updated),
						zap.Error(errNew),
					)
					updatedTimeNew = time.Time{} // Use zero time on error
				}
				newMod := db.Mod{
					ProjectSlug: p.Slug,
					ProjectID:   p.ID,
					Title:       p.Title,
					IconURL:     p.IconURL,
					Color:       p.Color,
					Updated:     updatedTimeNew,
					VersionID:   latestVersion.ID,
					FileName:    primaryFile.Filename,
					InstallPath: downloadPath, // Set InstallPath correctly
				}
				
				if err := db.DB.Create(&newMod).Error; err != nil {
					goroutineLogger.Warnw("Failed to save mod to database",
						zap.Error(err),
					)
				}
				
				downloadedCount.Add(1) // Increment atomic counter
			}
		}(project, projectName) // Pass colored name
	}

	// Wait for all download goroutines to complete
	wg.Wait()

	logger.Log.Infof("Finished. Downloaded %d new mods, updated %d existing mods.", downloadedCount.Load(), updatedCount.Load())
}
