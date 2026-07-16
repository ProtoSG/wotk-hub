package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var cookieJar *cookiejar.Jar

func init() {
	jar, _ := cookiejar.New(nil)
	cookieJar = jar
}

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management and info",
	}

	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new user (admin only)",
		Run:   runUserRegister,
	}
	registerCmd.Flags().String("name", "", "User name (required)")
	registerCmd.Flags().String("email", "", "User email (required)")
	registerCmd.Flags().String("password", "", "User password (min 8 chars, required)")
	cmd.AddCommand(registerCmd)

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login as an existing user (stores session cookie)",
		Run:   runUserLogin,
	}
	loginCmd.Flags().String("email", "", "User email")
	loginCmd.Flags().String("password", "", "User password")
	cmd.AddCommand(loginCmd)

	meCmd := &cobra.Command{
		Use:   "me",
		Short: "Show identity used by the CLI (from stored CLI token)",
		Run:   runUserMe,
	}
	cmd.AddCommand(meCmd)

	return cmd
}

func newClient() *http.Client {
	return &http.Client{
		Jar:     cookieJar,
		Timeout: 10 * time.Second,
	}
}

func runUserRegister(cmd *cobra.Command, args []string) {
	email, _ := cmd.Flags().GetString("email")
	password, _ := cmd.Flags().GetString("password")
	name, _ := cmd.Flags().GetString("name")

	if name == "" || email == "" || password == "" {
		color.Red("Error: --name, --email and --password are required")
		color.Cyan("Usage: workhubctl user register --name \"John\" --email user@example.com --password secret123")
		return
	}
	if len(password) < 8 {
		color.Red("Error: password must be at least 8 characters")
		return
	}

	payload := map[string]string{"name": name, "email": email, "password": password}
	body, _ := json.Marshal(payload)

	token := loadToken()
	req, err := http.NewRequest("POST", serverURL+"/api/auth/register", bytes.NewBuffer(body))
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := newClient().Do(req)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	switch resp.StatusCode {
	case http.StatusCreated:
		color.Green("✓ User registered: %s", email)
		if msg, ok := result["message"].(string); ok {
			color.Cyan("  %s", msg)
		}
	case http.StatusForbidden:
		color.Red("✗ Registration closed (max users reached)")
	case http.StatusUnauthorized:
		color.Red("✗ Unauthorized. Set CLI_TOKEN env var or run 'workhubctl auth login <token>'.")
	case http.StatusBadRequest:
		if msg, ok := result["message"].(string); ok {
			color.Red("✗ %s", msg)
		}
	default:
		color.Red("✗ Failed (status: %d)", resp.StatusCode)
	}
}

func runUserLogin(cmd *cobra.Command, args []string) {
	email, _ := cmd.Flags().GetString("email")
	password, _ := cmd.Flags().GetString("password")

	if email == "" || password == "" {
		color.Red("Error: --email and --password are required")
		color.Cyan("Usage: workhubctl user login --email user@example.com --password secret123")
		return
	}

	payload := map[string]string{"email": email, "password": password}
	body, _ := json.Marshal(payload)

	resp, err := newClient().Post(serverURL+"/api/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode == http.StatusOK {
		color.Green("✓ Logged in as %s", email)
		if name, ok := result["name"].(string); ok {
			color.Cyan("  Name: %s", name)
		}
		if role, ok := result["role"].(string); ok {
			color.Cyan("  Role: %s", role)
		}
	} else {
		if msg, ok := result["message"].(string); ok {
			color.Red("✗ %s", msg)
		} else if errMsg, ok := result["error"].(string); ok {
			color.Red("✗ %s", errMsg)
		} else {
			color.Red("✗ Login failed (status: %d)", resp.StatusCode)
		}
	}
}

func runUserMe(cmd *cobra.Command, args []string) {
	token := strings.TrimSpace(loadToken())
	if token == "" {
		color.Red("✗ Not authenticated. Set CLI_TOKEN env var or run:")
		color.Cyan("  workhubctl auth login <your-cli-token>")
		return
	}

	req, err := http.NewRequest("GET", serverURL+"/api/cli/me", nil)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		color.Red("✗ Token invalid or expired.")
		color.Cyan("Run 'workhubctl auth login <token>' to update.")
		return
	}
	if resp.StatusCode != http.StatusOK {
		color.Red("✗ Request failed (status: %d)", resp.StatusCode)
		return
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var user map[string]interface{}
	json.Unmarshal(bodyBytes, &user)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Field", "Value"})
	for _, field := range []string{"id", "name", "email", "role"} {
		if val, ok := user[field]; ok {
			table.Append([]string{field, fmt.Sprintf("%v", val)})
		}
	}
	table.Render()
}
