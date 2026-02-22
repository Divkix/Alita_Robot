---
title: Database Configuration
description: Configure and manage PostgreSQL database for Alita Robot.
---

# Database Configuration

Alita Robot uses PostgreSQL as its primary database with GORM as the ORM layer. This guide covers database setup, migrations, connection pooling, and schema design.

## Requirements

:::note
PostgreSQL is the only supported database engine. MySQL, SQLite, and other databases are not supported.
:::

- **PostgreSQL** 14 or higher
- UTF-8 encoding support
- Minimum 512MB RAM for the database

## Connection String

Configure the database URL in your environment:

```bash
# Format
DATABASE_URL=postgres://username:password@host:port/database?sslmode=disable

# Example (local development)
DATABASE_URL=postgres://postgres:password@localhost:5432/alita_robot?sslmode=disable

# Example (Docker Compose)
DATABASE_URL=postgresql://alita:alita@postgres:5432/alita

# Example (production with SSL)
DATABASE_URL=postgres://user:pass@db.example.com:5432/alita?sslmode=require
```

### SSL Modes

:::caution
Never use `sslmode=disable` in production. Always use `require` or stronger to protect data in transit.
:::

| Mode | Description |
|------|-------------|
| `disable` | No SSL (development only) |
| `require` | Require SSL but don't verify certificate |
| `verify-ca` | Require SSL and verify server certificate |
| `verify-full` | Require SSL, verify certificate and hostname |

## Automatic Migrations

Alita Robot supports automatic database migrations on startup, eliminating the need to manually run migration commands.

### Enabling Auto-Migration

```bash
# Enable automatic migrations
AUTO_MIGRATE=true

# Optional: Continue running even if migrations fail (not recommended for production)
AUTO_MIGRATE_SILENT_FAIL=false

# Optional: Custom migration directory
MIGRATIONS_PATH=migrations
```

### How Auto-Migration Works

:::tip
Auto-migration is safe to enable on every startup. Migrations are tracked and only applied once, so re-running them is a no-op.
:::

1. **Migration Files**: SQL migrations are stored in the `migrations/` directory
2. **Version Tracking**: Applied migrations are tracked in the `schema_migrations` table
3. **Idempotent**: Migrations are only applied once; safe to run multiple times
4. **Transactional**: Each migration runs in a transaction for atomicity
5. **Auto-Cleaning**: Supabase-specific SQL (GRANT statements, RLS policies) is automatically removed

### Migration Process

When `AUTO_MIGRATE=true`, the bot will:

1. Check for pending migrations in `migrations/`
2. Clean any Supabase-specific SQL commands automatically
3. Apply migrations in alphabetical order (by filename)
4. Track applied migrations in `schema_migrations` table
5. Log migration status and any errors

Example log output:

```
[Migrations] Starting automatic database migration...
[Migrations] Found 15 migration files
[Migrations] Applying 20240101_initial_schema.sql...
[Migrations] Successfully applied 20240101_initial_schema.sql
[Migrations] Migration complete - Applied: 5, Skipped: 10
```

## Manual Migration Commands

If you prefer manual control over migrations, use the Makefile targets:

### Apply Migrations

```bash
# Set required environment variables
export PSQL_DB_HOST=localhost
export PSQL_DB_NAME=alita
export PSQL_DB_USER=postgres
export PSQL_DB_PASSWORD=password
export PSQL_DB_PORT=5432  # Optional, defaults to 5432

# Apply all pending migrations
make psql-migrate
```

### Check Migration Status

```bash
make psql-status
```

Output:

```
          version           |        executed_at
----------------------------+----------------------------
 20240315_add_captcha.sql   | 2024-03-15 10:30:00.000000
 20240301_add_notes.sql     | 2024-03-01 09:15:00.000000
 20240101_initial.sql       | 2024-01-01 00:00:00.000000
```

### Reset Database (DANGEROUS)

:::danger[Irreversible Data Loss]
This will drop ALL tables and delete ALL data. There is no undo. Make sure you have a verified backup before running this command.
:::

This will drop all tables and recreate the schema:

```bash
make psql-reset
```

You will be prompted to confirm with `yes` before proceeding.

## Connection Pool Configuration

Optimize database performance with connection pooling:

```bash
# Maximum idle connections in the pool
# Default: 50, Recommended: 30-80 depending on deployment size
DB_MAX_IDLE_CONNS=50

# Maximum open connections to the database
# Default: 200, Recommended: 150-400 depending on deployment size
DB_MAX_OPEN_CONNS=200

# Maximum connection lifetime in minutes
# Default: 240, Recommended: 120-480 minutes
DB_CONN_MAX_LIFETIME_MIN=240

# Maximum idle time in minutes
# Default: 60, Recommended: 30-120 minutes
DB_CONN_MAX_IDLE_TIME_MIN=60
```

### Sizing Guidelines

