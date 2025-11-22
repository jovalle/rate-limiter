# Operations Runbook

This runbook provides step-by-step procedures for common operational tasks and incident response scenarios for the rate limiter service.

## Table of Contents

- [Service Overview](#service-overview)
- [Common Operations](#common-operations)
- [Incident Response](#incident-response)
- [Maintenance Procedures](#maintenance-procedures)
- [Performance Tuning](#performance-tuning)
- [Monitoring and Alerts](#monitoring-and-alerts)

## Service Overview

### Architecture

- **Type**: eBPF/XDP-based packet rate limiter
- **Deployment**: DaemonSet on Kubernetes (one pod per node)
- **Port**: 8080 (metrics, health checks)
- **Dependencies**: Linux kernel 4.18+, network interface with XDP support

### Key Components

1. **XDP Program**: Kernel-space packet filtering
2. **Rate Limiter Service**: User-space control and metrics
3. **Prometheus Exporter**: Metrics endpoint
4. **eBPF Maps**: Connection state tracking

### SLOs

- **Availability**: 99.9%
- **Packet Processing Latency**: < 1ms p99
- **CPU Usage**: < 20% per core under normal load
- **Memory Usage**: < 512MB per pod

## Common Operations

### 1. Checking Service Health

#### Quick Health Check
```bash
# Kubernetes
kubectl get pods -n rate-limiter
kubectl exec -n rate-limiter -it <pod-name> -- curl http://localhost:8080/health

# Docker
docker exec rate-limiter curl http://localhost:8080/health
```

#### Detailed Status
```bash
# Check logs
kubectl logs -n rate-limiter -l app=rate-limiter --tail=100

# Check metrics
kubectl port-forward -n rate-limiter daemonset/rate-limiter 8080:8080
curl http://localhost:8080/metrics | grep rate_limited
```

### 2. Viewing Current Rate Limit Status

#### Check Active Connections
```bash
# Get all connections being tracked
curl -s http://localhost:8080/metrics | grep rate_limited{
```

#### Identify Rate Limited Connections
```bash
# Find connections currently being rate limited
curl -s http://localhost:8080/metrics | grep 'rate_limited{.*} 1'
```

#### Check Drop Counters
```bash
# See which connections have dropped packets
curl -s http://localhost:8080/metrics | grep rate_limited_drops | grep -v ' 0$'
```

### 3. Updating Configuration

#### Change Network Interface
```bash
# Kubernetes
kubectl set env daemonset/rate-limiter -n rate-limiter INTERFACE=eth1
kubectl rollout status daemonset/rate-limiter -n rate-limiter

# Docker
docker stop rate-limiter
docker run -d --name rate-limiter --privileged --network host \
  -e INTERFACE=eth1 -e LOG_LEVEL=info rate-limiter:latest
```

#### Change Log Level
```bash
# Kubernetes
kubectl set env daemonset/rate-limiter -n rate-limiter LOG_LEVEL=debug
kubectl rollout restart daemonset/rate-limiter -n rate-limiter
```

### 4. Scaling Operations

#### Deploy to New Nodes
```bash
# Automatically handled by DaemonSet
# Verify new pods are running
kubectl get pods -n rate-limiter -o wide
```

#### Drain Node for Maintenance
```bash
# Cordon node
kubectl cordon <node-name>

# Drain node (rate limiter pod will terminate)
kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data

# Uncordon after maintenance
kubectl uncordon <node-name>
```

### 5. Log Management

#### View Real-time Logs
```bash
# Kubernetes - all pods
kubectl logs -n rate-limiter -l app=rate-limiter -f

# Kubernetes - specific pod
kubectl logs -n rate-limiter <pod-name> -f

# Docker
docker logs -f rate-limiter
```

#### Search Logs for Errors
```bash
kubectl logs -n rate-limiter -l app=rate-limiter --tail=1000 | grep -i error
```

#### Export Logs for Analysis
```bash
kubectl logs -n rate-limiter -l app=rate-limiter --tail=10000 > rate-limiter-logs.txt
```

## Incident Response

### Incident 1: High Packet Drop Rate

#### Symptoms
- Increased `rate_limited_drops` metric
- User reports of connection issues
- High number of rate-limited connections

#### Diagnosis
```bash
# 1. Check drop rates
curl -s http://localhost:8080/metrics | grep rate_limited_drops

# 2. Identify top droppers
curl -s http://localhost:8080/metrics | grep rate_limited_drops | \
  awk '{print $2, $1}' | sort -rn | head -10

# 3. Check if legitimate traffic spike or attack
# Review connection patterns in metrics
```

#### Resolution
```bash
# If legitimate traffic:
# 1. Increase rate limits (requires rebuild and redeploy)
# 2. Or temporarily disable for specific IPs (feature to be added)

# If attack:
# 1. Monitor and log attacking IPs
# 2. Consider upstream filtering
# 3. Verify rate limiter is working as expected
```

#### Verification
```bash
# Monitor drop rate over time
watch -n 5 'curl -s http://localhost:8080/metrics | grep rate_limited_drops'
```

### Incident 2: Rate Limiter Pod Not Starting

#### Symptoms
- Pod in CrashLoopBackOff
- XDP attachment errors in logs

#### Diagnosis
```bash
# 1. Check pod events
kubectl describe pod -n rate-limiter <pod-name>

# 2. Check logs
kubectl logs -n rate-limiter <pod-name>

# 3. Verify kernel version on node
kubectl get node <node-name> -o jsonpath='{.status.nodeInfo.kernelVersion}'

# 4. Check if interface exists on node
kubectl exec -n rate-limiter <pod-name> -- ip link show
```

#### Resolution
```bash
# If kernel version < 4.18:
# Upgrade node kernel (requires node maintenance)

# If interface doesn't exist:
# Update INTERFACE environment variable
kubectl set env daemonset/rate-limiter -n rate-limiter INTERFACE=<correct-interface>

# If another XDP program is attached:
# Remove conflicting XDP program on the node
sudo ip link set dev <interface> xdp off
# Then restart pod
kubectl delete pod -n rate-limiter <pod-name>
```

### Incident 3: Metrics Not Available

#### Symptoms
- Prometheus not scraping metrics
- Grafana dashboards showing no data
- `/metrics` endpoint unreachable

#### Diagnosis
```bash
# 1. Check if pod is running
kubectl get pods -n rate-limiter

# 2. Test metrics endpoint directly
kubectl port-forward -n rate-limiter <pod-name> 8080:8080
curl http://localhost:8080/metrics

# 3. Check service
kubectl get svc -n rate-limiter
kubectl describe svc rate-limiter-metrics -n rate-limiter

# 4. Check ServiceMonitor (if using Prometheus Operator)
kubectl get servicemonitor -n rate-limiter
```

#### Resolution
```bash
# If endpoint works but Prometheus doesn't scrape:
# 1. Verify ServiceMonitor selector matches service labels
kubectl get servicemonitor rate-limiter -n rate-limiter -o yaml

# 2. Check Prometheus configuration
kubectl logs -n monitoring prometheus-xxx

# If endpoint doesn't work:
# 1. Restart pod
kubectl delete pod -n rate-limiter <pod-name>

# 2. Check network policies
kubectl get networkpolicies -n rate-limiter
```

### Incident 4: High CPU Usage

#### Symptoms
- Pod CPU usage exceeds limits
- CPU throttling observed
- Slow packet processing

#### Diagnosis
```bash
# 1. Check current CPU usage
kubectl top pods -n rate-limiter

# 2. Check resource limits
kubectl get pod -n rate-limiter <pod-name> -o yaml | grep -A 5 resources

# 3. Check number of tracked connections
curl -s http://localhost:8080/metrics | grep rate_limited_tokens | wc -l

# 4. Review eBPF map size
kubectl logs -n rate-limiter <pod-name> | grep -i "map entries"
```

#### Resolution
```bash
# 1. Increase CPU limits if needed
kubectl edit daemonset rate-limiter -n rate-limiter
# Update limits.cpu value

# 2. Optimize metrics collection interval if too frequent
# Edit main.go and rebuild if necessary

# 3. Consider filtering metrics for specific connections only
```

### Incident 5: Memory Leak

#### Symptoms
- Steadily increasing memory usage
- Pod OOMKilled events
- eBPF map growth

#### Diagnosis
```bash
# 1. Check memory usage trend
kubectl top pods -n rate-limiter

# 2. Check for OOM kills
kubectl get events -n rate-limiter | grep OOMKilled

# 3. Check eBPF map size
# (Would require adding debug endpoint)

# 4. Check for connection cleanup
curl -s http://localhost:8080/metrics | grep rate_limited | wc -l
```

#### Resolution
```bash
# 1. Restart affected pod
kubectl delete pod -n rate-limiter <pod-name>

# 2. Increase memory limits if insufficient
kubectl edit daemonset rate-limiter -n rate-limiter

# 3. Review code for potential leaks
# Check if old connections are being removed from maps
```

## Maintenance Procedures

### Updating Rate Limits

1. Edit `main.go` and update the generate directive:
```go
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go rate_limiter rate_limiter.c -- \
  -DPACKET_BURST_LIMIT=20000 \
  -DPACKETS_PER_SECOND=5000 \
  -DPACKET_BURST_REPLENISH_SECONDS=600
```

2. Rebuild and push new image
3. Update deployment:
```bash
kubectl set image daemonset/rate-limiter -n rate-limiter \
  rate-limiter=rate-limiter:new-version
```

### Rolling Updates

```bash
# Update image
kubectl set image daemonset/rate-limiter -n rate-limiter rate-limiter=rate-limiter:v2

# Monitor rollout
kubectl rollout status daemonset/rate-limiter -n rate-limiter

# Verify new version
kubectl get pods -n rate-limiter -o jsonpath='{.items[*].spec.containers[*].image}'
```

### Backup and Recovery

```bash
# Export current configuration
kubectl get all -n rate-limiter -o yaml > rate-limiter-backup.yaml
kubectl get configmap -n rate-limiter -o yaml >> rate-limiter-backup.yaml

# Restore from backup
kubectl apply -f rate-limiter-backup.yaml
```

## Performance Tuning

### Optimizing for High Throughput

1. **Increase burst limits**:
   - Edit `PACKET_BURST_LIMIT` in main.go
   - Rebuild and redeploy

2. **Tune resource limits**:
```yaml
resources:
  requests:
    cpu: "500m"
    memory: "512Mi"
  limits:
    cpu: "2000m"
    memory: "2Gi"
```

3. **Optimize metrics collection**:
   - Reduce scrape frequency in main.go (currently 0.5s)
   - Filter metrics to only critical connections

### Optimizing for Low Latency

1. Use native XDP mode (hardware offload if supported)
2. Pin to specific CPU cores
3. Reduce metrics collection frequency
4. Increase kernel buffer sizes

### Capacity Planning

**Per-node capacity**:
- Connections tracked: Up to 65,536 (MAX_MAP_ENTRIES)
- Packet processing: > 10M packets/sec (hardware dependent)
- Memory per connection: ~128 bytes

**Scaling considerations**:
- Use DaemonSet for per-node deployment
- Each node independently rate limits
- No coordination between nodes needed

## Monitoring and Alerts

### Key Metrics to Monitor

```promql
# High drop rate
rate(rate_limited_drops[5m]) > 1000

# Service availability
up{job="rate-limiter"} == 0

# High CPU usage
rate(container_cpu_usage_seconds_total{pod=~"rate-limiter.*"}[5m]) > 0.8

# Memory usage
container_memory_usage_bytes{pod=~"rate-limiter.*"} > 500000000

# Rate limited connections
rate_limited > 0.5
```

### Recommended Alerts

#### Critical Alerts

```yaml
# Rate limiter down
- alert: RateLimiterDown
  expr: up{job="rate-limiter"} == 0
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Rate limiter is down"

# High packet drop rate
- alert: HighPacketDropRate
  expr: rate(rate_limited_drops[5m]) > 10000
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "High packet drop rate detected"
```

#### Warning Alerts

```yaml
# Elevated CPU usage
- alert: RateLimiterHighCPU
  expr: rate(container_cpu_usage_seconds_total{pod=~"rate-limiter.*"}[5m]) > 0.7
  for: 10m
  labels:
    severity: warning

# Elevated memory usage
- alert: RateLimiterHighMemory
  expr: container_memory_usage_bytes{pod=~"rate-limiter.*"} > 400000000
  for: 10m
  labels:
    severity: warning
```

### Dashboard Panels

Key panels for operations dashboard:
1. Active rate-limited connections (current)
2. Packet drop rate (rate over time)
3. Available tokens distribution (histogram)
4. CPU and memory usage per pod
5. Pod status and restarts

## Emergency Contacts

- **On-call Engineer**: [Contact details]
- **Infrastructure Team**: [Contact details]
- **Security Team**: [Contact details]

## Related Documentation

- [Deployment Guide](DEPLOYMENT.md)
- [README](README.md)
- [GitHub Issues](https://github.com/jovalle/rate-limiter/issues)
