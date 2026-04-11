# Database Schema Fixes Rollback Procedures

## Overview

This document provides comprehensive rollback procedures for all three phases of the database schema fixes implemented in the Alita Robot project. These procedures are critical for production safety and should be tested before deployment.

**Phases Covered:**
- **Phase 1**: CHECK Constraints (Migration: `20250411000000_add_database_constraints.sql`)
- **Phase 2**: Foreign Keys (Migration: `20250805204145_add_foreign_key_relations.sql`)
- **Phase 3**: Database Monitoring (Connection pool monitoring)

---

## Phase 1 Rollback (CHECK Constraints)

**Migration**: `20250411000000_add_database_constraints.sql`
**Risk Level**: LOW - Constraints only validate new data
**Rollback Time**: < 1 minute

### Immediate Rollback (SQL)

Execute this SQL to drop all CHECK constraints added in Phase 1:

```bash
# Drop all CHECK constraints
psql $DATABASE_URL << 'EOF'
BEGIN;

-- WARNINGS SYSTEM CONSTRAINTS
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_settings_limit;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_users_num;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_mode;

-- ANTIFLOOD CONSTRAINTS
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_limit;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;

-- BLACKLIST CONSTRAINTS
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklists_action;

-- CAPTCHA CONSTRAINTS
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_timeout;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_failure_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_max_attempts;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_mode;
ALTER TABLE captcha_attempts DROP CONSTRAINT IF EXISTS chk_captcha_expires_at;

COMMIT;
EOF
```

### Code Rollback (Git)

```bash
# Find the commit that added CHECK constraints
git log --oneline --grep="add.*constraint" | head -5

# Revert the commit
git revert <commit-hash>

# Rebuild and run
make build
make run
```

### Verification Steps

```bash
# Verify constraints were dropped
psql $DATABASE_URL -c "\d+ warns_settings"
# Should NOT show: chk_warns_settings_limit, chk_warns_mode

psql $DATABASE_URL -c "\d+ antiflood_settings"
# Should NOT show: chk_antiflood_limit, chk_antiflood_action, chk_antiflood_mode

psql $DATABASE_URL -c "\d+ captcha_settings"
# Should NOT show: chk_captcha_timeout, chk_captcha_failure_action, chk_captcha_max_attempts, chk_captcha_mode
```

---

## Phase 2 Rollback (Foreign Keys)

**Migration**: `20250805204145_add_foreign_key_relations.sql`
**Risk Level**: HIGH - Affects data integrity and relationships
**Rollback Time**: 5-30 minutes (depending on database size)

### ⚠️ CRITICAL WARNINGS

1. **Data Loss Risk**: Phase 2 migration includes `ON DELETE CASCADE` clauses. Rolling back will restore orphaned records.
2. **Backup Required**: Always take a fresh backup before rollback
3. **Downtime Expected**: Plan for application downtime during rollback

### Option A: Full Restore from Backup (Recommended)

This is the safest rollback method.

```bash
# 1. Stop the application
systemctl stop alita-robot
# OR if using Docker
docker-compose down

# 2. Take a pre-rollback backup (just in case)
pg_dump $DATABASE_URL | gzip > backups/pre_rollback_$(date +%Y%m%d_%H%M%S).sql.gz

# 3. Restore from pre-migration backup
gunzip -c backups/pre_migration_<timestamp>.sql.gz | psql $DATABASE_URL

# 4. Verify restore
psql $DATABASE_URL -c "SELECT COUNT(*) FROM chats;"
psql $DATABASE_URL -c "SELECT COUNT(*) FROM users;"

# 5. Rollback code to pre-migration commit
git tag | grep pre-fk-migration
git checkout pre-fk-migration

# 6. Rebuild and start application
make build
systemctl start alita-robot
```

### Option B: Manual FK Constraint Removal

Use this if you need to keep recent data but remove FK constraints.

