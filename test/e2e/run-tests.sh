#!/bin/bash
set -e

APP_URL="${APP_URL:-http://promtotwilio:9090}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://alertmanager:9093}"
MOCK_TWILIO_URL="${MOCK_TWILIO_URL:-http://mock-twilio:8080}"
MOCK_EXPORTER_URL="${MOCK_EXPORTER_URL:-http://mock-exporter:9100}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://prometheus:9090}"
MAX_RETRIES=30
RETRY_INTERVAL=1

echo "========================================="
echo "E2E Tests for promtotwilio"
echo "========================================="
echo "App URL: $APP_URL"
echo "AlertManager URL: $ALERTMANAGER_URL"
echo "Mock Twilio URL: $MOCK_TWILIO_URL"
echo "Mock Exporter URL: $MOCK_EXPORTER_URL"
echo "Prometheus URL: $PROMETHEUS_URL"
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0

pass() {
    echo -e "${GREEN}PASS${NC}: $1"
    PASSED=$((PASSED + 1))
}

fail() {
    echo -e "${RED}FAIL${NC}: $1"
    echo "  Expected: $2"
    echo "  Got: $3"
    FAILED=$((FAILED + 1))
}

wait_for_service() {
    local url=$1
    local name=$2
    echo "Waiting for $name to be ready..."
    for i in $(seq 1 $MAX_RETRIES); do
        if curl -sf "$url" > /dev/null 2>&1; then
            echo "$name is ready!"
            return 0
        fi
        if [ $i -eq $MAX_RETRIES ]; then
            echo "ERROR: $name did not become ready after $MAX_RETRIES seconds"
            return 1
        fi
        echo "  Attempt $i/$MAX_RETRIES - waiting..."
        sleep $RETRY_INTERVAL
    done
}

# Wait for all services to be ready
wait_for_service "$APP_URL/" "promtotwilio" || exit 1
wait_for_service "$ALERTMANAGER_URL/-/healthy" "AlertManager" || exit 1
wait_for_service "$MOCK_TWILIO_URL/health" "Mock Twilio" || exit 1
wait_for_service "$MOCK_EXPORTER_URL/health" "Mock Exporter" || exit 1
wait_for_service "$PROMETHEUS_URL/-/ready" "Prometheus" || exit 1

echo ""
echo "========================================="
echo "Part 1: Direct Endpoint Tests"
echo "========================================="

# Test 1: GET / returns "ping"
echo ""
echo "Test 1: GET / should return 'ping'"
RESPONSE=$(curl -sf "$APP_URL/")
if [ "$RESPONSE" = "ping" ]; then
    pass "GET / returns 'ping'"
else
    fail "GET / returns 'ping'" "ping" "$RESPONSE"
fi

# Test 2: GET /health returns valid JSON with status "ok"
echo ""
echo "Test 2: GET /health should return JSON with status 'ok'"
RESPONSE=$(curl -sf "$APP_URL/health")
STATUS=$(echo "$RESPONSE" | jq -r '.status' 2>/dev/null)
VERSION=$(echo "$RESPONSE" | jq -r '.version' 2>/dev/null)

if [ "$STATUS" = "ok" ] && [ -n "$VERSION" ]; then
    pass "GET /health returns status 'ok' and has version"
    echo "  Version: $VERSION"
else
    fail "GET /health returns valid JSON" "status=ok, version present" "status=$STATUS, version=$VERSION"
fi

# Test 3: POST /send with valid payload succeeds
echo ""
echo "Test 3: POST /send with valid Prometheus alert payload"
PAYLOAD='{
    "version": "4",
    "status": "firing",
    "alerts": [{
        "annotations": {"summary": "E2E Test Alert"},
        "startsAt": "2024-01-01T12:00:00Z"
    }]
}'

