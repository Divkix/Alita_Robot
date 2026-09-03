# Repository Guidelines

Alita Robot — Telegram group-management bot in **Go 1.26** / **gotgbot/v2** `v2.0.0-rc.36`.
Features: admin, filters, notes, greetings, antiflood/antiraid/antispam, captcha, warns, locks, backups, connections, reactions, i18n (en/es/fr/hi/ru/pt/id).

> `CLAUDE.md` and `GEMINI.md` are symlinks to `AGENTS.md` — edit only this file.

## 0. Maintaining this file

- File is hand-maintained. When you change build, env, routes, DB, or layout — update it in the same commit.
- Record **why / gotcha / coupling**, not what the code already shows. Be specific: name file/func/env/table.
- Consolidate in place; delete stale notes. One accurate sentence > three vague ones.
- Verify before trusting: if a note names a symbol, confirm it still exists.

---

## 1. Mental model

```
Telegram ──► polling OR webhook /webhook POST
          ──► ext.Dispatcher (TracingProcessor span per update)
          ──► handlers by group: -10..-1 interceptors → 0 commands (EndGroups) → 4..11 watchers (ContinueGroups)
          ──► repo (GORM/Postgres + Redis read-through) → reply via i18n + media
```

- **Config + DB open in `init()`**, not `main()`. `alita/config` → `config.AppConfig`, `alita/db` → Postgres pool. Both no-op for `--version`/`--health` or missing env (so tests can import). Don't move to `main()`.
- **DB:** `alita/db/<domain>/` repos + `alita/db/models/` structs. `alita/db/db.go` is a re-export shim for model types & `TEXT`..`VIDEO_NOTE` constants only — not cache helpers.
- **Schema source is `migrations/*.sql`** via `alita/db/migrations/runner.go`, not `gorm.AutoMigrate`. Tests use SQLite `AutoMigrate` (`testmain_test.go`) — struct↔SQL drift is not caught.
- **Cache is Redis-only** (`eko/gocache` + `go-redis`), nil-safe (nil marshaler → direct DB).
- **Modules self-register in `init()`**, load ascending priority; Help loads last (deferred).
- **Callback data:** versioned codec `<namespace>|v1|<url-encoded>` capped at **64 bytes** — never `strings.Split` raw data.

---

## 2. Project structure

- `main.go` — CLI flags, polling/webhook branch, dispatcher, shutdown, tuned Bot-API transport.
- `alita/main.go` — `LoadModules`, `InitialChecks`, `ListModules`.
- `alita/config/` — manual env load/validate + `logredact` wiring in `init()`. No viper. `types.go` has `typeConvertor`.
- `alita/db/` — `db.go` (OTel CRUD wrappers + shim), `conn.go` (pool, `AUTO_MIGRATE`), `models/` (all GORM structs + `types.go` JSONB: `ButtonArray`/`StringArray`/`Int64Array`), `<domain>/` repos (`admin, antiflood, antiraid, approvals, blacklists, captcha, channels, chats, connections, devs, disabling, federations, filters, greetings, lang, locks, logchannels, notes, pins, reports, rules, user, warns`), `cache/` (`CacheKey`, `GetFromCacheOrLoad` singleflight, `DeleteCache`), `migrations/runner.go`, `monitoring/metrics.go`, `backup/` (19 modules).
- `alita/i18n/` — singleton `LocaleManager`, `go:embed` `locales/`, yaml→`map[string]any`, dot-path lookup + case-insensitive fallback. No viper.
- `alita/modules/` — feature modules + `registry.go`/`core.go`.
- `alita/utils/` — `chat_status`, `helpers` (command pipeline), `cache`, `callbackcodec`, `formatting`, `keyboard`, `keyword_matcher`, `media`, `content`, `extraction`, `error_handling`, `errors`, `logredact`, `ratelimit`, `constants`, `monitoring`, `shutdown`, `tracing`, `httpserver`, `actionlog`.
- `locales/` — 7 yml + `config.yml` (pseudo-language `"config"` → `alt_names` + `db_default_*`).
- `migrations/` — timestamped SQL (source of truth).
- `scripts/` — `generate_docs/` (root module), `check_translations/` (separate go.mod), `validate_orphaned_data.go`, `migrate_psql.sh`, `backup_database.sh`, `bump_version.sh`.
- `internal/repo_checks/` — structural-invariant tests (fragile to renames).
- `docs/` — Blume site (`bun`, Cloudflare Workers), `docs/blume.config.ts`.
- `.github/workflows/` — `ci.yml`, `release.yml`, `docs.yml`, `dependabot-native-merge.yml`, `pullfrog.yml`.
- `docker/` — `alpine` (prod distroless), `alpine.debug`, `goreleaser`, `pr-build`.

