package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage WorkHub configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run: func(cmd *cobra.Command, args []string) {
			color.Cyan("Current Configuration:")
			fmt.Printf("  Server URL:  %s\n", serverURL)
			fmt.Printf("  Port:        %s\n", "8080 (default)")
			fmt.Printf("  Database:    %s\n", "configured via DATABASE_URL")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate .env configuration",
		Run: func(cmd *cobra.Command, args []string) {
			color.Cyan("Validating configuration...")

			required := []string{"JWT_SECRET", "DATABASE_URL"}
			missing := 0

			for _, key := range required {
				if val := os.Getenv(key); val == "" {
					color.Yellow("  ⚠ %s: not set", key)
					missing++
				} else {
					color.Green("  ✓ %s: configured", key)
				}
			}

			if missing > 0 {
				color.Red("\n✗ Configuration invalid: %d required vars missing", missing)
			} else {
				color.Green("\n✓ Configuration valid")
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "generate",
		Short: "Generate .env from template",
		Run: func(cmd *cobra.Command, args []string) {
			path, _ := cmd.Flags().GetString("output")

			template := `# WorkHub Configuration
# Copy this file to .env and fill in your values

# Required
JWT_SECRET=your-super-secret-jwt-key-change-me
DATABASE_URL=postgres://user:password@localhost:5432/workhub

# Optional
PORT=8080
CORS_ORIGIN=http://localhost:5173
COOKIE_SECURE=false

# yt-dlp (optional)
YTDLP_COOKIES_PATH=
YTDLP_PROXY_URL=
YTDLP_PUBLIC_TOKEN=
`
			if path == "" {
				path = ".env"
			}

			if _, err := os.Stat(path); err == nil {
				color.Red("✗ File already exists: %s", path)
				return
			}

			if err := os.WriteFile(path, []byte(template), 0644); err != nil {
				color.Red("Error: %v", err)
				return
			}

			color.Green("✓ Generated: %s", path)
		},
	})

	if c, _ := cmd.Flags().GetString("output"); c == "" {
		// noop - flag is registered below
	}
	cmd.Flags().String("output", "", "Output file path (default: .env)")

	return cmd
}
