# Codebase Concerns

**Analysis Date:** 2026-02-23

---

## Tech Debt

**Legacy dot-notation callback parsing still widespread:**
- Issue: `strings.Split(query.Data, ".")` used across 14+ modules alongside the newer versioned codec. Both paths exist, creating dual maintenance burden.
- Files: `alita/modules/blacklists.go:497`, `alita/modules/notes.go:638`, `alita/modules/bans.go:1093`, `alita/modules/warns.go:566`, `alita/modules/warns.go:796`, `alita/modules/filters.go:503`, `alita/modules/help.go:208`, `alita/modules/greetings.go:1053`, `alita/modules/connections.go:228`, `alita/modules/reports.go:434`, `alita/modules/language.go:94`, `alita/modules/purges.go:277`, `alita/modules/captcha.go:1279`, `alita/modules/captcha.go:1518`
- Impact: Every new callback handler continues using the old pattern. No bounds checking on `args[N]` when split result is shorter than expected.
- Fix approach: Migrate all handlers to `decodeCallbackData()` / the versioned codec in `alita/utils/callbackcodec/`. The `alita/modules/callback_parse_overwrite.go` bridge already exists but callers haven't been converted.

**Redundant alias fields in DB structs — `Dev`/`IsDev` and `Action`/`Mode`:**
- Issue: `DevSettings` has both `IsDev` and `Dev` columns (`alita/db/db.go:390-391`). `AntifloodSettings` has both `Action` and `Mode` columns (`alita/db/db.go:424-426`). Only one field is actually read in production code; the alias exists for "compatibility" with no migration to remove the dead column.
- Files: `alita/db/db.go:386-428`, `alita/db/devs_db.go:71-76`, `alita/modules/antiflood.go:264-322`
- Impact: `Mode` is never read in antiflood module (only `Action` is used). Writes to `Dev` are redundant. Confusion for new contributors.
- Fix approach: Add migration dropping the unused columns, remove alias fields from structs.

**`CleanupExpiredCaptchaAttempts` is dead code:**
- Issue: The function is defined in `alita/db/captcha_db.go:306` with a `NOTE` comment explicitly saying it is "not called from anywhere."
- Files: `alita/db/captcha_db.go:303-318`
- Impact: Expired captcha rows accumulate in the database indefinitely unless the full-cleanup goroutine in `alita/modules/captcha.go` runs. The goroutine uses `GetExpiredCaptchaAttempts` + per-row deletion, not this bulk cleanup function.
- Fix approach: Either wire `CleanupExpiredCaptchaAttempts` into the periodic cleanup loop or delete the function.

**`helpers.go` is a God file:**
- Issue: `alita/utils/helpers/helpers.go` is 1011 lines with 30 functions spanning formatting, keyboard building, note/filter parsing, Telegram message sending, and connection checks.
- Files: `alita/utils/helpers/helpers.go`
- Impact: Any change requires understanding the entire file's surface area. Functions with unrelated concerns are co-located.
- Fix approach: Split into domain-specific files: `keyboard_helpers.go`, `message_helpers.go`, `note_filter_helpers.go`, `connection_helpers.go`.

**`ClearCacheOnStartup` is unconditionally forced to `true`:**
- Issue: `alita/config/config.go:467` hardcodes `cfg.ClearCacheOnStartup = true` in `setDefaults`, overriding any env var setting. The env var `CLEAR_CACHE_ON_STARTUP` is parsed but immediately overwritten.
- Files: `alita/config/config.go:466-467`, `alita/config/config_test.go:431-438`
- Impact: Every bot restart triggers a full `FLUSHDB` on Redis. Warm cache is always destroyed. In multi-instance deployments this causes thundering herd on the database after restart.
- Fix approach: Remove the unconditional override and honor the env var; default to `false` or make it deployment-specific.

---

## Known Bugs