---

## 3. Build, Test, Lint

```bash
make run                # go run main.go
make build              # goreleaser snapshot
make lint               # golangci-lint v2
make test               # go test -tags testtools -race -coverprofile -coverpkg=<alita/*> -count=1 -timeout 10m ./...
make test-postgres-integrity  # needs DATABASE_URL (Postgres constraints/concurrency)
make tidy / vendor
make check-translations # missing-key gate (separate module)
make check-duplicates   # dupl on Go code, not translations
make generate-docs      # regenerate docs
make check-docs         # docs drift gate
make inventory          # .planning/INVENTORY.{json,md}
make docs-dev           # blume dev server
make psql-migrate
make psql-status
make psql-reset
make validate-db        # orphan checks
make backup-db
make bump-version TAG=vX.Y.Z
```

- Default tests are self-contained (SQLite + miniredis). `CGO_ENABLED=1` needed for `-race`; binaries use `CGO_ENABLED=0`.
- `-coverpkg` excludes root `main` + `scripts/`; **coverage gate 78%** in `ci.yml`.

---

## 4. CI/CD

- `release.yml` (`v*` tag or dispatch): same gates + `goreleaser` v2.13.0 → GHCR `{{.Tag}}`/`{{.Version}}`/`latest`, SLSA attest, Trivy info. Tag must be `vMAJOR.MINOR.PATCH[-prerelease]` (`^vMAJOR.MINOR.PATCH(-prerelease)?$`). Dispatch normalizes tag, bumps `BotVersion` in `config.go` + `main.go`, re-creates exact tested tree (fails if `main` moved). No `main.version` ldflags (no such vars).
- `docs.yml` (path-filtered) → `make generate-docs` → Bun build → Cloudflare Workers (wrangler@4) on push to main.
- `dependabot-native-merge.yml` (`pull_request_target`, no checkout): auto-merge patch/minor except `gotgbot`/`gotg_md2html`.
- Local gates: `pre-commit` (trailing-whitespace, yaml, large-file 1000KB, private-key, golangci-lint v2.11.4, `gofmt`, `go mod tidy`), `.golangci.yml` (`godox`, `dupl` 100, `gocyclo` 20, `new:true`).
- **Version:** `BotVersion` in `alita/config/config.go` + fallback `version = "v…"` in `main.go`. Don't hand-edit — use `make bump-version`; goreleaser greps both and fails on mismatch (currently `2.22.0`).

---

## 5. Startup & shutdown

`main()` (config+DB already in `init()`):
1. `appStartTime` → `/health` uptime.
2. Raw `os.Args` flags: `--health` (GET /health) / `--version`.
3. Panic `defer` → `os.Exit(1)`.
4. `cache.InitCache()` first, fatal; FLUSHDB if `ClearCacheOnStartup` (default true).
5. `i18n.Initialize` (embed locales).
6. `tracing.InitTracing()` non-fatal.
7. `gotgbot.NewBot` (tuned transport, `API_SERVER`) → `alita.InitialChecks` (`EnsureBotInDb`).
8. Dispatcher (`TracingProcessor`, `dispatcherErrorHandler`, `MaxRoutines` 200) → monitoring → shutdown → HTTP server.
9. Branch `UseWebhooks`: webhook (needs `WEBHOOK_DOMAIN`+`WEBHOOK_SECRET`, `select{}`) or polling (`DeleteWebhook`→`StartPolling`). `postInit` loads modules, restores captcha, sets `/start`/`/help`, startup msg to `MESSAGE_DUMP`.

