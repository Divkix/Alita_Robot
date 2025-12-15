# MongoDB to PostgreSQL Migration Guide

This guide explains how to migrate your Alita Robot data from MongoDB to PostgreSQL using the built-in migration tool.

## Overview

The migration tool (`cmd/migrate`) provides a complete solution for migrating all Alita Robot data from MongoDB to PostgreSQL with batch processing, error handling, and validation.

## Prerequisites

1. **MongoDB instance** with existing Alita Robot data
2. **PostgreSQL database** with the schema already created
3. **Go 1.21+** installed on your system
4. **Network access** to both databases

## Quick Start

### 1. Prepare PostgreSQL Database

First, ensure your PostgreSQL database has the correct schema:

```bash
# Apply the initial migration
psql -d your_database < migrations/20250805200527_initial_migration.sql
psql -d your_database < migrations/20250805204145_add_foreign_key_relations.sql
```

### 2. Build the Migration Tool

```bash
cd cmd/migrate
go build -o migrate
```

### 3. Configure Environment

Create a `.env` file in the `cmd/migrate` directory:

```env
# MongoDB Configuration
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=alita

# PostgreSQL Configuration
DATABASE_URL=postgres://user:password@localhost:5432/alita_db?sslmode=disable
```

### 4. Run Migration

```bash
# Test run (recommended first)
./migrate -dry-run -verbose

# Actual migration
./migrate -verbose
```

## Configuration Options

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MONGO_URI` | MongoDB connection string | Required |
| `MONGO_DATABASE` | MongoDB database name | `alita` |
| `DATABASE_URL` | PostgreSQL connection string | Required |

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-mongo-uri` | MongoDB connection URI | From env |
| `-mongo-db` | MongoDB database name | `alita` |
| `-postgres-dsn` | PostgreSQL connection DSN | From env |
| `-batch-size` | Records per batch | `1000` |
| `-dry-run` | Test without writing data | `false` |
| `-verbose` | Enable detailed logging | `false` |

## Migration Process

### Collections Migrated

The tool migrates data in this order:

**Primary Collections** (migrated first):
1. `users` - User profiles and settings
2. `chats` - Chat groups and metadata

**Dependent Collections**:
3. `admin` - Admin permissions per chat
4. `notes_settings` - Note configuration
5. `notes` - Saved notes and messages
6. `filters` - Auto-response filters
7. `greetings` - Welcome/goodbye messages
8. `locks` - Permission restrictions
9. `pins` - Pin settings
10. `rules` - Chat rules
11. `warns_settings` - Warning configuration
12. `warns_users` - User warnings
13. `antiflood_settings` - Anti-flood settings
14. `blacklists` - Blacklisted words
15. `channels` - Linked channels
16. `connection` - User-chat connections
17. `connection_settings` - Connection config
18. `disable` - Disabled commands
19. `report_user_settings` - User report settings
20. `report_chat_settings` - Chat report settings

### Data Transformations

The migration handles several complex transformations:

- **Nested documents** → Flattened structures
- **Arrays** → JSONB columns or separate rows
- **MongoDB Long types** → PostgreSQL bigint
- **Missing fields** → Appropriate defaults
- **Complex objects** → Normalized relations

## Usage Examples

### Basic Migration

```bash
./migrate
```

### Custom Configuration

```bash
./migrate \
  -mongo-uri="mongodb://mongo.example.com:27017" \
  -mongo-db="alita_prod" \
  -postgres-dsn="postgres://user:pass@pg.example.com/alita" \
  -batch-size=500 \
  -verbose
```

### Dry Run Testing

```bash
# Test the migration without writing data
./migrate -dry-run -verbose
```

### Large Dataset Migration

```bash
# Use smaller batches for large datasets
./migrate -batch-size=100 -verbose
```

## Monitoring Progress

The tool provides real-time progress information:

```
Starting migration...
Migrating collection: users
Migrating collection: chats
Loaded 1250 valid chat IDs
Loaded 5430 valid user IDs
Migrating collection: admin
...

=== Migration Statistics ===
Duration: 2m34s
Total Collections: 20
Total Records: 45,230
Successful Records: 45,180
Failed Records: 50
Success Rate: 99.89%
```

## Error Handling

### Common Issues

**Connection Errors**:
```bash
# Check MongoDB connectivity
mongo --eval "db.runCommand('ping')" mongodb://localhost:27017/alita

# Check PostgreSQL connectivity
psql -d "postgres://user:pass@localhost/alita" -c "SELECT 1"
```

**Schema Errors**:
- Ensure PostgreSQL schema is up-to-date
- Run all migration SQL files before data migration

**Memory Issues**:
- Reduce batch size: `-batch-size=100`
- Monitor system resources during migration

### Recovery

If migration fails partway through:

1. **Check the error logs** for specific issues
2. **Fix the underlying problem** (schema, connectivity, etc.)
3. **Re-run the migration** - the tool uses upserts to handle existing data
4. **Use dry-run mode** to test fixes before actual migration

## Validation

### Post-Migration Checks

1. **Verify record counts**:
```sql
-- PostgreSQL
SELECT 'users' as table_name, COUNT(*) FROM users
UNION ALL
SELECT 'chats', COUNT(*) FROM chats
UNION ALL
SELECT 'notes', COUNT(*) FROM notes;
```

```javascript
// MongoDB
db.users.countDocuments()
db.chats.countDocuments()
db.notes.countDocuments()
```

2. **Test application functionality**:
- Start the bot with PostgreSQL configuration
- Test core features (notes, filters, admin commands)
- Verify user settings are preserved

3. **Check data integrity**:
```sql
-- Verify foreign key relationships
SELECT COUNT(*) FROM notes WHERE chat_id NOT IN (SELECT chat_id FROM chats);
SELECT COUNT(*) FROM admin WHERE chat_id NOT IN (SELECT chat_id FROM chats);
```

## Performance Optimization

### During Migration

- **Batch size**: Start with 1000, adjust based on performance
- **Network**: Run migration tool close to databases
- **Resources**: Ensure adequate RAM and CPU

### Post-Migration

```sql
-- Update PostgreSQL statistics
ANALYZE;

-- Consider additional indexes
CREATE INDEX CONCURRENTLY idx_notes_chat_id ON notes(chat_id);
CREATE INDEX CONCURRENTLY idx_filters_chat_id ON filters(chat_id);
```

## Troubleshooting

### Debug Mode

```bash
# Maximum verbosity
./migrate -verbose -dry-run -batch-size=10
```

### Common Solutions

| Issue | Solution |
|-------|----------|
| Connection timeout | Check network, increase timeout in connection string |
| Out of memory | Reduce batch size (`-batch-size=100`) |
| Foreign key errors | Ensure users/chats are migrated first |
| Duplicate key errors | Normal with upserts, check for actual data issues |
| Schema mismatch | Update PostgreSQL schema before migration |

### Getting Help

1. Check the migration logs for specific error messages
2. Run with `-verbose` flag for detailed information
3. Use `-dry-run` to test without making changes
4. Verify database connectivity independently
5. Check that all PostgreSQL migrations have been applied

## Security Considerations

- **Credentials**: Store database credentials securely
- **Network**: Use encrypted connections in production
- **Access**: Limit migration tool access to necessary databases only
- **Backup**: Always backup both databases before migration

## Best Practices

1. **Test first**: Always run with `-dry-run` before actual migration
2. **Backup**: Create backups of both MongoDB and PostgreSQL
3. **Monitor**: Watch system resources during migration
4. **Validate**: Verify data integrity after migration
5. **Staged approach**: Consider migrating in stages for very large datasets