**Nil dereference on `update.Message.From` in webhook handler:**
- Symptoms: Bot panics on channel posts sent as messages, where `From` is nil.
- Files: `alita/utils/httpserver/server.go:260`
- Trigger: Any Telegram update where `update.Message != nil` but `update.Message.From == nil` (channel posts, anonymous admin messages).
- Workaround: `RecoverFromPanic` wraps the outer goroutine, but the span attribute write happens before dispatch, outside the goroutine — it is not protected.
- Current code: `attribute.Int64("message.from_id", update.Message.From.Id)` with no nil check on `From`.

**Text truncation at byte boundary for multibyte (Unicode) messages:**
- Symptoms: Trace span attribute `message.text_preview` may contain a truncated invalid UTF-8 string for Cyrillic, Arabic, CJK, or emoji content.
- Files: `alita/utils/httpserver/server.go:254-255`
- Trigger: Any message with non-ASCII text longer than 100 bytes. `textPreview[:100]` slices by byte index, not rune boundary.
- Fix: Use `[]rune(text)[:100]` or `strings.Cut` approach.

**Captcha answer-not-in-options causes unbounded recursion:**
- Symptoms: `generateTextCaptcha()` and `generateMathImageCaptcha()` call themselves recursively if the generated answer is not found in options. Under entropy starvation or library bugs this could stack overflow.
- Files: `alita/modules/captcha.go:833-834`, `alita/modules/captcha.go:879-880`
- Trigger: `secureIntn` fails 10 consecutive times (logs error at line 247) and returns 0 repeatedly, creating identical options where the answer may not match.
- Fix approach: Add a recursion depth counter and return an error after N retries.

---

## Security Considerations

**Webhook secret comparison is not timing-safe:**
- Risk: String equality `secretToken != s.secret` is subject to timing attacks. An attacker probing the secret character-by-character could deduce it.
- Files: `alita/utils/httpserver/server.go:311`
- Current mitigation: Secret is in URL path and header, not purely in header — but header comparison is still vulnerable.
- Recommendations: Replace with `subtle.ConstantTimeCompare([]byte(secretToken), []byte(s.secret))` from `crypto/subtle`.

**pprof endpoints have no authentication:**
- Risk: `ENABLE_PPROF=true` exposes heap dumps, goroutine stacks, and CPU profiles at `/debug/pprof/*`. These leak internal memory layout, goroutine counts, and potentially stack variables including credentials.
- Files: `alita/utils/httpserver/server.go:131-151`, `alita/config/config.go:150`
- Current mitigation: Documented as "dangerous in production." No enforcement prevents it being enabled in production.
- Recommendations: Add IP allowlist middleware or basic auth wrapper on pprof routes. Fail startup if `ENABLE_PPROF=true` and `DEBUG=false`.

**gosec runs with `-no-fail` flag:**
- Risk: `gosec` findings are uploaded as SARIF but the CI build does not fail on security issues. Security regressions can ship unblocked.
- Files: `.github/workflows/ci.yml:42`, `.github/workflows/release.yml:41`
- Current mitigation: SARIF is uploaded to GitHub Security tab for visibility.
- Recommendations: Remove `-no-fail` flag or gate release builds on zero high/critical findings.

---

## Performance Bottlenecks

**`time.Sleep` inside goroutines blocking Telegram API retry loops:**
- Problem: Exponential backoff uses `time.Sleep` in the calling goroutine for Redis reconnect (`alita/utils/cache/cache.go:62`), admin cache retry (`alita/utils/cache/adminCache.go:77`, `:128`), and captcha cleanup retry (`alita/modules/captcha.go:1870`). For captcha cleanup the sleep can be up to 3 seconds per message across potentially many messages.
- Files: `alita/utils/cache/adminCache.go:77-128`, `alita/modules/captcha.go:1870`, `alita/modules/reports.go:473`
- Cause: `time.Sleep` blocks the goroutine's stack for the sleep duration instead of using a timer/channel.
- Improvement path: Replace with `time.After` in a `select` to allow context cancellation; use a proper retry library with jitter.

