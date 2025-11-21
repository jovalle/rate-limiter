# Rate Limiter - Production Deployment Guide

High-performance eBPF-based packet rate limiter using XDP (eXpress Data Path) for in-kernel packet filtering.

## Overview

This rate limiter provides:
- **High Performance**: Processes packets in kernel space using eBPF/XDP
- **Token Bucket Algorithm**: Configurable burst limits and sustained rates
- **Prometheus Metrics**: Real-time visibility into rate limiting behavior
- **Production Ready**: Kubernetes/Helm deployment, health checks, comprehensive testing

## Features

### Core Features
- eBPF/XDP-based packet filtering for minimal latency
- Per-connection rate limiting (by source IP and port)
- Token bucket algorithm with configurable parameters
- Support for TCP and UDP protocols
- IPv4 packet processing

### Observability
- **Prometheus Metrics**:
  - `rate_limited{connection, network_interface}` - Rate limiting status
  - `rate_limited_drops{connection, network_interface}` - Dropped packets per connection
  - `rate_limited_tokens{connection, network_interface}` - Available tokens per connection
- **Health Endpoints**:
  - `/health` - Liveness probe
  - `/ready` - Readiness probe
  - `/metrics` - Prometheus-formatted metrics

### Deployment Options
- **Docker**: Production-ready multi-stage Dockerfile
- **Kubernetes**: DaemonSet deployment with ConfigMap
- **Helm**: Customizable Helm chart for easy deployment
- **CI/CD**: GitHub Actions pipeline for automated testing

### Testing
- **Unit Tests**: Comprehensive test coverage for core functions
- **Integration Tests**: End-to-end validation with test web server
- **Load Tests**: Automated load testing with metrics validation
- **Benchmarks**: Performance benchmarks for critical paths

## Quick Start

### Prerequisites

```bash
# Install eBPF dependencies (Ubuntu/Debian)
sudo apt-get update
sudo apt-get install -y libbpf-dev libelf-dev clang llvm make pkg-config

# Verify kernel version (4.18+ required, 5.4+ recommended)
uname -r
```

### Local Development

```bash
# Clone and build
git clone https://github.com/jovalle/rate-limiter.git
cd rate-limiter
make compile

# Run rate limiter (requires root for eBPF/XDP)
sudo INTERFACE=eth0 LOG_LEVEL=info ./rate-limiter

# In another terminal, run test web app
make testapp

# View metrics
curl http://localhost:8080/metrics
curl http://localhost:8080/health
```

### Docker Deployment

```bash
# Build image
make docker-build

# Run container
docker run -d \
  --name rate-limiter \
  --privileged \
  --network host \
  -e INTERFACE=eth0 \
  -e LOG_LEVEL=info \
  rate-limiter:latest

# Check logs
docker logs -f rate-limiter

# View metrics
curl http://localhost:8080/metrics
```

### Kubernetes Deployment

```bash
# Deploy with kubectl
kubectl apply -f k8s/

# Or deploy with Helm
helm install rate-limiter ./helm/rate-limiter \
  --namespace rate-limiter \
  --create-namespace

# Check status
kubectl get pods -n rate-limiter
kubectl logs -n rate-limiter -l app=rate-limiter

# Port-forward to view metrics
kubectl port-forward -n rate-limiter daemonset/rate-limiter 8080:8080
curl http://localhost:8080/metrics
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INTERFACE` | `ens33` | Network interface to attach XDP program |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |

### Rate Limiting Parameters

Set at compile time in `main.go`:

```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go rate_limiter rate_limiter.c -- \
  -DPACKET_BURST_LIMIT=10000 \              // Maximum burst size
  -DPACKETS_PER_SECOND=3200 \               // Sustained rate limit
  -DPACKET_BURST_REPLENISH_SECONDS=600      // Burst refill interval
```

## Testing

### Run Unit Tests

```bash
# Run all tests
make test

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...

# Run only fast tests
make test-short

# Run integration tests
make test-integration
```

### Load Testing

```bash
# Run automated load test suite
make loadtest

# Or run with custom parameters
cd loadtest
TARGET_URL=http://localhost:8000/hello \
METRICS_URL=http://localhost:8080/metrics \
DURATION=60 \
CONCURRENCY=100 \
RPS=1000 \
./run-test.sh
```

### Manual Load Testing

```bash
# Install wrk
sudo apt-get install wrk

# Run load test
ulimit -n 100000
wrk -t16 -c10000 -d60s http://localhost:8000/hello

# Monitor metrics during test
watch -n 1 'curl -s http://localhost:8080/metrics | grep rate_limited'
```

## Monitoring

### Prometheus Setup

Add to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'rate-limiter'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8080']
```

For Kubernetes with Prometheus Operator:
```bash
kubectl apply -f k8s/servicemonitor.yaml
```

### Grafana Dashboard

Import the pre-configured dashboard:

```bash
# Dashboard is available at monitoring/dashboard.json
# Import into Grafana UI or use:
curl -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @monitoring/dashboard.json
```

### Key Metrics to Monitor

```promql
# Active rate-limited connections
rate_limited > 0

# Packet drop rate
rate(rate_limited_drops[5m])

# Available tokens (low values indicate throttling)
rate_limited_tokens < 100