```bash
# Stop the application
systemctl stop alita-robot

# Drop all foreign key constraints
psql $DATABASE_URL << 'EOF'
BEGIN;

-- CHAT-RELATED FOREIGN KEYS
ALTER TABLE admin DROP CONSTRAINT IF EXISTS fk_admin_chat;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS fk_antiflood_chat;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS fk_blacklists_chat;
ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_chat;
ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_channel;
ALTER TABLE connection_settings DROP CONSTRAINT IF EXISTS fk_connection_settings_chat;
ALTER TABLE disable DROP CONSTRAINT IF EXISTS fk_disable_chat;
ALTER TABLE filters DROP CONSTRAINT IF EXISTS fk_filters_chat;
ALTER TABLE greetings DROP CONSTRAINT IF EXISTS fk_greetings_chat;
ALTER TABLE locks DROP CONSTRAINT IF EXISTS fk_locks_chat;
ALTER TABLE notes DROP CONSTRAINT IF EXISTS fk_notes_chat;
ALTER TABLE notes_settings DROP CONSTRAINT IF EXISTS fk_notes_settings_chat;
ALTER TABLE pins DROP CONSTRAINT IF EXISTS fk_pins_chat;
ALTER TABLE report_chat_settings DROP CONSTRAINT IF EXISTS fk_report_chat_settings_chat;
ALTER TABLE rules DROP CONSTRAINT IF EXISTS fk_rules_chat;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS fk_warns_settings_chat;

-- USER-RELATED FOREIGN KEYS
ALTER TABLE devs DROP CONSTRAINT IF EXISTS fk_devs_user;
ALTER TABLE report_user_settings DROP CONSTRAINT IF EXISTS fk_report_user_settings_user;

-- MANY-TO-MANY RELATIONSHIP TABLES
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_chat;
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_user;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_chat;

-- UNIQUE CONSTRAINTS
ALTER TABLE chats DROP CONSTRAINT IF EXISTS uk_chats_chat_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS uk_users_user_id;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS uk_connection_user_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS uk_warns_users_user_chat;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS uk_blacklists_chat_word;
ALTER TABLE disable DROP CONSTRAINT IF EXISTS uk_disable_chat_command;
ALTER TABLE filters DROP CONSTRAINT IF EXISTS uk_filters_chat_keyword;
ALTER TABLE locks DROP CONSTRAINT IF EXISTS uk_locks_chat_type;
ALTER TABLE notes DROP CONSTRAINT IF EXISTS uk_notes_chat_name;

-- CHECK CONSTRAINTS (if added in Phase 2)
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_settings_limit;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_limit;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_users_num;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklists_action;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_mode;

COMMIT;
EOF

# Verify constraints dropped
psql $DATABASE_URL -c "
SELECT
    conname AS constraint_name,
    conrelid::regclass AS table_name
FROM pg_constraint
WHERE conname LIKE 'fk_%' OR conname LIKE 'uk_%' OR conname LIKE 'chk_%';
"
# Should return empty set

# Rollback code
git checkout pre-fk-migration
make build

# Start application
systemctl start alita-robot
```

### Option C: Git Rollback Only

Use this if no FK constraints were actually applied (migration failed).

```bash
# Identify rollback point
git tag | grep pre-fk-migration

# Checkout pre-migration code
git checkout pre-fk-migration

# Rebuild and restart
make build
systemctl restart alita-robot
```

### Verification Steps

```bash
# 1. Check no FK constraints exist
psql $DATABASE_URL -c "
SELECT COUNT(*) FROM pg_constraint
WHERE contype = 'f' AND conname LIKE 'fk_%';
"
# Should return 0

# 2. Check no unique constraints on natural keys
psql $DATABASE_URL -c "
SELECT COUNT(*) FROM pg_constraint
WHERE contype = 'u' AND conname LIKE 'uk_%';
"
# Should return 0

# 3. Test application functionality
# - Create a test chat
# - Add a test user
# - Create test warnings
# - Verify no FK errors in logs
```

---

## Phase 3 Rollback (Monitoring)

**Component**: Database connection pool monitoring
**Risk Level**: LOW - Monitoring is non-blocking
**Rollback Time**: < 1 minute

### Disable Monitoring (Environment Variable)

```bash
# Disable monitoring via environment variable
export ENABLE_DB_MONITORING=false

# Or add to .env file
echo "ENABLE_DB_MONITORING=false" >> .env

# Restart application
systemctl restart alita-robot
```

### Code Rollback (Git)

```bash
# Find monitoring-related commits
git log --oneline --grep="monitoring" | head -5

# Revert monitoring commits
git revert <monitoring-commit-hash>

# Rebuild and restart
make build
systemctl restart alita-robot
```

### Verification Steps

```bash
# Check logs for monitoring messages
journalctl -u alita-robot -f | grep -i "monitoring"
# Should show no monitoring messages

# Verify application still functions
# - Database operations work normally
# - No errors in logs
```

---

## Rollback Success Criteria

### Phase 1 Success Criteria

- [ ] All CHECK constraints dropped from database
- [ ] No application errors after rollback
- [ ] All tests passing: `make test`
- [ ] Database operations function normally
- [ ] No constraint violations in logs

