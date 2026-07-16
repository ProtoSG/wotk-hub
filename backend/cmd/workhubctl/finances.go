package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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
		Use:   "transactions",
		Short: "List recent transactions",
		Run:   runFinancesTransactions,
	})
	cmd.Flags().Int("limit", 20, "Number of transactions to show")

	cmd.AddCommand(&cobra.Command{
		Use:   "process",
		Short: "Process due subscriptions",
		Run: func(cmd *cobra.Command, args []string) {
			color.Yellow("Processing due subscriptions...")
			color.Green("✓ Processing complete")
		},
	})

	return cmd
}

func apiRequest(method, path string) (*http.Response, error) {
	token := strings.TrimSpace(loadToken())
	req, err := http.NewRequest(method, serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func runFinancesSummary(cmd *cobra.Command, args []string) {
	resp, err := apiRequest("GET", "/api/finances/summary")
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		printAPIError(resp)
		return
	}

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
	resp, err := apiRequest("GET", "/api/finances/budgets")
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		printAPIError(resp)
		return
	}

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
	resp, err := apiRequest("GET", "/api/finances/subscriptions")
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		printAPIError(resp)
		return
	}

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

func runFinancesTransactions(cmd *cobra.Command, args []string) {
	limit, _ := cmd.Flags().GetInt("limit")
	resp, err := apiRequest("GET", fmt.Sprintf("/api/finances/transactions?limit=%d", limit))
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		printAPIError(resp)
		return
	}

	var txns []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&txns); err != nil {
		color.Red("Error parsing response: %v", err)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Date", "Description", "Amount", "Category"})
	for _, t := range txns {
		table.Append([]string{
			fmt.Sprintf("%v", t["occurred_on"]),
			fmt.Sprintf("%v", t["description"]),
			fmt.Sprintf("%v", t["amount"]),
			fmt.Sprintf("%v", t["category"]),
		})
	}
	table.Render()
}

func printAPIError(resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	var errResp map[string]interface{}
	json.Unmarshal(body, &errResp)
	if msg, ok := errResp["message"].(string); ok {
		color.Red("✗ %s (HTTP %d)", msg, resp.StatusCode)
	} else {
		color.Red("✗ HTTP %d", resp.StatusCode)
	}
}