# Service health
up{job="rate-limiter"}
```

## Architecture

### Rate Limiting Logic

1. **Packet Arrival**: XDP program intercepts packets at network interface
2. **Validation**: Checks for valid IPv4 TCP/UDP packets
3. **Connection Tracking**: Looks up source IP:Port in eBPF map
4. **Token Bucket**:
   - Each connection has a token bucket
   - Tokens refill at configured rate (PACKETS_PER_SECOND)
   - Burst capacity limited by PACKET_BURST_LIMIT
   - Full refill every PACKET_BURST_REPLENISH_SECONDS
5. **Decision**: XDP_PASS if tokens available, XDP_DROP otherwise
6. **Metrics**: Updates per-connection state and exposes via Prometheus

### Components

```
┌─────────────────────────────────────────┐
│         Network Interface (XDP)         │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │   eBPF/XDP Program (Kernel)       │ │
│  │   - Packet filtering              │ │
│  │   - Token bucket algorithm        │ │
│  │   - Connection state tracking     │ │
│  └───────────────────────────────────┘ │
└─────────────────────────────────────────┘
                   │
                   ↓
┌─────────────────────────────────────────┐
│    Rate Limiter Service (User Space)   │
│                                         │
│  ┌─────────────────┐  ┌──────────────┐ │
│  │ Metrics Reader  │  │ HTTP Server  │ │
│  │ - Read eBPF map │  │ - /metrics   │ │
│  │ - Update metrics│  │ - /health    │ │
│  │                 │  │ - /ready     │ │
│  └─────────────────┘  └──────────────┘ │
└─────────────────────────────────────────┘
                   │
                   ↓
┌─────────────────────────────────────────┐
│            Prometheus                   │
│         (Metrics Storage)               │
└─────────────────────────────────────────┘
                   │
                   ↓
┌─────────────────────────────────────────┐
│             Grafana                     │
│         (Visualization)                 │
└─────────────────────────────────────────┘
```

## Performance

### Expected Performance

- **Throughput**: > 10M packets/second (hardware dependent)
- **Latency**: < 1ms p99 packet processing
- **CPU Usage**: < 10% per core under normal load
- **Memory**: < 256MB per instance
- **Connections**: Up to 65,536 tracked connections (configurable)

### Benchmarks

```bash
# Run benchmarks
go test -bench=. -benchmem ./...
```

## Production Deployment

For comprehensive production deployment instructions, see:

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Complete deployment guide
  - Docker, Kubernetes, and Helm deployment
  - Configuration options
  - Monitoring setup
  - Load testing
  - Troubleshooting

- **[RUNBOOK.md](RUNBOOK.md)** - Operations runbook
  - Common operations
  - Incident response procedures
  - Maintenance tasks
  - Performance tuning
  - Alerting guidelines

## CI/CD

GitHub Actions workflow automatically:
- Runs unit and integration tests
- Performs linting with golangci-lint
- Builds Docker images
- Validates Helm charts
- Uploads artifacts

Workflow is triggered on:
- Push to main/develop branches
- Pull requests
- Release creation

## Development

### Building from Source

```bash
# Install dependencies
make dev-setup

# Generate eBPF code
make generate

# Build binary
make build

# Run tests
make test

# Lint code
make lint
```

### Project Structure

```
.
├── main.go                  # Main application
├── rate_limiter.c          # eBPF/XDP program
├── main_test.go            # Unit tests
├── integration_test.go     # Integration tests
├── Dockerfile              # Multi-stage production build
├── Makefile                # Build automation
├── k8s/                    # Kubernetes manifests
├── helm/                   # Helm chart
├── loadtest/               # Load testing suite
├── monitoring/             # Grafana dashboard, Prometheus config
├── .github/workflows/      # CI/CD pipelines
├── DEPLOYMENT.md           # Deployment guide
└── RUNBOOK.md             # Operations runbook
```

## Security

### Capabilities Required

The rate limiter requires the following Linux capabilities:
- `CAP_NET_ADMIN` - Attach XDP programs to network interfaces
- `CAP_SYS_ADMIN` - Load eBPF programs
- `CAP_BPF` - eBPF operations (kernel 5.8+)

### Running Securely

```bash
# Instead of --privileged, use specific capabilities
docker run -d \
  --cap-add=NET_ADMIN \
  --cap-add=SYS_ADMIN \
  --cap-add=BPF \
  --network host \
  rate-limiter:latest
```

In Kubernetes, capabilities are configured in the DaemonSet securityContext.

## Troubleshooting

### Common Issues

**Issue**: `pattern rate_limiter_bpfel.o: no matching files found`
- **Solution**: Run `make generate` to generate eBPF code first

**Issue**: XDP program fails to attach
- **Solution**: Verify interface exists and supports XDP: `ip link show <interface>`

**Issue**: Permission denied
- **Solution**: Run with sudo or ensure proper capabilities are granted

**Issue**: No metrics appearing
- **Solution**: Check that metrics endpoint is accessible: `curl http://localhost:8080/metrics`

For more troubleshooting, see [DEPLOYMENT.md](DEPLOYMENT.md#troubleshooting).

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run linter: `make lint`
6. Run tests: `make test`
7. Submit a pull request

## License

[License information to be added]

## References

- [eBPF Documentation](https://ebpf.io/)
- [XDP Tutorial](https://github.com/xdp-project/xdp-tutorial)
- [Cilium eBPF Library](https://github.com/cilium/ebpf)
- [Token Bucket Algorithm](https://en.wikipedia.org/wiki/Token_bucket)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)

## Support

- **Issues**: https://github.com/jovalle/rate-limiter/issues
- **Documentation**: See [DEPLOYMENT.md](DEPLOYMENT.md) and [RUNBOOK.md](RUNBOOK.md)