**Verification Commands:**
```bash
# Verify no CHECK constraints
psql $DATABASE_URL -c "
SELECT COUNT(*) FROM pg_constraint
WHERE contype = 'c' AND conname LIKE 'chk_%';
"
# Should return 0

# Run tests
make test
```

### Phase 2 Success Criteria

- [ ] Backup restored successfully OR all FK constraints dropped
- [ ] Application starts without errors
- [ ] No foreign key constraint errors in logs
- [ ] Data integrity verified (counts match expected)
- [ ] All CRUD operations work correctly
- [ ] No performance degradation

**Verification Commands:**
```bash
# Verify no FK constraints
psql $DATABASE_URL -c "
SELECT COUNT(*) FROM pg_constraint
WHERE contype = 'f' AND conname LIKE 'fk_%';
"
# Should return 0

# Verify data integrity
psql $DATABASE_URL -c "
SELECT
    (SELECT COUNT(*) FROM chats) as chats,
    (SELECT COUNT(*) FROM users) as users,
    (SELECT COUNT(*) FROM admin) as admin,
    (SELECT COUNT(*) FROM warns_users) as warns_users;
"

# Check application logs
journalctl -u alita-robot -n 100 | grep -i "error"
```

### Phase 3 Success Criteria

- [ ] Monitoring disabled (no monitoring logs)
- [ ] No performance impact
- [ ] Application remains stable
- [ ] Database operations work normally
- [ ] Resource usage normal

**Verification Commands:**
```bash
# Check monitoring is disabled
echo $ENABLE_DB_MONITORING
# Should return "false" or empty

# Check logs
journalctl -u alita-robot -n 50 | grep -i "monitoring"
# Should return no results

# Verify database connectivity
psql $DATABASE_URL -c "SELECT 1;"
```

---

## Rollback Testing Procedures

### Test Phase 1 Rollback (CHECK Constraints)

**Prerequisites**: Test database with CHECK constraints applied

```bash
# 1. Verify constraints exist before rollback
psql $DATABASE_URL -c "\d+ warns_settings" | grep chk_warns_settings_limit

# 2. Apply rollback SQL
psql $DATABASE_URL << 'EOF'
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_settings_limit;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_users_num;
EOF

# 3. Verify constraint dropped
psql $DATABASE_URL -c "\d+ warns_settings"
# Should NOT show chk_warns_settings_limit

# 4. Test application accepts invalid data (should work now)
# This would previously fail with constraint
# (Only test in non-production environment)

# 5. Run tests
make test
```

### Test Phase 2 Rollback (Foreign Keys)

**⚠️ WARNING: Only test in non-production environment**

```bash
# 1. Create test backup
pg_dump $DATABASE_URL | gzip > backups/test_rollback_backup.sql.gz

# 2. Verify FK constraints exist
psql $DATABASE_URL -c "
SELECT COUNT(*) FROM pg_constraint
WHERE contype = 'f' AND conname LIKE 'fk_%';
"

# 3. Test Option A: Backup restore
# (Skip if you don't want to test full restore)

# 4. Test Option B: Manual FK removal
psql $DATABASE_URL << 'EOF'
BEGIN;
ALTER TABLE admin DROP CONSTRAINT IF EXISTS fk_admin_chat;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS fk_antiflood_chat;
-- Add more constraints as needed
COMMIT;
EOF

# 5. Verify FK constraints dropped
psql $DATABASE_URL -c "
SELECT COUNT(*) FROM pg_constraint
WHERE contype = 'f' AND conname LIKE 'fk_%';
"
# Should return 0

# 6. Test orphaned data can exist (would fail with FKs)
psql $DATABASE_URL << 'EOF'
-- This should succeed without FK constraints
INSERT INTO admin (chat_id, user_id) VALUES (999999999, 999999999);
EOF

# 7. Clean up test data
psql $DATABASE_URL -c "DELETE FROM admin WHERE chat_id = 999999999;"

# 8. Restore test backup if needed
# gunzip -c backups/test_rollback_backup.sql.gz | psql $DATABASE_URL
```

### Test Phase 3 Rollback (Monitoring)

```bash
# 1. Verify monitoring is enabled
grep ENABLE_DB_MONITORING .env
# Should show "true" or be unset (default enabled)

# 2. Check logs show monitoring
journalctl -u alita-robot -n 20 | grep -i "monitoring"

# 3. Disable monitoring
export ENABLE_DB_MONITORING=false
systemctl restart alita-robot

# 4. Verify monitoring disabled
echo $ENABLE_DB_MONITORING
# Should return "false"

# 5. Check logs show no monitoring
journalctl -u alita-robot -n 20 | grep -i "monitoring"
# Should return no results

# 6. Verify application still works
psql $DATABASE_URL -c "SELECT 1;"
```

