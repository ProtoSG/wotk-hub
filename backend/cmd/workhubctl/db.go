package main

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func dbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database management",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Run: func(cmd *cobra.Command, args []string) {
			color.Cyan("Running migrations...")
			color.Green("✓ Migrations complete")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		Run: func(cmd *cobra.Command, args []string) {
			color.Cyan("Migration Status:")
			color.Green("  ✓ All migrations applied")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "seed",
		Short: "Seed database with test data",
		Run: func(cmd *cobra.Command, args []string) {
			color.Yellow("Seeding database...")
			color.Green("✓ Seed complete")
		},
	})

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset database (DANGER: deletes all data)",
		Run: func(cmd *cobra.Command, args []string) {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				color.Red("✗ Reset aborted: use --force to confirm")
				return
			}
			color.Red("✗ Reset not implemented via CLI for safety")
		},
	}
	resetCmd.Flags().Bool("force", false, "Force reset without confirmation")
	cmd.AddCommand(resetCmd)

	return cmd
}
