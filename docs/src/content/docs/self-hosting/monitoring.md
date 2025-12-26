---
title: Monitoring and Observability
description: Monitor Alita Robot health, metrics, and errors.
---

# Monitoring and Observability

Alita Robot provides comprehensive monitoring capabilities including health checks, Prometheus metrics, Sentry error tracking, and resource monitoring.

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

## Sentry Error Tracking

Sentry provides real-time error tracking and performance monitoring.

### Configuration

```bash
# Enable Sentry integration
ENABLE_SENTRY=true

# Sentry DSN from your project settings
SENTRY_DSN=https://your-key@o123456.ingest.sentry.io/1234567

# Environment name (helps organize errors)
SENTRY_ENVIRONMENT=production

# Sample rate for error events (0.0-1.0)
# Default: 1.0 (100% of errors sent)
SENTRY_SAMPLE_RATE=1.0
```

### Sentry Features

1. **Error Tracking**: Automatically captures and reports errors
2. **Logrus Integration**: Error, Fatal, and Panic log levels are sent to Sentry
3. **Context Enrichment**: Errors include user ID, chat ID, and message context
4. **Filtering**: Sensitive data (bot token) is automatically redacted
5. **Expected Errors**: Common errors like "user blocked bot" are suppressed

### Error Context

When an error occurs, Sentry receives:

- Update ID
- User ID and username
- Chat ID and type
- Message ID and text
- Source file, line, and function

### Sentry Pricing

| Plan | Errors/Month | Cost |
|------|-------------|------|
| Free | 5,000 | $0 |
| Team | 50,000 | $29/month |

For high-volume bots, reduce `SENTRY_SAMPLE_RATE` to stay within limits.

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

When thresholds are exceeded, the system automatically:

1. Triggers garbage collection
2. Logs warnings about resource usage
3. Takes corrective action if configured

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

The system tracks:

- **DAG**: Daily Active Groups
- **WAG**: Weekly Active Groups
- **MAG**: Monthly Active Groups

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
| `ERROR` | Unexpected errors (sent to Sentry) |
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

### Sentry Alerting

Configure alerts in Sentry dashboard:
1. Go to Alerts > Create Alert
2. Set conditions (error count, unique users affected)
3. Configure notification channels (email, Slack, PagerDuty)
