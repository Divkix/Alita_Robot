# Split AGENTS.md into Scoped Subdirectory Files

## Problem

The current `AGENTS.md` is 442 lines — well past the 150-line threshold where research shows diminishing returns and increased inference costs (+20-23%). The file mixes concerns:
- Project-wide build commands and security rules
- Go-specific handler patterns and module system details
- Database-specific conventions (cache invalidation, surrogate keys, GORM patterns)

Agents editing `alita/db/cache_helpers.go` load the full 442-line context including irrelevant deployment info and frontend conventions. This wastes tokens and dilutes signal.

## Goal

Split the monolithic `AGENTS.md` into **3 focused files** (root + 2 subdirectory files) that follow the AGENTS.md standard's hierarchical precedence model. Each file should stay under 150 lines and contain only the rules relevant to the code being edited.

## Design

### File Structure

```
AGENTS.md              # Root: project-wide rules (~90 lines)
CLAUDE.md -> AGENTS.md # Symlink for Claude Code compatibility
alita/
  AGENTS.md            # Go-specific: modules, handlers, errors (~140 lines)
  db/
    AGENTS.md          # DB-specific: cache, migrations, GORM (~100 lines)
```

### Root `AGENTS.md` (~90 lines)

**Scope:** Universal rules that apply to any agent working anywhere in the repo.

**Sections:**
1. **Project Overview** (3 lines) — One-sentence description, stack versions
2. **Key Commands** (15 lines) — Exact `make` commands with descriptions
3. **Project Structure** (10 lines) — Flat file map with purpose annotations
4. **Testing Guidelines** (10 lines) — Framework, coverage threshold (78%), test naming
5. **Commit & PR Conventions** (15 lines) — Conventional commits format, pre-commit hooks, PR requirements
6. **Security Best Practices** (10 lines) — No secrets in commits, `.env` rules, `ENABLE_PPROF` warning
7. **Boundaries** (15 lines) — ✅ Do / ⚠️ Ask / 🚫 Never
8. **Environment Configuration** (5 lines) — Reference to `sample.env`, critical vars only
9. **Subdirectory Files Note** (3 lines) — "For Go-specific rules, see `alita/AGENTS.md`. For DB rules, see `alita/db/AGENTS.md`."

**What stays from current file:**
- Lines 1-5: Project description
- Lines 20-57: Build commands (trimmed to essential ones)
- Lines 58-62: Auto-migration note (trimmed)
- Lines 134-150: Testing guidelines
- Lines 288-299: Graceful Shutdown + Monitoring (high-level only)
- Lines 316-346: Commit & PR guidelines
- Lines 384-394: Pre-commit hooks (trimmed to essentials)
- Lines 395-418: Environment config (trimmed to reference)
- Lines 419-434: Deployment + CI/CD (trimmed to 3 lines)
- Lines 435-442: Security best practices

**What gets cut/moved:**
- Code Style & Conventions → `alita/AGENTS.md`
- Architecture Overview → `alita/AGENTS.md` and `alita/db/AGENTS.md`
- Critical Rules → split between `alita/` and `alita/db/`
- Permission System → `alita/AGENTS.md`
- Additional Utility Packages → `alita/AGENTS.md`

### `alita/AGENTS.md` (~140 lines)

**Scope:** Rules for any agent editing Go code in the `alita/` tree.

**Sections:**
1. **Module System** (25 lines) — Registration patterns, `LoadModules()`, `moduleStruct` fields, handler groups, command registration (legacy vs new `WrapCommand`)
2. **Handler Patterns** (20 lines) — Value receivers, return values (`ext.EndGroups`/`ext.ContinueGroups`), callback codec (`Encode`/`Decode`), double-answer bug, `IsUserConnected()`
3. **Error Handling** (15 lines) — Four-layer recovery, `RecoverFromPanic()`, `Wrap()`/`Wrapf()`, `IsExpectedTelegramError()`, new utilities (`SendMessageWithErrorHandling`, `IsPermissionError`)
4. **Critical Go Rules** (25 lines) — Never ignore DB errors, nil sender check, `IsUserAdmin` channel ID behavior, sync before confirm, async DB wrappers, struct alias fields
5. **i18n Patterns** (15 lines) — YAML quoting, printf safety, key verification, parse mode (`tgmd2html.MD2HTMLV2`)
6. **Code Style** (20 lines) — Imports order, gofmt, golangci-lint, naming conventions, line length, comments
7. **Boolean Logic Warning** (5 lines) — Filter functions caveat
8. **Adding a New Module** (10 lines) — 5-step checklist

**What moves here from current file:**
- Lines 63-133: Code Style & Conventions
- Lines 151-213: Architecture Overview → Module System section
- Lines 251-269: Permission System
- Lines 270-275: Internationalization
- Lines 276-287: Error Handling
- Lines 300-313: Additional Utility Packages (high-level)
- Lines 348-366: Critical Rules (Go + Handler patterns)
- Lines 367-373: i18n Patterns (from Critical Rules)
- Lines 382-383: Boolean Logic

