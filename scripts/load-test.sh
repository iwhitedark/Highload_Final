#!/bin/bash
# Load testing script for Highload Service
# Supports Apache Bench (ab) and Locust

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

HOST="${1:-http://localhost:8080}"
DURATION="${2:-300}"  # 5 minutes default
RPS="${3:-500}"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Highload Service Load Test${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "Host: ${HOST}"
echo -e "Duration: ${DURATION}s"
echo -e "Target RPS: ${RPS}"
echo -e "${GREEN}========================================${NC}"

# Generate test data
generate_metric() {
    CPU=$(echo "scale=2; 30 + ($RANDOM % 40)" | bc)
    RPS_VAL=$(echo "scale=2; 300 + ($RANDOM % 400)" | bc)
    echo "{\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"cpu\":${CPU},\"rps\":${RPS_VAL}}"
}

# Test with Apache Bench
test_with_ab() {
    echo -e "\n${YELLOW}Running Apache Bench test...${NC}"

    # Create temporary file with JSON data
    TMPFILE=$(mktemp)
    generate_metric > "$TMPFILE"

    # Calculate requests based on duration and target RPS
    TOTAL_REQUESTS=$((RPS * DURATION))
    CONCURRENCY=$((RPS / 10))
    [ $CONCURRENCY -lt 10 ] && CONCURRENCY=10
    [ $CONCURRENCY -gt 100 ] && CONCURRENCY=100

    echo "Total requests: ${TOTAL_REQUESTS}"
    echo "Concurrency: ${CONCURRENCY}"

    # Run ab test
    ab -n "$TOTAL_REQUESTS" \
       -c "$CONCURRENCY" \
       -T "application/json" \
       -p "$TMPFILE" \
       "${HOST}/metrics"

    rm -f "$TMPFILE"
}

# Test with curl (simple)
test_with_curl() {
    echo -e "\n${YELLOW}Running curl load test...${NC}"

    START_TIME=$(date +%s)
    END_TIME=$((START_TIME + DURATION))
    REQUEST_COUNT=0
    ERROR_COUNT=0
    TOTAL_LATENCY=0

    echo "Sending requests for ${DURATION} seconds..."

    while [ $(date +%s) -lt $END_TIME ]; do
        DATA=$(generate_metric)
        START_REQ=$(date +%s%N)

        RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null \
            -X POST "${HOST}/metrics" \
            -H "Content-Type: application/json" \
            -d "$DATA" 2>/dev/null)

        END_REQ=$(date +%s%N)
        LATENCY=$(((END_REQ - START_REQ) / 1000000))  # in ms

        if [ "$RESPONSE" = "200" ]; then
            REQUEST_COUNT=$((REQUEST_COUNT + 1))
            TOTAL_LATENCY=$((TOTAL_LATENCY + LATENCY))
        else
            ERROR_COUNT=$((ERROR_COUNT + 1))
        fi

        # Rate limiting
        sleep 0.001
    done

    ACTUAL_DURATION=$(($(date +%s) - START_TIME))
    ACTUAL_RPS=$((REQUEST_COUNT / ACTUAL_DURATION))
    AVG_LATENCY=$((TOTAL_LATENCY / REQUEST_COUNT))

    echo -e "\n${GREEN}Results:${NC}"
    echo "Total requests: ${REQUEST_COUNT}"
    echo "Errors: ${ERROR_COUNT}"
    echo "Duration: ${ACTUAL_DURATION}s"
    echo "Actual RPS: ${ACTUAL_RPS}"
    echo "Average latency: ${AVG_LATENCY}ms"
}

# Test with Locust
test_with_locust() {
    echo -e "\n${YELLOW}Running Locust test...${NC}"

    SCRIPT_DIR="$(dirname "$0")"

    if command -v locust >/dev/null 2>&1; then
        locust -f "${SCRIPT_DIR}/locustfile.py" \
            --host="${HOST}" \
            --users="${RPS}" \
            --spawn-rate=50 \
            --run-time="${DURATION}s" \
            --headless \
            --only-summary
    else
        echo "Locust not installed. Install with: pip install locust"
        echo "Running curl test instead..."
        test_with_curl
    fi
}

# Health check
health_check() {
    echo -e "\n${YELLOW}Health Check:${NC}"
    curl -s "${HOST}/health" | python3 -m json.tool 2>/dev/null || curl -s "${HOST}/health"
}

# Get analysis
get_analysis() {
    echo -e "\n${YELLOW}Current Analysis:${NC}"
    curl -s "${HOST}/analyze" | python3 -m json.tool 2>/dev/null || curl -s "${HOST}/analyze"
}

# Main
main() {
    health_check

    case "${4:-locust}" in
        "ab")
            test_with_ab
            ;;
        "curl")
            test_with_curl
            ;;
        "locust"|*)
            test_with_locust
            ;;
    esac

    get_analysis

    echo -e "\n${GREEN}========================================${NC}"
    echo -e "${GREEN}Load Test Complete${NC}"
    echo -e "${GREEN}========================================${NC}"
}

main "$@"
