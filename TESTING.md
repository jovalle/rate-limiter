# Testing Guide

This document describes the testing strategy and procedures for the rate limiter project.

## Test Types

### 1. Unit Tests (`main_test.go`)

Unit tests validate individual functions and components in isolation.

**Run unit tests:**
```bash
make test-unit
# or
go test -v -short ./...
```

**Coverage:**
- `reverseByteOrder()` - IP address byte order conversion
- `boolToFloat64()` - Boolean to float conversion for metrics
- `newMetrics()` - Prometheus metrics initialization
- Struct definitions (PortKey, PacketState, config)

**Example:**
```go
func TestReverseByteOrder(t *testing.T) {
    result := reverseByteOrder(0x0100007F)
    expected := "127.0.0.1"
    if result.String() != expected {
        t.Errorf("Expected %s, got %s", expected, result)
    }
}
```

### 2. Integration Tests (`integration_test.go`)

Integration tests validate the complete system working together.

**Run integration tests:**
```bash
make test-integration
# or
go test -v -run Integration ./...
```

**Coverage:**
- Health check endpoint validation
- Web server integration
- Load pattern handling
- Metrics format validation
- Configuration parsing

**Note:** Integration tests require the test web server and are skipped when `SKIP_INTEGRATION_TESTS` environment variable is set.

### 3. Load Tests (`loadtest/`)

Automated load testing suite validates performance under stress.

**Run load tests:**
```bash
# Default configuration
make loadtest

# Custom configuration
cd loadtest
TARGET_URL=http://your-host:8000/hello \
METRICS_URL=http://your-host:8080/metrics \
DURATION=120 \
CONCURRENCY=200 \
RPS=2000 \
./run-test.sh
```

**Features:**
- Configurable duration, concurrency, and RPS
- Automatic metrics validation
- Success/failure tracking
- Rate limiting detection

**Expected Results:**
```
Total Requests:     60000
Success:            58000 (96.67%)
Failed:             0 (0.00%)
Rate Limited:       2000 (3.33%)
Duration:           60s
Requests/sec:       1000.00
```

### 4. Benchmarks

Performance benchmarks for critical code paths.

**Run benchmarks:**
```bash
go test -bench=. -benchmem ./...
```

**Benchmarks:**
- `BenchmarkReverseByteOrder` - IP conversion performance
- `BenchmarkBoolToFloat64` - Metric conversion performance

**Example output:**
```
BenchmarkReverseByteOrder-8     50000000    25.3 ns/op    16 B/op    1 allocs/op
BenchmarkBoolToFloat64-8       2000000000   0.25 ns/op    0 B/op    0 allocs/op
```

## Test Requirements

### Prerequisites

```bash
# Install dependencies
go mod download

# Install testing tools (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### eBPF Generation

Tests that don't require eBPF functionality can run without generation:
```bash
SKIP_INTEGRATION_TESTS=1 go test -v -short ./...
```

For full testing including eBPF:
```bash
# Install eBPF dependencies
sudo apt-get install -y libbpf-dev libelf-dev clang llvm

# Generate eBPF code
make generate

# Run all tests
make test
```

## Testing in CI/CD

The GitHub Actions workflow (`.github/workflows/ci.yaml`) automatically:

1. **Unit Tests**: Runs on every push/PR
   ```yaml
   - name: Run tests
     run: go test -v -race -coverprofile=coverage.out ./...
   ```

2. **Linting**: Validates code quality
   ```yaml
   - name: golangci-lint
     uses: golangci/golangci-lint-action@v3
   ```

3. **Build**: Ensures code compiles
   ```yaml
   - name: Build binary
     run: go build -v -o rate-limiter .
   ```

4. **Docker**: Builds and validates container
   ```yaml
   - name: Build Docker image
     uses: docker/build-push-action@v5
   ```

5. **Helm**: Validates Kubernetes manifests
   ```yaml
   - name: Lint Helm chart
     run: helm lint helm/rate-limiter
   ```

## Test Coverage

### Current Coverage

Run coverage analysis:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

View coverage report in browser or terminal:
```bash
go tool cover -func=coverage.out
```

### Coverage Goals

- **Unit Tests**: 80%+ coverage of non-eBPF code
- **Integration Tests**: Cover all HTTP endpoints
- **Load Tests**: Validate under expected production load

## Writing Tests

### Unit Test Template

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected OutputType
    }{
        {
            name:     "description",
            input:    testInput,
            expected: expectedOutput,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := FunctionName(tt.input)
            if result != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

### Integration Test Template

```go
func TestIntegrationFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
        t.Skip("Skipping integration tests")
    }

    // Test implementation
    // ...
}
```

### Benchmark Template

```go
func BenchmarkFunctionName(b *testing.B) {
    // Setup
    testData := prepareTestData()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        FunctionName(testData)
    }
}
```

## Load Testing Strategy

### Test Scenarios

1. **Baseline Test**
   - Purpose: Establish baseline performance
   - Duration: 60s
   - Concurrency: 100
   - RPS: 1000
   - Expected: All requests succeed

2. **Sustained Load Test**
   - Purpose: Validate sustained rate limiting
   - Duration: 300s
   - Concurrency: 200
   - RPS: 4000 (above limit)
   - Expected: Rate limiting activates, drops visible in metrics

3. **Burst Test**
   - Purpose: Validate burst handling
   - Duration: 10s
   - Concurrency: 1000
   - RPS: 20000
   - Expected: Burst bucket absorbs spike, then rate limits

4. **Recovery Test**
   - Purpose: Validate token replenishment
   - Phases:
     1. Send high load for 60s
     2. Pause for 120s
     3. Send normal load for 60s
   - Expected: Tokens refill during pause, normal load succeeds

### Load Test Tools

**Included Load Tester:**
```bash
cd loadtest
go run main.go -target=http://localhost:8000/hello -duration=60s -concurrency=100 -rps=1000
```

**External Tools:**

1. **wrk** (Recommended for HTTP load testing)
   ```bash
   wrk -t16 -c1000 -d60s http://localhost:8000/hello
   ```

2. **vegeta**
   ```bash
   echo "GET http://localhost:8000/hello" | vegeta attack -duration=60s -rate=1000 | vegeta report
   ```

3. **hey**
   ```bash
   hey -z 60s -c 100 -q 1000 http://localhost:8000/hello
   ```

## Metrics Validation

### Expected Metrics

The rate limiter should expose these metrics:

```
# HELP rate_limited If the connection is being rate limited
# TYPE rate_limited gauge
rate_limited{connection="192.168.1.1:54321",network_interface="eth0"} 0

