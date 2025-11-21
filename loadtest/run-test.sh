#!/bin/bash

# Load test script with automated validation
# This script runs comprehensive load tests and validates results

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_URL="${TARGET_URL:-http://localhost:8000/hello}"
METRICS_URL="${METRICS_URL:-http://localhost:8080/metrics}"
DURATION="${DURATION:-60}"
CONCURRENCY="${CONCURRENCY:-100}"
RPS="${RPS:-1000}"

echo "========================================"
echo "Rate Limiter Load Test Suite"
echo "========================================"
echo ""
echo "Configuration:"
echo "  Target URL: $TARGET_URL"
echo "  Metrics URL: $METRICS_URL"
echo "  Duration: ${DURATION}s"
echo "  Concurrency: $CONCURRENCY"
echo "  Target RPS: $RPS"
echo ""

# Check if target is reachable
echo "Checking target availability..."
if ! curl -f -s "$TARGET_URL" > /dev/null; then
    echo "Error: Target $TARGET_URL is not reachable"
    exit 1
fi
echo "✓ Target is reachable"

# Check if metrics endpoint is available
echo "Checking metrics endpoint..."
if ! curl -f -s "$METRICS_URL" > /dev/null; then
    echo "Error: Metrics endpoint $METRICS_URL is not available"
    exit 1
fi
echo "✓ Metrics endpoint is available"

echo ""
echo "Starting load test..."
echo ""

# Run the load test
cd "$SCRIPT_DIR"
go run main.go \
    -target="$TARGET_URL" \
    -metrics="$METRICS_URL" \
    -duration="${DURATION}s" \
    -concurrency="$CONCURRENCY" \
    -rps="$RPS" \
    -validate=true

echo ""
echo "========================================"
echo "Load test completed successfully!"
echo "========================================"
