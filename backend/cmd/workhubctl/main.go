package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0"
	serverURL string
	apiKey    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "workhubctl",
		Short: "WorkHub CLI - Manage your WorkHub server from the command line",
		Long: `WorkHub CLI - A powerful command-line interface for managing your WorkHub server.

Quick start:
  workhubctl server start
  workhubctl user create --email admin@test.com --role admin
  workhubctl health`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "Server URL (default: http://localhost:8080)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	rootCmd.AddCommand(serverCmd())
	rootCmd.AddCommand(userCmd())
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
