package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCLI(buildVersion string) *cobra.Command {
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
				fmt.Println(buildVersion)
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
