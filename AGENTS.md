# Repository Guidelines

Alita Robot is a Telegram group-management bot written in **Go 1.26** on top of
the **gotgbot/v2** library (`v2.0.0-rc.35`). It provides admin tools, filters,
notes, greetings, anti-flood / anti-raid / anti-spam, captcha verification,
warns, locks, backups, connections, reactions and multi-language support
(en, es, fr, hi, ru, pt, id).

> `CLAUDE.md` and `GEMINI.md` are symlinks to this file — **AGENTS.md is the single
> source of truth** for agent/contributor guidance. Edit this file only.

## Maintaining This File

This file is **not** auto-generated. When you make changes that affect anything
documented here — build pipeline, scripts, env vars, routes, key systems,
dependencies, directory layout, or code-style rules — update the relevant
section in the same change so it stays accurate. `CLAUDE.md` and `GEMINI.md` are
symlinks to this file, so edit `AGENTS.md`.

---

## 0. Maintaining this document (READ FIRST)

**This file is a living knowledge base. Keep it current as you work.**

- When you discover something **non-obvious, load-bearing, or surprising** about
  the codebase (a hidden coupling, a "why it's done this way" decision, a footgun,
  a corrected fact, a new subsystem), **record it here in the most relevant
  section before you finish the task.** Treat it as part of "done."
- **Consolidate, don't append.** Before adding a note, find where it belongs and
  merge it with what's there. Fix stale/contradictory statements in place rather
  than stacking a second version next to them. Prefer one accurate sentence over
  three vague ones. Remove notes that have become false.
- **Be specific and verifiable**: name the file/function/env-var/table/constant.
  A future agent must be able to act on the note without re-deriving it.
- **Verify before trusting.** This document reflects the code at the time each
  note was written. If a note names a file, function, flag, table, or default,
  confirm it still exists before relying on it — and if it has changed, update the
  note as part of your change.
- Don't duplicate what the code/tests/git history already make obvious; capture
  the *why* and the *gotcha*, not a restatement of the code.
- This document was last fully reconciled against the source by a whole-codebase
  read; sections below marked with ⚠️ call out where older docs had drifted.

---

## 1. Mental model — how it fits together

A Telegram **update** flows like this:

```
Telegram ──► (polling updater  OR  webhook /webhook POST)
          ──► ext.Dispatcher (tracing.TracingProcessor wraps each update in a span;
                              dispatcherErrorHandler classifies errors → Noop)
          ──► handlers, executed in HANDLER-GROUP order (negative → 0 → positive)
                 • group -10/-5/-2/-1 : early interceptors (captcha pending, antiraid,
                                        antispam, passive Users tracker)
                 • group 0            : normal command handlers (return ext.EndGroups)
                 • group 4..10        : message watchers (antiflood, locks, blacklists,
                                        filters, reactions, reports) (return ext.ContinueGroups)
          ──► handler reads/writes DB (GORM/Postgres) through per-domain repos,
              which read-through a Redis cache; replies via i18n + media/formatting
```

Big architectural facts an agent must hold in mind:

- **Config and the DB connection are opened in package `init()` functions, not in
  `main()`.** Importing `alita/config` loads+validates config into the global
  `config.AppConfig`; importing `alita/db` opens the Postgres connection. Both
  short-circuit for CLI flags (`--version`/`--health`) and when their required env
  is unset (so tests can import them). Do **not** move this into `main()`.
- **The DB layer is split into per-domain sub-packages** (`alita/db/<domain>/`)
  with all GORM structs in `alita/db/models/`. `alita/db/db.go` is a
  backward-compat shim that re-exports model types (`db.User = models.User`), cache
  helpers, and TTL constants. ⚠️ Older docs described a flat `alita/db/*_db.go`
  layout — that no longer exists.
- **Schema source of truth is raw SQL in `migrations/*.sql`**, applied by a custom
  runtime engine (`alita/db/migrations/runner.go`), **not** `gorm.AutoMigrate`.
  GORM struct tags only affect runtime CRUD. Tests bootstrap schema via SQLite
  `AutoMigrate` (`testmain_test.go`), so struct↔SQL drift is possible and not
  caught by tests — keep them in sync manually.
- **Cache is Redis-only** (via `eko/gocache` + `go-redis`). There is no in-memory
  production fallback. Every cache helper is nil-safe: when the marshaler is nil
  it bypasses caching and hits the DB directly.
- **Modules self-register in `init()`** and load in ascending-priority order; the
  Help module loads last (deferred) so it can render every module's metadata.
- **Callback data uses a versioned codec** (`<namespace>|v1|<url-encoded>`) capped
  at Telegram's 64-byte limit — never `strings.Split` raw callback data.

---

## 2. Project structure

- **`main.go`** — process entry point (CLI flags, bootstrap, polling/webhook
  branch, dispatcher, shutdown wiring, custom Bot-API rewrite transport).
- **`alita/`** — application code
  - `main.go` — `LoadModules`, `InitialChecks`, `ListModules`.
  - `config/` — `config.go` (manual env loading, defaults, validation, logredact
    wiring in `init()`), `types.go` (`typeConvertor`). **No viper here.**
  - `db/`
    - `db.go` — OTel-traced CRUD wrappers + re-export shim for models/cache/TTLs.
    - `conn.go` — Postgres connection (opened in `init()`), pool tuning, optional `AUTO_MIGRATE`.
    - `models/` — **all GORM structs** (one file per table) + `types.go` (JSONB types).
    - `<domain>/` — per-domain repositories: `admin, antiflood, antiraid, approvals,
      blacklists, captcha, channels, chats, connections, devs, disabling, filters,
      greetings, lang, locks, notes, pins, reports, rules, user, warns`
      (usually `repository.go` + optional `optimized.go`).
    - `cache/` — `CacheKey`, `GetFromCacheOrLoad` (singleflight read-through), `DeleteCache`, TTL constants.
    - `migrations/` — `runner.go` (custom SQL migration engine).
    - `monitoring/` — `metrics.go` (DB pool metrics for `/db_metrics`).
    - `backup/` — `backup.go` + `types.go` (per-module export/import/clear, **16 modules**).
  - `i18n/` — singleton `LocaleManager`, per-language `Translator`, `go:embed` locales.
    Locale YAML is parsed into `map[string]any` (yaml.v3); key lookup is a dot-path
    descent with case-insensitive fallback (for `alt_names.<Module>`). **No viper.**
  - `modules/` — bot feature modules + shared plumbing (see §6).
  - `utils/` — `chat_status` (permissions), `helpers` (command pipeline), `cache`,
    `callbackcodec`, `formatting`, `keyboard`, `keyword_matcher`, `media`, `content`,
    `extraction`, `error_handling`, `errors`, `logredact`, `ratelimit`, `constants`,
    `monitoring`, `shutdown`, `tracing`, `httpserver`.