Shutdown (`alita/utils/shutdown`): SIGTERM/SIGINT → LIFO handlers (reverse registration) with panic recovery, **60s** timeout → `os.Exit`. Registration order is DB first, updater/HTTP last; execution (LIFO) is updater/HTTP first, DB last — keep this order when adding handlers.

---

## 6. Module system

- `RegisterLegacyModule(name, priority, loadFunc)` in each module's `init()`; dedup by name (first wins). `LoadAllModules` stable-sorts ascending (lower = earlier). `alita.LoadModules` resets `AbleMap`, defers `LoadHelp`.
- Priorities — edit literal in `init()` to reorder:

| Pri | Module | Pri | Module | Pri | Module |
|----:|--------|----:|--------|----:|--------|
| -10 | BotUpdates | 80 | Mutes | 190 | Rules |
| 10 | Antispam | 90 | Purges | 200 | Warns |
| 20 | Languages | 100 | Users | 210 | Greetings |
| 30 | Admin | 110 | Reports | 220 | Captcha |
| 40 | Approvals | 120 | Dev | 230 | AntiRaid |
| 50 | Pins | 130 | Locks | 235 | Federations |
| 55 | LogChannels | 140 | Filters | 240 | Blacklists |
| 60 | Misc | 150 | Antiflood | 250 | Reactions |
| 70 | Bans | 160 | Notes | 260 | Formatting |
|  |  | 170 | Connections | 270 | Backup |
|  |  | 180 | Disabling |  |  |

Help not in registry (last). `moduleStruct` (in `core.go` — no `helpers.go`) holds `AbleMap` (`map[string]bool` + `ableMapMu`), `helpableKb` (Title-cased keys, i18n `<lowercase>_help_msg`), `AltHelpOptions`. Writes happen at startup only; don't write `AbleMap` from handlers. Value-receiver on handler methods so struct must not embed mutex.

Adding a module: `migrations/*.sql` → `models/<domain>.go` + alias in `db.go` → `db/<domain>/repository.go` → `modules/<name>.go` with `LoadXxx` (`RegisterLegacyModule` + `AbleMap[name]=true`) → keys in all 7 locales.

Command registration:
- New: `helpers.WrapCommand(dispatcher, CommandDescriptor, handler)` (`command_pipeline.go`, used by `admin`/`pins`) — panic recovery → `BuildCommandContext` (sentinel `ext.EndGroups`) → ordered `RequiredChecks` (`RequireGroup`, `RequireUserAdmin`, `CanUserPromote`…). `Disableable:true` auto-registers aliases.
- Legacy: `dispatcher.AddHandler(handlers.NewCommand(...))` / `helpers.MultiCommand` + `helpers.AddCmdToDisableable`.

---

## 7. Handlers, callbacks, routing, permissions

**Handler groups:** -10 captcha-pending, -6 federations watcher, -5 antiraid, -2 antispam, -1 Users tracker (must return `ContinueGroups` and synchronously create/update chat+user parent rows via `updateCurrentChat`/`updateCurrentUser` before later groups write FK-dependent rows — do not move to goroutines); 4 antiflood, 5 locks perm / 6 restr, 7 blacklists, 8 reports+reactions, 9 filters, 10 pins, 11 log-channel capture. Commands → `ext.EndGroups`, watchers → `ext.ContinueGroups`.

**Callbacks:** `alita/utils/callbackcodec` + `modules/callback_codec.go` → `<ns>|v1|<url-encoded>`, 64B cap. `encodeCallbackData` returns `""` on overflow (broken button). For user text use **token pattern** (store in Redis, short hex token in callback; filters/notes). `decodeCallbackData` is strict, rejects dot-notation. Guard every callback with `callbackQueryFromContext(ctx)` (nil-safe, also check `query.Message`); `CallbackQuery.Message` is a `gotgbot.Message` value not pointer — use interface methods + `ctx.EffectiveMessage`.

