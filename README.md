# promtotwilio

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.txt)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)

**Send Prometheus alerts as SMS via Twilio.** Get notified on your phone when things go wrong.

**Zero external dependencies** — built entirely with Go's standard library for maximum reliability and minimal attack surface.

```
┌────────────┐     ┌──────────────┐     ┌──────────────┐     ┌────────┐     ┌───────┐
│ Prometheus │────▶│ AlertManager │────▶│ promtotwilio │────▶│ Twilio │────▶│  SMS  │
└────────────┘     └──────────────┘     └──────────────┘     └────────┘     └───────┘
```

---

## Quick Start

```bash
# 1. Pull the image
docker pull ghcr.io/swatto/promtotwilio:latest

# 2. Run with your Twilio credentials
docker run -d \
  -e SID=your_twilio_sid \
  -e TOKEN=your_twilio_token \
  -e SENDER=+15551234567 \
  -e RECEIVER=+15559876543 \
  -p 9090:9090 \
  ghcr.io/swatto/promtotwilio:latest

# 3. Test it
curl -X POST http://localhost:9090/send \
  -H "Content-Type: application/json" \
  -d '{"version":"2","status":"firing","alerts":[{"annotations":{"summary":"Test alert"}}]}'
```

That's it. Point your AlertManager webhook at `http://promtotwilio:9090/send`.

---

## Table of Contents

- [Configuration](#configuration)
- [API](#api)
- [AlertManager Setup](#alertmanager-setup)
- [Message Format](#message-format)
- [Development](#development)
- [License](#license)

---

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SID` | ✅ | — | Twilio Account SID |
| `TOKEN` | ✅* | — | Twilio Auth Token |
| `API_KEY` | — | — | Twilio API Key SID (recommended for production) |
| `API_KEY_SECRET` | — | — | Twilio API Key Secret (required if `API_KEY` is set) |
| `SENDER` | ✅ | — | Twilio phone number (e.g., `+15551234567`) |
| `RECEIVER` | — | — | Default receiver(s), comma-separated |
| `PORT` | — | `9090` | HTTP server port |
| `SEND_RESOLVED` | — | `false` | Send notifications when alerts resolve |
| `MAX_MESSAGE_LENGTH` | — | `150` | Truncate messages beyond this length |
| `MESSAGE_PREFIX` | — | — | Prefix for all messages (e.g., `[PROD]`) |

*\* `TOKEN` is required unless `API_KEY` and `API_KEY_SECRET` are provided.*

### Authentication Methods

promtotwilio supports two authentication methods for the Twilio API:

**Option 1: Account SID + Auth Token** (simpler, for development)
```bash
export SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export TOKEN=your_auth_token
export SENDER=+15551234567
```

**Option 2: API Key (recommended for production)**
```bash
export SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export API_KEY=SKxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export API_KEY_SECRET=your_api_key_secret
export SENDER=+15551234567
```

API Keys are recommended for production because:
- Compromised keys can be revoked individually without affecting other integrations
- Keys can have restricted permissions
- Your account-level Auth Token remains protected

Create API Keys in the [Twilio Console](https://www.twilio.com/console/runtime/api-keys).

### Multiple Receivers

```bash
export RECEIVER="+1234567890,+0987654321,+1122334455"
```

---

## API

### `GET /`
Health ping. Returns `200 OK`.

### `GET /health`
Returns JSON with status, version, and uptime.

### `POST /send`
Receives Prometheus/AlertManager webhook payloads and sends SMS.

**Query Parameters:**
- `receiver` — Override default receiver(s). Comma-separated, URL-encoded.

**Example:**
```bash
curl -X POST "http://localhost:9090/send?receiver=%2B1234567890" \
  -H "Content-Type: application/json" \
  -d '{"version":"2","status":"firing","alerts":[{"annotations":{"summary":"Server down"}}]}'
```

**Response:**
```json
{
  "success": true,
  "sent": 1,
  "failed": 0,
  "errors": []
}
```

---

## AlertManager Setup

### Minimal Config

```yaml
route:
  receiver: 'sms'

receivers:
  - name: 'sms'
    webhook_configs:
      - url: 'http://promtotwilio:9090/send'
```

### Docker Compose

```yaml
services:
  promtotwilio:
    image: ghcr.io/swatto/promtotwilio:latest
    environment:
      SID: ${TWILIO_SID}
      # Option 1: Auth Token
      TOKEN: ${TWILIO_TOKEN}
      # Option 2: API Key (recommended for production)
      # API_KEY: ${TWILIO_API_KEY}
      # API_KEY_SECRET: ${TWILIO_API_KEY_SECRET}
      SENDER: ${TWILIO_SENDER}
      RECEIVER: ${ALERT_RECEIVERS}
      SEND_RESOLVED: "true"
    ports:
      - "9090:9090"

  alertmanager:
    image: prom/alertmanager:latest
    volumes:
      - ./alertmanager.yml:/etc/alertmanager/config.yml
    depends_on:
      - promtotwilio
```

### Prometheus Alert Rule Example

```yaml
groups:
  - name: example
    rules:
      - alert: NodeDown
        expr: up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: '{{ $labels.instance }} is down'
```

> **Note:** Alerts require either a `summary` or `description` annotation.

---

## Message Format

**Firing:**
```
[MESSAGE_PREFIX] [AlertName] "Summary" alert starts at <timestamp>
```

**Resolved** (when `SEND_RESOLVED=true`):
```
[MESSAGE_PREFIX] RESOLVED: [AlertName] "Summary" alert starts at <timestamp>
```

**Examples:**
| Config | Message |
|--------|---------|
| Default | `[NodeDown] "Server is down" alert starts at Mon, 15 Jan 2024 10:30:00 UTC` |
| With `MESSAGE_PREFIX=[PROD]` | `[PROD] [NodeDown] "Server is down" alert starts at Mon, 15 Jan 2024 10:30:00 UTC` |
| Resolved | `[PROD] RESOLVED: [NodeDown] "Back online" alert starts at Mon, 15 Jan 2024 10:30:00 UTC` |

---

## Development

### Zero Dependencies

This project uses **only Go's standard library** — no external dependencies. This means:

- **Minimal attack surface** — no third-party supply chain risks
- **Fast builds** — no dependency downloads required
- **Maximum reliability** — only battle-tested stdlib code
- **Tiny binary** — ~5MB statically compiled

### Prerequisites

- Go 1.26+
- Docker (optional)

### Commands

```bash
make build      # Build binary
make test       # Run tests
make coverage   # Tests with coverage
make lint       # Run linter
make check      # All checks
make e2e        # Run E2E tests (requires Docker)
make dev        # Run locally
```

### Run Locally

```bash
export SID=your_twilio_sid
export TOKEN=your_twilio_token
export SENDER=+1234567890

make dev
```

### Docker Images

```bash
docker pull ghcr.io/swatto/promtotwilio:latest
docker pull ghcr.io/swatto/promtotwilio:1.0.0   # Specific version
docker pull ghcr.io/swatto/promtotwilio:1.0     # Latest patch
docker pull ghcr.io/swatto/promtotwilio:1       # Latest minor
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines and release process.

---

## License

[MIT](LICENSE.txt)
