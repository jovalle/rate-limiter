# Deployment Guide

This guide provides comprehensive instructions for deploying the eBPF-based rate limiter in production environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Helm Deployment](#helm-deployment)
- [Configuration](#configuration)
- [Monitoring](#monitoring)
- [Load Testing](#load-testing)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

- Linux kernel version 4.18 or later (5.4+ recommended)
- x86_64 or ARM64 architecture
- Network interface with XDP support
- Minimum 2 CPU cores
- Minimum 4GB RAM

### Software Requirements

- Docker 20.10+ (for containerized deployment)
- Kubernetes 1.24+ (for k8s deployment)
- Helm 3.x (for Helm deployment)
- Go 1.21+ (for building from source)

### eBPF Build Dependencies

If building from source:

```bash
sudo apt-get update
sudo apt-get install -y \
    libbpf-dev \
    libelf-dev \
    clang \
    llvm \
    make \
    pkg-config
```

## Quick Start

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/jovalle/rate-limiter.git
cd rate-limiter
```

2. Build and run:
```bash
make compile
sudo ./rate-limiter
```

The rate limiter will:
- Attach to the default network interface (configured via `INTERFACE` env var)
- Expose metrics on port 8080
- Start rate limiting based on configured parameters

## Docker Deployment

### Build Docker Image

```bash
docker build -t rate-limiter:latest .
```

### Run Container

```bash
docker run -d \
  --name rate-limiter \
  --privileged \
  --network host \
  -e INTERFACE=eth0 \
  -e LOG_LEVEL=info \
  rate-limiter:latest
```

**Important Notes:**
- `--privileged` is required for eBPF/XDP operations
- `--network host` is required to access the host's network interfaces
- Alternative to `--privileged`: Use `--cap-add=NET_ADMIN --cap-add=SYS_ADMIN --cap-add=BPF`

### Docker Compose

```yaml
version: '3.8'
services:
  rate-limiter:
    image: rate-limiter:latest
    privileged: true
    network_mode: host
    environment:
      - INTERFACE=eth0
      - LOG_LEVEL=info
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
```

## Kubernetes Deployment

### Using kubectl

1. Apply the manifests:
```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/daemonset.yaml
kubectl apply -f k8s/service.yaml
```

2. Verify deployment:
```bash
kubectl get pods -n rate-limiter
kubectl logs -n rate-limiter -l app=rate-limiter
```

3. Check metrics:
```bash
kubectl port-forward -n rate-limiter daemonset/rate-limiter 8080:8080
curl http://localhost:8080/metrics
```

### Configuration

Edit the ConfigMap to customize settings:

```bash
kubectl edit configmap rate-limiter-config -n rate-limiter
```

After editing, restart pods:
```bash
kubectl rollout restart daemonset/rate-limiter -n rate-limiter
```

## Helm Deployment

### Install from Local Chart

```bash
helm install rate-limiter ./helm/rate-limiter \
  --namespace rate-limiter \
  --create-namespace
```

### Install with Custom Values

Create a `custom-values.yaml`:

```yaml
config:
  interface: "eth0"
  logLevel: "info"

resources:
  requests:
    memory: "256Mi"
    cpu: "200m"
  limits:
    memory: "1Gi"
    cpu: "2000m"

serviceMonitor:
  enabled: true
```

Install with custom values:
```bash
helm install rate-limiter ./helm/rate-limiter \
  --namespace rate-limiter \
  --create-namespace \
  -f custom-values.yaml
```

### Upgrade Deployment

```bash
helm upgrade rate-limiter ./helm/rate-limiter \
  --namespace rate-limiter \
  -f custom-values.yaml
```

### Uninstall

```bash
helm uninstall rate-limiter --namespace rate-limiter
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INTERFACE` | `ens33` | Network interface to attach XDP program |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

### eBPF Rate Limiting Parameters

These are set at compile time in `main.go`:

```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go rate_limiter rate_limiter.c -- \
  -DPACKET_BURST_LIMIT=10000 \
  -DPACKETS_PER_SECOND=3200 \
  -DPACKET_BURST_REPLENISH_SECONDS=600
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `PACKET_BURST_LIMIT` | 10000 | Maximum burst size (tokens) |
| `PACKETS_PER_SECOND` | 3200 | Token refill rate per second |
| `PACKET_BURST_REPLENISH_SECONDS` | 600 | Full burst replenishment interval |

To modify these parameters:
1. Edit `main.go`
2. Rebuild: `make compile`
3. Redeploy

## Monitoring

### Prometheus Integration

The rate limiter exposes metrics on port 8080 at `/metrics`.

**Available Metrics:**

- `rate_limited{connection, network_interface}` - Whether a connection is being rate limited (0 or 1)
- `rate_limited_drops{connection, network_interface}` - Total packets dropped per connection
- `rate_limited_tokens{connection, network_interface}` - Available tokens per connection

### Grafana Dashboards

A pre-configured Grafana dashboard is available at `monitoring/dashboard.json`.

Import steps:
1. Open Grafana
2. Go to Dashboards → Import
3. Upload `monitoring/dashboard.json`
4. Select your Prometheus data source

### Prometheus Configuration

Example `prometheus.yml` snippet:

```yaml
scrape_configs:
  - job_name: 'rate-limiter'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8080']
```

For Kubernetes with ServiceMonitor:
```bash
kubectl apply -f k8s/servicemonitor.yaml
```

### Health Checks

- **Health endpoint**: `GET /health` - Returns 200 if service is running
- **Readiness endpoint**: `GET /ready` - Returns 200 if service is ready to accept traffic
- **Metrics endpoint**: `GET /metrics` - Prometheus-formatted metrics

## Load Testing

### Automated Load Test

Run the automated load test suite:

```bash
cd loadtest
./run-test.sh
```

Customize parameters:
```bash
TARGET_URL=http://your-target:8000/hello \
METRICS_URL=http://your-metrics:8080/metrics \
DURATION=120 \
CONCURRENCY=200 \
RPS=2000 \
./run-test.sh
```

### Manual Load Testing with wrk

Install wrk:
```bash
sudo apt-get install wrk
```

Run test:
```bash
wrk -t16 -c1000 -d60s http://localhost:8000/hello
```

### Expected Performance

With default settings:
- **Sustained throughput**: 3200 packets/second per connection
- **Burst capacity**: 10,000 packets
- **CPU usage**: < 10% per core under normal load
- **Memory usage**: < 256MB

## Troubleshooting

### Rate Limiter Not Starting

**Issue**: Container or pod fails to start

**Solutions**:
1. Check kernel version: `uname -r` (needs 4.18+)
2. Verify eBPF support: `cat /proc/sys/kernel/unprivileged_bpf_disabled`
3. Check privileges: Ensure container has `CAP_NET_ADMIN`, `CAP_SYS_ADMIN`, `CAP_BPF`
4. Review logs: `kubectl logs -n rate-limiter -l app=rate-limiter`

### Metrics Not Appearing

**Issue**: No metrics visible in Prometheus

**Solutions**:
1. Check metrics endpoint: `curl http://localhost:8080/metrics`
2. Verify Prometheus scrape config
3. Check network connectivity to metrics port
4. Review ServiceMonitor configuration (Kubernetes)

### High Packet Drop Rate

**Issue**: Too many packets being dropped

**Solutions**:
1. Increase `PACKETS_PER_SECOND` parameter
2. Increase `PACKET_BURST_LIMIT` for better burst handling
3. Check if source IPs are legitimate
4. Review rate limiting configuration

### XDP Attachment Failure

**Issue**: Cannot attach XDP program to interface

**Solutions**:
1. Verify interface exists: `ip link show`
2. Check interface supports XDP: `ethtool -i <interface> | grep driver`
3. Ensure no other XDP programs are attached: `ip link show <interface> | grep xdp`
4. Try different XDP mode (native vs generic)

### Performance Issues

**Issue**: High CPU usage or latency

**Solutions**:
1. Check system resources: `top`, `htop`
2. Review metrics collection frequency
3. Optimize eBPF map size if needed
4. Consider hardware acceleration (if available)

## Security Considerations

1. **Privileged Containers**: Required for eBPF operations. Use SELinux/AppArmor policies to limit scope.
2. **Network Access**: Rate limiter operates at packet level with host network access.
3. **Resource Limits**: Set appropriate CPU/memory limits to prevent resource exhaustion.
4. **Log Sensitivity**: Metrics expose connection information. Secure Prometheus/Grafana access.

## Production Checklist

- [ ] Kernel version 4.18+ confirmed
- [ ] XDP support verified on network interface
- [ ] Resource limits configured
- [ ] Monitoring and alerting set up
- [ ] Rate limiting parameters tuned for workload
- [ ] Load testing completed successfully
- [ ] Backup/failover strategy in place
- [ ] Security policies reviewed
- [ ] Documentation updated with environment-specific details
- [ ] Runbook created for common scenarios

## Support

For issues and questions:
- GitHub Issues: https://github.com/jovalle/rate-limiter/issues
- Documentation: See README.md

## References

- [eBPF Documentation](https://ebpf.io/)
- [XDP Tutorial](https://github.com/xdp-project/xdp-tutorial)
- [Cilium eBPF Library](https://github.com/cilium/ebpf)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