**Anonymous admin:** `GroupAnonymousBot` → `chat_status.checkAnonAdmin` (bypass if `AnonAdmin` on, else cache `alita:anonAdmin:<chat>:<msg>` 20s + prove-admin button) → `verifyAnonymousAdmin` re-checks, restores `EffectiveMessage`, nils `SenderChat`/`CallbackQuery`, re-dispatches. Bypasses `WrapCommand` checks — anon wrappers must re-enforce perms.

**Deep links** (`deeplink_router.go`): `/start <payload>` private 2-arg → `HandleDeepLink` (exact then longest-prefix: `help_`, `about`, `rules_`, `notes_`, `note_`, `note`, `connect_`). ⚠️ Every chat-scoped link must gate on `chat_status.IsUserInChat` (notes also `IsUserAdmin`) — else leaks private data. `connect_` revalidates; transient lookup preserves connection, definitive non-member disconnects.

**Double-answer:** `RequireUserAdmin`/`RequireUserOwner` with `justCheck=false` already answers — don't answer again; pipeline uses `WithReplyFallback()`.

**Permissions** (`alita/utils/chat_status/` — `access.go` + `chat_status.go` + `permission_responder.go`):
- `RequireGroup`/`RequirePrivate`, `RequireBotAdmin`/`RequireUserAdmin`/`RequireUserOwner` are pure bool; messaging via `NewPermissionResponder(b).Respond(...)` (always false, picks callback vs `SendMessage`/`Reply`).
- `CanUser*` share `hasUserPermission` (creator bypasses all); `CanBot*` have no anon/creator fallback and nil-guard bot.
- ⚠️ `IsUserAdmin` false for channel IDs and `id<=0` (`IsValidUserId`, `IsChannelId` id<-1e12) — never pass chat ID as user ID. `IsBotAdmin` true in private else `status=="administrator"`. `tgAdminList` = 1087968824 + 777000 (not 136817688).
- `IsUserConnected` (PM → connected chat) — caller must reassign `ctx.EffectiveChat`.
- Admin lookups via Redis admin cache (30m); invalidation is admin module's job. `GetEffectiveUser`/`RequireUser` nil-safe (channel posts).

---

## 8. Database, cache, migrations

**Wrappers** (`alita/db/db.go`): OTel-traced `GetRecord`/`GetRecords`/`CreateRecord`/`UpdateRecord`/`UpdateRecordWithZeroValues` + `ChatExists`. `conn.go`: `PrepareStmt:true`, UTC `NowFunc`, logrus GORM logger (slow 1s), 5-retry backoff.

- ⚠️ `UpdateRecord` ignores zero values — use `UpdateRecordWithZeroValues(map[string]any)` for `false`/`0`/`""`.
- `UpdateRecord*` returns `ErrRecordNotFound` when `RowsAffected==0`. `ChatExists` treats any error as absent.

**Models** (`alita/db/models/`):
- Surrogate `ID uint` PK; Telegram id is separate **unique** column. SQL `bigint` is authoritative.
- JSONB `ButtonArray`/`StringArray`/`Int64Array` (`Scan`/`Value`, empty→`"[]"`).
- `GreetingSettings` embeds `*WelcomeSettings`/`*GoodbyeSettings` (`embeddedPrefix:welcome_` etc.) — pointers may be nil, map upserts need prefixed columns.
- ⚠️ Table names ≠ struct names: `AdminSettings→admin`, `ConnectionSettings→connection` (per-user) vs `ConnectionChatSettings→connection_settings` (per-chat, inverted), `WarnSettings→warns_settings`, `Warns→warns_users`, `DisableSettings→disable`. Check `TableName()` before raw SQL.
- Dead fields don't use: `antiflood_settings.limit/.mode`→`flood_limit`/`action`, `devs.dev`→`is_dev`, `connection_settings.enabled`→`allow_connect`; `chat_users` removed (use `chats.users` JSONB). `ReportChatSettings`/`ReportUserSettings` need `Enabled`+`Status` both set.
- Uniqueness includes: one `connection` per `user_id`, one `captcha_attempts`/`captcha_muted_users` per `(user,chat)`, one case-insensitive `channels.username`. `connection` disconnect keeps `chat_id` for `/reconnect` (gate = admin or `AllowConnect`+membership).
- Checklist for schema change: **migration → struct → optimized query column list → repository → `testmain_test.go` AutoMigrate**.