**Full `sync.Map` scan on every antiflood message:**
- Problem: `antifloodModule.syncHelperMap` is a `sync.Map` keyed by `chatId:userId`. `checkFlood` loads/stores on every message. Cleanup runs every 5 minutes scanning the entire map with `Range`.
- Files: `alita/modules/antiflood.go:85-93`, `alita/modules/antiflood.go:113-156`
- Cause: `sync.Map.Range` acquires internal locks per bucket and is O(n) in map size.
- Improvement path: For a high-traffic bot, replace with a per-chat sharded map or an LRU with TTL.

**`users.go` in-memory rate-limit caches are never evicted:**
- Problem: `userUpdateCache`, `chatUpdateCache`, and `channelUpdateCache` are `*sync.Map` instances storing `time.Time` values for every unique user/chat/channel ever seen. They grow without bound and have no cleanup goroutine.
- Files: `alita/modules/users.go:49-56`, `alita/modules/users.go:61-68`
- Cause: No TTL or eviction mechanism. Long-running bots in large groups will accumulate tens of thousands of entries.
- Improvement path: Replace with a bounded TTL cache (e.g., `ristretto`) or add a periodic cleanup goroutine that deletes entries older than the update interval.

**i18n translation strings are cached in Redis but FLUSHDB destroys them on restart:**
- Problem: `alita/i18n/translator.go` caches translation strings with `i18n:{lang}:` prefix. Since `ClearCacheOnStartup` always runs `FLUSHDB`, every restart causes a cold cache for all translations until they are re-populated one key at a time under load.
- Files: `alita/i18n/translator.go:29-69`, `alita/config/config.go:467`
- Cause: The unconditional `ClearCacheOnStartup = true` override. Translation data is static and does not need invalidation.
- Improvement path: Exclude the `i18n:` key prefix from startup flush, or stop using `FLUSHDB` and instead invalidate only application-state keys on startup.

---

## Fragile Areas

**`captcha.go` — 1985-line monolith with multiple internal goroutine loops:**
- Files: `alita/modules/captcha.go`
- Why fragile: Contains captcha generation, sending, verification callbacks, recovery-on-startup, periodic cleanup, and periodic unmute — all in one file. Multiple `go func()` calls with inline `recover` and `time.Sleep`. Two documented BUG log lines (`captcha.go:833`, `captcha.go:879`). Global `captchaBotRef` variable needed to allow the cleanup goroutine to access the bot.
- Safe modification: Any change must trace the lifecycle of `CaptchaAttempts` rows end-to-end. Run the captcha DB tests (`alita/db/captcha_db_test.go`) after any change. The global `captchaBotRef` must be set before the cleanup goroutine fires.
- Test coverage: DB layer is tested; handler logic and goroutine interactions are not tested.

**`bans.go` — delayed unban goroutines with `select` + `time.After`:**
- Files: `alita/modules/bans.go:136-160`, `alita/modules/bans.go:280-294`, `alita/modules/bans.go:380-395`
- Why fragile: Three separate delayed unban patterns exist in the same file. Each spawns a goroutine with a `time.After` channel. If the bot restarts during the wait, the pending unban is silently lost — no persistence.
- Safe modification: Do not add more `time.After` patterns here. Any new timed action should be persisted to DB (like captcha does with `expires_at`).
- Test coverage: No handler-level tests for ban/unban timing.

**`antiflood.go` — semaphore timeout silently fails open:**
- Files: `alita/modules/antiflood.go:189-244`
- Why fragile: When the `adminCheckSemaphore` channel is full or the 5-second select timeout fires, flood checking is skipped entirely ("fail open"). A burst of messages could all bypass flood detection simultaneously if admin checks are slow.
- Safe modification: Document the fail-open behavior explicitly. Do not increase `maxConcurrentAdminChecks` without benchmarking.
- Test coverage: No tests for the semaphore timeout path.

