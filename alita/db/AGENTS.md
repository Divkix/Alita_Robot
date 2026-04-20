# alita/db - Database Layer Guidelines

**Scope:** PostgreSQL + GORM database operations, caching, migrations

## WHERE TO LOOK

| Task | File Pattern | Notes |
|------|--------------|-------|
| Models/Schema | `db.go` | GORM models, connection setup, pool config |
| Domain Operations | `*_db.go` | `Get*`, `Add*`, `Update*`, `Delete*` functions |
| Cache Helpers | `cache_helpers.go` | TTL management, invalidation patterns |
| Optimized Queries | `optimized_queries.go` | Minimal column selection, singleflight caching |
| Migrations Engine | `migrations.go` | Runtime migration execution |
| Schema SQL | `../../migrations/*.sql` | Timestamped source of truth |

## CONVENTIONS

### Surrogate Key Pattern
- Auto-increment `id` as Primary Key
- External IDs (`user_id`, `chat_id`) = unique constraints only

### Cache Key Format
```go
"alita:{module}:{identifier}"  // e.g., "alita:adminCache:123"
```

### Cache Invalidation Rule (CRITICAL)
**Every DB update MUST invalidate the corresponding cache key.**

```go
// Pattern: update DB → invalidate cache
if err := db.UpdateChat(chat); err != nil {
    return err
}
cacheKey := fmt.Sprintf("alita:chat:%d", chat.ID)
cache.Marshal.Delete(cache.Context, cacheKey)
```

### Singleflight Protection
Use `getFromCacheOrLoad()` in `cache_helpers.go` for thundering herd prevention:
```go
result, err := getFromCacheOrLoad(cacheKey, ttl, func() (any, error) {
    return db.GetUser(userID)
})
```

### Custom GORM Types
Implement driver interfaces for JSONB arrays:
```go
func (a *ArrayType) Scan(value any) error { ... }
func (a ArrayType) Value() (driver.Value, error) { ... }
```

### Error Handling
- **Never ignore DB errors with `_`** — nil returns cause panics
- Wrap with context: `errors.Wrap(err, "context")`

## ANTI-PATTERNS

| Pattern | Why Wrong | Correct Approach |
|---------|-----------|------------------|
| `result, _ := db.Query()` | Silent nil panics | Always check `err` |
| `go db.Update()` (fire-forget) | Loses errors, race conditions | Synchronous or proper async wrapper |
| Using external ID as PK | GORM limitations | Surrogate key + unique constraint |
| Direct cache Get/Set for reads | Cache stampedes | Use `getFromCacheOrLoad()` |

## INDEXING GUIDELINES

Add composite indexes for frequent patterns:
```sql
CREATE INDEX idx_user_chat ON table(user_id, chat_id);
```
