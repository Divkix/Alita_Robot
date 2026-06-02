# Spec: Split Monolithic `alita/db` Package into Domain-Focused Sub-Packages

**Status:** Draft  
**Author:** Plan Agent  
**Date:** 2026-05-31  
**Approach:** Hybrid (Infrastructure First, Then Domains)

---

## 1. Goal

Improve compile times, testability, and domain clarity by decomposing the 942-line `alita/db/db.go` god file and 22 domain-specific `*_db.go` files into focused sub-packages with stable boundaries.

**Non-goals:**
- No behavior changes to any handler or module
- No database schema changes
- No changes to the Telegram bot logic
- No introduction of new dependencies

---

## 2. Current State

### 2.1 File Inventory

| File | Lines | Responsibility |
|------|-------|----------------|
| `alita/db/db.go` | 942 | Connection setup, ALL GORM models (~25), custom types, generic CRUD helpers |
| `alita/db/cache_helpers.go` | 153 | Cache TTL constants, `CacheKey()`, `getFromCacheOrLoad()`, singleflight group |
| `alita/db/optimized_queries.go` | 561 | Optimized SELECT queries with minimal column selection |
| `alita/db/backup_db.go` | 881 | Import/export with large switch statements for 13 modules |
| `alita/db/migrations.go` | ~200 | Custom SQL migration runner |
| `alita/db/monitoring.go` | ~100 | Database pool metrics |
| `alita/db/*_db.go` (22 files) | 50-200 each | Domain-specific queries (bans, locks, filters, notes, etc.) |

### 2.2 Import Graph

- **84 files** across the codebase import `alita/db`
- Every module handler file imports the full package to access 2-3 functions
- `db.go` is the compilation bottleneck: any model change triggers recompilation of all 84 consumers

### 2.3 Key Pain Points

1. **God file:** `db.go` contains connection logic, 25 models, custom types, and CRUD helpers
2. **Hidden coupling:** `backup_db.go` imports all domain logic via switch statements
3. **Global state:** `var DB *gorm.DB` accessed directly by all domain files and external packages
4. **Cache entanglement:** `cache_helpers.go` depends on `alita/utils/cache` but lives in `db/`
5. **Testability:** Cannot mock database access without monkey-patching `db.DB`

---

## 3. Proposed Architecture

### 3.1 Target Structure

```
alita/db/
  conn.go              # DB handle + connection setup (temporary, see Phase 4)
  models/
    types.go           # ButtonArray, StringArray, Int64Array
    user.go            # User, Chat, ChatUser
    chat_settings.go   # Chat, ChatUser, etc.
    bans.go            # BanSettings
    locks.go           # LockSettings
    filters.go         # ChatFilters
    notes.go           # Notes, NotesSettings
    ...                # One file per model (or grouped by domain)
  cache/
    keys.go            # CacheKey()
    loader.go          # getFromCacheOrLoad()
    ttl.go             # TTL constants
  queries/             # OPTIONAL: if domain packages are too small
    optimized.go       # OptimizedLockQueries, OptimizedUserQueries
  bans/                # Domain: ban queries + model access
    repository.go      # BanRepository interface + implementation
  locks/               # Domain: lock queries
    repository.go
  filters/             # Domain: filter queries
    repository.go
  notes/               # Domain: note queries
    repository.go
  ...                  # (20+ domain packages)
  backup/
    export.go          # ExportModuleData
    import.go          # ImportModuleData
    types.go           # BackupFormat, module constants
  migrations/
    runner.go           # MigrationRunner
    status.go           # Migration status helpers
  monitoring/
    metrics.go          # DatabaseMetrics, StartMonitoring
```

### 3.2 Design Principles

1. **Models are data, not behavior:** GORM structs live in `models/` with no methods except `TableName()`
2. **Domain packages own their queries:** Each domain package (`bans/`, `locks/`) contains the functions that operate on its model
3. **Cache is infrastructure:** `cache/` package provides generic loading primitives; domains call them
4. **Backup is orchestration:** `backup/` imports all domains but contains no models itself
5. **Connection is temporary:** `conn.go` holds the global `DB` during migration; eventually injected (see Candidate 1)

---

## 4. Phased Migration Plan

### Phase 1: Extract Infrastructure (No Behavior Change)

**Goal:** Create stable foundation packages that other code can depend on.

**Files to create:**
1. `alita/db/models/types.go` — Move `ButtonArray`, `StringArray`, `Int64Array`
2. `alita/db/models/user.go` — Move `User`, `Chat`, `ChatUser`
3. `alita/db/models/*.go` — Move each GORM model to its own file (or domain-grouped)
4. `alita/db/cache/keys.go` — Move `CacheKey()`
5. `alita/db/cache/ttl.go` — Move TTL constants
6. `alita/db/cache/loader.go` — Move `getFromCacheOrLoad()`
7. `alita/db/conn.go` — Move `DB` global and connection setup from `db.go`

**Files to modify:**
- `alita/db/db.go` — Remove everything except re-exports for backward compatibility

