package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCLI(appVersion string) *cobra.Command {
	if appVersion == "" {
		appVersion = "(unknown)"
	}

	rootCmd := &cobra.Command{
		Use:           "hookrelay",
		Short:         "Webhook Relay",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			if version, _ := cmd.Flags().GetBool("version"); version {
				fmt.Println(appVersion)
				return
			}

			cmd.Print(cmd.UsageString())
		},
	}

	rootCmd.Flags().BoolP("version", "v", false, "Show version information")
	rootCmd.PersistentFlags().StringP("config", "c", "", "Configuration file for hookrelay")

	rootCmd.AddCommand(ServerCmd(), WorkerCmd())
	return rootCmd
}
