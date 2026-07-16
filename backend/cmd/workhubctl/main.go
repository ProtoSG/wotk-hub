package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.2"
	serverURL string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "workhubctl",
		Short: "WorkHub CLI - Manage your WorkHub server from the command line",
		Long: `WorkHub CLI - A powerful command-line interface for managing your WorkHub server.

Quick start:
  workhubctl auth login
  workhubctl server start
  workhubctl health

Auth: Set the CLI_TOKEN environment variable, or place your token in
  ~/.config/workhub/credentials (0600 permissions).
  Environment variable takes precedence.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "Server URL (default: http://localhost:8080)")

	rootCmd.AddCommand(serverCmd())
	rootCmd.AddCommand(userCmd())
	rootCmd.AddCommand(authCmd())
	rootCmd.AddCommand(dbCmd())
	rootCmd.AddCommand(financesCmd())
	rootCmd.AddCommand(ytdlpCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(healthCmd())

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("workhubctl version %s\n", version)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		color.Red("Error: %v", err)
		os.Exit(1)
	}
}
