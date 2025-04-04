package main

import (
	"modrinth-mod-updater/cmd"
	"modrinth-mod-updater/logger"

	_ "go.uber.org/automaxprocs/maxprocs"
)

func main() {
	logger.InitLogger() // Initialize the logger first
	defer logger.Sync()   // Ensure logs are flushed on exit
	cmd.Execute()
}
