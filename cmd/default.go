package cmd

import (
	"github.com/spf13/cobra"
)

// defaultCmd represents the command that runs when no subcommand is specified
var defaultCmd = &cobra.Command{
	Use:   "default",
	Short: "Default command when no subcommand is provided",
	Long:  `Runs the update command by default for backwards compatibility.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Simply run the update command with default parameters
		updateCmd.Run(updateCmd, []string{})
	},
}

func init() {
	// Set as default command to run when no subcommand is provided
	rootCmd.AddCommand(defaultCmd)
	// Set rootCmd to call defaultCmd if no subcommand is provided
	cobra.OnInitialize(func() {
		// If there are no arguments (only program name), set defaultCmd as the command to run
		if len(rootCmd.Commands()) > 0 && len(rootCmd.Flags().Args()) == 0 {
			rootCmd.SetArgs([]string{"default"})
		}
	})
}
