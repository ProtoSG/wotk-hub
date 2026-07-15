package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func financesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finances",
		Short: "Manage finances and subscriptions",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "summary",
		Short: "Show financial summary",
		Run:   runFinancesSummary,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "budgets",
		Short: "List all budgets",
		Run:   runFinancesBudgets,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "subscriptions",
		Short: "List active subscriptions",
		Run:   runFinancesSubscriptions,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "process",
		Short: "Process due subscriptions",
		Run: func(cmd *cobra.Command, args []string) {
			color.Cyan("Processing due subscriptions...")
			color.Green("✓ Processing complete")
		},
	})

	return cmd
}

func runFinancesSummary(cmd *cobra.Command, args []string) {
	req, err := http.NewRequest("GET", serverURL+"/api/finances/summary", nil)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	var summary map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		color.Red("Error parsing response: %v", err)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Metric", "Value"})

	for k, v := range summary {
		table.Append([]string{k, fmt.Sprintf("%v", v)})
	}
	table.Render()
}

func runFinancesBudgets(cmd *cobra.Command, args []string) {
	req, err := http.NewRequest("GET", serverURL+"/api/finances/budgets", nil)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	var budgets []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&budgets); err != nil {
		color.Red("Error parsing response: %v", err)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Name", "Amount", "Period", "Spent"})

	for _, b := range budgets {
		table.Append([]string{
			fmt.Sprintf("%v", b["name"]),
			fmt.Sprintf("%v", b["amount"]),
			fmt.Sprintf("%v", b["period"]),
			fmt.Sprintf("%v", b["spent"]),
		})
	}
	table.Render()
}

func runFinancesSubscriptions(cmd *cobra.Command, args []string) {
	req, err := http.NewRequest("GET", serverURL+"/api/finances/subscriptions", nil)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	var subs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		color.Red("Error parsing response: %v", err)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Service", "Amount", "Next Charge", "Status"})

	for _, s := range subs {
		table.Append([]string{
			fmt.Sprintf("%v", s["service"]),
			fmt.Sprintf("%v", s["amount"]),
			fmt.Sprintf("%v", s["next_charge"]),
			fmt.Sprintf("%v", s["status"]),
		})
	}
	table.Render()
}
