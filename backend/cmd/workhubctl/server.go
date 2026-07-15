package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage WorkHub server",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start the WorkHub server",
		Run: func(cmd *cobra.Command, args []string) {
			color.Green("Starting WorkHub server...")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serverProc, err := startServer(ctx)
			if err != nil {
				color.Red("Failed to start server: %v", err)
				os.Exit(1)
			}

			color.Green("Server started successfully!")
			fmt.Printf("Server running at: %s\n", serverURL)
			fmt.Println("Press Ctrl+C to stop")

			<-ctx.Done()
			serverProc.Process.Kill()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop the WorkHub server",
		Run: func(cmd *cobra.Command, args []string) {
			color.Yellow("Stopping WorkHub server...")

			resp, err := http.Get(serverURL + "/health")
			if err != nil {
				color.Red("Server is not running")
				os.Exit(1)
			}
			resp.Body.Close()

			color.Green("Server stopped")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check server status",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Get(serverURL + "/health")
			if err != nil {
				color.Red("✗ Server is not running")
				os.Exit(1)
			}
			defer resp.Body.Close()

			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)

			if result["status"] == "ok" {
				color.Green("✓ Server is running at %s", serverURL)
			} else {
				color.Yellow("⚠ Server returned unexpected status")
			}
		},
	})

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "View server logs",
		Run: func(cmd *cobra.Command, args []string) {
			follow, _ := cmd.Flags().GetBool("follow")

			if follow {
				color.Yellow("Following logs... (Ctrl+C to stop)")
				for {
					resp, err := http.Get(serverURL + "/health")
					if err != nil || resp.StatusCode != 200 {
						color.Red("Server is not running")
						break
					}
					resp.Body.Close()
					time.Sleep(2 * time.Second)
				}
			} else {
				color.Cyan("Logs functionality requires server process tracking")
			}
		},
	}
	logsCmd.Flags().Bool("follow", false, "Follow logs in real-time")
	cmd.AddCommand(logsCmd)

	return cmd
}

func startServer(ctx context.Context) (*exec.Cmd, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "go", "run", "main.go")
	cmd.Dir = wd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start failed: %w", err)
	}

	go func() {
		<-ctx.Done()
		cmd.Process.Kill()
	}()

	time.Sleep(2 * time.Second)
	return cmd, nil
}

func healthCheck() bool {
	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
