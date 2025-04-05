package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
// Values are loaded by Viper from a config file and/or environment variables.
type Config struct {
	MinecraftInstallationType string `mapstructure:"MINECRAFT_INSTALLATION_TYPE"`
	MinecraftLoader           string `mapstructure:"MINECRAFT_LOADER"`
	MinecraftVersion          string `mapstructure:"MINECRAFT_VERSION"`
	ModrinthAPIKey            string `mapstructure:"MODRINTH_API_KEY"`
	UserAgent                 string `mapstructure:"USERAGENT"`
	ModrinthUser              string `mapstructure:"MODRINTH_USER"` // Note: This might not be needed if API key implies user
	MinecraftDir              string `mapstructure:"MINECRAFT_DIR"` // Renamed from ModsDir
	DatabasePath              string `mapstructure:"-"`              // Not from env, derived
	KeepOldVersions           bool   `mapstructure:"KEEP_OLD_VERSIONS"`
}

// LoadConfig reads configuration from file and environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)   // Path to look for the config file in
	viper.SetConfigName(".env") // Name of config file (without extension)
	viper.SetConfigType("env")  // REQUIRED if the config file does not have the extension in the name

	vip_err := viper.ReadInConfig()
	if _, ok := vip_err.(viper.ConfigFileNotFoundError); ok {
		slog.Info("Config file (.env) not found, relying on environment variables.")
	} else if vip_err != nil {
		return Config{}, fmt.Errorf("fatal error config file: %w", vip_err)
	}

	// Bind environment variables automatically.
	// Viper will check for an environment variable matching the key name (e.g., MODRINTH_API_KEY)
	viper.AutomaticEnv()

	// Set default values
	vip_err = viper.BindEnv("keep_old_versions", "KEEP_OLD_VERSIONS")
	if vip_err != nil {
		slog.Warn("Unable to bind KEEP_OLD_VERSIONS env var", "error", vip_err)
	}
	vip_err = viper.BindEnv("minecraft_dir", "MINECRAFT_DIR")
	if vip_err != nil {
		slog.Warn("Unable to bind MINECRAFT_DIR env var", "error", vip_err)
	}

	// Set default user agent if not provided
	vip_err = viper.BindEnv("useragent", "USERAGENT")
	if vip_err != nil {
		slog.Warn("Unable to bind USERAGENT env var", "error", vip_err)
	}
	vip_err = viper.BindEnv("modrinth_api_key", "MODRINTH_API_KEY")
	if vip_err != nil {
		slog.Warn("Unable to bind MODRINTH_API_KEY env var", "error", vip_err)
	}
	vip_err = viper.BindEnv("minecraft_installation_type", "MINECRAFT_INSTALLATION_TYPE")
	if vip_err != nil {
		slog.Warn("Unable to bind MINECRAFT_INSTALLATION_TYPE env var", "error", vip_err)
	}
	vip_err = viper.BindEnv("minecraft_loader", "MINECRAFT_LOADER")
	if vip_err != nil {
		slog.Warn("Unable to bind MINECRAFT_LOADER env var", "error", vip_err)
	}
	vip_err = viper.BindEnv("minecraft_version", "MINECRAFT_VERSION")
	if vip_err != nil {
		slog.Warn("Unable to bind MINECRAFT_VERSION env var", "error", vip_err)
	}
	vip_err = viper.BindEnv("modrinth_user", "MODRINTH_USER")
	if vip_err != nil {
		slog.Warn("Unable to bind MODRINTH_USER env var", "error", vip_err)
	}

	// Unmarshal the config
	vip_err = viper.Unmarshal(&config)
	if vip_err != nil {
		return Config{}, fmt.Errorf("unable to decode into struct, %w", vip_err)
	}

	// --- Post-unmarshal processing and defaults ---

	// Set defaults if not provided
	if config.MinecraftLoader == "" {
		config.MinecraftLoader = "fabric" // Default loader
	}
	if config.MinecraftInstallationType == "" {
		config.MinecraftInstallationType = "server" // Default installation type
	}

	// Default KeepOldVersions if not explicitly set (Viper doesn't handle bool defaults from env well without explicit SetDefault)
	// We check the string value from Viper directly before unmarshal might coerce it.
	keepOldStr := viper.GetString("KEEP_OLD_VERSIONS")
	if keepOldStr == "" {
		config.KeepOldVersions = false // Default to false
		slog.Info("KEEP_OLD_VERSIONS not set, defaulting to false")
	} else {
		// Attempt to parse the boolean string value from env/config
		keepOld, err := strconv.ParseBool(keepOldStr)
		if err != nil {
			slog.Warn("Warning: Invalid value for KEEP_OLD_VERSIONS ('"+keepOldStr+"'), defaulting to false. Error:", "error", err)
			config.KeepOldVersions = false
		} else {
			config.KeepOldVersions = keepOld
		}
	}

	// Basic validation
	if config.UserAgent == "" {
		// Set a default or return an error if UserAgent is absolutely required
		// For now, let's set a generic default.
		config.UserAgent = "modrinth-mod-updater/dev (unknown-user)"
		slog.Warn("USERAGENT not set in config or environment, using default.")
	}

	// Validate MinecraftDir - needs to be set
	if config.MinecraftDir == "" {
		slog.Error("MINECRAFT_DIR is not set")
		return Config{}, fmt.Errorf("MINECRAFT_DIR is required")
	}
	// Ensure MinecraftDir exists, create if not
	if _, err := os.Stat(config.MinecraftDir); os.IsNotExist(err) {
		slog.Info("Minecraft directory does not exist, creating it", "path", config.MinecraftDir)
		if err := os.MkdirAll(config.MinecraftDir, 0755); err != nil {
			slog.Error("Failed to create Minecraft directory", "path", config.MinecraftDir, "error", err)
			return Config{}, err
		}
	} else if err != nil {
		slog.Error("Failed to check Minecraft directory", "path", config.MinecraftDir, "error", err)
		return Config{}, err
	}

	// Ensure mods subdirectory exists
	modsDir := filepath.Join(config.MinecraftDir, "mods")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		slog.Info("Mods directory does not exist, creating it", "path", modsDir)
		if err := os.MkdirAll(modsDir, 0755); err != nil {
			slog.Error("Failed to create mods directory", "path", modsDir, "error", err)
			return Config{}, err
		}
	} else if err != nil {
		slog.Error("Failed to check mods directory", "path", modsDir, "error", err)
		return Config{}, err
	}

	// Ensure shaderpacks subdirectory exists
	shaderpacksDir := filepath.Join(config.MinecraftDir, "shaderpacks")
	if _, err := os.Stat(shaderpacksDir); os.IsNotExist(err) {
		slog.Info("Shaderpacks directory does not exist, creating it", "path", shaderpacksDir)
		if err := os.MkdirAll(shaderpacksDir, 0755); err != nil {
			slog.Error("Failed to create shaderpacks directory", "path", shaderpacksDir, "error", err)
			return Config{}, err
		}
	} else if err != nil {
		slog.Error("Failed to check shaderpacks directory", "path", shaderpacksDir, "error", err)
		return Config{}, err
	}

	// Derive DatabasePath (place it in the Minecraft dir for portability)
	config.DatabasePath = filepath.Join(config.MinecraftDir, "mods.db")

	return config, nil
}
