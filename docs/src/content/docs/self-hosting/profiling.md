---
title: Performance Profiling
description: Debug performance issues using Go pprof.
---

# Performance Profiling

Alita Robot supports Go's pprof profiling tool for diagnosing performance bottlenecks. This guide covers how to enable and use profiling endpoints.

## ⚠️ Security Warning

**pprof endpoints should NEVER be enabled in production.** They expose detailed runtime information that can aid attackers. Only enable for development or debugging temporary issues in staging.

## Enabling Profiling

### Environment Variable

```bash
# Enable pprof endpoints (development only!)
ENABLE_PPROF=true
```

When enabled, the following endpoints become available:

| Endpoint | Description |
|----------|-------------|
| `/debug/pprof/` | Index of available profiles |
| `/debug/pprof/heap` | Heap memory profile |
| `/debug/pprof/goroutine` | Goroutine stack trace |
| `/debug/pprof/threadcreate` | Thread creation profile |
| `/debug/pprof/block` | Block (goroutine blocking) profile |
| `/debug/pprof/mutex` | Mutex contention profile |

### CPU Profiling

CPU profiling requires a separate request:

```bash
# Collect 30 seconds of CPU profile
curl -o cpu.pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

## Using pprof

### Interactive Analysis

Start the pprof interactive console:

```bash
go tool pprof http://localhost:8080/debug/pprof/heap
```

Common commands in pprof:

| Command | Description |
|---------|-------------|
| `top` | Show top functions by resource usage |
| `web` | Open visual graph in browser |
| `list funcname` | Show source for specific function |
| `traces` | Print all sample traces |

### Examples

#### Top Memory Consumers

```bash
# Get heap profile
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/heap

# In pprof console
top
```

#### Goroutine Analysis

```bash
# Get goroutine dump
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/goroutine

# Check for goroutine leaks
top
```

#### CPU Profiling

```bash
# Collect and analyze CPU profile
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/profile?seconds=30
```

## Flame Graphs

Flame graphs provide a visual representation of CPU or memory usage.

### Installation

```bash
# Install flamegraph tool
go install github.com/brendangregg/FlameGraph@latest
```

### Generation

```bash
# Get profile data
curl -s http://localhost:8080/debug/pprof/heap > heap.out

# Generate flame graph
flamegraph.pl heap.out > flamegraph.svg

# Or with go-torch (for CPU profiles)
go install github.com/uber/go-torch@latest
go-torch -u http://localhost:8080 -f flamegraph.svg
```

## Common Performance Issues

### High Memory Usage

1. Collect heap profile during peak usage
2. Look for objects that shouldn't be retained
3. Check for unbounded caches or slices

### Goroutine Leaks

1. Compare goroutine profiles over time
2. Look for goroutines waiting on channels
3. Check for missing context cancellations

### CPU Spikes

1. Collect CPU profile during spike
2. Identify hot code paths
3. Look for busy loops or excessive locking

## Production Alternatives

For production monitoring without pprof:

- Use Prometheus metrics for observability
- Enable `ENABLE_PERFORMANCE_MONITORING` for auto-remediation
- Monitor `/metrics` endpoint for custom metrics
- Use external APM tools (Datadog, New Relic)

## Troubleshooting

### Profile is Empty

- Ensure traffic is hitting the bot during collection
- CPU profiles require active processing

### Connection Refused

- Verify `ENABLE_PPROF=true` is set
- Check bot is running and port is correct
- Ensure firewall allows access to pprof port
