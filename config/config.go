package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
// Values are loaded by Viper from a config file and/or environment variables.
type Config struct {
	MinecraftInstallation string `mapstructure:"MINECRAFT_INSTALLATION"`
	MinecraftLoader       string `mapstructure:"MINECRAFT_LOADER"`
	MinecraftVersion      string `mapstructure:"MINECRAFT_VERSION"`
	ModrinthAPIKey        string `mapstructure:"MODRINTH_API_KEY"`
	UserAgent             string `mapstructure:"USERAGENT"`
	ModrinthUser          string `mapstructure:"MODRINTH_USER"` // Note: This might not be needed if API key implies user
	DatabasePath          string `mapstructure:"DATABASE_PATH"`
	KeepOldVersions       bool   `mapstructure:"KEEP_OLD_VERSIONS"`
}

// LoadConfig reads configuration from file and environment variables.
func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)  // Path to look for the config file in
	viper.SetConfigName(".env") // Name of config file (without extension)
	viper.SetConfigType("env") // REQUIRED if the config file does not have the extension in the name

	vip_err := viper.ReadInConfig()
	if _, ok := vip_err.(viper.ConfigFileNotFoundError); ok {
		fmt.Println("Config file (.env) not found, relying on environment variables.")
	} else if vip_err != nil {
		return Config{}, fmt.Errorf("fatal error config file: %w", vip_err)
	}

	// Bind environment variables automatically.
	// Viper will check for an environment variable matching the key name (e.g., MODRINTH_API_KEY)
	viper.AutomaticEnv()

	// Set default values
	vip_err = viper.BindEnv("keep_old_versions", "KEEP_OLD_VERSIONS")
	if vip_err != nil {
		fmt.Println("Unable to bind KEEP_OLD_VERSIONS env var")
	}
	vip_err = viper.BindEnv("database_path", "DATABASE_PATH")
	if vip_err != nil {
		fmt.Println("Unable to bind DATABASE_PATH env var")
	}

	// Set default user agent if not provided
	vip_err = viper.BindEnv("useragent", "USERAGENT")
	if vip_err != nil {
		fmt.Println("Unable to bind USERAGENT env var")
	}
	vip_err = viper.BindEnv("modrinth_api_key", "MODRINTH_API_KEY")
	if vip_err != nil {
		fmt.Println("Unable to bind MODRINTH_API_KEY env var")
	}
	vip_err = viper.BindEnv("minecraft_installation", "MINECRAFT_INSTALLATION")
	if vip_err != nil {
		fmt.Println("Unable to bind MINECRAFT_INSTALLATION env var")
	}
	vip_err = viper.BindEnv("minecraft_loader", "MINECRAFT_LOADER")
	if vip_err != nil {
		fmt.Println("Unable to bind MINECRAFT_LOADER env var")
	}
	vip_err = viper.BindEnv("minecraft_version", "MINECRAFT_VERSION")
	if vip_err != nil {
		fmt.Println("Unable to bind MINECRAFT_VERSION env var")
	}
	vip_err = viper.BindEnv("modrinth_user", "MODRINTH_USER")
	if vip_err != nil {
		fmt.Println("Unable to bind MODRINTH_USER env var")
	}

	// Unmarshal the config
	vip_err = viper.Unmarshal(&config)
	if vip_err != nil {
		return Config{}, fmt.Errorf("unable to decode into struct: %w", vip_err)
	}

	// --- Post-unmarshal processing and defaults ---

	// Default DatabasePath if not set
	if config.DatabasePath == "" {
		wd, err := os.Getwd() // Get current working directory
		if err != nil {
			return Config{}, fmt.Errorf("failed to get current working directory: %w", err)
		}
		config.DatabasePath = filepath.Join(wd, "mods.db") // Default to mods.db in the current directory
		fmt.Printf("DATABASE_PATH not set, defaulting to %s\n", config.DatabasePath)
	}

	// Default KeepOldVersions if not explicitly set (Viper doesn't handle bool defaults from env well without explicit SetDefault)
	// We check the string value from Viper directly before unmarshal might coerce it.
	keepOldStr := viper.GetString("KEEP_OLD_VERSIONS")
	if keepOldStr == "" {
		config.KeepOldVersions = false // Default to false
		fmt.Println("KEEP_OLD_VERSIONS not set, defaulting to false")
	} else {
		// Attempt to parse the boolean string value from env/config
		keepOld, err := strconv.ParseBool(keepOldStr)
		if err != nil {
			fmt.Printf("Warning: Invalid value for KEEP_OLD_VERSIONS ('%s'), defaulting to false. Error: %v\n", keepOldStr, err)
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
		fmt.Println("Warning: USERAGENT not set in config or environment, using default.")
	}

	return config, nil
}