**Backward compatibility:**
```go
// alita/db/db.go (compatibility shim)
package db

import (
    "github.com/divkix/Alita_Robot/alita/db/models"
    "github.com/divkix/Alita_Robot/alita/db/cache"
    "github.com/divkix/Alita_Robot/alita/db/conn"
)

// Re-export for backward compatibility during migration
var DB = conn.DB
var CacheKey = cache.CacheKey
var getFromCacheOrLoad = cache.GetFromCacheOrLoad
// ... etc
```

**Validation:**
- `make test` passes
- `make build` passes
- No import changes needed in any consumer file

### Phase 2: Migrate Domain Packages (One at a Time)

**Goal:** Move domain query files to their own packages.

**Order (smallest/easiest first):**
1. `locks/` — Small, well-contained, already has `OptimizedLockQueries`
2. `pins/` — Very small (~50 lines)
3. `rules/` — Simple CRUD
4. `notes/` — Medium complexity, good test coverage
5. `filters/` — Uses keyword matcher, interesting cache patterns
6. `greetings/` — Embedded structs (`WelcomeSettings`)
7. `bans/` / `mute/` / `warns/` — Moderation group
8. `admin/` — Permission-related
9. `antiflood/` / `captcha/` / `antiraid/` — Anti-spam group
10. `connections/` / `disabling/` / `lang/` — Utility settings
11. `chats/` / `users/` — Core entities
12. `devs/` / `channels/` / `reports/` / `approvals/` — Remaining

**Per-domain migration steps:**
1. Create `alita/db/<domain>/repository.go`
2. Move model from `models/` to `<domain>/model.go` (optional — can keep in `models/`)
3. Move query functions from `<domain>_db.go` to `<domain>/repository.go`
4. Update imports in `alita/db/db.go` compatibility shim
5. Run `make test` and `make lint`

**Example: locks migration**
```go
// alita/db/locks/repository.go
package locks

import (
    "gorm.io/gorm/clause"
    "github.com/divkix/Alita_Robot/alita/db/cache"
    "github.com/divkix/Alita_Robot/alita/db/conn"
    "github.com/divkix/Alita_Robot/alita/db/models"
)

func GetChatLocks(chatID int64) map[string]bool {
    // ... moved from locks_db.go
}

func UpdateLock(chatID int64, perm string, val bool) error {
    // ... moved from locks_db.go
}
```

### Phase 3: Migrate Backup Package

**Goal:** Move `backup_db.go` to `alita/db/backup/`.

**Challenge:** `backup_db.go` has switch statements calling functions from all 13 domains.

**Solution:** After all domains are migrated, `backup/` can import them cleanly:
```go
import (
    "github.com/divkix/Alita_Robot/alita/db/bans"
    "github.com/divkix/Alita_Robot/alita/db/locks"
    "github.com/divkix/Alita_Robot/alita/db/filters"
    // ... etc
)
```

### Phase 4: (Optional) Remove Compatibility Shim

**Goal:** Once all consumers are updated, remove `alita/db/db.go` re-exports.

**This is a separate effort** (Candidate 1: Dependency Graph) and out of scope for this spec. The shim can remain indefinitely without harm.

---

## 5. Backward Compatibility Strategy

### 5.1 Compatibility Shim

The existing `alita/db` package will remain as a thin re-export layer during the entire migration. This means:
- **No consumer files need to change** during Phases 1-3
- **Incremental adoption:** Module authors can update imports at their own pace
- **Zero downtime:** The bot can be deployed after any phase

### 5.2 Deprecation Timeline

| Phase | Duration | Consumer Impact |
|-------|----------|-----------------|
| Phase 1 | 1-2 days | None — all re-exports |
| Phase 2 | 1-2 weeks | None — optional import updates |
| Phase 3 | 2-3 days | None — backup is internal |
| Phase 4 | Future | Requires updating all 84 imports (separate project) |

---

## 6. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Import cycles between domains | Medium | High | Keep models in `models/`; domains only depend on `models/`, `cache/`, `conn/` |
| GORM model references break | Low | High | Models are moved, not changed; `TableName()` methods preserved |
| Cache invalidation missed | Medium | High | Every moved function keeps its `InvalidateXxxCache()` call; audit via grep |
| Test fixtures break | Medium | Medium | `testmain_test.go` sets up `db.DB`; update to use `conn.DB` |
| Merge conflicts during long migration | Medium | Medium | Small PRs per domain; rebase frequently |
| Backup switch statements desync | Low | High | Backup is Phase 3, after all domains are stable |

---

## 7. Definition of Done

- [ ] `alita/db/db.go` is <100 lines (compatibility shim only)
- [ ] All GORM models live in `alita/db/models/` or domain packages
- [ ] All cache helpers live in `alita/db/cache/`
- [ ] All domain query files moved to `alita/db/<domain>/`
- [ ] `alita/db/backup/` contains backup logic
- [ ] `alita/db/migrations/` contains migration runner
- [ ] `make test` passes with no failures
- [ ] `make lint` passes with no new issues
- [ ] `make build` produces working binary
- [ ] No behavioral changes to any module

