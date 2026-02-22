---
title: Troubleshooting
description: Common issues and solutions for Alita Robot.
---

# Troubleshooting

This guide covers common issues you may encounter when running Alita Robot and how to resolve them.

## Bot Won't Start

### Invalid Bot Token

**Error:**
```
Failed to create new bot: invalid token
```

**Solution:**
1. Verify your bot token from [@BotFather](https://t.me/BotFather)
2. Ensure no extra spaces or newlines in the token
3. Check that the token format is correct: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`

:::caution
Do not wrap the token in quotes in your `.env` file. The quotes become part of the value and will cause authentication failure.
:::

```bash
# Correct format in .env
BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrsTUVwxyz

# Wrong - has quotes
BOT_TOKEN="123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
```

### Database Connection Failed

**Error:**
```
[Database][Connection] Failed after 5 attempts: connection refused
```

**Solutions:**

1. **Check PostgreSQL is running:**
   ```bash
   # Linux
   sudo systemctl status postgresql

   # Docker
   docker-compose ps postgres
   ```

2. **Verify connection string:**
   ```bash
   # Test connection directly
   psql "postgres://user:pass@localhost:5432/alita?sslmode=disable"
   ```

3. **Check network access:**
   ```bash
   # Is the port open?
   nc -zv localhost 5432
   ```

4. **Docker Compose:** Ensure PostgreSQL is healthy before Alita starts:
   ```yaml
   depends_on:
     postgres:
       condition: service_healthy
   ```

### Redis Connection Failed

**Error:**
```
[Redis] Failed to connect: connection refused
```

**Solutions:**

1. **Check Redis is running:**
   ```bash
   redis-cli ping
   # Should return: PONG
   ```

2. **Verify address and password:**
   ```bash
   REDIS_ADDRESS=localhost:6379
   REDIS_PASSWORD=your_password  # Leave empty if no password
   ```

3. **Docker:** Ensure Redis is started before Alita

### MESSAGE_DUMP Invalid

**Error:**
```
[Bot] Failed to send startup message to log group
```

:::tip
The easiest way to get a channel ID is to forward any message from the channel to [@userinfobot](https://t.me/userinfobot).
:::

**Solutions:**

1. **Format:** Channel ID must start with `-100`:
   ```bash
   MESSAGE_DUMP=-100123456789
   ```

2. **Bot access:** Add the bot as an admin to the channel

3. **Get correct ID:** Forward a message from the channel to [@userinfobot](https://t.me/userinfobot)

## Webhook Issues

### Not Receiving Updates

**Symptoms:**
- Bot starts successfully
- No messages are processed
- Health check returns `healthy`

**Solutions:**

1. **Check webhook status:**
   ```bash
   curl "https://api.telegram.org/bot<TOKEN>/getWebhookInfo"
   ```

   Look for:
   - `url` should match your `WEBHOOK_DOMAIN`
   - `has_custom_certificate` if using self-signed cert
   - `last_error_message` for any errors

2. **Verify domain is accessible:**
   ```bash
   curl -I https://your-domain.com/health
   ```

3. **Check SSL certificate:**
   ```bash
   openssl s_client -connect your-domain.com:443 -servername your-domain.com
   ```

### 401 Unauthorized

**Error in Telegram webhook info:**
```
"last_error_message": "Unauthorized"
```

**Solutions:**

1. **Check WEBHOOK_SECRET matches:**
   - The URL path must include your secret
   - Example: `/webhook/your-secret-here`

2. **Verify configuration:**
   ```bash
   USE_WEBHOOKS=true
   WEBHOOK_DOMAIN=https://your-domain.com
   WEBHOOK_SECRET=your-secret-here
   ```

### Connection Timeout

**Error:**
```
"last_error_message": "Connection timed out"
```

**Solutions:**

1. **Verify port 8080 is accessible from the internet**
2. **Check firewall rules:**
   ```bash
   # Allow port 8080
   sudo ufw allow 8080/tcp
   ```
3. **Check reverse proxy/tunnel is running**

## Database Issues

### Migration Failed

**Error:**
```
[Database][AutoMigrate] Migration failed: column already exists
```

**Solutions:**

:::caution
Only use `AUTO_MIGRATE_SILENT_FAIL=true` in development. In production, always investigate and resolve migration failures before continuing.
:::

1. **Skip with silent fail (development only):**
   ```bash
   AUTO_MIGRATE_SILENT_FAIL=true
   ```

2. **Mark migration as applied manually:**
   ```sql
   INSERT INTO schema_migrations (version, executed_at)
   VALUES ('problematic_migration.sql', NOW());
   ```

3. **Check migration status:**
   ```bash
   make psql-status
   ```

### Too Many Connections

**Error:**
```
pq: too many connections for role "alita"
```

**Solutions:**

1. **Reduce connection pool size:**
   ```bash
   DB_MAX_OPEN_CONNS=50
   DB_MAX_IDLE_CONNS=10
   ```

2. **Increase PostgreSQL max connections:**
   ```bash
   # In postgresql.conf
   max_connections = 200
   ```

3. **Use connection pooling (PgBouncer):**
   ```bash
   DATABASE_URL=postgres://user:pass@pgbouncer:6432/alita
   ```

### Query Timeout

**Error:**
```
pq: canceling statement due to statement timeout
```

**Solutions:**

1. **Check for slow queries:**
   ```sql
   SELECT pid, query, state, query_start
   FROM pg_stat_activity
   WHERE state != 'idle'
   ORDER BY query_start;
   ```

2. **Add indexes for slow queries**
3. **Increase timeout (not recommended for production)**

## Permission Errors

### Bot Lacks Admin Rights

**Error:**
```
telegram: Bad Request: need administrator rights in the chat
```

**Solutions:**

1. **Promote the bot to admin in the group**
2. **Grant specific permissions:**
   - Delete messages
   - Ban users
   - Pin messages
   - Manage topics (for forum groups)

### User Not Admin

**Error:**
```
You need to be an admin to use this command
```

**This is expected behavior.** Admin commands require the user to be a group admin.

### Cannot Restrict Chat Owner

**Error:**
```
telegram: Bad Request: can't restrict chat owner
```

**This is a Telegram limitation.** The chat owner cannot be:
- Banned
- Muted
- Warned

### Admin Commands Fail for Unfamiliar Users

**Symptoms:**
- `/promote @username` fails even though the user exists
- Commands work when replying but not when using usernames

**Resolved in v1.x:** Previously, admin commands required users to exist in the bot's local database. The bot now queries Telegram's API as a fallback when username lookup fails locally, allowing admin commands to work on any valid Telegram user.

If you're running an older version, upgrade to get this fix.

## Performance Issues

### High Memory Usage

**Symptoms:**
- Memory exceeds `RESOURCE_MAX_MEMORY_MB`
- Bot becomes slow or unresponsive

**Solutions:**

1. **Enable auto-remediation:**
   ```bash
   ENABLE_PERFORMANCE_MONITORING=true
   RESOURCE_MAX_MEMORY_MB=500
   RESOURCE_GC_THRESHOLD_MB=400
   ```

2. **Reduce worker pools:**
   ```bash
   DISPATCHER_MAX_ROUTINES=100
   DATABASE_WORKERS=3
   ```

3. **Check for memory leaks:**
   ```bash
   DEBUG=true  # Enable detailed logging
   ```

### Slow Response Times

**Symptoms:**
- Commands take several seconds to execute
- Database queries are slow

**Solutions:**

1. **Enable query caching:**
   ```bash
   ENABLE_RESPONSE_CACHING=true
   RESPONSE_CACHE_TTL=30
   ```

2. **Check database performance:**
   ```sql
   -- Find slow queries
   SELECT query, calls, mean_exec_time
   FROM pg_stat_statements
   ORDER BY mean_exec_time DESC
   LIMIT 10;
   ```

3. **Optimize connection pooling:**
   ```bash
   DB_MAX_IDLE_CONNS=50
   DB_MAX_OPEN_CONNS=200
   ```

4. **Enable HTTP connection pooling:**
   ```bash
   ENABLE_HTTP_CONNECTION_POOLING=true
   HTTP_MAX_IDLE_CONNS=100
   ```

### High CPU Usage

**Solutions:**

1. **Limit concurrent goroutines:**
   ```bash
   DISPATCHER_MAX_ROUTINES=100
   RESOURCE_MAX_GOROUTINES=1000
   ```

2. **Check for infinite loops in logs**
3. **Profile with pprof (development only)**

## Log Analysis

### Enable Debug Logging

:::tip
Enable debug logging when investigating issues, then disable it once the problem is resolved. Leaving debug mode on degrades performance.
:::

```bash
DEBUG=true
```

### Common Log Fields

| Field | Description |
|-------|-------------|
| `update_id` | Telegram update identifier |
| `error_type` | Error type (e.g., `*TelegramError`) |
| `file` | Source file |
| `line` | Line number |
| `function` | Function name |

### Finding Errors in Logs

```bash
# Docker
docker-compose logs alita 2>&1 | grep -i error

# Systemd
journalctl -u alita-robot | grep -i error

# Last 100 errors
docker-compose logs --tail=1000 alita 2>&1 | grep -i error | tail -100
```

### Log Levels

| Level | When to Use |
|-------|-------------|
| DEBUG | Verbose debugging (requires `DEBUG=true`) |
| INFO | Normal operations |
| WARN | Expected issues (e.g., user blocked bot) |
| ERROR | Unexpected failures |
| FATAL | Critical errors that stop the bot |

## Docker-Specific Issues

### Container Keeps Restarting

```bash
# Check exit code
docker-compose ps

# View logs
docker-compose logs alita

# Check for OOM kill
docker inspect alita-robot | grep -i oom
```

### Health Check Failing

```bash
# Test health endpoint manually
docker-compose exec alita /alita_robot --health

# Or from host
curl http://localhost:8080/health
```

### Cannot Connect to Other Services

:::caution
Inside Docker Compose, services communicate by service name, not `localhost`. Use the Docker Compose service name (e.g., `postgres`, `redis`) as the hostname in connection strings.
:::

```bash
# Check network
docker network inspect alita_robot_default

# Verify service names match in DATABASE_URL and REDIS_ADDRESS
DATABASE_URL=postgresql://alita:alita@postgres:5432/alita  # Use service name, not localhost
```

## Internationalization (i18n) Issues

### Empty Bot Responses

**Symptoms:**
- Bot sends empty messages
- Commands execute but no text is displayed
- Works in some languages but not others

**Cause:** Translation key mismatch between code and locale files.

**Solutions:**

1. **Check translation key exists in all locale files:**
   ```bash
   # Search for a key in all locale files
   grep -r "misc_user_not_found" locales/
   ```

2. **Verify key names match exactly:**
   - Code uses: `tr.GetString("misc_translate_need_text")`
   - Locale file must have: `misc_translate_need_text: "..."`
   - Common issue: Similar but different key names (e.g., `misc_need_text_and_lang` vs `misc_translate_need_text`)

3. **Add missing keys:** If a key exists in one locale but not another, add it to all supported locales.

4. **Check YAML syntax:**
   ```yaml
   # Correct - double quotes for escape sequences
   misc_result: "Line 1\nLine 2"

   # Wrong - single quotes preserve \n literally
   misc_result: 'Line 1\nLine 2'
   ```

### Translation Errors Logged

**Error:**
```
[i18n] Translation key not found: misc_example_key
```

**Solution:** Add the missing key to all locale files in `locales/`.

## Getting Help

If you cannot resolve an issue:

1. **Check existing issues:** [GitHub Issues](https://github.com/divkix/Alita_Robot/issues)
2. **Enable debug logging** and collect relevant logs
3. **Open a new issue** with:
   - Full error message
   - Steps to reproduce
   - Environment details (OS, Docker version, etc.)
   - Relevant configuration (without secrets)