**Per-domain repos:**
- Read-through `cache.GetFromCacheOrLoad(cache.CacheKey(module,id), ttl, loader)` — singleflight, 30s timeout (`Forget` on timeout). Writes must `cache.DeleteCache` every affected key; don't bypass.
- ⚠️ Key prefixes ≠ package names: `blacklists→"blacklist"`, `channels→"channel"`, `chats→"chat"`, `captcha→"captcha_settings"`, `notes→"notes_settings"`, `disabling→"disabled_cmds"`, `warns→"warns"`+`"warn_settings"`, `filters→"filter_list"`+`"filters_optimized"`, `locks→"lock"`+`"locks_map"`, `lang→"chat_lang"`/`"user_lang"` (also `"chat_settings"`/`"chat"`/`"user"`), `federations→"fed"`+`"fed_chat"`+`"fed_admins"`+`"fed_ban"`+`"fed_subs"`, `logchannels→"log_channel"`. `admin, connections, devs, pins, reports, rules` have **no cache**.
- Upserts use `clause.OnConflict` (locks, captcha, filters, notes, connections, user/chat anchors). Warns/reports lock parent row; channels clear prior owner+caches. `chats.UpdateChat` appends JSONB via `users || to_jsonb(...)` (pg-specific). Disabling load errors never cached as empty list. Most reads swallow errors and return defaults (`"en"`, empty slice) — don't rely on error to detect missing data. `user.GetUserBasicInfoCached` negative-caches missing as `UserId:-9999`.

**Migrations** (`alita/db/migrations/runner.go`, manual `scripts/migrate_psql.sh`):
- Runtime runner runs only when `AUTO_MIGRATE=true`; manual script / `make psql-migrate` is explicit and does not check `AUTO_MIGRATE`. Lexically sorted, one transaction per file (records `schema_migrations` in same tx).
- **SHA-256 over raw bytes** → applied files are immutable; even whitespace edit fails startup (unless `AUTO_MIGRATE_SILENT_FAIL`). Always add new file with greater timestamp; never edit applied one.
- `cleanSupabaseSQL` strips GRANT/POLICY, injects `IF NOT EXISTS` / `DO $$` for idempotency. `splitSQLStatements`/`findDollarQuoteBlocks` share tokenizer. `CREATE INDEX CONCURRENTLY` can't run inside tx. Keep runner and `migrate_psql.sh` cleaning aligned. Forward-only, no rollback.

---

## 9. Cache layer (`alita/utils/cache/`)

- `InitCache` — 5-retry, optional FLUSHDB (`ClearCacheOnStartup` default true). ⚠️ `ClearAllCaches` FLUSHDBs whole DB — Redis assumed dedicated. Default `RedisDB=1` (explicit 0 honored). `REDIS_URL` (`ParseURL`) vs `REDIS_ADDRESS` (direct, ignores URL creds); `REDIS_PASSWORD` overrides; else `localhost:6379`. Always `cache.GetMarshal()` nil-check.
- Key `alita:{module}:{id}…`. Admin cache `alita:adminCache:<chat>` 30m (O(1) `UserMap`, negative cached, singleflight fetch with `ReturnBots:true`). Restricted cache `alita:restricted:<chat>` 30m, 5m probe `SETNX` (`restricted_probe`), fail-open on nil/malformed.
- Driven by `media.Send` / `helpers.SendMessageWithErrorHandling`.

---

## 10. i18n (`alita/i18n/`)

