package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var authToken string
var cookieJar *cookiejar.Jar

func init() {
	jar, _ := cookiejar.New(nil)
	cookieJar = jar
}

func tokenFile() string {
	return filepath.Join(os.Getenv("HOME"), ".workhub", "token")
}

func saveToken(token string) error {
	dir := filepath.Dir(tokenFile())
	os.MkdirAll(dir, 0700)
	return os.WriteFile(tokenFile(), []byte(token), 0600)
}

func loadToken() string {
	data, err := os.ReadFile(tokenFile())
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(data))
}

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User authentication and info",
	}

	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new user",
		Run:   runUserRegister,
	}
	registerCmd.Flags().String("name", "", "User name (required)")
	registerCmd.Flags().String("email", "", "User email (required)")
	registerCmd.Flags().String("password", "", "User password (min 8 chars, required)")
	cmd.AddCommand(registerCmd)

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login and get auth token",
		Run:   runUserLogin,
	}
	loginCmd.Flags().String("email", "", "User email")
	loginCmd.Flags().String("password", "", "User password")
	cmd.AddCommand(loginCmd)

	meCmd := &cobra.Command{
		Use:   "me",
		Short: "Show current authenticated user info",
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

func extractToken(resp *http.Response) string {
	for _, c := range resp.Cookies() {
		if c.Name == "access_token" {
			return c.Value
		}
	}
	return ""
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

	payload := map[string]string{
		"name":     name,
		"email":    email,
		"password": password,
	}

	body, _ := json.Marshal(payload)
	resp, err := newClient().Post(serverURL+"/api/auth/register", "application/json", bytes.NewBuffer(body))
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
	case http.StatusBadRequest:
		if errMsg, ok := result["error"].(string); ok {
			color.Red("✗ %s", errMsg)
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

	client := newClient()
	resp, err := client.Post(serverURL+"/api/auth/login", "application/json", bytes.NewBuffer(body))
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

		// Extract token from cookie and save
		token := extractToken(resp)
		if token != "" {
			if err := saveToken(token); err != nil {
				color.Yellow("Warning: could not save token: %v", err)
			} else {
				color.Cyan("  Token saved to ~/.workhub/token")
			}
		}
	} else {
		if errMsg, ok := result["error"].(string); ok {
			color.Red("✗ %s", errMsg)
		} else {
			color.Red("✗ Login failed (status: %d)", resp.StatusCode)
		}
	}
}

func runUserMe(cmd *cobra.Command, args []string) {
	token := loadToken()
	if token == "" {
		color.Red("✗ Not authenticated. Run 'workhubctl user login --email ... --password ...' first")
		return
	}

	req, err := http.NewRequest("GET", serverURL+"/api/auth/me", nil)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := newClient()
	resp, err := client.Do(req)
	if err != nil {
		color.Red("Error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Token expired - remove it
		os.Remove(tokenFile())
		color.Red("✗ Token expired. Run 'workhubctl user login --email ... --password ...' again")
		return
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var user map[string]interface{}
	json.Unmarshal(bodyBytes, &user)

	if resp.StatusCode != http.StatusOK {
		color.Red("✗ Request failed (status: %d)", resp.StatusCode)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Field", "Value"})

	for _, field := range []string{"id", "name", "email", "role", "created_at"} {
		if val, ok := user[field]; ok {
			table.Append([]string{field, fmt.Sprintf("%v", val)})
		}
	}
	table.Render()
}
