# Database Schema Fixes Deployment Checklist

## Pre-Deployment
- [ ] All tests passing: `make test`
- [ ] Linting passing: `make lint`
- [ ] Validation script shows no orphaned records: `make validate-db`
- [ ] Database backup created: `make backup-db`
- [ ] Rollback procedure documented and tested

## Phase 1 Deployment (Critical Fixes - Deploy Today)
- [ ] Model synchronization verified (Task 1)
- [ ] CHECK constraints migration deployed to dev
- [ ] All tests passing with new constraints
- [ ] Performance regression tests passing

## Phase 2 Deployment (Data Safety - Deploy This Week)
- [ ] Orphaned data cleanup performed in staging
- [ ] Foreign key migration deployed to staging
- [ ] All tests passing with FK constraints
- [ ] Performance impact validated

## Production Deployment
- [ ] Backup created immediately before deployment
- [ ] AUTO_MIGRATE=true enabled in production environment
- [ ] Migration applied successfully
- [ ] Post-deployment validation passing
- [ ] Monitoring shows no performance degradation
- [ ] Rollback ready if issues arise

## Post-Deployment
- [ ] Monitor database performance for 24 hours
- [ ] Validate application functionality
- [ ] Document any issues and resolutions
- [ ] Archive backup for retention period

## Critical Success Criteria
✅ **Phase 1 Success:**
- All CHECK constraints deployed
- No application errors
- Tests passing
- Zero performance impact

✅ **Phase 2 Success:**
- Foreign key constraints deployed
- No orphaned records
- Referential integrity enforced
- Tests passing

✅ **Phase 3 Success (Optional):**
- Monitoring active
- Performance metrics collected
- System stable under load

## Emergency Rollback Procedure
If critical issues arise:
1. Immediate: `git checkout pre-fk-migration`
2. Database: Restore from backup
3. Restart application
4. Verify functionality
5. Investigate logs
