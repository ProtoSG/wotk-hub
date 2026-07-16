package main

import (
	"os"
	"path/filepath"
	"strings"
)

// credentialFile returns the path to the CLI token file.
// Order of precedence: CLI_TOKEN env var > ~/.config/workhub/credentials.
func credentialFile() string {
	if token := os.Getenv("CLI_TOKEN"); token != "" {
		return ""
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "workhub", "credentials")
}

// loadToken returns the CLI token from env or credential file.
// Env var takes precedence over the file.
func loadToken() string {
	if token := os.Getenv("CLI_TOKEN"); token != "" {
		return strings.TrimSpace(token)
	}
	path := credentialFile()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