---

## 8. Open Questions

1. **Model placement:** Should models stay in `models/` or move to domain packages? 
   - *Recommendation:* Keep in `models/` to avoid import cycles. Domain packages import `models.BanSettings`.
   
2. **Optimized queries:** Should `OptimizedLockQueries` stay in `queries/` or move to `locks/`?
   - *Recommendation:* Move to `locks/` — it's domain-specific.

3. **Repository interfaces:** Should we define `BanRepository` interfaces now or later?
   - *Recommendation:* Later. This spec is about package structure, not abstraction depth. Interfaces add complexity.

4. **Monitoring and migrations:** These are infrastructure, not domains. Keep in `monitoring/` and `migrations/`.

---

## 9. Related Work

- **Candidate 1 (Dependency Graph):** After this split, injecting `*gorm.DB` becomes practical because each domain package can accept it in a constructor.
- **Candidate 5 (Command Pipeline):** Independent — can happen in parallel.
- **Candidate 3 (moduleStruct):** Independent — can happen in parallel.

---

## 10. Appendix: File Mapping

### Phase 1: Infrastructure Extraction

| Source | Destination | Notes |
|--------|-------------|-------|
| `db.go:66-169` | `models/types.go` | ButtonArray, StringArray, Int64Array |
| `db.go:171-187` | `models/user.go` | User |
| `db.go:189-206` | `models/chat.go` | Chat, ChatUser |
| `db.go:214-241` | `models/warns.go` | WarnSettings, Warns |
| `db.go:243-279` | `models/greetings.go` | GreetingSettings, WelcomeSettings, GoodbyeSettings |
| `db.go:281-297` | `models/filters.go` | ChatFilters |
| `db.go:299-310` | `models/admin.go` | AdminSettings |
| `db.go:312-357` | `models/blacklists.go` | BlacklistSettings |
| `db.go:359-372` | `models/pins.go` | PinSettings |
| `db.go:374-401` | `models/reports.go` | ReportChatSettings, ReportUserSettings |
| `db.go:403-415` | `models/devs.go` | DevSettings |
| `db.go:417-430` | `models/channels.go` | ChannelSettings |
| `db.go:432-445` | `models/antiflood.go` | AntifloodSettings |
| `db.go:447-473` | `models/connections.go` | ConnectionSettings, ConnectionChatSettings |
| `db.go:475-488` | `models/disabling.go` | DisableSettings |
| `db.go:490-500` | `models/disabling.go` | DisableChatSettings |
| `db.go:503-516` | `models/rules.go` | RulesSettings |
| `db.go:518-530` | `models/locks.go` | LockSettings |
| `db.go:532-549` | `models/notes.go` | NotesSettings |
| `db.go:551-572` | `models/notes.go` | Notes |
| `db.go:574-587` | `models/approvals.go` | ApprovedUsers |
| `db.go:589-653` | `models/captcha.go` | CaptchaSettings, CaptchaAttempts, StoredMessages, CaptchaMutedUsers |
| `db.go:655-668` | `models/antiraid.go` | AntiRaidSettings |
| `db.go:670-783` | `conn.go` | DB global, init(), connection setup |
| `db.go:785-942` | `conn.go` + `models/` | CRUD helpers (keep in conn or extract?) |
| `cache_helpers.go` | `cache/keys.go`, `cache/ttl.go`, `cache/loader.go` | Split by concern |
| `optimized_queries.go` | Domain packages or `queries/` | Move to respective domains |
| `backup_db.go` | `backup/export.go`, `backup/import.go` | Split export/import |
| `migrations.go` | `migrations/runner.go` | MigrationRunner |
| `monitoring.go` | `monitoring/metrics.go` | DatabaseMetrics |

### Phase 2: Domain Migrations

| Source | Destination |
|--------|-------------|
| `admin_db.go` | `admin/repository.go` |
| `antiflood_db.go` | `antiflood/repository.go` |
| `antiraid_db.go` | `antiraid/repository.go` |
| `approvals_db.go` | `approvals/repository.go` |
| `bans_db.go` | `bans/repository.go` |
| `blacklists_db.go` | `blacklists/repository.go` |
| `captcha_db.go` | `captcha/repository.go` |
| `channels_db.go` | `channels/repository.go` |
| `chats_db.go` | `chats/repository.go` |
| `connections_db.go` | `connections/repository.go` |
| `devs_db.go` | `devs/repository.go` |
| `disable_db.go` | `disabling/repository.go` |
| `filters_db.go` | `filters/repository.go` |
| `greetings_db.go` | `greetings/repository.go` |
| `lang_db.go` | `lang/repository.go` |
| `locks_db.go` | `locks/repository.go` |
| `notes_db.go` | `notes/repository.go` |
| `pin_db.go` | `pins/repository.go` |
| `reports_db.go` | `reports/repository.go` |
| `rules_db.go` | `rules/repository.go` |
| `user_db.go` | `users/repository.go` |
| `warns_db.go` | `warns/repository.go` |

---

*End of Spec*
