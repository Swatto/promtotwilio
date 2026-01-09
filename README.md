# Prometheus alert with text message

This is a simple and stupid program that will receive webhooks from [Prometheus](https://prometheus.io/) to send them as text message (using [Twilio](https://www.twilio.com/)) with the summary of the alert.

## Configuration

It needs 4 environment variables:

- `SID` - Twilio Account SID
- `TOKEN` - Twilio Auth Token
- `RECEIVER` - Phone number(s) of receiver (optional, supports comma-separated list for multiple receivers)
- `SENDER` - Phone number managed by Twilio (full number, formatted with a '+' and country code, e.g., `+15551234567`)
- `PORT` - Port to listen on (optional, defaults to `9090`)
- `SEND_RESOLVED` - Enable sending notifications for resolved alerts (optional, defaults to `false`)
- `MAX_MESSAGE_LENGTH` - Maximum message length before truncation (optional, defaults to `150`). Messages longer than this will be truncated with "..." suffix. Note: SMS messages are typically limited to 160 characters per segment.

### Multiple Receivers

You can specify multiple default receivers by providing a comma-separated list:

```bash
export RECEIVER="+1234567890,+0987654321,+1122334455"
```

You can see a basic launch inside the Makefile.

## API

`GET /`: ping promtotwilio application. Returns 200 OK if application works fine.

`GET /health`: health check endpoint returning JSON with status, version, and uptime.

`POST /send?receiver=<rcv>`: send Prometheus firing alerts from payload to receiver(s). 

- If `receiver` query parameter is specified, it overrides the default `RECEIVER` environment variable
- Supports multiple receivers as comma-separated values: `?receiver=%2B1234567890,%2B0987654321`
- If no receiver is specified (neither query param nor environment variable), returns 400 Bad Request

### Response Format

The `/send` endpoint returns a JSON response:

```json
{
  "success": true,
  "sent": 3,
  "failed": 0,
  "errors": []
}
```

- `success`: `true` if all messages were sent successfully, `false` if any failed
- `sent`: Number of successfully sent messages
- `failed`: Number of failed messages
- `errors`: Array of error messages for failed sends

Returns HTTP 200 on success, HTTP 500 if any message fails to send.

## Test it

To send test sms to a phone +zxxxyyyyyyy use the following command (please notice `%2B` symbols, representing a url encoded `+` sign)

```bash
curl -H "Content-Type: application/json" -X POST -d \
'{"version":"2","status":"firing","alerts":[{"annotations":{"summary":"Server down"},"startsAt":"2016-03-19T05:54:01Z"}]}' \
http://localhost:9090/send?receiver=%2Bzxxxyyyyyyy
```

To send to multiple receivers:

```bash
curl -H "Content-Type: application/json" -X POST -d \
'{"version":"2","status":"firing","alerts":[{"annotations":{"summary":"Server down"},"startsAt":"2016-03-19T05:54:01Z"}]}' \
"http://localhost:9090/send?receiver=%2B1234567890,%2B0987654321"
```

## Development

### Prerequisites

- Go 1.23+
- Docker (optional)

### Build

```bash
# Build binary
make build

# Run tests
make test

# Run tests with coverage
make coverage

# Run linter
make lint

# Run all checks
make check
```

### Run locally

```bash
# Set environment variables
export SID=your_twilio_sid
export TOKEN=your_twilio_token
export SENDER=+1234567890

# Run
make dev
```

## Releasing

### Automated Release Process

Releases are fully automated via GitHub Actions to prevent human errors and ensure consistency. The release process is triggered manually through the GitHub Actions UI, which then automatically handles all steps safely.

#### Why Automation Prevents Mistakes

The automated release process eliminates common human errors:

- **Version Format Validation**: Automatically validates that the version follows the `v1.2.3` format, preventing typos like `v1.0` or `1.0.0`
- **Pre-Release Testing**: Runs all tests, linting, and E2E tests before creating any tags or releases - broken code cannot be released
- **Consistent Tagging**: Creates git tags automatically with proper format and messages, preventing tag naming mistakes
- **Multi-Platform Builds**: Builds binaries for all platforms (Linux amd64/arm64, Darwin amd64/arm64) consistently
- **Automatic Release Creation**: Creates GitHub Releases with all binaries attached automatically
- **Docker Image Tagging**: Automatically creates multiple Docker image tags (version, semver patterns, latest) to ensure proper versioning

#### How to Create a Release

1. **Navigate to GitHub Actions**:
   - Go to the repository on GitHub
   - Click on the "Actions" tab
   - Select the "CI" workflow from the left sidebar

2. **Trigger the Release Workflow**:
   - Click "Run workflow" button (top right)
   - Enter the version in the format `v1.2.3` (e.g., `v1.0.0`, `v1.0.1`, `v1.1.0`)
   - Click "Run workflow"

3. **Wait for Completion**:
   - The workflow will automatically:
     - ✅ Run all tests (unit tests, linting, E2E tests)
     - ✅ Validate the version format
     - ✅ Create and push the git tag (only if all tests pass)
     - ✅ Build binaries for all platforms
     - ✅ Create a GitHub Release with binaries attached
     - ✅ Build and push Docker images with proper tags

**Important**: The release will only proceed if all checks pass. If any test fails, the process stops and no tag or release is created, preventing broken releases.

#### What Gets Created Automatically

When the workflow completes successfully:

- **Git Tag**: Annotated tag `v1.2.3` is created and pushed to the repository
- **GitHub Release**: A release is created with:
  - Release notes automatically generated from commits
  - Binaries for all platforms attached:
    - `promtotwilio-linux-amd64`
    - `promtotwilio-linux-arm64`
    - `promtotwilio-darwin-amd64`
    - `promtotwilio-darwin-arm64`
- **Docker Images**: Multi-platform images are pushed to GitHub Container Registry with multiple tags:
  - `ghcr.io/swatto/promtotwilio:1.2.3` - exact version
  - `ghcr.io/swatto/promtotwilio:1.2` - minor version (latest patch)
  - `ghcr.io/swatto/promtotwilio:1` - major version (latest minor)
  - `ghcr.io/swatto/promtotwilio:latest` - latest stable release

### Docker Images

Images are automatically published to GitHub Container Registry:

```bash
docker pull ghcr.io/swatto/promtotwilio:1.0.0
```

Available tags for each release:
- `ghcr.io/swatto/promtotwilio:1.0.0` - exact version
- `ghcr.io/swatto/promtotwilio:1.0` - minor version (latest patch)
- `ghcr.io/swatto/promtotwilio:1` - major version (latest minor)
- `ghcr.io/swatto/promtotwilio:latest` - latest stable release

### Troubleshooting

If a release fails:

1. **Check the workflow logs** in GitHub Actions to see which step failed
2. **Fix the issue** (e.g., failing tests, linting errors)
3. **Re-run the workflow** with the same version (the workflow will handle existing tags gracefully)

**Note**: Do not manually create tags or releases. The automated process ensures consistency and prevents mistakes.

## Configuration example

Here's a sample Docker Compose file to use it with [cAdvisor](https://github.com/google/cadvisor), [Prometheus](http://prometheus.io/), [Alertmanager](https://github.com/prometheus/alertmanager) and [Grafana](https://github.com/grafana/grafana).

```yml
sms:
  image: ghcr.io/swatto/promtotwilio:latest
  environment:
    SID: xxx
    TOKEN: xxx
    RECEIVER: "+1234567890,+0987654321"
    SENDER: xxx
    PORT: 9090
    SEND_RESOLVED: "true"  # Optional: enable resolved alert notifications

alert:
  image: prom/alertmanager:latest
  links:
   - sms
  volumes:
   - ./alertmanager.yml:/etc/alertmanager/config.yml

container:
  image: google/cadvisor:latest
  volumes:
   - /:/rootfs:ro
   - /var/run:/var/run:rw
   - /sys:/sys:ro
   - /var/lib/docker/:/var/lib/docker:ro

prometheus:
  image: prom/prometheus:latest
  links:
   - container
   - alert
  volumes:
   - ./prometheus.yml:/etc/prometheus/prometheus.yml
   - ./alerts.conf:/etc/prometheus/alerts.conf
  entrypoint: /bin/prometheus -config.file=/etc/prometheus/prometheus.yml -alertmanager.url=http://alert:9093

web:
  image: grafana/grafana:latest
  links:
   - prometheus
  ports:
   - "3000:3000"
  environment:
    GF_SERVER_ROOT_URL: http://stats.example.com
    GF_SECURITY_ADMIN_PASSWORD: 123456
```

Here's the AlertManager config where `sms` will be provided by Docker Compose

```yml
route:
  receiver: 'admin'

receivers:
- name: 'admin'
  webhook_configs:
  - url: 'http://sms:9090/send'
```

### Example Prometheus Alert Rule

The alert message is taken from the `summary` annotation field. Here's an example alert rule:

```yml
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

**Note:** The `summary` annotation is **required**. Alerts without a `summary` annotation will fail with a "missing summary annotation" error.

### Resolved Alerts

By default, only alerts with status "firing" are sent. To also receive notifications when alerts are resolved, set the `SEND_RESOLVED` environment variable to `"true"`. Resolved alerts will be prefixed with "RESOLVED: " in the message.

Example resolved alert message:
```
RESOLVED: "Server is back online" alert starts at Mon, 15 Jan 2024 10:30:00 UTC
```

## License

MIT
