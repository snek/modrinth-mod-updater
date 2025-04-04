package cmd

import (
	"os"

	"modrinth-mod-updater/logger"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "modrinth-mod-updater",
	Short: "A tool to download followed Modrinth mods",
	Long: `modrinth-mod-updater checks your followed mods on Modrinth
and downloads the latest versions compatible with your configured
Minecraft version and loader.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Removed redundant logger sync deferral
	err := rootCmd.Execute()
	if err != nil {
		logger.Log.Error(err) // Log Cobra errors using Zap
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.modrinth-mod-updater.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
