package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check server health status",
		Run: func(cmd *cobra.Command, args []string) {
			client := &http.Client{Timeout: 5 * time.Second}

			resp, err := client.Get(serverURL + "/health")
			if err != nil {
				color.Red("✗ Cannot connect to server at %s", serverURL)
				color.Yellow("  Is the server running?")
				return
			}
			defer resp.Body.Close()

			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)

			if resp.StatusCode == http.StatusOK && result["status"] == "ok" {
				color.Green("✓ Server is healthy at %s", serverURL)
				fmt.Printf("  Status: %s\n", result["status"])
			} else {
				color.Yellow("⚠ Server returned unexpected response")
				fmt.Printf("  Status: %s\n", resp.Status)
			}
		},
	}
}
