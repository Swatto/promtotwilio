#!/bin/bash
set -e

APP_URL="${APP_URL:-http://promtotwilio:9090}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://alertmanager:9093}"
MOCK_TWILIO_URL="${MOCK_TWILIO_URL:-http://mock-twilio:8080}"
MAX_RETRIES=30
RETRY_INTERVAL=1

echo "========================================="
echo "E2E Tests for promtotwilio"
echo "========================================="
echo "App URL: $APP_URL"
echo "AlertManager URL: $ALERTMANAGER_URL"
echo "Mock Twilio URL: $MOCK_TWILIO_URL"
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

echo ""
echo "========================================="
echo "Part 2: AlertManager Integration Tests"
echo "========================================="

# Clear mock-twilio message store before AlertManager tests
echo ""
echo "Clearing mock-twilio message store..."
curl -sf -X DELETE "$MOCK_TWILIO_URL/messages" || true

# Test 7: AlertManager webhook integration
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