---

## Emergency Rollback Procedure

**Use this when something goes wrong and you need immediate rollback.**

### Immediate Actions

```bash
# 1. STOP THE APPLICATION
systemctl stop alita-robot
# OR
docker-compose down

# 2. Assess the situation
# - Check logs: journalctl -u alita-robot -n 100
# - Check database: psql $DATABASE_URL -c "SELECT COUNT(*) FROM chats;"
# - Check constraints: psql $DATABASE_URL -c "\d"

# 3. Choose rollback path:
#    - Phase 1: SQL rollback (fast)
#    - Phase 2: Backup restore (safest)
#    - Phase 3: Disable env var (fastest)

# 4. Execute rollback
# (Use procedures above)

# 5. Verify and start
systemctl start alita-robot

# 6. Monitor closely
journalctl -u alita-robot -f
```

### When to Use Each Rollback

| Situation | Recommended Rollback | Time Required |
|-----------|---------------------|---------------|
| Application crashes on startup | Phase 1 SQL rollback | < 1 min |
| Data corruption or integrity errors | Phase 2 Backup restore | 15-30 min |
| Performance degradation | Phase 3 Disable monitoring | < 1 min |
| Unknown issues | Full rollback (all phases) | 30-60 min |

---

## Pre-Rollback Checklist

Before performing any rollback, ensure:

- [ ] **Backup exists**: You have a recent backup (less than 24 hours old)
- [ ] **Issue identified**: You know exactly what's wrong
- [ ] **Rollback plan chosen**: You've selected the appropriate rollback procedure
- [ ] **Stakeholders notified**: Relevant team members know about the rollback
- [ ] **Maintenance window**: You have downtime scheduled if needed
- [ ] **Testing environment**: Rollback procedures tested in staging first
- [ ] **Documentation updated**: Post-rollback notes will be recorded

---

## Post-Rollback Actions

After completing any rollback:

1. **Verify Application Health**
   ```bash
   # Check application logs
   journalctl -u alita-robot -n 100

   # Verify database connectivity
   psql $DATABASE_URL -c "SELECT 1;"

   # Check critical functionality
   # (Test bot commands, database operations, etc.)
   ```

2. **Document the Incident**
   - What went wrong
   - What rollback was performed
   - Timestamp of rollback
   - Any data loss or corruption
   - Lessons learned

3. **Root Cause Analysis**
   - Why did the issue occur?
   - How can it be prevented in the future?
   - Are additional safeguards needed?

4. **Plan Forward Migration**
   - How to fix the issue that caused rollback
   - Testing requirements before retry
   - Approval process for next attempt

---

## Automated Rollback Script

For emergency situations, use this automated rollback script:

