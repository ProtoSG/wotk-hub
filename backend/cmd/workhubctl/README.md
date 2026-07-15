# workhubctl — WorkHub CLI

Command-line interface for managing WorkHub server.

## Installation

```bash
cd cmd/workhubctl
go install
```

Or build manually:

```bash
go build -o workhubctl
```

## Quick Start

```bash
# Check server status
workhubctl health

# Start server
workhubctl server start

# Create admin user
workhubctl user create --email admin@test.com --password "secret" --role admin

# List users
workhubctl user list

# Generate auth token
workhubctl user token --email admin@test.com --password "secret"

# Check finances
workhubctl finances summary

# Configure yt-dlp cookies
workhubctl ytdlp cookies set --path ./cookies.txt

# Validate config
workhubctl config validate

# Generate .env template
workhubctl config generate --output .env
```

## Commands

### server
- `start` — Start the WorkHub server
- `stop` — Stop the server
- `status` — Check server status
- `logs` — View server logs (with `--follow` for real-time)

### user
- `create` — Create a new user (`--email`, `--password`, `--role`)
- `list` — List all users (`--role` to filter)
- `delete` — Delete a user (`--email`)
- `role` — Change user role (`--email`, `--role`)
- `token` — Generate auth token (`--email`, `--password`)

### db
- `migrate` — Run database migrations
- `status` — Show migration status
- `seed` — Seed database with test data
- `reset` — Reset database (requires `--force`)

### finances
- `summary` — Show financial summary
- `budgets` — List all budgets
- `subscriptions` — List active subscriptions
- `process` — Process due subscriptions

### ytdlp
- `config` — Show current yt-dlp configuration
- `cookies set/clear` — Set or clear cookies file
- `proxy set/clear` — Set or clear proxy URL

### config
- `show` — Show current configuration
- `validate` — Validate .env configuration
- `generate` — Generate .env from template

## Flags

- `-s, --server` — Server URL (default: http://localhost:8080)
- `--api-key` — API key for authentication

## Examples

```bash
# Connect to remote server
workhubctl -s https://api.myserver.com health

# List only admin users
workhubctl user list --role admin

# Set proxy with specific URL
workhubctl ytdlp proxy set --url http://proxy.example.com:8080
```