### `alita/db/AGENTS.md` (~100 lines)

**Scope:** Rules for any agent editing database code.

**Sections:**
1. **Database Architecture** (15 lines) — PostgreSQL + GORM, file organization (`*_db.go`, `cache_helpers.go`, `optimized_queries.go`, `migrations.go`, `monitoring.go`)
2. **Surrogate Key Pattern** (10 lines) — Auto-increment `id` as PK, external IDs as unique constraints
3. **Cache Invalidation** (15 lines) — Key format (`alita:{module}:{identifier}`), `CacheKey()`, `singleflight`, `getFromCacheOrLoad`, nil cache safety, **must invalidate on every write**
4. **GORM Conventions** (15 lines) — `PrepareStmt: true`, custom types (`Scan`/`Value`), `clause.OnConflict`, composite indexes
5. **Migration Workflow** (15 lines) — 4-step process: add migration → update struct → update optimized queries → update function
6. **Query Patterns** (15 lines) — `getFromCacheOrLoad`, `CreateRecordWithContext`/`UpdateRecordWithContext`, `DatabaseMetrics`
7. **Backup & Export** (10 lines) — `BackupFormat`, `BackupRateLimiter`, 13 modules supported

**What moves here from current file:**
- Lines 214-236: Database Layer
- Lines 237-250: Cache Layer
- Lines 374-380: Database Patterns (from Critical Rules)

### `CLAUDE.md` Symlink

**Strategy:** Symlink `CLAUDE.md` → `AGENTS.md` at repo root.

**Rationale:**
- Claude Code doesn't read `AGENTS.md` natively (uses `CLAUDE.md`)
- Symlink ensures Claude gets the same root-level rules as other tools
- Since Claude Code doesn't support subdirectory AGENTS.md files anyway, the root file is the right scope
- If Claude-specific rules are needed later, convert symlink to a thin file that imports `@AGENTS.md`

**Command:** `ln -s AGENTS.md CLAUDE.md`

## Validation Criteria

1. **Line count:** Each file ≤ 150 lines
2. **No duplication:** No section appears in more than one file
3. **Completeness:** Every line from the current 442-line file is accounted for in one of the 3 files
4. **Cross-references:** Root file mentions subdirectory files; subdirectory files don't need to reference root (precedence model handles this)
5. **Symlink works:** `CLAUDE.md` resolves to `AGENTS.md` content
6. **No drift:** When conventions change, only one file needs updating

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Agent misses root rules when in subdirectory | Subdirectory files only contain *additional* or *overriding* rules; root rules are still loaded by tools that support merging (Codex, Copilot). For tools that don't merge (Kilo), the subdirectory file should be self-contained for its scope. |
| Inconsistency between files | Add a comment in each file: "Keep in sync with root AGENTS.md for cross-cutting changes" |
| CLAUDE.md symlink breaks on Windows | Git handles symlinks on Windows with `core.symlinks=true`. Document this in README if needed. |
| Agent doesn't know to look for subdirectory files | Root file includes explicit note: "For Go-specific rules, see `alita/AGENTS.md`. For DB rules, see `alita/db/AGENTS.md`." |

## Documentation Sync Requirement

Any change to `AGENTS.md` files must be reflected in the human-facing documentation at `docs/src/content/docs/`. The docs site serves contributors who don't use AI agents, so critical rules must be documented in both places.

**Files to keep in sync:**
- `docs/src/content/docs/contributing/index.mdx` — Development setup, code style, common pitfalls
- `docs/src/content/docs/architecture/` — System architecture, module system, database patterns
- `docs/src/content/docs/self-hosting/` — Environment variables, deployment

**Sync rules:**
- When adding a new critical rule to `AGENTS.md`, add it to the relevant docs page
- When removing a rule from `AGENTS.md`, verify it's not referenced in docs (or remove it)
- When changing a command or pattern, update both files
- The docs can be more verbose and explanatory; `AGENTS.md` should be concise and imperative

## Definition of Done

- [ ] Root `AGENTS.md` written and ≤ 150 lines
- [ ] `alita/AGENTS.md` written and ≤ 150 lines
- [ ] `alita/db/AGENTS.md` written and ≤ 150 lines
- [ ] `CLAUDE.md` symlink created pointing to `AGENTS.md`
- [ ] All content from original 442-line file migrated (no lost rules)
- [ ] No duplicate sections across files
- [ ] `make lint` passes (no markdown issues if linting applies)
- [ ] Files committed to git
- [ ] Docs updated: `docs/src/content/docs/contributing/index.mdx` synced with new AGENTS.md structure
- [ ] Docs updated: `docs/src/content/docs/architecture/` pages synced with module/DB patterns

## Execution Options

1. **Subagent-Driven Development** (recommended) — Delegate file writing to subagents, review each
2. **Inline Execution** — Write all files in this session
3. **Stop here** — Hand off to another agent or manual execution
