package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage CLI authentication",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "login [token]",
		Short: "Save CLI token to credentials file",
		Long: `Save the CLI token to ~/.config/workhub/credentials.
Takes the token as argument or reads from CLI_TOKEN env var.
The environment variable takes precedence.`,
		Args: cobra.MaximumNArgs(1),
		Run:  runAuthLogin,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		Run: func(cmd *cobra.Command, args []string) {
			path := credentialPath()
			if _, err := os.Stat(path); os.IsNotExist(err) {
				color.Yellow("No credentials file found")
				return
			}
			if err := os.Remove(path); err != nil {
				color.Red("Failed to remove credentials: %v", err)
				return
			}
			color.Green("Logged out. Credentials removed.")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check if stored token is valid",
		Run:   runAuthStatus,
	})
	return cmd
}

func credentialPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "workhub", "credentials")
}

func saveTokenToFile(token string) error {
	path := credentialPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func runAuthLogin(cmd *cobra.Command, args []string) {
	token := strings.TrimSpace(loadToken())

	// If no env var, require token as argument
	if token == "" {
		if len(args) == 0 {
			color.Red("Error: token required. Pass it as argument or set CLI_TOKEN env var.")
			color.Cyan("Usage: workhubctl auth login <token>")
			color.Cyan("       CLI_TOKEN=<token> workhubctl auth login")
			return
		}
		token = strings.TrimSpace(args[0])
	}

	if token == "" {
		color.Red("Error: empty token")
		return
	}

	if err := saveTokenToFile(token); err != nil {
		color.Red("Failed to save token: %v", err)
		return
	}
	color.Green("Token saved to %s", credentialPath())
}

func runAuthStatus(cmd *cobra.Command, args []string) {
	token := strings.TrimSpace(loadToken())
	if token == "" {
		color.Yellow("No credentials found.")
		color.Cyan("Run 'workhubctl auth login <token>' to configure.")
		return
	}

	req, err := http.NewRequest("GET", serverURL+"/api/cli/me", nil)
	if err != nil {
		color.Red("Request failed: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		color.Red("Cannot connect to server at %s", serverURL)
		color.Yellow("Is the server running?", serverURL)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		color.Green("✓ Token is valid")
	} else if resp.StatusCode == http.StatusUnauthorized {
		color.Red("✗ Token is invalid or expired")
		color.Cyan("Run 'workhubctl auth login <token>' to update.")
	} else {
		color.Yellow("⚠ Unexpected status: %d", resp.StatusCode)
	}
}
