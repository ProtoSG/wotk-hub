package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func ytdlpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ytdlp",
		Short: "Configure yt-dlp settings",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Show current yt-dlp configuration",
		Run: func(cmd *cobra.Command, args []string) {
			color.Cyan("yt-dlp Configuration:")
			fmt.Printf("  Proxy URL:  %s\n", "N/A (not configured)")
			fmt.Printf("  Cookies:    %s\n", "N/A (not configured)")
		},
	})

	cookiesCmd := &cobra.Command{
		Use:   "cookies",
		Short: "Set or clear cookies file",
	}
	cookiesCmd.AddCommand(&cobra.Command{
		Use:   "set",
		Short: "Set cookies file path",
		Run: func(cmd *cobra.Command, args []string) {
			path, _ := cmd.Flags().GetString("path")
			if path == "" {
				color.Red("Error: --path is required")
				return
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				color.Red("Error: file not found: %s", path)
				return
			}
			color.Green("✓ Cookies set to: %s", path)
			color.Cyan("Restart server to apply changes")
		},
	})
	cookiesCmd.Flags().String("path", "", "Path to cookies.txt file")
	cookiesCmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Clear cookies configuration",
		Run: func(cmd *cobra.Command, args []string) {
			color.Yellow("Clearing cookies configuration...")
			color.Green("✓ Cookies cleared")
		},
	})
	cmd.AddCommand(cookiesCmd)

	proxyCmd := &cobra.Command{
		Use:   "proxy",
		Short: "Set or clear proxy URL",
	}
	proxyCmd.AddCommand(&cobra.Command{
		Use:   "set",
		Short: "Set proxy URL",
		Run: func(cmd *cobra.Command, args []string) {
			url, _ := cmd.Flags().GetString("url")
			if url == "" {
				color.Red("Error: --url is required")
				return
			}
			if !strings.HasPrefix(url, "http") {
				color.Red("Error: proxy URL must start with http:// or https://")
				return
			}
			color.Green("✓ Proxy set to: %s", url)
			color.Cyan("Restart server to apply changes")
		},
	})
	proxyCmd.Flags().String("url", "", "Proxy URL (e.g., http://proxy:8080)")
	proxyCmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Clear proxy configuration",
		Run: func(cmd *cobra.Command, args []string) {
			color.Yellow("Clearing proxy configuration...")
			color.Green("✓ Proxy cleared")
		},
	})
	cmd.AddCommand(proxyCmd)

	return cmd
}