:::tip[Performance Tuning]
Start with the defaults and only adjust pool settings if you observe connection exhaustion or idle timeout issues in the logs. Over-provisioning connections wastes database resources.
:::

| Deployment Size | MAX_IDLE_CONNS | MAX_OPEN_CONNS | Use Case |
|-----------------|----------------|----------------|----------|
| Small | 10-30 | 100 | < 50 groups |
| Medium | 30-50 | 200 | 50-500 groups |
| Large | 50-80 | 300-400 | 500+ groups |

## Schema Design Patterns

Alita Robot uses a **surrogate key pattern** for all database tables:

### Primary Keys

Each table has an auto-incremented `id` field as the primary key (internal identifier):

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,           -- Internal ID
    user_id BIGINT NOT NULL UNIQUE,  -- Telegram user ID
    username VARCHAR(255),
    name VARCHAR(255),
    language VARCHAR(10) DEFAULT 'en',
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### Benefits

1. **Decoupling**: Internal schema is independent of external systems (Telegram IDs)
2. **Stability**: If external IDs change or new platforms are added, internal references remain stable
3. **Performance**: Integer primary keys are faster for joins and indexing
4. **GORM Compatibility**: Consistent integer primary keys simplify ORM operations

### Business Keys

External identifiers like `user_id` (Telegram user ID) and `chat_id` (Telegram chat ID) are stored with unique constraints:

```sql
user_id BIGINT NOT NULL UNIQUE  -- Prevents duplicates
chat_id BIGINT NOT NULL UNIQUE  -- Ensures one row per chat
```

### Exception: Join Tables

The `chat_users` join table uses a composite primary key:

```sql
CREATE TABLE chat_users (
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    PRIMARY KEY (chat_id, user_id)
);
```

## Database Tables

Alita Robot creates the following tables:

| Table | Purpose |
|-------|---------|
| `users` | Telegram user data |
| `chats` | Chat/group information |
| `chat_users` | User-chat membership |
| `warns_settings` | Warning configuration per chat |
| `warns_users` | User warning records |
| `greetings` | Welcome/goodbye messages |
| `filters` | Chat filters |
| `notes` | Saved notes |
| `notes_settings` | Notes configuration |
| `rules` | Chat rules |
| `blacklists` | Blacklisted words |
| `locks` | Lock settings |
| `pins` | Pin settings |
| `admin` | Admin settings |
| `antiflood_settings` | Anti-flood configuration |
| `connection` | Connection settings |
| `connection_settings` | Chat connection config |
| `disable` | Disabled commands |
| `disable_chat_settings` | Per-chat disable settings |
| `report_chat_settings` | Report configuration |
| `report_user_settings` | User report preferences |
| `devs` | Developer settings |
| `channels` | Linked channels |
| `captcha_settings` | Captcha configuration |
| `captcha_attempts` | Active captcha attempts |
| `captcha_muted_users` | Users muted due to captcha failure |
| `stored_messages` | Messages stored during captcha |
| `schema_migrations` | Migration tracking |

## Backup and Restore

:::caution
Always test your backup restoration process before relying on it in production. A backup that cannot be restored is worthless.
:::

### Backup

```bash
# Using pg_dump
pg_dump -h localhost -U postgres -d alita > backup.sql

# Compressed backup
pg_dump -h localhost -U postgres -d alita | gzip > backup.sql.gz

# Docker
docker-compose exec -T postgres pg_dump -U alita -d alita > backup.sql
```

### Restore

```bash
# From SQL file
psql -h localhost -U postgres -d alita < backup.sql

# From compressed file
gunzip -c backup.sql.gz | psql -h localhost -U postgres -d alita

# Docker
docker-compose exec -T postgres psql -U alita -d alita < backup.sql
```

## Troubleshooting

### Connection refused

```
Failed to connect to database: connection refused
```

- Verify PostgreSQL is running
- Check host and port in `DATABASE_URL`
- Ensure firewall allows connections on port 5432

### Authentication failed

```
password authentication failed for user
```

- Verify username and password in connection string
- Check `pg_hba.conf` authentication settings

### Too many connections

```
too many connections for role
```

- Reduce `DB_MAX_OPEN_CONNS`
- Increase PostgreSQL `max_connections` in `postgresql.conf`
- Consider using connection pooling (PgBouncer)

### Migration failed

```
Migration failed: column already exists
```

This usually means the migration was partially applied. Options:

1. Set `AUTO_MIGRATE_SILENT_FAIL=true` and let the bot continue
2. Manually fix the schema and mark the migration as applied:
   ```sql
   INSERT INTO schema_migrations (version, executed_at)
   VALUES ('problematic_migration.sql', NOW());
   ```

### Slow queries

Enable query logging in PostgreSQL:

```sql
ALTER SYSTEM SET log_min_duration_statement = 1000;  -- Log queries > 1 second
SELECT pg_reload_conf();
```

Or enable debug mode in the bot:

```bash
DEBUG=true
```
