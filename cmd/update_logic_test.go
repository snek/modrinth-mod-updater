package cmd

import (
	"testing"

	"modrinth-mod-updater/config"
	"modrinth-mod-updater/modrinth"

	"go.uber.org/zap"
)

func TestShouldProcessProject(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name     string
		project  modrinth.Project
		cfg      config.Config
		expected bool
	}{
		{
			"valid client mod",
			modrinth.Project{ClientSide: "required", ServerSide: "optional"},
			config.Config{MinecraftInstallationType: "client"},
			true,
		},
		{
			"unsupported client mod",
			modrinth.Project{ClientSide: "unsupported", ServerSide: "required"},
			config.Config{MinecraftInstallationType: "client"},
			false,
		},
		{
			"valid server mod",
			modrinth.Project{ClientSide: "optional", ServerSide: "required"},
			config.Config{MinecraftInstallationType: "server"},
			true,
		},
		{
			"unsupported server mod",
			modrinth.Project{ClientSide: "required", ServerSide: "unsupported"},
			config.Config{MinecraftInstallationType: "server"},
			false,
		},
		{
			"incompatible installation type (general check)",
			modrinth.Project{ClientSide: "unsupported", ServerSide: "unsupported"},
			config.Config{MinecraftInstallationType: "client"},
			false,
		},
		{
			"both type allows anything",
			modrinth.Project{ClientSide: "unsupported", ServerSide: "unsupported"},
			config.Config{MinecraftInstallationType: "both"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldProcessProject(tt.project, &tt.cfg, logger)
			if result != tt.expected {
				t.Errorf("shouldProcessProject(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}