- `LocaleManager` singleton (`GetManager()`/`sync.Once`), `Initialize` once (after cache), `go:embed` all `locales/*.yml` keyed by filename. `locales/config.yml` is pseudo-language `"config"` for `alt_names.<Module>` + `db_default_*` — don't move.
- `ENABLED_LOCALES` only filters `/lang` picker; all locales always loaded. Callback allowlist in `alita/modules/language.go` must match embedded files (exclude `"config"`).
- `MustNewTranslator(lang)` falls back to English (382 call sites). Language via `lang.GetLanguage(ctx)` (user in PM, group in groups, `"en"` default).
- `GetString` falls back to English, supports `{named}` + legacy `%s`/`%d`; named→positional via `commonKeys` order in `extractOrderedValues` (`first,second,…,question,answer,number,count,value,name,user,username,…`) — extend it if you add a new `%verb` name.
- Help/status strings are mixed Markdown vs HTML. Convert Markdown with `formatting.ToTelegramHTML` (keeps `<b>`/`<code>` etc. when opener+closer present, escapes `<keyword>`). Don't run `MD2HTMLV2` on already-HTML strings or on concatenated header+body; Markdown bodies via `MD2HTMLV2`, HTML bodies keep tags.
- Add keys to **all 7** locales; `%d` needs an int.

---

## 11. Anti-abuse & content — essentials

- **Antiflood** (group 4): per-user count (`*sync.Mutex` per key + map, cleaned 5m). `/setflood` `off`/`0` or `3..100`. Warm admin cache trusted; miss → bounded `IsUserAdmin` lookup, only timeout/semaphore-full → fail-open (assume admin); semaphore released before punishment; cleanup recovers per tick.
- **Antiraid** (group -5, Redis-only `alita:antiraid:state:<chat>` + join zset, CAS scripts, 30s expiry poller `Start/StopAntiRaidExpiryPoller`). `parseDuration` needs unit `s/m/h/d/w`, cap 366d. Defaults `RaidTime 21600s`, `RaidActionTime 3600s`, `AutoAntiRaidThreshold 0`.
- **Federations** (group -6, pri 235): one fed per owner, chat joins one fed, max 5 subs (`federation_subs`). Watcher fbans local + subscribed feds. `DeleteFederation` locks row + lists chat/ban/sub keys inside tx then invalidates. Backup: membership only (`fed_id`+`quiet`). `/stats` includes global federation totals via `federations.LoadFederationStats` (same as `/fedinfo` per-fed); `/fedstat` is per-user lookup.
- **Log channels** (group 11): `/setlog` in channel stores `alita:setlog:<chan>:<msgId>` 1h (exact msgId, no `:0` wildcard); forward binds `log_channels`. Categories `settings/admin/user/automated/reports/other` default on. `actionlog` must check `chat.Type=="channel"`.
- **Antispam** (group -2): local 18/sec telemetry only, always `ContinueGroups` — not a global ban.
- **Captcha** (~2100 lines): math/image verification, refresh cooldown 5s max 3, single attempt per `(user,chat)`, callback carries `refresh_count` + attempt ID/answer/msg/version checks, atomic claim+retry row, `kick` via `unbanChatMember(only_if_banned=false)`, `mute` 24h; disabling/approval releases pending. Group -10 deletes pending msgs.
- **Approvals:** whitelist skips antiflood/blacklists/locks/captcha/antispam. `/unapproveall` owner-only.
- **Disabling:** `CheckDisabledCmd` (bypasses admins/PM, optional delete via `ShouldDel`); only cmds registered via `AddCmdToDisableable` are disableable.
- **Filters/Blacklists:** Aho-Corasick (`keyword_matcher`) with separate named caches (`"filters"`/`"blacklists"`), `FirstMatch` + `Find` for action, `MutedPermissions`, match text from `text+caption+URL entities` (both `Entities`+`CaptionEntities`, slice via `extractEntityText` — offsets are UTF-16).
- **Filters/Notes overwrite:** Redis token `alita:{filter|note}_overwrite:<token>` 5m, `GETDEL` on confirm, `ON CONFLICT DO NOTHING` preserves existing.
- **Greetings:** join fires `ChatMemberUpdated` + service msg deduped via `claimRecentJoinProcessing` (SETNX 5s); `SendCaptcha` owns mute→restrict→challenge+rollback.
- **Locks:** `lockMap` (perm g5) + `restrMap` (g6), skip admins/approved, need `CanBotDelete`; `bots` lock is separate `ChatMember` handler.
- **Rules:** stored HTML (`MD2HTMLV2`), legacy Markdown re-rendered when no tags; no cache.
- **Reactions:** only Telegram built-in emoji, HTML-escaped; `FormattingReplacer` handles `{rules}` only in template. `{count}` via `cachedMemberCount` (sync.Map 60s TTL).
- **Media:** `Send` on `MsgType` 1..8 (0→text, empty FileID→text), respects `IsChatRestricted`; `SendNote`/`SendFilter` do `%%%` variants + `FormattingReplacer`; only URL buttons survive storage.
- Moderation: `moderationCommand` (`RequireUser`→gates→extract→validate→execute→reply, `EndGroups`); `standardModGates`/`deleteModGates`; `ExtractUserAndText` returns `-1` (already replied, abort) vs `0` (empty).

