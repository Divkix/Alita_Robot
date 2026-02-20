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

> Note: The block and mutex profiles are **disabled by default** in Go. With `ENABLE_PPROF=true` set, the endpoints are exposed but will return empty data unless you enable collection via `GODEBUG=blockprofilerate=1,mutexprofilefraction=1` environment variable or `runtime.SetBlockProfileRate()` in code.

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

#### Option 1: Web UI Mode

```bash
# Open web UI at http://localhost:8081
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/heap
```

Use the web interface to explore the profile visually.

#### Option 2: Interactive Console

```bash
# Drop into interactive console
go tool pprof http://localhost:8080/debug/pprof/heap

# Then run commands like:
(pprof) top
(pprof) web
(pprof) list functionname
```

#### Goroutine Analysis

```bash
# Get goroutine dump in console mode
go tool pprof http://localhost:8080/debug/pprof/goroutine

# Check for goroutine leaks
(pprof) top
```

#### CPU Profiling

```bash
# Collect 30 seconds of CPU profile
# Note: The server has a 10s WriteTimeout - use shorter duration or profile externally
go tool pprof -seconds=30 http://localhost:8080/debug/pprof/profile

## Flame Graphs

Flame graphs provide a visual representation of CPU or memory usage.

### Option 1: go tool pprof (Recommended)

The simplest way to generate flame graphs:

```bash
# Generate SVG flame graph from heap profile
go tool pprof -svg -output=heap-flamegraph.svg http://localhost:8080/debug/pprof/heap

# Generate SVG flame graph from CPU profile (30 seconds)
go tool pprof -svg -output=cpu-flamegraph.svg http://localhost:8080/debug/pprof/profile?seconds=30

# Or open in browser directly
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/heap
```

### Option 2: FlameGraph Perl Scripts

For more control, use Brendan Gregg's FlameGraph tools:

```bash
# Clone the FlameGraph repository
git clone https://github.com/brendangregg/FlameGraph.git
cd FlameGraph

# Generate heap flame graph from pprof
# First, get the profile as raw protobuf
curl -s http://localhost:8080/debug/pprof/heap > heap.pb

# Convert to SVG using go tool pprof to export folded stacks
go tool pprof -proto -output=heap.folded ./your-binary heap.pb

# Generate flame graph
./flamegraph.pl heap.folded > heap-flamegraph.svg
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
