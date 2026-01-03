package cmd

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modrinth-mod-updater/db"
	"modrinth-mod-updater/logger"
	"modrinth-mod-updater/modrinth"

	"go.uber.org/zap"
)

// importInstalledMods scans the mods directory and adds unknown mods to the database
func importInstalledMods(client *modrinth.Client, minecraftDir string) error {
	logger.Log.Info("Scanning for existing mods...")

	dirs := []string{
		filepath.Join(minecraftDir, "mods"),
		filepath.Join(minecraftDir, "shaderpacks"),
		filepath.Join(minecraftDir, "resourcepacks"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return processModFile(client, path, info)
		})

		if err != nil {
			logger.Log.Errorw("Error scanning directory", zap.String("dir", dir), zap.Error(err))
		}
	}

	return nil
}

func processModFile(client *modrinth.Client, path string, info os.FileInfo) error {
	if info.IsDir() {
		if info.Name() == "versions" {
			return filepath.SkipDir
		}
		return nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".jar" && ext != ".zip" {
		return nil
	}

	filename := info.Name()
	var count int64
	db.DB.Model(&db.Mod{}).Where("file_name = ?", filename).Count(&count)
	if count > 0 {
		return nil
	}

	hash, err := calculateSHA1(path)
	if err != nil {
		logger.Log.Warnw("Failed to calculate hash", zap.String("file", filename), zap.Error(err))
		return nil
	}

	version, err := client.GetVersionByHash(hash)
	if err != nil {
		logger.Log.Debugw("Mod not found on Modrinth by hash", zap.String("file", filename), zap.Error(err))
		return nil
	}

	project, err := client.GetProject(version.ProjectID)
	if err != nil {
		logger.Log.Warnw("Failed to get project details", zap.String("project_id", version.ProjectID), zap.Error(err))
		return nil
	}

	newMod := db.Mod{
		ProjectSlug:   project.Slug,
		ProjectID:     project.ID,
		Title:         project.Title,
		IconURL:       project.IconURL,
		Color:         project.Color,
		Updated:       time.Now(),
		VersionID:     version.ID,
		VersionNumber: version.VersionNumber,
		FileName:      filename,
		InstallPath:   path,
	}

	if err := db.DB.Create(&newMod).Error; err != nil {
		logger.Log.Errorw("Failed to save imported mod to DB", zap.String("slug", project.Slug), zap.Error(err))
	} else {
		logger.Log.Infow("Imported existing mod", zap.String("title", project.Title), zap.String("version", version.VersionNumber))
	}

	return nil
}

func calculateSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
