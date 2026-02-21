---
title: Monitoring and Observability
description: Monitor Alita Robot health, metrics, and errors.
---

# Monitoring and Observability

Alita Robot provides comprehensive monitoring capabilities including health checks, Prometheus metrics, and resource monitoring.

## Health Endpoint

The health endpoint provides real-time status of the bot and its dependencies.

### Endpoint

```
GET /health
```

### Response Format

```json
{
  "status": "healthy",
  "checks": {
    "database": true,
    "redis": true
  },
  "version": "1.0.0",
  "uptime": "24h30m15s"
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Overall health: `healthy` or `unhealthy` |
| `checks.database` | boolean | PostgreSQL connection status |
| `checks.redis` | boolean | Redis connection status |
| `version` | string | Bot version |
| `uptime` | string | Time since bot started |

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | All systems healthy |
| 503 | One or more checks failed |

### Health Check Logic

The health check performs:

1. **Database check**: Pings PostgreSQL with a 2-second timeout
2. **Redis check**: Sets and gets a test key with a 2-second timeout

Both checks must pass for `healthy` status.

### Usage Examples

```bash
# Simple health check
curl http://localhost:8080/health

# Docker health check (built-in)
/alita_robot --health

# Kubernetes liveness probe
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
```

## Prometheus Metrics

The metrics endpoint exposes Prometheus-compatible metrics for monitoring.

### Endpoint

```
GET /metrics
```

### Prometheus Scrape Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'alita-robot'
    static_configs:
      - targets: ['alita:8080']
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: /metrics
```

### Docker Compose with Prometheus

```yaml
services:
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    ports:
      - "9090:9090"
    depends_on:
      - alita

volumes:
  prometheus_data:
```

### Grafana Dashboard

For visualization, add Grafana:

```yaml
services:
  grafana:
    image: grafana/grafana:latest
    volumes:
      - grafana_data:/var/lib/grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    depends_on:
      - prometheus

volumes:
  grafana_data:
```

## Resource Monitoring

Alita Robot includes automatic resource monitoring to prevent resource exhaustion.

### Configuration

```bash
# Maximum goroutines before triggering cleanup
# Default: 1000, Recommended: 1000-5000
RESOURCE_MAX_GOROUTINES=1000

# Maximum memory usage in MB before triggering cleanup
# Default: 500, Recommended: 500-2000
RESOURCE_MAX_MEMORY_MB=500

# Memory threshold for triggering garbage collection
# Default: 400, Recommended: 80% of RESOURCE_MAX_MEMORY_MB
RESOURCE_GC_THRESHOLD_MB=400
```

### Auto-Remediation

The system uses a 4-tier auto-remediation approach when resource thresholds are exceeded:

1. **Tier 0 — Warning**: Logs warnings at 80% goroutine threshold or 50% memory usage
2. **Tier 1 — GC Trigger**: Triggers garbage collection at 60% memory usage or 50ms GC pause times
3. **Tier 2 — Aggressive Cleanup**: Runs multiple GC cycles when memory exceeds the GC threshold
4. **Tier 3 — Restart Recommendation**: Logs a restart recommendation at 150%+ goroutines or 160%+ memory usage

## Activity Monitoring

Track group activity automatically:

```bash
# Days of inactivity before marking chat as inactive
# Default: 30, Range: 1-365
INACTIVITY_THRESHOLD_DAYS=30

# Hours between automatic activity checks
# Default: 1, Range: 1-24
ACTIVITY_CHECK_INTERVAL=1

# Enable automatic cleanup of inactive chats
# Default: true
ENABLE_AUTO_CLEANUP=true
```

### Activity Metrics

The system tracks group activity:

- **DAG**: Daily Active Groups
- **WAG**: Weekly Active Groups
- **MAG**: Monthly Active Groups

The activity monitor also tracks individual user activity, calculating Daily Active Users (DAU), Weekly Active Users (WAU), and Monthly Active Users (MAU).

Groups are automatically:
- Marked inactive after the threshold period
- Reactivated when they become active again

## Performance Monitoring

### Enable Performance Tracking

```bash
# Enable automatic performance monitoring
ENABLE_PERFORMANCE_MONITORING=true

# Enable background statistics collection
ENABLE_BACKGROUND_STATS=true
```

### Collected Metrics

- Message processing time
- Database query duration
- Cache hit/miss rates
- API response times
- Goroutine count
- Memory usage

## Debug Mode

For detailed logging during development:

```bash
DEBUG=true
```

Debug mode:
- Increases log verbosity
- Disables background monitoring
- Shows detailed error stack traces

## Log Analysis

### Log Fields

Structured log entries include:

| Field | Description |
|-------|-------------|
| `update_id` | Telegram update ID |
| `error_type` | Error type (e.g., `*TelegramError`) |
| `file` | Source file where error occurred |
| `line` | Line number |
| `function` | Function name |

### Example Log Entry

```json
{
  "level": "error",
  "msg": "Handler error occurred: user blocked bot",
  "update_id": 123456789,
  "error_type": "*gotgbot.TelegramError",
  "file": "alita/modules/admin.go",
  "line": 45,
  "function": "handleAdminCommand",
  "time": "2024-03-15T10:30:00Z"
}
```

### Log Levels

| Level | Description |
|-------|-------------|
| `DEBUG` | Verbose debugging (DEBUG=true only) |
| `INFO` | Normal operation events |
| `WARN` | Expected issues (e.g., user blocked bot) |
| `ERROR` | Unexpected errors |
| `FATAL` | Critical errors that stop the bot |
| `PANIC` | Unrecoverable errors |

## Alerting

### Prometheus Alerting Rules

Create `alerts.yml`:

```yaml
groups:
  - name: alita-alerts
    rules:
      - alert: AlitaUnhealthy
        expr: up{job="alita-robot"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Alita Robot is down"

      - alert: HighMemoryUsage
        expr: process_resident_memory_bytes > 500000000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage detected"

      - alert: DatabaseConnectionFailed
        expr: alita_health_database == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Database connection failed"
```