# HELP rate_limited_drops Total number of dropped packets by connection
# TYPE rate_limited_drops gauge
rate_limited_drops{connection="192.168.1.1:54321",network_interface="eth0"} 1234

# HELP rate_limited_tokens Available tokens
# TYPE rate_limited_tokens gauge
rate_limited_tokens{connection="192.168.1.1:54321",network_interface="eth0"} 5000
```

### Validation Checks

```bash
# All expected metrics present
curl -s http://localhost:8080/metrics | grep -c "rate_limited" # Should be > 0

# Health endpoint responds
curl -f http://localhost:8080/health # Should return 200 OK

# Readiness endpoint responds
curl -f http://localhost:8080/ready # Should return 200 READY

# Metrics are valid Prometheus format
curl -s http://localhost:8080/metrics | promtool check metrics
```

## Debugging Tests

### Verbose Output

```bash
go test -v ./...
```

### Run Specific Test

```bash
go test -v -run TestFunctionName ./...
```

### Enable Race Detection

```bash
go test -race ./...
```

### View Detailed Logs

```bash
LOG_LEVEL=debug go test -v ./...
```

### Debug Integration Tests

```bash
# Start test web server manually
go run web/main.go &

# Run specific integration test
go test -v -run TestIntegrationHealthCheck ./...

# Kill web server
pkill -f "go run web/main.go"
```

## Continuous Testing

### Pre-commit Hooks

Add to `.git/hooks/pre-commit`:
```bash
#!/bin/bash
make test-short
make lint
```

### Watch Mode

Use `entr` or similar tool for continuous testing:
```bash
# Install entr
sudo apt-get install entr

# Watch for changes and run tests
find . -name '*.go' | entr -c make test-short
```

## Test Maintenance

### Updating Tests

When changing code:
1. Update relevant unit tests
2. Verify integration tests still pass
3. Update load test expectations if needed
4. Run full test suite: `make test`

### Adding New Tests

1. Create test file: `*_test.go`
2. Follow naming conventions: `Test*`, `Benchmark*`, `Example*`
3. Add to appropriate test category
4. Update this documentation

## Known Test Limitations

1. **eBPF Tests**: Full eBPF functionality tests require Linux kernel and dependencies
2. **Performance Tests**: Results vary by hardware
3. **Network Tests**: Some tests require actual network interfaces
4. **Privilege Tests**: XDP attachment tests require root/CAP_NET_ADMIN

## Test Results Archive

Store test results for tracking:
```bash
# Run tests with output
go test -v ./... 2>&1 | tee test-results-$(date +%Y%m%d-%H%M%S).log

# Generate coverage HTML
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage-$(date +%Y%m%d).html
```

## Troubleshooting

### Tests Fail: eBPF Files Not Found

```bash
# Generate eBPF code first
make generate
```

### Tests Fail: Port Already in Use

```bash
# Kill processes using test ports
sudo lsof -ti:8000,8080 | xargs kill -9
```

### Load Tests Fail: Too Many Open Files

```bash
# Increase file descriptor limit
ulimit -n 100000
```

### Integration Tests Hang

```bash
# Set shorter timeout
SKIP_INTEGRATION_TESTS=1 go test ./...
```

## References

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table-Driven Tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Advanced Go Testing](https://www.gophercon.com/agenda/session/27503)
