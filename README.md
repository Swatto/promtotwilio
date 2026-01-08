# Prometheus alert with text message

This is a simple and stupid program that will receive webhooks from [Prometheus](https://prometheus.io/) to send them as text message (using [Twilio](https://www.twilio.com/)) with the summary of the alert.

## Configuration

It needs 4 environment variables:

- `SID` - Twilio Account SID
- `TOKEN` - Twilio Auth Token
- `RECEIVER` - Phone number(s) of receiver (optional, supports comma-separated list for multiple receivers)
- `SENDER` - Phone number managed by Twilio (friendly name)
- `PORT` - Port to listen on (optional, defaults to `9090`)

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

### Create a Release

Releases are automated via GitHub Actions. When you push a version tag, the CI pipeline will:

1. Run tests, linting, and build validation
2. Build multi-platform Docker images (linux/amd64, linux/arm64)
3. Push to GitHub Container Registry

```bash
# Make sure you're on main with latest changes
git checkout main
git pull

# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag
git push origin v1.0.0
```

### Docker Images

Images are automatically published to GitHub Container Registry:

```bash
docker pull ghcr.io/swatto/promtotwilio:1.0.0
```

Available tags for each release:
- `ghcr.io/swatto/promtotwilio:1.0.0` - exact version
- `ghcr.io/swatto/promtotwilio:1.0` - minor version (latest patch)
- `ghcr.io/swatto/promtotwilio:1` - major version (latest minor)

### Version Convention

| Tag | When to Use |
|-----|-------------|
| `v1.0.0` | First stable release |
| `v1.0.1` | Bug fixes only |
| `v1.1.0` | New features, backward compatible |
| `v2.0.0` | Breaking changes |

### Useful Commands

```bash
# List existing tags
git tag -l

# Delete a tag (if you made a mistake)
git tag -d v1.0.0
git push origin --delete v1.0.0

# Create a release from an older commit
git tag -a v1.0.0 <commit-sha> -m "Release v1.0.0"
```

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

## License

MIT