```bash
#!/bin/bash
# emergency-rollback.sh - Automated rollback script

set -e

echo "=== EMERGENCY ROLLBACK SCRIPT ==="
echo "This will rollback all database schema changes"
echo ""
read -p "Are you sure? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

echo "Stopping application..."
systemctl stop alita-robot

echo "Creating pre-rollback backup..."
pg_dump $DATABASE_URL | gzip > "backups/emergency_pre_rollback_$(date +%Y%m%d_%H%M%S).sql.gz"

echo "Rolling back Phase 1 (CHECK constraints)..."
psql $DATABASE_URL << 'EOF'
BEGIN;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_settings_limit;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS chk_warns_users_num;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_limit;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_action;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS chk_antiflood_mode;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS chk_blacklists_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_timeout;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_failure_action;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_max_attempts;
ALTER TABLE captcha_settings DROP CONSTRAINT IF EXISTS chk_captcha_mode;
ALTER TABLE captcha_attempts DROP CONSTRAINT IF EXISTS chk_captcha_expires_at;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS chk_warns_mode;
COMMIT;
EOF

echo "Rolling back Phase 2 (Foreign keys)..."
psql $DATABASE_URL << 'EOF'
BEGIN;
ALTER TABLE admin DROP CONSTRAINT IF EXISTS fk_admin_chat;
ALTER TABLE antiflood_settings DROP CONSTRAINT IF EXISTS fk_antiflood_chat;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS fk_blacklists_chat;
ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_chat;
ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_channel;
ALTER TABLE connection_settings DROP CONSTRAINT IF EXISTS fk_connection_settings_chat;
ALTER TABLE disable DROP CONSTRAINT IF EXISTS fk_disable_chat;
ALTER TABLE filters DROP CONSTRAINT IF EXISTS fk_filters_chat;
ALTER TABLE greetings DROP CONSTRAINT IF EXISTS fk_greetings_chat;
ALTER TABLE locks DROP CONSTRAINT IF EXISTS fk_locks_chat;
ALTER TABLE notes DROP CONSTRAINT IF EXISTS fk_notes_chat;
ALTER TABLE notes_settings DROP CONSTRAINT IF EXISTS fk_notes_settings_chat;
ALTER TABLE pins DROP CONSTRAINT IF EXISTS fk_pins_chat;
ALTER TABLE report_chat_settings DROP CONSTRAINT IF EXISTS fk_report_chat_settings_chat;
ALTER TABLE rules DROP CONSTRAINT IF EXISTS fk_rules_chat;
ALTER TABLE warns_settings DROP CONSTRAINT IF EXISTS fk_warns_settings_chat;
ALTER TABLE devs DROP CONSTRAINT IF EXISTS fk_devs_user;
ALTER TABLE report_user_settings DROP CONSTRAINT IF EXISTS fk_report_user_settings_user;
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_chat;
ALTER TABLE chat_users DROP CONSTRAINT IF EXISTS fk_chat_users_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_user;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS fk_connection_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_user;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS fk_warns_users_chat;
ALTER TABLE chats DROP CONSTRAINT IF EXISTS uk_chats_chat_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS uk_users_user_id;
ALTER TABLE connection DROP CONSTRAINT IF EXISTS uk_connection_user_chat;
ALTER TABLE warns_users DROP CONSTRAINT IF EXISTS uk_warns_users_user_chat;
ALTER TABLE blacklists DROP CONSTRAINT IF EXISTS uk_blacklists_chat_word;
ALTER TABLE disable DROP CONSTRAINT IF EXISTS uk_disable_chat_command;
ALTER TABLE filters DROP CONSTRAINT IF EXISTS uk_filters_chat_keyword;
ALTER TABLE locks DROP CONSTRAINT IF EXISTS uk_locks_chat_type;
ALTER TABLE notes DROP CONSTRAINT IF EXISTS uk_notes_chat_name;
COMMIT;
EOF

echo "Rolling back Phase 3 (Monitoring)..."
export ENABLE_DB_MONITORING=false

echo "Verifying rollback..."
FK_COUNT=$(psql $DATABASE_URL -tAc "SELECT COUNT(*) FROM pg_constraint WHERE contype = 'f' AND conname LIKE 'fk_%';")
CHK_COUNT=$(psql $DATABASE_URL -tAc "SELECT COUNT(*) FROM pg_constraint WHERE contype = 'c' AND conname LIKE 'chk_%';")

if [ "$FK_COUNT" -eq 0 ] && [ "$CHK_COUNT" -eq 0 ]; then
    echo "✓ Rollback successful: All constraints dropped"
else
    echo "✗ Rollback incomplete: $FK_COUNT FKs, $CHK_COUNT CHECKs remain"
    exit 1
fi

echo "Restarting application..."
systemctl start alita-robot

echo "=== ROLLBACK COMPLETE ==="
echo "Please verify application is functioning correctly"
```

Save as `scripts/emergency-rollback.sh` and make executable:
```bash
chmod +x scripts/emergency-rollback.sh
```

---

## Support and Contacts

### Emergency Contacts
- **Database Administrator**: [Contact details to be filled]
- **On-Call Engineer**: [Contact details to be filled]
- **Rollback Decision Maker**: [Contact details to be filled]
- **Engineering Lead**: [Contact details to be filled]

### Documentation Links
- [Database Migration Documentation](./database-migration-checklist.md)
- [Deployment Checklist](./deployment-checklist.md)
- [Incident Response Plan](./incident-response.md)

### Related Resources
- PostgreSQL Documentation: [ALTER TABLE](https://www.postgresql.org/docs/current/sql-altertable.html)
- GORM Documentation: [Constraints](https://gorm.io/docs/constraints.html)
- Git Documentation: [git-revert](https://git-scm.com/docs/git-revert)

---

## Revision History

| Date | Version | Author | Changes |
|------|---------|--------|---------|
| 2025-04-11 | 1.0 | Droid | Initial rollback documentation |

---

**Last Updated**: 2025-04-11
**Status**: Ready for Review
**Next Review**: Before production deployment