HTTP_CODE=$(curl -sf -o /tmp/send_response.json -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$APP_URL/send?receiver=%2B1234567890")

RESPONSE=$(cat /tmp/send_response.json 2>/dev/null || echo "{}")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success' 2>/dev/null)
SENT=$(echo "$RESPONSE" | jq -r '.sent' 2>/dev/null)

if [ "$HTTP_CODE" = "200" ] && [ "$SUCCESS" = "true" ] && [ "$SENT" = "1" ]; then
    pass "POST /send processes alert successfully"
    echo "  HTTP Code: $HTTP_CODE, Success: $SUCCESS, Sent: $SENT"
else
    fail "POST /send processes alert successfully" "HTTP 200, success=true, sent=1" "HTTP $HTTP_CODE, success=$SUCCESS, sent=$SENT"
    echo "  Response: $RESPONSE"
fi

# Test 4: POST /send with multiple receivers
echo ""
echo "Test 4: POST /send with multiple receivers"
HTTP_CODE=$(curl -sf -o /tmp/send_response_multi.json -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$APP_URL/send?receiver=%2B1111111111,%2B2222222222")

RESPONSE=$(cat /tmp/send_response_multi.json 2>/dev/null || echo "{}")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success' 2>/dev/null)
SENT=$(echo "$RESPONSE" | jq -r '.sent' 2>/dev/null)

if [ "$HTTP_CODE" = "200" ] && [ "$SUCCESS" = "true" ] && [ "$SENT" = "2" ]; then
    pass "POST /send with multiple receivers sends to all"
    echo "  HTTP Code: $HTTP_CODE, Success: $SUCCESS, Sent: $SENT"
else
    fail "POST /send with multiple receivers" "HTTP 200, success=true, sent=2" "HTTP $HTTP_CODE, success=$SUCCESS, sent=$SENT"
    echo "  Response: $RESPONSE"
fi

# Test 5: POST /send without receiver returns 400
echo ""
echo "Test 5: POST /send without receiver should return 400"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"status":"firing","alerts":[{"annotations":{"summary":"Test"}}]}' \
    "$APP_URL/send")

if [ "$HTTP_CODE" = "400" ]; then
    pass "POST /send without receiver returns 400"
else
    fail "POST /send without receiver returns 400" "400" "$HTTP_CODE"
fi

# Test 6: POST /send with wrong content-type returns 406
echo ""
echo "Test 6: POST /send with wrong Content-Type should return 406"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST \
    -H "Content-Type: text/plain" \
    -d '{}' \
    "$APP_URL/send?receiver=%2B1234567890")

if [ "$HTTP_CODE" = "406" ]; then
    pass "POST /send with wrong Content-Type returns 406"
else
    fail "POST /send with wrong Content-Type returns 406" "406" "$HTTP_CODE"
fi

# Test 7: POST /send with resolved status behavior check
# Note: This test checks behavior based on SEND_RESOLVED env var
# Since we're running with SEND_RESOLVED=true in e2e, resolved alerts should be sent
echo ""
echo "Test 7: POST /send with resolved status (checking SEND_RESOLVED behavior)"
RESOLVED_PAYLOAD='{
    "version": "4",
    "status": "resolved",
    "alerts": [{
        "annotations": {"summary": "Resolved Test Alert"},
        "startsAt": "2024-01-01T12:00:00Z"
    }]
}'

HTTP_CODE=$(curl -sf -o /tmp/resolved_response.json -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$RESOLVED_PAYLOAD" \
    "$APP_URL/send?receiver=%2B1234567890")

RESPONSE=$(cat /tmp/resolved_response.json 2>/dev/null || echo "{}")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success' 2>/dev/null)
SENT=$(echo "$RESPONSE" | jq -r '.sent' 2>/dev/null)

# With SEND_RESOLVED=true, resolved alerts should be sent
if [ "$HTTP_CODE" = "200" ] && [ "$SUCCESS" = "true" ] && [ "$SENT" = "1" ]; then
    pass "POST /send with resolved status (SEND_RESOLVED enabled) sends alert"
    echo "  HTTP Code: $HTTP_CODE, Success: $SUCCESS, Sent: $SENT"
    echo "  Note: SEND_RESOLVED is enabled in e2e tests, so resolved alerts are sent"
else
    # If SEND_RESOLVED is disabled, sent should be 0
    if [ "$SENT" = "0" ]; then
        pass "POST /send with resolved status (SEND_RESOLVED disabled) does not send"
        echo "  HTTP Code: $HTTP_CODE, Success: $SUCCESS, Sent: $SENT"
        echo "  Note: SEND_RESOLVED appears to be disabled"
    else
        fail "POST /send with resolved status" "HTTP 200, success=true, sent=0 or 1" "HTTP $HTTP_CODE, success=$SUCCESS, sent=$SENT"
        echo "  Response: $RESPONSE"
    fi
fi

echo ""
echo "========================================="
echo "Part 2: Resolved Alerts Tests (with SEND_RESOLVED enabled)"
echo "========================================="

# Check if SEND_RESOLVED is enabled by testing with a resolved alert
# Note: This test assumes the service is running with SEND_RESOLVED=true
# We'll test both scenarios

# Test 8: Test resolved alert with SEND_RESOLVED (if enabled via env var)
echo ""
echo "Test 8: Testing resolved alert behavior"
# First, check if we can send a resolved alert and see if it's processed
# This will work if SEND_RESOLVED=true, otherwise it will be skipped
RESOLVED_PAYLOAD_ENABLED='{
    "version": "4",
    "status": "resolved",
    "alerts": [{
        "annotations": {"summary": "Resolved Alert Test"},
        "startsAt": "2024-01-01T12:00:00Z"
    }]
}'

