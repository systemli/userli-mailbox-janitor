# userli-mailbox-janitor

[![Integration](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/integration.yml/badge.svg)](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/integration.yml)
[![Quality](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/quality.yml/badge.svg)](https://github.com/systemli/userli-mailbox-janitor/actions/workflows/quality.yml)

A webhook-based daemon that automatically purges deleted user mailboxes after a retention period.

## Features

- Listens for user deletion webhooks from userli
- Stores mailbox deletion tasks in a simple CSV file (easy to edit manually)
- Automatically purges mailboxes using `doveadm` after configured retention period (default: 24h)
- HMAC SHA256 webhook signature verification
- Background worker with ticker for processing tasks
- Structured logging with zap
- Configurable via environment variables

## How it works

1. **Webhook Reception**: Receives `user.deleted` events via HTTP POST to `/userli`
2. **CSV Storage**: Stores the email and creation timestamp in a CSV file
3. **Background Processing**: A ticker runs periodically (configurable interval) to check for due mailboxes
4. **Mailbox Purging**: Executes `sudo doveadm purge <email>` for each due mailbox
5. **Cleanup**: Removes successfully purged mailboxes from the CSV file

## Installation

### From Source

```bash
git clone https://github.com/systemli/userli-mailbox-janitor.git
cd userli-mailbox-janitor
go build -o userli-mailbox-janitor
```

## Configuration

Configuration is done via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `LISTEN_ADDR` | HTTP server listen address | `:8080` |
| `WEBHOOK_SECRET` | Secret for HMAC SHA256 signature verification | *required* |
| `DATABASE_PATH` | Path to CSV file for storing mailbox data | `./mailboxes.csv` |
| `RETENTION_HOURS` | Hours to wait before purging mailbox | `24` |
| `TICK_INTERVAL` | Interval for checking due mailboxes (e.g., "5m", "1h") | `5m` |
| `DOVEADM_PATH` | Path to doveadm executable | `/usr/bin/doveadm` |
| `USE_SUDO` | Whether to use sudo for doveadm | `true` |

## Usage

### Running the Service

```bash
export WEBHOOK_SECRET="your-secret-here"
export DATABASE_PATH="/var/lib/mailbox-janitor/mailboxes.csv"
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