---

## 12. Observability, backups, scripts

- **Monitoring** (`alita/utils/monitoring` not `db/monitoring`): `ActivityMonitor` (DAU/WAU/MAU), `BackgroundStatsCollector` (30s/1m/5m tickers under mutex), `AutoRemediationManager` (1/min, 4 tiers: LogWarning 0 at goroutines>0.8× or mem>0.5×, GC 1 at mem>0.6× or GCPause>50ms, MemoryCleanup 2 at `ResourceGCThresholdMB` raw MB, RestartRecommendation 10). Honors explicit `ENABLE_…=false`.
- **Tracing:** OTel OTLP gRPC or stdout (`OTEL_*` via `os.Getenv`, not config); `TracingProcessor` 1 span/update.
- **Backups** (`alita/db/backup`, `BackupFormatVersion "1.1"` compat `1.0`): 19 modules (admin, antiflood, antiraid, approvals, blacklists, captcha, connections, disabling, filters, greetings, locks, notes, pins, reactions, reports, rules, warns, federations, logchannels). Validates first then replaces all requested modules in one transaction (all-or-nothing), invalidates caches. Federation membership only. Module `backup.go` adds one-use nonce 10m + Redis/in-mem rate limit (export 5m/import 10m/reset 1h, atomic `SETNX`, fail-open without Redis, 10MB Telegram file limit with host check).
- **Errors/logging:** 4-layer recovery (dispatcher→worker→`WrapCommand`→handler); fire-and-forget must `defer error_handling.RecoverFromPanic`. `errors.Wrap/Wrapf` via `runtime.Caller(1)`. `logredact` hook scrubs tokens/DSN/`Authorization` + `RegisterSecret` (≥6 chars, longest-first) — add new secrets there. Never ignore DB errors (`_`) on state-changing paths; `IsExpectedTelegramError` vs `IsPermissionError` are separate lists; `SendMessageWithErrorHandling` may return `(nil,nil)`.
- **Scripts:** `generate_docs` (regex parsers) updates unfrozen `commands/users|` `federations|` `logchannels/index.md` + `api-reference/lock-types.md`; frozen files have `<!-- MANUALLY MAINTAINED: do not regenerate -->`. `check_translations` validates literal `GetString` keys only. `validate_orphaned_data.go` 26 FK checks. `bump_version.sh` patches both version strings.

---

## 13. Coding conventions

- Imports: stdlib → third-party → internal, blank lines. `gofmt`, ~100 cols, `// Func` sentences.
- Naming: exported PascalCase, unexported camelCase, tests `TestXxx`, `_test.go` same package. Handler methods value receiver, named `(m moduleStruct)` only when accessing fields.
- `helpers.Ptr[T]` for `*bool`/`*int` in gotgbot opts.
- Commits: `feat:` `fix:` `refactor:` `perf:` `test:` `docs:` `chore:` `deps:` + scope. Before commit: `git status`, `git diff`, stage only relevant, `make lint` + `make test`, add keys to all locales, never commit secrets.

---

## 14. Critical rules — break these → real bugs

