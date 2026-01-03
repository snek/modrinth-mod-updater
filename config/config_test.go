package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestProcessConfigDefaults(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		viper.Reset()
		cfg := Config{}
		processConfigDefaults(&cfg)

		if cfg.MinecraftLoader != "fabric" {
			t.Errorf("Expected MinecraftLoader to be fabric, got %s", cfg.MinecraftLoader)
		}
		if cfg.MinecraftInstallationType != "server" {
			t.Errorf("Expected MinecraftInstallationType to be server, got %s", cfg.MinecraftInstallationType)
		}
		if cfg.UserAgent == "" {
			t.Error("Expected UserAgent to have a default value")
		}
	})

	t.Run("respects existing values", func(t *testing.T) {
		viper.Reset()
		cfg := Config{
			MinecraftLoader:           "forge",
			MinecraftInstallationType: "client",
			UserAgent:                 "custom-agent",
		}
		processConfigDefaults(&cfg)

		if cfg.MinecraftLoader != "forge" {
			t.Errorf("Expected MinecraftLoader to stay forge, got %s", cfg.MinecraftLoader)
		}
		if cfg.MinecraftInstallationType != "client" {
			t.Errorf("Expected MinecraftInstallationType to stay client, got %s", cfg.MinecraftInstallationType)
		}
		if cfg.UserAgent != "custom-agent" {
			t.Errorf("Expected UserAgent to stay custom-agent, got %s", cfg.UserAgent)
		}
	})
}

func TestValidateAndEnsureDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("missing minecraft dir", func(t *testing.T) {
		cfg := Config{MinecraftDir: ""}
		err := validateAndEnsureDirectories(&cfg)
		if err == nil {
			t.Error("Expected error for missing MinecraftDir")
		}
	})

	t.Run("creates directories", func(t *testing.T) {
		mcDir := filepath.Join(tmpDir, "mc")
		cfg := Config{MinecraftDir: mcDir}
		err := validateAndEnsureDirectories(&cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		subDirs := []string{"mods", "shaderpacks", "resourcepacks"}
		for _, sub := range subDirs {
			path := filepath.Join(mcDir, sub)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Directory %s was not created", sub)
			}
		}
	})
}