- **`locales/`** — `en/es/fr/hi/ru/pt/id.yml` translations + **`config.yml`** (loaded
  as a pseudo-language `"config"`; holds `alt_names.<Module>` and `db_default_*`).
- **`migrations/`** — timestamped `.sql` schema files (source of truth).
- **`scripts/`** — `generate_docs/` (root module), `check_translations/` (**separate
  go.mod**), `validate_orphaned_data.go`, `migrate_psql.sh`, `backup_database.sh`.
- **`internal/repo_checks/`** — test-only structural-invariant assertions.
- **`docs/`** — Astro + Starlight docs site (bun, deployed to Cloudflare Workers).
- **`.github/workflows/`** — `ci.yml`, `release.yml`, `docs.yml`, `dependabot-native-merge.yml`.
- **`docker/`** — `alpine` (prod), `alpine.debug`, `goreleaser`, `pr-build`.

---

## 3. Build, Test & Development commands

```bash
make run                # go run main.go
make build              # goreleaser release --snapshot --skip=publish --clean --skip=sign
make lint               # golangci-lint run (v2 config)
make test               # go test -tags testtools -v -race -coverprofile=coverage.out \
                        #   -coverpkg=<all except root main + scripts/> -count=1 -timeout 10m ./...
make tidy / make vendor

# Single tests
go test -v -run TestXxx ./alita/db
go test -v -count=1 -timeout 10m ./alita/db

# Translations & docs
make check-translations # runs scripts/check_translations (separate module) — missing-key gate
make check-duplicates   # golangci-lint --enable dupl (duplicate Go CODE, NOT translation keys) ⚠️
make generate-docs      # regenerate docs from source (no-op for sentinel-frozen files)
make check-docs         # docs drift gate (diff regenerated vs committed)
make inventory          # .planning/INVENTORY.{json,md} (authoritative command list)
make docs-dev           # bun run dev (Astro, localhost:4321)

# Postgres migrations (require PSQL_DB_* env)
make psql-prepare / psql-migrate / psql-status / psql-rollback / psql-verify / psql-reset
make validate-db        # scripts/validate_orphaned_data.go
make backup-db          # scripts/backup_database.sh

# Release version bump (patches BotVersion in config.go + main.go fallback)
make bump-version TAG=vX.Y.Z   # wraps scripts/bump_version.sh
```

**Tests require live Postgres + Redis and `CGO_ENABLED=1`** (the `-race` detector
needs a C toolchain). The shipped binary is `CGO_ENABLED=0`, but tests are not.
`-coverpkg` excludes the root `main` package and `scripts/`, so changes there do
not move coverage; `alita/*` changes do. Coverage gate is **78%** (hardcoded in
`ci.yml`).

---

## 4. CI/CD — how it actually works

### `ci.yml` (push to `main` with **tags ignored**, all PRs, manual dispatch)

Concurrency cancels in-progress runs per PR/ref. Top-level perms `contents: read`
+ `security-events: write`; all checkouts use `persist-credentials: false`.

Parallel jobs (no `needs`), then aggregation:

| Job | What it does | Gating? |
|-----|--------------|---------|
| `security` | gosec `-no-fail` → SARIF upload (`continue-on-error`); govulncheck (`continue-on-error`) | ⚠️ **Non-gating** — nothing here can fail the build despite being "required" by `ci-success`. |
| `lint` | golangci-lint **binary v2.9.0**, `--timeout 10m`, `only-new-issues:true`; second run with `--enable dupl`; informational TODO/FIXME + gocyclo>15 step summaries | New issues block; pre-existing tolerated. |
| `build` | `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`, then `./alita_robot --version` from `/tmp` | Yes |
| `test` | Service containers **postgres:16** + **redis:7**; `CGO_ENABLED=1`; `make test`; then coverage gate **≥78%** | Yes |
| `docs-check` | `make check-translations` + `make check-docs` (translation + docs drift gate) | Yes |
| `docker-verify` | single-arch `docker build -f docker/alpine` (no push) | Yes |
| `docker-publish` | main-push only; multi-arch `linux/amd64,linux/arm64` → GHCR tags `dev`, `dev-<sha7>`, `<sha7>` (NOT `latest`), with `provenance:true` + `sbom:true`; `needs: [security,lint,build,test,docker-verify]` (NOT docs-check) | Yes (on main push) |
| `ci-success` | `if: always()`; re-checks each result; enforces `docker-publish` only on main-push | Final gate |

### `release.yml` (tag push `*` or manual dispatch with `tag` input)

`release-ci-checks` (gosec `-no-fail`, govulncheck `continue-on-error`, build) →
`goreleaser` (**v2.13.0**, deletes any pre-existing release for the tag to handle
tag moves) → then `attest-artifacts` (SLSA `attest-build-provenance` over `dist/*`)
**and** `post-release-scan` (Trivy `CRITICAL,HIGH`, `exit-code:0`, informational).
GoReleaser's `dockers_v2` publishes GHCR `{{.Tag}}`, `{{.Version}}`, **`latest`**
(only the release path publishes `latest`).