**Connection module — `isConnected()` not using the codec:**
- Files: `alita/modules/connections.go:228-235`
- Why fragile: `connectionButtons` still uses `strings.Split(query.Data, ".")` without length validation. If `args` has fewer than 2 elements, accessing `args[1]` panics. The `decodeCallbackData` fallback exists but only activates if data starts with the versioned prefix.
- Safe modification: Add `len(args) < 2` guard before `args[1]` access. Migrate to versioned codec.
- Test coverage: `alita/modules/callback_parse_overwrite_test.go` covers the legacy bridge, not individual module callback handlers.

---

## Scaling Limits

**Single Redis database with `FLUSHDB` on every restart:**
- Current capacity: One Redis `db` (default db 0). All cache types (admin, antiflood, captcha, i18n, user data) share the same keyspace.
- Limit: If Redis is shared with other applications, `FLUSHDB` will destroy their data. Even without sharing, rolling restarts in multi-instance deployment are unsafe.
- Scaling path: Use `FLUSHDB ASYNC` or targeted key deletion; separate Redis databases by concern; remove the unconditional flush default.

**No horizontal scaling support for in-memory state:**
- Current capacity: Single instance. `antifloodModule.syncHelperMap`, `delMsgs`, `userUpdateCache`, and `captchaBotRef` are process-local.
- Limit: Running two instances simultaneously causes split-brain: flood counts, pending purges, and captcha bot references are not shared.
- Scaling path: Move flood counters to Redis with atomic increments; move pending purge tracking to Redis too.

---

## Dependencies at Risk

**`gotgbot/v2` is pinned to a release-candidate (`v2.0.0-rc.33`):**
- Risk: RC versions can introduce breaking API changes between releases. No stability guarantees. The library has been on RC for years with no stable release, suggesting the RC series is de-facto stable but carries no semantic versioning contract.
- Impact: Future RC bumps may require handler signature changes across all modules.
- Files: `go.mod:7`
- Migration plan: Monitor for a stable `v2` release. Pin to exact RC hash and review changelogs before each bump.

---

## Missing Critical Features

**Pending unban/unmute actions are not persisted across restarts:**
- Problem: Timed bans (`/ban 1h`), timed mutes, and timed kicks use in-process `time.After` goroutines. A restart loses all pending actions.
- Blocks: Reliable timed moderation in production deployments that require restarts (deployments, crashes, OOM).
- Fix: Persist pending timed actions to a `scheduled_actions` table with `execute_at` column; run a recovery goroutine on startup (same pattern as captcha).

---

## Test Coverage Gaps

**All module handler functions (captcha, bans, warns, filters, greetings, etc.):**
- What's not tested: The Telegram handler functions themselves — command parsing, permission checks, goroutine spawning, callback routing.
- Files: `alita/modules/captcha.go`, `alita/modules/bans.go`, `alita/modules/warns.go`, `alita/modules/filters.go`, `alita/modules/greetings.go`, `alita/modules/locks.go`, `alita/modules/mute.go`
- Risk: Regressions in handler logic, nil dereferences, and incorrect bot responses go undetected.
- Priority: High

**Antiflood semaphore timeout and fail-open path:**
- What's not tested: The `select` case where `adminCheckSemaphore` times out and flood check is skipped.
- Files: `alita/modules/antiflood.go:189-244`
- Risk: Flood bypass under load is not validated.
- Priority: Medium

**Webhook handler nil-From dereference:**
- What's not tested: Webhook processing of channel post updates where `Message.From == nil`.
- Files: `alita/utils/httpserver/server.go:250-263`
- Risk: Unhandled panic (caught by recover but produces no span attributes, masking the issue).
- Priority: High

**Timed ban/mute restart-persistence gap:**
- What's not tested: No test verifies that a timed ban fires after bot restart — because it does not work across restarts.
- Files: `alita/modules/bans.go:136-160`
- Risk: Silent failure of timed moderation actions.
- Priority: High

---

*Concerns audit: 2026-02-23*
