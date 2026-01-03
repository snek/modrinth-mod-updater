package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
// Values are loaded by Viper from a config file and/or environment variables.
type Config struct {
	MinecraftInstallationType string `mapstructure:"minecraft_installation_type"`
	MinecraftLoader           string `mapstructure:"minecraft_loader"`
	MinecraftVersion          string `mapstructure:"minecraft_version"`
	ModrinthAPIKey            string `mapstructure:"modrinth_api_key"`
	UserAgent                 string `mapstructure:"useragent"`
	ModrinthUser              string `mapstructure:"modrinth_user"`
	MinecraftDir              string `mapstructure:"minecraft_dir"`
	DatabasePath              string `mapstructure:"-"`
	KeepOldVersions           bool   `mapstructure:"keep_old_versions"`
}

// LoadConfig reads configuration from file and environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Info("Config file (.env) not found, relying on environment variables.")
		} else {
			return Config{}, fmt.Errorf("fatal error config file: %w", err)
		}
	}

	viper.AutomaticEnv()
	bindEnvVars()

	if err := viper.Unmarshal(&config); err != nil {
		return Config{}, fmt.Errorf("unable to decode into struct, %w", err)
	}

	processConfigDefaults(&config)

	if err := validateAndEnsureDirectories(&config); err != nil {
		return Config{}, err
	}

	config.DatabasePath = filepath.Join(config.MinecraftDir, "mods.db")
	return config, nil
}

func bindEnvVars() {
	// Bind each struct key to its corresponding environment variable name.
	// This ensures that even if .env is missing, the tool works with shell exports.
	vars := map[string]string{
		"keep_old_versions":           "KEEP_OLD_VERSIONS",
		"minecraft_dir":               "MINECRAFT_DIR",
		"useragent":                   "USERAGENT",
		"modrinth_api_key":            "MODRINTH_API_KEY",
		"minecraft_installation_type": "MINECRAFT_INSTALLATION_TYPE",
		"minecraft_loader":            "MINECRAFT_LOADER",
		"minecraft_version":           "MINECRAFT_VERSION",
		"modrinth_user":               "MODRINTH_USER",
	}
	for key, env := range vars {
		_ = viper.BindEnv(key, env)
	}
}

func processConfigDefaults(config *Config) {
	if config.MinecraftLoader == "" {
		config.MinecraftLoader = "fabric"
	}
	if config.MinecraftInstallationType == "" {
		config.MinecraftInstallationType = "server"
	}

	if config.UserAgent == "" {
		config.UserAgent = "modrinth-mod-updater/dev (unknown-user)"
		slog.Warn("USERAGENT not set, using default.")
	}
}

func validateAndEnsureDirectories(config *Config) error {
	if config.MinecraftDir == "" {
		return fmt.Errorf("MINECRAFT_DIR is required")
	}

	dirs := []string{
		config.MinecraftDir,
		filepath.Join(config.MinecraftDir, "mods"),
		filepath.Join(config.MinecraftDir, "shaderpacks"),
		filepath.Join(config.MinecraftDir, "resourcepacks"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			slog.Info("Directory does not exist, creating it", "path", dir)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check directory %s: %w", dir, err)
		}
	}

	return nil
}