⚠️ **Tags must be `v`-prefixed** (`on: push: tags: ["v*"]`). The `goreleaser` job's
**Resolve release tag** step normalizes the `workflow_dispatch` input to one `v`
prefix and strictly validates `vMAJOR.MINOR.PATCH[-prerelease]` (on tag-push it
passes `github.ref_name` through), exposing `steps.tag.outputs.tag`. On
`workflow_dispatch` it then runs `scripts/bump_version.sh <tag>` to patch the
version (commits to `main` and pushes), tags that commit, and pushes the tag — all
git pushes use a token-in-URL (`https://x-access-token:$GITHUB_TOKEN@…`) because
checkout keeps `persist-credentials: false`. `GITHUB_TOKEN` pushes don't re-trigger
workflows, so there's no double release. `--version` reads
`config.AppConfig.BotVersion` (patched by the bump script; currently `"2.19.6"`),
with a hard-coded local fallback `version = "v2.19.6"` in `main.go` (used only when
config didn't load). There are **no** `-X main.version/commit/date` ldflags anymore
(they were no-ops — `package main` declares no such vars). ⚠️ After the bump step,
`goreleaser` runs a **"Verify BotVersion matches tag"** gate that `grep`s **both**
`alita/config/config.go` (`BotVersion:  "<ver>"`) **and** `main.go`
(`version = "v<tag>"`) and fails the release on mismatch — this is the enforcement
behind "don't hand-edit BotVersion."

### `docs.yml` (path-filtered to docs/alita/scripts/locales)

`make generate-docs` → Node 22 + Bun → `bun run build` → deploy to **Cloudflare
Workers** via `wrangler@4` (only on push to `main`). ⚠️ Note: tags pushes never run
`ci.yml`, so there is **no coverage/docs gate on the release path**.

### `dependabot-native-merge.yml`

Auto-approves + `gh pr merge --auto --squash` for **patch/minor**; **major**
updates get a warning comment only. ⚠️ Per §22, do not let gotgbot RC bumps or the
untagged `gotg_md2html` pseudo-version auto-merge without a compatibility review.

### Local quality gates

- **Pre-commit** (`.pre-commit-config.yaml`): trailing-whitespace, end-of-file,
  check-yaml, large-file (max 1000 KB), merge-conflict, detect-private-key,
  golangci-lint **v2.11.4** (note: differs from CI's v2.9.0 — they can disagree),
  `gofmt -l -w`, `go mod tidy`. Install: `pip install pre-commit && pre-commit install`.
- **`.golangci.yml`** (v2 format): linters `godox`, `dupl` (threshold 100),
  `gocyclo` (min-complexity **20**); `new:true` (only-new-issues); build-tag
  `testtools`; excludes tests/generated-docs/db-migrations.

### Deploy targets (they disagree — check the specific one)

Docker Compose/Dokploy (`AUTO_MIGRATE=false`, port 8080), Railway (`RAILPACK`,
healthcheck `/health`), Render (`AUTO_MIGRATE=true`, `HTTP_PORT=10000`), Heroku
(`Procfile` → `bin/Alita_Robot` capitalized ⚠️, `app.json`), Nixpacks. Prod image
is `distroless/static-debian12`, non-root UID 65532, EXPOSE 8080, healthcheck via
the `--health` flag.

---

## 5. Startup / bootstrap flow

`main()` order (config + DB are already loaded by package `init()` before this runs):

1. Capture `appStartTime` (for `/health` uptime).
2. **CLI flags** by raw `os.Args`: `--health` GETs `/health` and exits 0/1
   (distroless has no curl); `--version`/`-v` prints `BotVersion` and exits.
3. Main-goroutine panic-recovery `defer` (`os.Exit(1)`).
4. **`cache.InitCache()` FIRST** (i18n depends on it) — fatal on failure;
   FLUSHDBs Redis when `ClearCacheOnStartup` (default **true**).
5. `i18n.GetManager().Initialize(&Locales, "locales", …)` (embedded YAML).
6. `tracing.InitTracing()` — **non-fatal** (warns and continues).
7. HTTP transport (with optional `API_SERVER` rewrite) → `gotgbot.NewBot` → resolve
   username → goroutine pre-warming Telegram connections.
8. `alita.InitialChecks(b)` — `user.EnsureBotInDb` (blocking, FK anchor) +
   `checkDuplicateAliases` (fatal on dup).
9. dispatcher (`TracingProcessor`, `dispatcherErrorHandler`,
   `MaxRoutines` default 200) → monitoring systems → shutdown manager →
   unified HTTP server.
10. **Mode branch** on `UseWebhooks`: webhook (requires `WEBHOOK_DOMAIN` +
    `WEBHOOK_SECRET`, else fatal; `select {}`) or polling (default;
    `DeleteWebhook` then `StartPolling`; `updater.Idle()`). `postInit` (shared by
    both) calls `alita.LoadModules`, `SetMyCommands` for `/start` `/help`, and
    sends an HTML startup message to `MESSAGE_DUMP` (non-fatal).

**Graceful shutdown** (`alita/utils/shutdown`): a goroutine waits on
SIGTERM/SIGINT/Interrupt, then runs handlers **LIFO** (reverse of registration
order in `main`), each with panic recovery, under a **60s** total timeout, then
`os.Exit(0/1)`. ⚠️ Shutdown order is implicit — inserting a `RegisterHandler` call
shifts everything registered after it. The DB-monitoring-cancel handler is
deliberately registered *after* `closeDBConnections` so LIFO runs it *before* the
DB closes.

---

## 6. Module system

### Registry (`alita/modules/registry.go`)

- `RegisterModule(m Module)` (interface `Name()/Priority()/Load(dispatcher)`) or
  `RegisterLegacyModule(name, priority, loadFunc)` (wraps a `LoadXxx`). Dedup is by
  `Name()` (duplicates silently ignored, first wins).
- `LoadAllModules` stable-sorts **ascending** by priority. **Lower number loads
  earlier.** `alita.LoadModules` inits `AbleMap`, **defers `LoadHelp`** (so Help
  renders after every module pushed its metadata), then `LoadAllModules`.

**Priorities** (edit the literal in each module's `init()` to reorder):

| Pri | Module | Pri | Module | Pri | Module |
|----:|--------|----:|--------|----:|--------|
| -10 | BotUpdates | 80 | Mutes | 180 | Disabling |
| 10 | Antispam | 90 | Purges | 190 | Rules |
| 20 | Languages | 100 | Users | 200 | Warns |
| 30 | Admin | 110 | Reports | 210 | Greetings |
| 40 | Approvals | 120 | Dev | 220 | Captcha |
| 50 | Pins | 130 | Locks | 230 | AntiRaid |
| 60 | Misc | 140 | Filters | 240 | Blacklists |
| 70 | Bans | 150 | Antiflood | 250 | Reactions |
|     |        | 160 | Notes | 260 | Formatting |
|     |        | 170 | Connections | 270 | Backup |

Help is not in the registry (deferred-last). `bot_updates.go` is the **only**
module using the new `Module` interface directly; all others use `RegisterLegacyModule`.

### `moduleStruct` and the help registry (`core.go`)

⚠️ There is **no `alita/modules/helpers.go`** (older docs claimed one). `moduleStruct`
(fields `moduleName`, `handlerGroup`, `permHandlerGroup`, `restrHandlerGroup`,
`defaultRulesBtn`, `AbleMap`, `AltHelpOptions`, `helpableKb`) lives in `core.go`.

- A single package-global singleton `DefaultHelpRegistry()` doubles as the Help
  module's state **and** the cross-module registry. Each module, at the end of its
  `LoadXxx`, calls `DefaultHelpRegistry().AbleMap.Store(name, true)` and optionally
  sets `helpableKb[Name]` / `AltHelpOptions[Name]`. `AbleMap` is a plain
  `map[string]bool` wrapper (**not** `sync.Map`, no mutex) — safe only because all
  writes happen during single-threaded startup. Do not `Store` from a handler.
- `helpableKb` keys are the **Title-cased** module name; per-module help text comes
  from i18n key `<lowercase>_help_msg` rendered via `tgmd2html.MD2HTMLV2`.
- ⚠️ `moduleStruct` is passed **by value** to handler methods, so it must never
  embed a mutex/`sync.Map`. That's why `overwrite.go` keeps `notesOverwriteMap` as
  a package-level var (copylocks).

### Adding a module

1. DB ops in `alita/db/<domain>/repository.go` (+ optimized.go for hot reads),
   model in `alita/db/models/<domain>.go`, alias in `db.go`, migration in
   `migrations/`.
2. Handlers + `LoadYourModule(dispatcher)` in `alita/modules/your_module.go`.
3. `RegisterLegacyModule("YourModule", <priority>, LoadYourModule)` in `init()`;
   call `DefaultHelpRegistry().AbleMap.Store(...)` inside `LoadXxx`.
4. Add `<yourmodule>_help_msg` (and any keys) to **all** locale files.

### Command registration: two patterns coexist

- **New declarative pipeline** (`alita/utils/helpers/command_pipeline.go`) — used by
  `admin.go` and `pins.go`: `WrapCommand(dispatcher, CommandDescriptor, handler)`
  runs panic-recovery → `BuildCommandContext` → ordered `RequiredChecks`
  (`CheckFunc` builders like `RequireGroup`, `RequireUserAdmin`, `CanUserRestrict`)
  → handler. `BuildCommandContext`'s "error" sentinel **is `ext.EndGroups`**, not a
  real error. `Disableable:true` registers every alias as disableable.
- **Legacy** — everything else: `dispatcher.AddHandler(handlers.NewCommand(...))`,
  `helpers.MultiCommand(d, aliases, handler)`, `helpers.AddCmdToDisableable(cmd)`.

---

## 7. Handler, callback & routing patterns

- **Handler groups**: negative (early interceptors), 0 (commands), positive
  (watchers). In use: captcha-pending **-10**, antiraid module **-5**, antispam
  **-2**, Users tracker **-1**; locks perm **5** / restr **6**; blacklists **7**;
  reports `@admin` watcher & reactions **8**; filters **9**; pins & some watchers
  **10**; antiflood **4**.
- **Return values**: commands return `ext.EndGroups`; watchers return
  `ext.ContinueGroups` (so multiple watchers fire on one message). The Users
  tracker (group -1, every message) **must** return `ContinueGroups`.
- **Callback codec** (`alita/utils/callbackcodec`, wrapped by
  `modules/callback_codec.go`): `Encode(namespace, fields)` →
  `<namespace>|v1|<url-encoded fields>`, **hard-capped at 64 bytes**
  (`MaxCallbackDataLen`). `decodeCallbackData(data, expectedNamespaces…)` filters
  case-insensitively. Never `strings.Split` raw data. The module wrapper
  `encodeCallbackData` returns `""` on overflow (broken button) — for user-supplied
  values use the **token pattern** (store payload in Redis, put a short hex token
  in the callback; see filters/notes overwrite flows). `EncodeOrFallback` emits a
  legacy dot-notation string when encoding overflows; legacy dot-notation is still
  decoded for backward compat.
- **`callbackQueryFromContext(ctx)`** is the nil-safe guard at the top of every
  callback handler (duplicated verbatim in `chat_status` because Go can't share
  unexported helpers). Always nil-check `query.Message`.
- **Anonymous-admin flow**: on a `GroupAnonymousBot` sender, `chat_status.checkAnonAdmin`
  either bypasses (if the chat's `AnonAdmin` DB setting is on) or caches the
  original message (`alita:anonAdmin:<chat>:<msg>`, **20s TTL**) and shows a "prove
  admin" button. `bot_updates.go:verifyAnonymousAdmin` re-checks admin status,
  restores `ctx.EffectiveMessage`, **nils `SenderChat` and `CallbackQuery`** (to
  stop re-detection), and re-dispatches via `HandleAnonymousAdmin`. ⚠️ This path
  **bypasses `WrapCommand` RequiredChecks**, so anon wrappers (e.g. in `admin.go`)
  must re-enforce permissions manually.
- **Deep links** (`deeplink_router.go`): `/start <payload>` in private with 2 args →
  `HandleDeepLink` (exact-match first, then **longest-prefix**). Registered:
  `help_`, `about` (exact), `rules_`, `notes_`, `note_`, `note`, `connect_`.
  ⚠️ **Security invariant**: every chat-scoped deep link (rules/notes/connect) must
  gate data behind `chat_status.IsUserInChat` (and notes also `IsUserAdmin` for
  admin-only notes) — omitting it leaks another chat's private content to anyone
  who crafts a link. `connect_` performs a **synchronous** `ConnectId` before
  confirmation (issue #694).
- **Double-answer bug**: `RequireUserAdmin`/`RequireUserOwner` with `justCheck=false`
  already answer the callback — don't answer again. The pipeline relies on
  `WithReplyFallback()` to avoid duplicate answers.

---

## 8. Permission system (`alita/utils/chat_status/`)

Two-layer: public `Can*/Require*` exports in `chat_status.go` delegate to
unexported peers in `access.go` (edit the unexported layer). `permission_responder.go`
centralizes failure messaging.

- `RequireGroup`/`RequirePrivate`, `RequireBotAdmin`/`RequireUserAdmin`/
  `RequireUserOwner` are **pure boolean** guards (no messages); messaging is done by
  `NewPermissionResponder(b).Respond(ctx, cmdKey, btnKey, opts…)` which **always
  returns false** (use `return responder.Respond(...)`), choosing callback-answer
  vs `SendMessage`/`Reply` (`WithReply()`/`WithReplyFallback()`).
- Granular `CanUser*` checks share `hasUserPermission`, which grants **creator a
  bypass** for every flag. `CanBot*` checks have **no anon handling and no creator
  fallback** (bots can't be creator) and `nil`-guard the bot.
- ⚠️ **`IsUserAdmin` returns false for channel IDs and all non-positive IDs**, before
  any API call (`IsValidUserId(id)` = `id > 0`; `IsChannelId(id)` = `id < -1e12`).
  This is a privilege-escalation guard — do not weaken it. `IsBotAdmin` is true in
  private chats and otherwise requires status exactly `"administrator"`.
- `tgAdminList = {1087968824 (GroupAnonymousBot), 777000 (Telegram)}` are always
  admin (id `136817688` is documented but intentionally **not** in the list).
- `IsUserConnected(b, ctx, chatAdmin, botAdmin)` resolves the connected chat in PM
  (nil = abort) — **callers must reassign `ctx.EffectiveChat`** to the returned chat
  (why `antichannelpin`/`cleanlinked` stay raw handlers).
- `GetEffectiveUser`/`RequireUser` are nil-safe (nil for channel posts;
  `RequireUser` ignores its `b` arg). Admin lookups go through the Redis admin
  cache (30-min TTL); **invalidation is the admin module's job, not this package's.**

---

## 9. Database layer

### Shared wrappers (`alita/db/db.go`)

OTel-traced: `GetRecord`/`GetRecords`/`CreateRecord`/`UpdateRecord`/
`UpdateRecordWithZeroValues` + `ChatExists`. Connection (`conn.go`) uses
`PrepareStmt:true`, `NowFunc`=UTC, a logrus-backed GORM logger
(`SlowThreshold 1s`, `IgnoreRecordNotFoundError`), and 5-retry exponential backoff
(fatal on permanent failure).

- ⚠️ **`UpdateRecord` ignores zero-valued struct fields** (GORM semantics) — to
  persist `false`/`0`/`""` (e.g. turn a toggle OFF) you **must** use
  `UpdateRecordWithZeroValues` with a `map[string]any`. This is a recurring footgun.
- `UpdateRecord*` returns `gorm.ErrRecordNotFound` when `RowsAffected==0` (devs
  add/update path relies on this). `ChatExists` treats any non-not-found error as
  "exists" — not authoritative under DB stress.

### Models & schema (`alita/db/models/`)

- **Surrogate keys everywhere**: `ID uint` autoincrement PK; the real Telegram id
  (`chat_id`/`user_id`) is a separate **unique** column (single or composite named
  index). ⚠️ `id` is Go `uint` in structs but `bigint` in Postgres — SQL is
  authoritative.
- Custom JSONB types in `types.go`: `ButtonArray`, `StringArray`, `Int64Array` (each
  implements `Scan`/`Value`; empty slices persist as the literal `"[]"`, not NULL).
- `GreetingSettings` embeds `*WelcomeSettings`/`*GoodbyeSettings` with
  `embeddedPrefix:welcome_`/`goodbye_` → real columns `welcome_text`, `goodbye_btns`,
  … (the embedded pointers can be nil; nil-check before deref; map-based upserts must
  use the **prefixed** column names).
- ⚠️ **Table names ≠ struct names.** e.g. `AdminSettings→admin`,
  `ConnectionSettings→connection` (per-user), `ConnectionChatSettings→connection_settings`
  (per-chat — the naming is inverted), `WarnSettings→warns_settings`,
  `Warns→warns_users`, `DisableSettings→disable`. Confirm `TableName()` before
  writing raw SQL.
- Consolidated/dead fields — **do not reference**: `antiflood_settings.limit`/`.mode`
  (use `flood_limit`/`action`), `devs.dev` (use `is_dev`), `connection_settings.enabled`
  (use `allow_connect`); the `chat_users` table and its `ChatUser` GORM model have
  been removed (membership lives in the `chats.users` JSONB array).
  `ReportChatSettings`/`ReportUserSettings` still carry
  both `Enabled` and `Status` (alias) columns — set both consistently.
- Schema-change checklist: **migration → struct tag → optimized query column list →
  repository function** (and add the struct to `testmain_test.go`'s AutoMigrate list).

### Per-domain repositories

- Read-through cache via `cache.GetFromCacheOrLoad(cache.CacheKey(module, id), ttl,
  loader)` with **singleflight** stampede protection and a **30s timeout** (on
  timeout it `Forget`s the key and degrades to a direct DB load). Writes must
  **explicitly `cache.DeleteCache(...)`** every affected key.
- ⚠️ Cache key **prefixes differ from package/table names**: `blacklists→"blacklist"`,
  `channels→"channel"`, `chats→"chat"`, `captcha→"captcha_settings"`,
  `notes→"notes_settings"`, `disabling→"disabled_cmds"`, `warns→"warns"` (per-user)
  + `"warn_settings"` (per-chat), `filters→"filter_list"` + `"filters_optimized"`,
  `locks→"lock"` + `"locks_map"`, `lang→"chat_lang"`/`"user_lang"` (also invalidates
  `"chat_settings"`/`"chat"`/`"user"`). The `admin`, `connections`, `devs`, `pins`,
  `reports`, `rules` packages have **no cache** at all. Reuse the exact existing
  literal when invalidating.
- Upserts use `Where(...).Assign(map[string]any{...}).FirstOrCreate(...)` with **map**
  payloads (so zero values persist). `locks.UpdateLock` and
  `captcha.SetCaptchaMaxAttempts` are the true atomic `clause.OnConflict` upserts;
  `filters.AddFilter`/`notes.AddNote` are non-atomic (Take-then-Create, race-prone).
  `chats.UpdateChat` appends to the JSONB `users` array with Postgres-specific raw
  SQL (`users || to_jsonb(...)`).
- `user.GetUserBasicInfoCached` negative-caches a missing user as sentinel
  `User{UserId:-9999}` → maps back to `ErrRecordNotFound` (preserve on both sides).
- Most read helpers swallow errors and return safe defaults (empty slice/map,
  `"en"`, default struct) — callers can't rely on errors to detect missing data.

### Migrations (`alita/db/migrations/runner.go`)

- Runs only when `AUTO_MIGRATE=true`. Globs `migrations/*.sql`, sorts
  lexically (timestamp prefix = apply order), applies each unrecorded file in **one
  transaction** (recording the `schema_migrations` row in the same tx).
- **SHA-256 checksum over raw bytes** → applied files are **immutable**: editing one
  (even whitespace) hard-fails startup with a mismatch (unless
  `AUTO_MIGRATE_SILENT_FAIL`). **Always add a new timestamped file; never edit an
  applied one.** New timestamps must be greater than the latest existing.
- Runtime `cleanSupabaseSQL` strips Supabase GRANT/POLICY/extensions and injects
  idempotency (`CREATE TABLE/INDEX → IF NOT EXISTS`, wraps `ADD CONSTRAINT`/`CREATE
  TYPE` in `DO $$` blocks). A hand-rolled `splitSQLStatements` + `findDollarQuoteBlocks`
  share a tokenizer — edit both together. ⚠️ `CREATE INDEX CONCURRENTLY` cannot run
  inside the per-file transaction.
- ⚠️ Two schema definitions must be kept in sync with the SQL: GORM models and the
  SQLite `AutoMigrate` list in `testmain_test.go`. Forward-only — there is no working
  rollback automation (no `*.rollback.sql` files; the runner skips them).

---

## 10. Cache layer (`alita/utils/cache/`)

Redis-only via gocache. **Always** access the marshaler through mutex-guarded
`cache.GetMarshal()`/`SetMarshal()` and nil-check it (`if m := cache.GetMarshal();
m != nil`) — every helper bails when it's nil.

- `InitCache` connects with 5-retry backoff, optionally FLUSHDBs (default
  `ClearCacheOnStartup=true`), then installs the marshaler. ⚠️ `ClearAllCaches` does
  **FLUSHDB on the whole Redis DB** — Redis is assumed dedicated to the bot.
  Default `RedisDB=1` (you **cannot** select DB 0 via `REDIS_DB=0` — it's forced to 1).
- Key format `alita:{module}:{id}:{id}…` (`CacheKey` accepts variadic `...any`).
- **Admin cache** (`adminCache.go`, key `alita:adminCache:<chat>`, 30-min): caches
  Telegram admin lists with an O(1) `UserMap` + linear fallback; negative results
  (bot-not-admin) cached with `Cached:true` to avoid retry storms; the async `Set`
  means a read right after `LoadAdminCache` may miss until it lands. Two paths
  invalidate the key (`InvalidateAdminCache` + a raw delete in `admin.go`).
- **Restricted-chat cache** (`restrictedCache.go`, `alita:restricted:<chat>`, 30-min):
  tracks chats where the bot can't send; 5-min probe window with a Redis `SETNX`
  single-flight (`alita:restricted_probe:<chat>`). Fails **open** (returns false) on
  malformed timestamp or nil Redis — do not change to fail-closed.
- `MarkChatRestricted`/`IsChatRestricted`/`MarkChatNotRestricted` are driven by
  `media.Send` and `helpers.SendMessageWithErrorHandling`.

---

## 11. Internationalization (`alita/i18n/`)

- Singleton `LocaleManager` (`GetManager()` + `sync.Once`); `Initialize()` runs
  once from `main.go` (after `cache.InitCache`). `go:embed` pulls the **entire**
  `locales/` dir; each `.yml` becomes a language keyed by filename.
- ⚠️ **`locales/config.yml` is loaded as a pseudo-language `"config"`** and read via
  `i18n.MustNewTranslator("config")` for `alt_names.<Module>` (command aliases) and
  `db_default_*`. Don't rename/move it or change the embed pattern.
- ⚠️ **`ENABLED_LOCALES` does not control which locales load** — the manager always
  loads all embedded `.yml`. It only filters the `/lang` picker keyboard.
- `i18n.MustNewTranslator(langCode)` (382 call sites) never panics — falls back to
  English. Per-context language comes from `alita/db/lang.GetLanguage(ctx)` (user
  pref in private, group pref in groups, default `"en"`).
- `GetString(key, params…)` falls back to the default language on missing keys
  (recursion-guarded). Supports **both** `{named}` and legacy `%s`/`%d` placeholders;
  named→positional mapping uses a hard-coded `commonKeys` order in
  `extractOrderedValues` (`first,second,…,question,answer,number,count,value,name,
  user,username,…`). If you use a `%verb` with a param name not in that list, the
  mapping is dropped/misordered — extend `commonKeys`.
- ⚠️ Translation cache entries **never expire** (the configured 30-min TTL is never
  applied) — fine only because embedded locale content is immutable.
- **Parse mode**: locale strings are authored in Markdown but the bot sends HTML —
  convert via `tgmd2html.MD2HTMLV2`. Some short status strings are already authored
  in HTML; whether to convert depends on the specific key.
- Adding a user-facing string: add the key to **all 7** locale files (en-only works
  via fallback but is silent English leakage). `%d` needs a real int.

---

## 12. Anti-abuse internals (concise)

- **Antiflood** (`antiflood.go`, group 4): per-user count via a per-key `*sync.Mutex`
  (`floodMu`) + `syncHelperMap`, both cleaned together every 5 min. `/setflood`
  accepts `off`/`0` (disable) or `3..100`. Admin check **fails open** on timeout/
  semaphore-full (banning a real admin is worse than missing a flood). Mute/ban
  inline buttons reuse the `unrestrict` callback namespace handled in `bans.go`.
- **Antiraid** (`antiraid.go`, group -5): **Redis-only** live state
  (`alita:antiraid:state:<chat>`, 24h TTL) + a join sorted-set; 30s expiry poller
  (`StartAntiRaidExpiryPoller`). `parseDuration` treats a bare number as **seconds**;
  suffixes `s/m/h/d/w`. Defaults `RaidTime=21600s`, `RaidActionTime=3600s`,
  `AutoAntiRaidThreshold=0` (off).
- **Antispam** (`antispam.go`, group -2): ⚠️ a **local** in-memory rate limiter
  (18 msgs/sec), **not** a CAS/Spamwatch global-ban integration — no external
  service exists.
- **Captcha** (`captcha.go`, ~2100 lines): math-image/text verification with refresh
  (cooldown 5s, max 3), timeout, max-attempts. ⚠️ Three actors can finalize one
  attempt (verify callback, timeout goroutine, max-attempts) — all coordinate via
  `DeleteCaptchaAttemptByIDAtomic` as a single-winner claim; any new finalization
  path must claim atomically first. `kick`=ban-then-unban; `mute` relies on the 24h
  `captcha_muted_users` row + the 5-min unmute job. Pending messages are intercepted
  in group -10 and replayed on success.
- **Approvals**: per-chat whitelist exempt from antiflood/blacklists/locks/captcha/
  antispam (`chat_status.IsApproved` → `approvals.IsUserApproved`). `/unapproveall`
  is owner-only with synchronous confirm.
- **Disabling**: `chat_status.CheckDisabledCmd` is the gate (bypasses admins +
  private chats; optional message delete via `disabling.ShouldDel`). A command is
  only disableable if registered via `helpers.AddCmdToDisableable`.

---

## 13. Content modules (concise)

- **Filters/Blacklists** use Aho-Corasick (`keyword_matcher`) with **separate named
  caches** (`GetNamedCache("filters")` / `"blacklists")`) so they never evict each
  other — do not revert to the shared global cache. Watchers use `FirstMatch` (cheap)
  not `FindMatches` (expensive). Search text is built by `buildModerationMatchText`
  (text + caption + URL entities from **both** `Entities` and `CaptionEntities`).
- **Overwrite confirmation**: filters store the pending payload in **Redis**
  (`alita:filter_overwrite:<token>`, 5-min TTL, short hex token in callback); notes
  store it in an **in-memory** `notesOverwriteMap` (lost on restart, leaks if never
  answered).
- **Greetings**: a join fires **both** a `ChatMemberUpdated` and a service message —
  `claimRecentJoinProcessing` (Redis SETNX, 5s) dedupes to avoid double welcome/
  captcha. Captcha-on-join mutes with `MutedPermissions` then `SendCaptcha`.
- **Locks**: `lockMap` (content types, perm watcher group 5) + `restrMap`
  (restriction types, group 6); both skip admins/approved and require `CanBotDelete`.
  The `bots` lock is handled by a separate `ChatMember` handler.
- **Rules**: stored as HTML (`tgmd2html.MD2HTMLV2`); `normalizeRulesForHTML`
  re-renders legacy Markdown only when no HTML tags are present. **No Redis cache.**
- **Media** (`utils/media`): `Send` dispatches on `MsgType` (TEXT=1…VIDEO_NOTE=8;
  0/unknown → text; empty `FileID` → text fallback), short-circuits on
  `IsChatRestricted`, and marks chats restricted on permission errors. `SendNote`/
  `SendFilter` do `%%%` random-variant selection + `FormattingReplacer`.
  ⚠️ Only **URL** buttons survive note/filter storage (callback buttons are dropped).
- Moderation commands share `moderationCommand` (`moderation.go`):
  RequireUser → gates → extract → validate → execute → reply, always returning
  `ext.EndGroups`. `standardModGates` = RequireGroup→RequireUserAdmin→RequireBotAdmin
  →CanUserRestrict→CanBotRestrict; `deleteModGates` adds CanBot/UserDelete.
  ⚠️ `extraction.ExtractUserAndText` returns `-1` (error already replied — abort
  silently) vs `0` (nothing specified) — distinguish them. Warn-mode `kick`
  bans **without** unban (unlike the `/kick` command).

---

## 14. Observability & ops

- **`alita/utils/monitoring`** (distinct from `alita/db/monitoring`): three
  background services — `ActivityMonitor` (per-chat & per-user DAU/WAU/MAU, marks
  chats inactive; ⚠️ user counts ignore `is_inactive`, chat counts don't),
  `BackgroundStatsCollector` (3 ticker goroutines — 30s system / 1m DB / 5m report —
  that write the shared metrics struct directly under a mutex; no worker pool or
  channels), `AutoRemediationManager` (one action per minute, ascending severity,
  5-min cooldown; also emits a >100ms GC-pause warn each cycle). The 4 tiers:
  LogWarning(0) at goroutines>0.8× or mem>0.5×, GC(1) at mem>0.6× or GCPause>50ms,
  MemoryCleanup(2) at mem>`ResourceGCThresholdMB` (**raw MB**, not %),
  RestartRecommendation(10) at goroutines>1.5× or mem>1.6× (logs only). In non-Debug
  mode `EnableBackgroundStats`/`EnablePerformanceMonitoring` are force-on.
- **`tracing`**: OTel via OTLP gRPC or stdout console (env `OTEL_*` read with raw
  `os.Getenv`, not config). Disabled if neither exporter is set (propagator still
  installed). `TracingProcessor` roots one span per polling update. ⚠️ It has **no
  cache-key sanitization helpers** (older docs claimed it did — false).
- **`httpserver`**: single server on `HTTP_PORT` — `/health` (DB ping + Redis
  set/get/del; 503 if either fails), `/metrics` + `/db_metrics` (Bearer
  `METRICS_AUTH_TOKEN`, constant-time compare; unauthenticated with a warning if
  unset), `/debug/pprof/*` (only if `ENABLE_PPROF`), and webhook on a **static
  `/webhook` path** (secret in the `X-Telegram-Bot-Api-Secret-Token` header, plain
  `!=` compare; 10MB body limit applied before validation; update processed in a
  detached 30s-context goroutine).

---

## 15. Error handling & logging

- **Four-layer recovery**: dispatcher (`dispatcherErrorHandler`) → gotgbot worker
  goroutines → decorator (`WrapCommand`/`WrapCommandRaw`) → handler/goroutine. Use
  `defer error_handling.RecoverFromPanic(funcName, modName)` in every fire-and-forget
  goroutine (it logs + stack, invokes the global `onErrorCallback`, swallows the
  panic — it does not propagate, so forgetting the `defer` crashes the process).
- **`errors.Wrap`/`Wrapf`** capture file/line/function via `runtime.Caller(1)`
  (nil-safe; returns nil for nil err). Only `dispatcherErrorHandler` consumes the
  structured `*errors.WrappedError` fields.
- **`logredact`** (installed in `config.init()` before config load): a logrus hook
  scrubbing **all** levels/fields. Structural patterns mask Telegram bot tokens,
  DSN passwords (`scheme://user:pass@`), and `Authorization: Bearer/Basic`; exact
  secrets are registered via `RegisterSecret(BotToken, DatabaseURL, RedisPassword,
  WebhookSecret, MetricsAuthToken)` (longest-first, ≥6 chars). ⚠️ **When adding a new
  secret config field, add it to that `RegisterSecret` call.** `logredact.Scrub(s)`
  pre-sanitizes a string manually.
- ⚠️ **Never ignore DB errors with `_`** (nil returns cause panics) — except the
  backup import/clear funcs deliberately best-effort-swallow domain-setter errors.
- `helpers.IsExpectedTelegramError` (suppress noise) vs `IsPermissionError` (drives
  `MarkChatRestricted`) are **separate** hardcoded lists — update the right one.
  `SendMessageWithErrorHandling`/`DeleteMessageWithErrorHandling` may return
  `(nil, nil)` — nil-check the returned message.

---

## 16. Backups & rate limiting

- `alita/db/backup` exports/imports/clears **16 modules** (⚠️ older docs said 13):
  admin, antiflood, antiraid, approvals, blacklists, captcha, connections,
  disabling, filters, greetings, locks, notes, pins, reports, rules, warns.
  `BackupFormatVersion = "1.0"`. Export skips per-module failures; **import aborts**
  on the first failure. Some round-trips are lossy (filters export drops bodies;
  greetings/pins partial restores).
- `alita/modules/backup.go` adds Telegram UI, **in-memory** pending-import/reset
  confirmation state (lost on restart, not cross-instance), and rate limiting via
  `ratelimit.GetBackupRateLimiter()` (Redis-backed, **fail-open**; cooldowns export
  5m / import 10m / reset 1h; `Record*` only after success). Import download has an
  SSRF guard (scheme+host equality against `https://api.telegram.org/file/bot`).

---

## 17. Scripts & tooling

- **`scripts/generate_docs/`** — `package main` in the **root module** (`go run .`),
  regex/text parsers (not AST) of locales/modules/config/migrations/chat_status/
  locks → Starlight Markdown. ⚠️ Most generated files carry a
  `<!-- MANUALLY MAINTAINED: do not regenerate -->` sentinel, so `make generate-docs`
  is effectively a no-op except `api-reference/lock-types.md`; editing the frozen
  files by hand is the intended workflow. Lock descriptions are hardcoded in
  `getLockDescription()`. `-inventory` writes `.planning/INVENTORY.{json,md}`.
- **`scripts/check_translations/`** — a **separate Go module** (own `go.mod`); cannot
  import `alita`; uses hardcoded `../../alita` and `../../locales`. Only validates
  **string-literal** keys passed to `tr.GetString`/`GetStringSlice`.
- **`scripts/validate_orphaned_data.go`** — 21 referential-integrity checks
  (`defaultOrphanChecks()`); keep in sync with
  `migrations/20250805204145_add_foreign_key_relations.sql` step 1.
- **`internal/repo_checks/`** — test-only structural-invariant assertions (string/
  regex over source files via `../..`); **sensitive to renames/reformatting** of the
  functions it inspects — update expectations alongside refactors.
- `migrate_psql.sh` (forward-only; richer perl-based Supabase cleaning than the
  Makefile's sed-based `psql-prepare` — `psql-verify` compares against the sed
  output, so keep them aligned).
- **`scripts/bump_version.sh <vX.Y.Z>`** — sed-patches the two version strings
  (`BotVersion` in `alita/config/config.go` + the `--version` fallback in `main.go`);
  BSD/GNU-sed portable, idempotent (a no-op leaves the tree clean so the release
  workflow skips the commit). Wrapped by `make bump-version TAG=vX.Y.Z`.

---

## 18. Coding conventions

- **Imports**: stdlib → third-party → internal, blank-line separated.
- **gofmt** enforced (pre-commit); keep lines under ~100 chars; comments are full
  sentences starting with `// FunctionName`.
- **Naming**: exported PascalCase, unexported camelCase; tests `TestXxx`, helpers
  camelCase; `_test.go` in the same package.
- Value receiver on handler methods — unnamed `(moduleStruct)`, named
  `(m moduleStruct)` only when accessing fields.
- Use `helpers.Ptr[T]` for `*bool`/`*int` literals in gotgbot opts; do not roll your
  own.

### Conventional commits

`feat:` `fix:` `refactor:` `perf:` `test:` `docs:` `chore:` `deps:` (scopes like
`feat(i18n):`). Before committing: `git status`, review `git diff`, stage only
relevant files, run `make lint` + `make test`. Add translation keys to **all**
locale files for user-facing changes. Never commit secrets/`.env`.

---

## 19. Critical rules (hard-won — violating these causes real bugs)

**Go / data**
- Never ignore DB errors with `_`. `ctx.EffectiveSender` can be nil (channel posts).
- `IsUserAdmin` returns false for channel/non-positive IDs — never pass a chat ID
  as a user ID.
- DB writes that gate a user confirmation must be **synchronous** (not goroutines).
- `UpdateRecord` skips zero values — use `UpdateRecordWithZeroValues` for `false`/`0`/`""`.
- Set alias fields consistently (e.g. report `Enabled`+`Status`).

**Handlers / callbacks**
- Watchers return `ext.ContinueGroups`; commands return `ext.EndGroups`.
- Use the versioned callback codec; never `strings.Split` raw data; respect the
  64-byte limit (use the Redis-token pattern for user text).
- After `IsUserConnected`, reassign `ctx.EffectiveChat` to the returned chat.
- Don't double-answer callbacks already answered by `RequireUserAdmin`.
- Check both `msg.Entities` AND `msg.CaptionEntities` for URLs/mentions.
- Chat-scoped deep links must gate on `IsUserInChat`.

**Database**
- Migration → struct → optimized query → repository function (+ `testmain_test.go`).
- Invalidate the exact cache key on every write; key **prefixes ≠ package names**.
- Surrogate keys (`id` PK, external IDs unique). Never edit an applied migration.

**i18n**
- Double-quote YAML with escapes; `%d` needs a real int; verify keys exist in **all**
  locale files; convert Markdown→HTML for sends.

**Boolean logic**
- `IsAnonymousChannel() || IsLinkedChannel()` matches almost everything — test lock/
  filter predicates with multiple message types.

---

## 20. Environment configuration

See `sample.env`. **Required**: `BOT_TOKEN`, `OWNER_ID`, `MESSAGE_DUMP`,
`DATABASE_URL`, and one of `REDIS_ADDRESS`/`REDIS_URL`. Conditionally required when
`USE_WEBHOOKS=true`: `WEBHOOK_DOMAIN`, `WEBHOOK_SECRET`.

Notable defaults & ⚠️ gotchas (config is loaded manually in `config.go`; `validate:`
and `env:` struct tags are decorative — `ValidateConfig` is hand-written):

- `HTTP_PORT` 8080, `DISPATCHER_MAX_ROUTINES` 200, `REDIS_DB` **1** (you cannot
  select 0), pool: `DB_MAX_IDLE_CONNS` 50 / `DB_MAX_OPEN_CONNS` 200 /
  `DB_CONN_MAX_LIFETIME_MIN` 240 / `DB_CONN_MAX_IDLE_TIME_MIN` 60.
- ⚠️ `ENABLE_PERFORMANCE_MONITORING` and `ENABLE_BACKGROUND_STATS` **cannot be
  disabled via env** (forced true when not Debug). `ENABLE_AUTO_CLEANUP` and
  `CLEAR_CACHE_ON_STARTUP` default true but **do** honor an explicit `false`.
- `AUTO_MIGRATE` / `AUTO_MIGRATE_SILENT_FAIL`, `MIGRATIONS_PATH` (default
  `"migrations"`, relative to cwd), `ENABLED_LOCALES` (picker only), `API_SERVER`,
  `DROP_PENDING_UPDATES`, `ENABLE_PPROF`, `METRICS_AUTH_TOKEN`, `DEBUG`.
- `OTEL_*` (service name, sample rate, OTLP endpoint, console/insecure) are read via
  raw `os.Getenv`, not config, and are intentionally not in `sample.env`.
- `BotVersion` lives in `config.go` (currently `"2.19.6"`), mirrored by a CLI
  fallback `version = "v2.19.6"` in `main.go`. **Don't hand-edit it** —
  `scripts/bump_version.sh <vX.Y.Z>` patches both, and the release workflow runs it
  automatically on `workflow_dispatch`; the `goreleaser` job then re-greps both files
  and fails on mismatch. For a manual tag-push release, run the script (or
  `make bump-version TAG=vX.Y.Z`) and commit before tagging.

Additional env vars present in `config.go` (defaults in parens) not covered above:
`ENABLE_DB_MONITORING` (false; gates `/db_metrics`), `WEBHOOK_PORT` (deprecated,
legacy 8081, backfills `HTTP_PORT`), `INACTIVITY_THRESHOLD_DAYS` (30),
`ACTIVITY_CHECK_INTERVAL` (1), `HTTP_MAX_IDLE_CONNS` (100),
`HTTP_MAX_IDLE_CONNS_PER_HOST` (50), `RESOURCE_MAX_GOROUTINES` (1000),
`RESOURCE_MAX_MEMORY_MB` (500), `RESOURCE_GC_THRESHOLD_MB` (400, the raw-MB
`MemoryCleanup` trigger in §14).

---

## 21. Dependency risks (tracked, not oversights)

- **`gotgbot/v2 v2.0.0-rc.35`** — a release candidate; a future `rc.36`/`v2.0.0`
  may break the hot path (handler signatures, Update parsing). Evaluate/migrate when
  `v2.0.0` final ships. **Do not auto-merge** Dependabot PRs that bump its major or
  RC number without a code-compatibility review.
- **`gotg_md2html v0.0.0-20260314092343-…`** — an untagged pseudo-version; a force-
  push upstream breaks reproducible builds. Keep the `go.sum` entry pinned; prefer a
  tagged release if published. Don't run `go get ./...` blindly.

The dependabot auto-merge workflow has **no special-case exclusion** for these — the
safeguard relies on them being major bumps or on branch-protection settings.

---

## 22. Security notes

- Never commit secrets; pre-commit detects private keys + large files. Secrets are
  scrubbed from logs by `logredact` (register new secret fields there).
- Disable `ENABLE_PPROF` in production. Webhook mode needs HTTPS (Cloudflare Tunnel
  supported) and validates only the secret-token header on a static path.
- `/metrics` + `/db_metrics` require a Bearer token when `METRICS_AUTH_TOKEN` is set
  (constant-time compare); they are unauthenticated (with a warning) otherwise.
- Deep links and callback confirmation handlers **re-check permissions** (stale/
  forwarded buttons) — never remove those re-checks.

## Agent skills

### Issue tracker

Issues live as GitHub issues in `Divkix/Alita_Robot` (use the `gh` CLI); external PRs are not a triage surface. See `docs/agents/issue-tracker.md`.

### Triage labels

Five canonical triage roles mapped to GitHub labels with their default names (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