- Never `_` a DB error on a state-changing path; `ctx.EffectiveSender` can be nil. Announce success only after write succeeds.
- `IsUserAdmin` false for channel/non-positive IDs — never pass chat ID as user ID.
- Synchronous writes for confirmations; `UpdateRecord` skips zeros → `UpdateRecordWithZeroValues(map[string]any)` for `false`/`0`/`""`; set report alias fields `Enabled`+`Status` together.
- Commands `EndGroups`, watchers `ContinueGroups`.
- Callback codec only, never `strings.Split`; respect 64B cap, use Redis token for user text; after `IsUserConnected` reassign `EffectiveChat`; don't double-answer callbacks.
- Check `Entities` **and** `CaptionEntities`; entity offsets are UTF-16 → `extractEntityText`.
- Chat-scoped deep links must `IsUserInChat` (notes also `IsUserAdmin`).
- Migration → struct → optimized query → repo → `testmain_test.go`; invalidate exact cache key (prefixes differ); never edit applied migration; surrogate `ID` PK.
- i18n: double-quoted YAML, `%d` needs int, all 7 locales, `ToTelegramHTML` not `MD2HTMLV2` on HTML strings.
- `IsAnonymousChannel() || IsLinkedChannel()` is almost everything — test predicates with many message types.

---

## 15. Environment

See `sample.env`. Required: `BOT_TOKEN`, `OWNER_ID`, `MESSAGE_DUMP`, `DATABASE_URL`. Redis required (`localhost:6379` or `REDIS_ADDRESS`/`REDIS_URL`; `REDIS_PASSWORD` overrides). If `USE_WEBHOOKS=true`: `WEBHOOK_DOMAIN` + `WEBHOOK_SECRET`.

Defaults / gotchas (`config.go` manual load; `validate:`/`env:` tags are decorative):
- Port `HTTP_PORT`→`PORT`→8080; `DISPATCHER_MAX_ROUTINES` 200; pool 50 idle / 200 open / 240m lifetime / 60m idle.
- `REDIS_DB` **1** (explicit `0` honored); `CLEAR_CACHE_ON_STARTUP` true.
- `ENABLE_PERFORMANCE_MONITORING`/`ENABLE_BACKGROUND_STATS` default true only when `DEBUG=false` (explicit `false` honored; in debug mode both default false), `ENABLE_AUTO_CLEANUP` always defaults true; `ENABLE_DB_MONITORING` false (gates `/db_metrics`).
- `AUTO_MIGRATE`/`AUTO_MIGRATE_SILENT_FAIL`, `MIGRATIONS_PATH` `migrations`, `ENABLED_LOCALES` (picker only), `API_SERVER`, `DROP_PENDING_UPDATES`, `ENABLE_PPROF`, `METRICS_AUTH_TOKEN`, `DEBUG`.
- `OTEL_*` via `os.Getenv` (not in sample.env), `INACTIVITY_THRESHOLD_DAYS` 30, `ACTIVITY_CHECK_INTERVAL` 1, HTTP idle 100 / per-host 50, `RESOURCE_MAX_GOROUTINES` 1000, `RESOURCE_MAX_MEMORY_MB` 500, `RESOURCE_GC_THRESHOLD_MB` 400 (raw MB trigger).

---

## 16. Security & dependencies

- Never commit secrets; `logredact` scrubs logs — register new secrets there. Disable `ENABLE_PPROF` in prod. Webhook needs HTTPS, validates secret header on static path. `/metrics` needs Bearer token if `METRICS_AUTH_TOKEN` set (constant-time). Deep links/callbacks re-check perms — don't remove.
- `gotgbot/v2 rc.36` (RC) and `gotg_md2html` pseudo-version are pinned; Dependabot auto-merge excludes them.

## Agent skills

- **Issue tracker:** GitHub issues in `Divkix/Alita_Robot` (`gh` CLI). See `docs/agents/issue-tracker.md`.
- **Triage labels:** `needs-triage` / `needs-info` / `ready-for-agent` / `ready-for-human` / `wontfix`. See `docs/agents/triage-labels.md`.
- **Domain docs:** No `CONTEXT.md` / `docs/adr/` yet. See `docs/agents/domain.md`; `/domain-modeling` creates them lazily.

