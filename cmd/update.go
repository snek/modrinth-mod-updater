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

func runUpdate(forceUpdate bool) {
	
	// Load configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		logger.Log.Fatalw("Failed to load configuration", zap.Error(err))
	}
	
	// Initialize database
	db.InitDatabase(cfg.DatabasePath)
	logger.Log.Infow("Database initialized", zap.String("path", cfg.DatabasePath))

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

	modsDir := "mods"
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		logger.Log.Fatalw("Failed to create mods directory",
			zap.String("directory", modsDir),
			zap.Error(err),
		)
	}
	
	// Create versions directory if keeping old versions
	if cfg.KeepOldVersions {
		versionsDir := filepath.Join(modsDir, "versions")
		if err := os.MkdirAll(versionsDir, 0755); err != nil {
			logger.Log.Fatalw("Failed to create versions directory",
				zap.String("directory", versionsDir),
				zap.Error(err),
			)
		}
	}

	var downloadedCount atomic.Int64 // Atomic counter for concurrent updates
	var updatedCount atomic.Int64   // Atomic counter for updated mods
	var wg sync.WaitGroup

	for _, project := range followedProjects {
		if project.ProjectType != "mod" {
			logger.Log.Infow("Skipping non-mod project",
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
			goroutineLogger := logger.Log.With(zap.String("project", p.Title)) // Use plain title for logger context

			goroutineLogger.Infow("Checking project")

			versions, err := client.GetProjectVersions(p.Slug, cfg.MinecraftVersion, cfg.MinecraftLoader)
			if err != nil {
				goroutineLogger.Warnw("Failed to get versions for project", zap.Error(err))
				return
			}

			if len(versions) == 0 {
				goroutineLogger.Info("  No compatible versions found.")
				return
			}

			latestVersion := versions[0]
			goroutineLogger.Infof("  Latest version: %s (%s)", ui.Colorize(latestVersion.Name, p.Color), latestVersion.VersionNumber)

			var primaryFile *modrinth.File
			for i := range latestVersion.Files {
				if latestVersion.Files[i].Primary {
					primaryFile = &latestVersion.Files[i]
					break
				}
			}

			if primaryFile == nil {
				if len(latestVersion.Files) > 0 {
					primaryFile = &latestVersion.Files[0]
					goroutineLogger.Warnw("No primary file found, falling back to first file",
						zap.String("version", latestVersion.Name),
						zap.String("filename", primaryFile.Filename),
					)
				} else {
					goroutineLogger.Errorw("No files found for version", zap.String("version", latestVersion.Name))
					return
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
					oldFilePath := filepath.Join(modsDir, existingMod.FileName)
					if _, err := os.Stat(oldFilePath); err == nil {
						if cfg.KeepOldVersions {
							// Move old file to versions directory
							versionsDir := filepath.Join(modsDir, "versions")
							newPath := filepath.Join(versionsDir, existingMod.FileName)
							
							// If file with same name exists in versions directory, add a suffix
							if _, err := os.Stat(newPath); err == nil {
								ext := filepath.Ext(existingMod.FileName)
								baseName := existingMod.FileName[:len(existingMod.FileName)-len(ext)]
								newPath = filepath.Join(versionsDir, fmt.Sprintf("%s_%s%s", baseName, existingMod.VersionID, ext))
							}
							
							if err := os.Rename(oldFilePath, newPath); err != nil {
								goroutineLogger.Warnw("Failed to move old mod version",
									zap.String("from", oldFilePath),
									zap.String("to", newPath),
									zap.Error(err),
								)
							} else {
								goroutineLogger.Infow(ui.Colorize("Archived old mod version", existingMod.Color),
									zap.String("file", existingMod.FileName),
									zap.String("archive_path", newPath),
								)
								archivePath = newPath
							}
						} else {
							// Delete old file
							if err := os.Remove(oldFilePath); err != nil {
								goroutineLogger.Warnw("Failed to delete old mod file",
									zap.String("file", oldFilePath),
									zap.Error(err),
								)
							} else {
								goroutineLogger.Infow(ui.Colorize("Deleted old mod file", existingMod.Color),
									zap.String("file", existingMod.FileName),
								)
							}
						}
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

					// Download new file
					goroutineLogger.Infow(ui.Colorize("Downloading update", existingMod.Color),
						zap.String("filename", primaryFile.Filename),
					)
					err = client.DownloadModFile(goroutineLogger, primaryFile.Filename, primaryFile.URL)
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
					updatedTime, err := time.Parse(time.RFC3339Nano, p.Updated)
					if err != nil {
						goroutineLogger.Warnw("Failed to parse project updated timestamp",
							zap.String("timestamp", p.Updated),
							zap.Error(err),
						)
						updatedTime = time.Time{} // Use zero time on error
					}
					existingMod.VersionID = latestVersion.ID
					existingMod.FileName = primaryFile.Filename
					existingMod.InstallPath = filepath.Join(modsDir, primaryFile.Filename)
					existingMod.ProjectID = p.ID
					existingMod.Title = p.Title
					existingMod.IconURL = p.IconURL
					existingMod.Color = p.Color
					existingMod.Updated = updatedTime

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
					
					// Download new file
					goroutineLogger.Infow(ui.Colorize("Downloading mod", existingMod.Color),
						zap.String("filename", primaryFile.Filename),
					)
					err = client.DownloadModFile(goroutineLogger, primaryFile.Filename, primaryFile.URL)
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
						existingMod.InstallPath = filepath.Join(modsDir, primaryFile.Filename)
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
				
				goroutineLogger.Infow(ui.Colorize("Downloading primary file", p.Color),
					zap.String("filename", primaryFile.Filename))
				err = client.DownloadModFile(goroutineLogger, primaryFile.Filename, primaryFile.URL)
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
					InstallPath: filepath.Join(modsDir, primaryFile.Filename),
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
