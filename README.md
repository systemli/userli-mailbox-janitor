# userli-mailbox-janitor

[![Integration](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/integration.yml/badge.svg)](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/integration.yml)
[![Quality](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/quality.yml/badge.svg)](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/quality.yml)

A webhook-based daemon that automatically purges deleted user mailboxes after a retention period.

## Features

- Listens for user deletion webhooks from userli
- Stores mailbox deletion tasks in a simple CSV file (easy to edit manually)
- Automatically purges mailboxes using a configurable command after configured retention period (default: 24h)
- Updates last login time for inactive users who have sieve scripts with unconditional redirect rules
- HMAC SHA256 webhook signature verification
- Background worker with ticker for processing tasks
- Structured logging with zap
- Configurable via environment variables

## How it works

### Mailbox Purging
1. **Webhook Reception**: Receives `user.deleted` events via HTTP POST to `/userli`
2. **CSV Storage**: Stores the email and creation timestamp in a CSV file
3. **Background Processing**: A ticker runs periodically (configurable interval) to check for due mailboxes
4. **Mailbox Purging**: Executes the configured `PURGER_COMMAND` with placeholders replaced for each due mailbox
5. **Cleanup**: Removes successfully purged mailboxes from the CSV file

### Inactive User Retention (Toucher)
1. **Fetch Inactive Users**: Periodically queries the Userli API for users marked as inactive
2. **Sieve Script Analysis**: For each inactive user, reads their sieve configuration file
3. **Redirect Detection**: Checks if the sieve script contains an unconditional redirect rule (`if true { redirect ... }`)
4. **Touch User**: If a redirect rule is found, updates the user's last login timestamp via the Userli API
5. **Retention Extension**: This prevents the user from being marked for deletion, allowing mail forwarding to continue

## Installation

### From Source

```bash
git clone https://github.com/systemli/userli-mailbox-janitor.git
cd userli-mailbox-janitor
go build -o userli-mailbox-janitor
```

## Configuration

Configuration is done via environment variables:

| Variable                 | Description                                                                           | Default                                                                         |
|--------------------------|---------------------------------------------------------------------------------------|---------------------------------------------------------------------------------|
| `LOG_LEVEL`              | Logging level (debug, info, warn, error)                                              | `info`                                                                          |
| `LISTEN_ADDR`            | HTTP server listen address                                                            | `:8080`                                                                         |
| `WEBHOOK_SECRET`         | Secret for HMAC SHA256 signature verification                                         | *required*                                                                      |
| `PURGER_DATABASE_PATH`   | Path to CSV file for storing mailbox data                                             | `./mailboxes.csv`                                                               |
| `PURGER_RETENTION_HOURS` | Hours to wait before purging mailbox                                                  | `24`                                                                            |
| `PURGER_TICK_INTERVAL`   | Interval for checking due mailboxes (e.g., "5m", "1h")                                | `1h`                                                                            |
| `PURGER_COMMAND`         | Command to purge mailbox, with placeholders `{email}`, `{domain}`, and `{local_part}` | `echo 'No PURGER_COMMAND configured; skipping purge for {domain}/{local_part}'` |
| `TOUCHER_TICK_INTERVAL`  | Interval for checking inactive users for retention (e.g., "1h", "6h")                 | `24h`                                                                           |
| `TOUCHER_SIEVE_LOCATION` | Path pattern for sieve files with placeholders `{domain}` and `{local_part}`          | *required*                                                                      |
| `TOUCHER_USE_SUDO`       | Whether to use sudo when accessing sieve files (true/false)                           | `false`                                                                         |
| `USERLI_URL`             | Base URL of the Userli API                                                            | *required*                                                                      |
| `USERLI_TOKEN`           | API token for Userli authentication                                                   | *required*                                                                      |

## Usage

### Running the Service

```bash
export WEBHOOK_SECRET="your-secret-here"
export PURGER_DATABASE_PATH="/var/lib/mailbox-janitor/mailboxes.csv"
export TOUCHER_SIEVE_LOCATION="/var/vmail/{domain}/{local_part}/.dovecot.sieve"
export USERLI_URL="https://userli.example.org"
export USERLI_TOKEN="your-api-token-here"
./userli-mailbox-janitor
```

### Webhook Integration

Configure userli to send webhooks to your janitor instance:

```bash
WEBHOOK_URL="https://mailbox-janitor.example.org/userli"
SECRET="your-secret-here"
PAYLOAD='{"type":"user.deleted","timestamp":"2025-01-01T00:00:00.000000Z","data":{"email":"user@example.org"}}'
SIGNATURE=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')

curl -i "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: $SIGNATURE" \
  -d "$PAYLOAD"
```

### Userli API Integration

The Toucher feature requires access to the following Userli API endpoints:

- `GET /api/retention/users` - Fetch list of inactive users
- `PUT /api/retention/{email}/touch` - Update user's last login timestamp

## Sieve Redirect Detection

The Toucher feature analyzes sieve scripts to identify users who have configured unconditional email forwarding. This is useful for maintaining active forwarding for users who may appear inactive but are still using the service to redirect their emails.

### Detection Logic

The system looks for sieve scripts containing the following pattern:

```sieve
if true {
  redirect "destination@exampla.com";
}
```
### Example Sieve Configurations

**Will trigger touch (unconditional redirect):**
```sieve
# User has configured forwarding
if true {
  redirect "new-email@example.com";
}
```

**Will NOT trigger touch (conditional redirect):**
```sieve
# Spam filtering with conditional redirect
if header :contains "X-Spam-Flag" "YES" {
  redirect "spam-report@example.com";
}
```

## Development

### Running Tests

```bash
go test -v ./...
```

### Test Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Linting

```bash
golangci-lint run
```

## License

This project is licensed under the GNU Affero General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues and questions, please use the [GitHub issue tracker](https://github.com/systemli/userli-mailbox-janitor/issues).