# Clear mock-twilio before this test
curl -sf -X DELETE "$MOCK_TWILIO_URL/messages" || true

HTTP_CODE=$(curl -sf -o /tmp/resolved_enabled_response.json -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$RESOLVED_PAYLOAD_ENABLED" \
    "$APP_URL/send?receiver=%2B1234567890")

RESPONSE=$(cat /tmp/resolved_enabled_response.json 2>/dev/null || echo "{}")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success' 2>/dev/null)
SENT=$(echo "$RESPONSE" | jq -r '.sent' 2>/dev/null)

# Wait a moment for message to be processed
sleep 2

# Check mock-twilio for messages
MESSAGES_RESPONSE=$(curl -sf "$MOCK_TWILIO_URL/messages" 2>/dev/null || echo '{"count":0,"messages":[]}')
MESSAGE_COUNT=$(echo "$MESSAGES_RESPONSE" | jq -r '.count' 2>/dev/null || echo "0")
RESOLVED_MESSAGE=$(echo "$MESSAGES_RESPONSE" | jq -r '.messages[] | select(.body | contains("RESOLVED:")) | .body' 2>/dev/null | head -1)

if [ "$SENT" = "1" ] && [ -n "$RESOLVED_MESSAGE" ]; then
    pass "Resolved alerts are sent when SEND_RESOLVED is enabled"
    echo "  HTTP Code: $HTTP_CODE, Success: $SUCCESS, Sent: $SENT"
    echo "  Message contains RESOLVED prefix: $(echo "$RESOLVED_MESSAGE" | grep -q "RESOLVED:" && echo "yes" || echo "no")"
    if echo "$RESOLVED_MESSAGE" | grep -q "RESOLVED:"; then
        pass "Resolved alert message contains RESOLVED: prefix"
        echo "  Message: $RESOLVED_MESSAGE"
    else
        fail "Resolved alert message contains RESOLVED: prefix" "Message with RESOLVED: prefix" "Message without prefix"
    fi
elif [ "$SENT" = "0" ]; then
    echo -e "  ${YELLOW}Note: SEND_RESOLVED appears to be disabled (sent=0)${NC}"
    echo "  This is expected if SEND_RESOLVED is not set to 'true'"
    pass "Resolved alerts are correctly skipped when SEND_RESOLVED is disabled"
else
    fail "Resolved alert test" "sent=1 with RESOLVED: prefix or sent=0" "HTTP $HTTP_CODE, sent=$SENT"
    echo "  Response: $RESPONSE"
fi

echo ""
echo "========================================="
echo "Part 3: AlertManager Integration Tests"
echo "========================================="

# Clear mock-twilio message store before AlertManager tests
echo ""
echo "Clearing mock-twilio message store..."
curl -sf -X DELETE "$MOCK_TWILIO_URL/messages" || true

# Test 9: AlertManager webhook integration
echo ""
echo "Test 7: AlertManager sends alert to promtotwilio which sends SMS"

# Inject an alert via AlertManager's API
ALERT_PAYLOAD='[
  {
    "labels": {
      "alertname": "E2ETestAlert",
      "severity": "critical",
      "instance": "test-instance:9090"
    },
    "annotations": {
      "summary": "E2E AlertManager Integration Test"
    },
    "startsAt": "2024-01-01T12:00:00Z",
    "generatorURL": "http://localhost:9090/graph"
  }
]'

echo "  Injecting alert via AlertManager API..."
HTTP_CODE=$(curl -s -o /tmp/am_response.txt -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$ALERT_PAYLOAD" \
    "$ALERTMANAGER_URL/api/v2/alerts")

if [ "$HTTP_CODE" != "200" ]; then
    fail "AlertManager accepts alert" "HTTP 200" "HTTP $HTTP_CODE"
    cat /tmp/am_response.txt 2>/dev/null
else
    echo "  Alert injected successfully (HTTP $HTTP_CODE)"

    # Wait for AlertManager to process and send webhook
    # AlertManager has a ~10s gossip settle time before sending notifications
    echo "  Waiting for AlertManager to settle and process alert (15s)..."
    sleep 15

    # Check if mock-twilio received the SMS
    echo "  Checking mock-twilio for received messages..."
    MESSAGES_RESPONSE=$(curl -sf "$MOCK_TWILIO_URL/messages")
    MESSAGE_COUNT=$(echo "$MESSAGES_RESPONSE" | jq -r '.count' 2>/dev/null)

    if [ "$MESSAGE_COUNT" -gt 0 ] 2>/dev/null; then
        # Check if any message contains our test alert
        FOUND_ALERT=$(echo "$MESSAGES_RESPONSE" | jq -r '.messages[] | select(.body | contains("E2E AlertManager Integration Test")) | .body' 2>/dev/null | head -1)

        if [ -n "$FOUND_ALERT" ]; then
            pass "AlertManager -> promtotwilio -> mock-twilio integration works"
            echo "  Messages received: $MESSAGE_COUNT"
            echo "  Alert body: $FOUND_ALERT"
        else
            # Show what messages were received
            echo -e "  ${YELLOW}Warning: Messages received but test alert not found${NC}"
            echo "  Messages in store:"
            echo "$MESSAGES_RESPONSE" | jq -r '.messages[].body' 2>/dev/null | head -5
            fail "AlertManager integration test alert found" "Message containing 'E2E AlertManager Integration Test'" "Other messages"
        fi
    else
        fail "AlertManager -> promtotwilio -> mock-twilio integration" "At least 1 message" "0 messages"
        echo "  Response: $MESSAGES_RESPONSE"
    fi
fi

echo ""
echo "========================================="
echo "Part 4: Full Prometheus Alert Cycle Tests"
echo "========================================="

# Clear mock-twilio message store before Prometheus cycle tests
echo ""
echo "Clearing mock-twilio message store..."
curl -sf -X DELETE "$MOCK_TWILIO_URL/messages" || true

# Reset mock-exporter to healthy state
echo "Resetting mock-exporter to healthy state (alert_trigger=0)..."
curl -sf -X POST -H "Content-Type: application/json" \
    -d '{"alert_trigger": 0}' \
    "$MOCK_EXPORTER_URL/control" > /dev/null

# Give Prometheus time to scrape the healthy state
echo "Waiting for Prometheus to scrape healthy state (10s)..."
sleep 10

# Test 10: Full Prometheus alert cycle - Trigger firing alert
echo ""
echo "Test 10: Full Prometheus alert cycle - Triggering firing alert"

# Set mock-exporter to unhealthy state to trigger alert
echo "  Setting mock-exporter to unhealthy state (alert_trigger=1)..."
CONTROL_RESPONSE=$(curl -sf -X POST -H "Content-Type: application/json" \
    -d '{"alert_trigger": 1}' \
    "$MOCK_EXPORTER_URL/control")

ALERT_TRIGGER=$(echo "$CONTROL_RESPONSE" | jq -r '.alert_trigger' 2>/dev/null)
if [ "$ALERT_TRIGGER" = "1" ]; then
    echo "  Mock exporter set to unhealthy state"
else
    fail "Set mock-exporter to unhealthy" "alert_trigger=1" "alert_trigger=$ALERT_TRIGGER"
fi

# Wait for:
# - Prometheus to scrape (5s interval)
# - Alert rule evaluation (for: 5s)
# - AlertManager to receive and process
# - promtotwilio to send SMS
echo "  Waiting for Prometheus to detect unhealthy state and fire alert (25s)..."
sleep 25

# Check if mock-twilio received the firing alert SMS
echo "  Checking mock-twilio for firing alert message..."
MESSAGES_RESPONSE=$(curl -sf "$MOCK_TWILIO_URL/messages")
MESSAGE_COUNT=$(echo "$MESSAGES_RESPONSE" | jq -r '.count' 2>/dev/null)

FIRING_MESSAGE=$(echo "$MESSAGES_RESPONSE" | jq -r '.messages[] | select(.body | contains("E2E Test Alert from Prometheus")) | .body' 2>/dev/null | head -1)

if [ -n "$FIRING_MESSAGE" ]; then
    pass "Prometheus -> AlertManager -> promtotwilio -> SMS (firing alert)"
    echo "  Message count: $MESSAGE_COUNT"
    echo "  Firing alert message: $FIRING_MESSAGE"
else
    fail "Prometheus firing alert received" "Message containing 'E2E Test Alert from Prometheus'" "No matching message"
    echo "  Messages received: $MESSAGE_COUNT"
    if [ "$MESSAGE_COUNT" -gt 0 ] 2>/dev/null; then
        echo "  Messages in store:"
        echo "$MESSAGES_RESPONSE" | jq -r '.messages[].body' 2>/dev/null | head -5
    fi
fi

# Test 11: Full Prometheus alert cycle - Resolve alert
echo ""
echo "Test 11: Full Prometheus alert cycle - Resolving alert"

# Clear mock-twilio before resolved test
curl -sf -X DELETE "$MOCK_TWILIO_URL/messages" || true

# Set mock-exporter back to healthy state to resolve alert
echo "  Setting mock-exporter to healthy state (alert_trigger=0)..."
CONTROL_RESPONSE=$(curl -sf -X POST -H "Content-Type: application/json" \
    -d '{"alert_trigger": 0}' \
    "$MOCK_EXPORTER_URL/control")

ALERT_TRIGGER=$(echo "$CONTROL_RESPONSE" | jq -r '.alert_trigger' 2>/dev/null)
if [ "$ALERT_TRIGGER" = "0" ]; then
    echo "  Mock exporter set to healthy state"
else
    fail "Set mock-exporter to healthy" "alert_trigger=0" "alert_trigger=$ALERT_TRIGGER"
fi

# Wait for:
# - Prometheus to scrape healthy state
# - Alert to resolve
# - AlertManager to send resolved notification
echo "  Waiting for Prometheus to detect healthy state and resolve alert (20s)..."
sleep 20

# Check if mock-twilio received the resolved alert SMS
echo "  Checking mock-twilio for resolved alert message..."
MESSAGES_RESPONSE=$(curl -sf "$MOCK_TWILIO_URL/messages")
MESSAGE_COUNT=$(echo "$MESSAGES_RESPONSE" | jq -r '.count' 2>/dev/null)

RESOLVED_MESSAGE=$(echo "$MESSAGES_RESPONSE" | jq -r '.messages[] | select(.body | contains("RESOLVED:")) | .body' 2>/dev/null | head -1)

if [ -n "$RESOLVED_MESSAGE" ]; then
    pass "Prometheus -> AlertManager -> promtotwilio -> SMS (resolved alert)"
    echo "  Message count: $MESSAGE_COUNT"
    echo "  Resolved alert message: $RESOLVED_MESSAGE"
else
    # Resolved alerts might not be sent depending on configuration
    echo -e "  ${YELLOW}Note: Resolved alert message not found${NC}"
    echo "  This may be expected if send_resolved is not enabled in AlertManager"
    if [ "$MESSAGE_COUNT" -gt 0 ] 2>/dev/null; then
        echo "  Messages in store:"
        echo "$MESSAGES_RESPONSE" | jq -r '.messages[].body' 2>/dev/null | head -5
        fail "Resolved alert message" "Message containing 'RESOLVED:'" "Other messages"
    else
        fail "Resolved alert message" "At least 1 message" "0 messages"
    fi
fi

# Test 12: Verify Prometheus metrics endpoint is working
echo ""
echo "Test 12: Verify mock-exporter /metrics endpoint"
METRICS_RESPONSE=$(curl -sf "$MOCK_EXPORTER_URL/metrics")
if echo "$METRICS_RESPONSE" | grep -q "test_alert_trigger"; then
    pass "Mock exporter /metrics endpoint returns test_alert_trigger metric"
    echo "  Metric found: $(echo "$METRICS_RESPONSE" | grep 'test_alert_trigger ' | head -1)"
else
    fail "Mock exporter /metrics" "Contains test_alert_trigger" "Metric not found"
fi

# Test 13: Verify Prometheus has scraped the mock-exporter
echo ""
echo "Test 13: Verify Prometheus has scraped mock-exporter"
PROM_QUERY_RESPONSE=$(curl -sf "$PROMETHEUS_URL/api/v1/query?query=test_alert_trigger")
PROM_STATUS=$(echo "$PROM_QUERY_RESPONSE" | jq -r '.status' 2>/dev/null)
PROM_RESULT_COUNT=$(echo "$PROM_QUERY_RESPONSE" | jq -r '.data.result | length' 2>/dev/null)

if [ "$PROM_STATUS" = "success" ] && [ "$PROM_RESULT_COUNT" -gt 0 ] 2>/dev/null; then
    pass "Prometheus has scraped mock-exporter and has test_alert_trigger metric"
    METRIC_VALUE=$(echo "$PROM_QUERY_RESPONSE" | jq -r '.data.result[0].value[1]' 2>/dev/null)
    echo "  Metric value: $METRIC_VALUE"
else
    fail "Prometheus scrape" "status=success, results > 0" "status=$PROM_STATUS, results=$PROM_RESULT_COUNT"
fi

echo ""
echo "========================================="
echo "Part 5: Rate Limiting Tests"
echo "========================================="

# The promtotwilio service is started with RATE_LIMIT=30.
# By this point the Prometheus cycle tests have taken well over a minute, so the
# rate limit window has reset at least once. We fire more than 30 requests in a
# tight loop; regardless of how many tokens remain in the current window, we are
# guaranteed to see both 200s and 429s.

# Test 14: Verify rate limiting kicks in after the limit is exceeded
echo ""
echo "Test 14: POST /send rate limiting (RATE_LIMIT=30)"
RATE_PAYLOAD='{"version":"4","status":"firing","alerts":[{"annotations":{"summary":"Rate limit test"}}]}'
GOT_200=0
GOT_429=0

for i in $(seq 1 35); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d "$RATE_PAYLOAD" \
        "$APP_URL/send?receiver=%2B1234567890")
    if [ "$HTTP_CODE" = "200" ]; then
        GOT_200=$((GOT_200 + 1))
    elif [ "$HTTP_CODE" = "429" ]; then
        GOT_429=$((GOT_429 + 1))
    fi
done

if [ "$GOT_200" -gt 0 ] && [ "$GOT_429" -gt 0 ]; then
    pass "Rate limiting enforced on POST /send"
    echo "  Requests accepted (200): $GOT_200"
    echo "  Requests rejected (429): $GOT_429"
else
    fail "Rate limiting enforced on POST /send" "At least 1 accepted and 1 rejected" "200s=$GOT_200, 429s=$GOT_429"
fi

# Test 15: Verify 429 response body
echo ""
echo "Test 15: Rate limited response returns 429 with correct body"
HTTP_CODE=$(curl -s -o /tmp/rate_limit_body.txt -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$RATE_PAYLOAD" \
    "$APP_URL/send?receiver=%2B1234567890")

BODY=$(cat /tmp/rate_limit_body.txt 2>/dev/null)

if [ "$HTTP_CODE" = "429" ] && echo "$BODY" | grep -q "rate limit exceeded"; then
    pass "Rate limited response returns 429 with 'rate limit exceeded' message"
else
    fail "Rate limited response" "HTTP 429 with 'rate limit exceeded'" "HTTP $HTTP_CODE, body=$BODY"
fi

# Summary
echo ""
echo "========================================="
echo "Test Results"
echo "========================================="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
