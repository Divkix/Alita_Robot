# Research: Telegram Token Lifecycle — Validation, Encryption at Rest, Ownership (#804)

Source branch: `research/clone-token-security` · Part of #801 · Grilling 2026-08-08 locked.

## 1. Validation Flow

### 1.1 Happy path (`/clone <token>` DM-only)

```
User DM: /clone 123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw
  │
  ├─ 1. Gate: reject if ctx.EffectiveChat.Type != "private"
  │     → reply "Please use this command in private chat with me."
  │     → never echo token.
  ├─ 2. Extract token arg, trim. Validate shape with structural regex
  │     `\d{6,}:[A-Za-z0-9_-]{30,}` (same as logredact). Fail fast before network.
  ├─ 3. Best-effort delete the user's message: `bot.DeleteMessage(chatId, msgId)`
  │     (ignore 400 if already deleted / no rights — DM delete always allowed for bot's own delete of user msg? Actually bot can delete via Bot API in private? No — Telegram only allows bot to delete messages it sent. For user-sent token message, bot cannot delete it via Bot API. Workaround: instruct user to delete, or rely on message auto-expiry guidance. Rose/Moon clones note: "delete+getMe" in grilling lock — interpreted as: attempt DeleteMessage, and if 400, send ephemeral "Please delete your message containing the token" reply that self-deletes. Document limitation.)
  ├─ 4. Validate via `getMe` against Telegram Bot API before persisting:
  │     GET https://api.telegram.org/bot<token>/getMe
  │     (or `gotgbot.NewBot(token, opts).GetMe()` which does same)
  │     → 200 + {ok:true, result:{id, username, is_bot}} : valid
  │     → 401 Unauthorized ({"ok":false,"error_code":401,"description":"Unauthorized"}) : invalid/revoked
  │     → 429 / 5xx : transient → retry once with backoff, then surface "Telegram is busy, try again"
  │     NEVER log raw token; log only `bot_id` extracted from prefix before `:` or from getMe id.
  ├─ 5. On 200: derive `bot_id` (getMe.id, authoritative) and `bot_username` (getMe.username).
  │     Verify `is_bot == true`.
  ├─ 6. Per-clone webhook hygiene (webhook mode):
  │     `getWebhookInfo` via clone token → if `url != ""` then `deleteWebhook(drop_pending_updates=true)`.
  │     Clones run in same process with single dispatcher/polling or multiplexed webhook; leaving a stale webhook would steal updates.
  │     Polling mode: deleteWebhook is still correct — ensures long-polling can start.
  ├─ 7. `setMyCommands` per clone token (private scope): reuse `postInit` pattern (`BotCommandScopeAllPrivateChats`, en). Clone gets its own command list.
  ├─ 8. Encrypt token (see §2), `RegisterSecret(plaintext)` for logredact lifetime, persist row.
  └─ 9. Reply success (ephemeral, no token echo): "Clone @<username> is live. /myclones to manage."
```

### 1.2 Gotgbot mapping

`gotgbot.NewBot(token, &gotgbot.BotOpts{BotClient: ...})` internally calls `getMe` to verify token and populate `bot.User`. Constructor returns error on 401. For validation-only (no dispatcher yet), prefer raw `BaseBotClient.RequestWithContext(..., "getMe", ...)` or `NewBot` then immediately discard if only probing. Existing code `main.go:124-139` uses `gotgbot.NewBot(AppConfig.BotToken)` — clone path reuses same `newBotAPITransport` / `resolveBotAPIURL` (supports `API_SERVER` override).

### 1.3 Post-creation lifecycle

- On each clone start (process boot or after `/clone`), re-validate `getMe` before launching its dispatcher/updater. If 401 → mark `is_active=false` (see revocation).
- `getWebhookInfo`/`deleteWebhook` are idempotent — safe to call on every start.

### 1.4 DM-only + delete + never log raw — rationale

- Group exposure: token in group history is visible to all members + bots + export. Grilling lock confirms DM-only.
- Delete: Telegram Bot API `deleteMessage` can only delete messages sent by the bot in most contexts; bot cannot delete a user's DM message via API unless it has appropriate rights (in private chat it generally cannot). So "auto-delete" is best-effort + fallback to instructing user to delete. Implementation must not fail the flow if delete returns 400.
- Never log: use `logredact.Scrub` + `RegisterSecret` (structural regex already redacts `\d{6,}:[A-Za-z0-9_-]{30,}` in URLs and bare tokens). Any error wrapping the token must go through scrubbed logger.

## 2. Storage

### 2.1 Migration sketch

Latest migration is `20260730030000_use_timestamptz_for_captcha_attempts.sql`. Next clone migration should be `20260808000000_add_clones_table.sql` (timestamp after grilling date).

```sql
-- 20260808000000_add_clones_table.sql
CREATE TABLE IF NOT EXISTS clones (
    id            BIGSERIAL PRIMARY KEY,
    owner_id      BIGINT      NOT NULL,          -- Telegram user_id of cloner
    bot_id        BIGINT      NOT NULL UNIQUE,   -- getMe.id, immutable, extracted from token prefix or getMe
    bot_username  TEXT        NOT NULL,
    bot_token_enc TEXT        NOT NULL,          -- AES-GCM ciphertext, base64
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clones_owner_id ON clones(owner_id);
CREATE INDEX IF NOT EXISTS idx_clones_bot_id ON clones(bot_id);
CREATE INDEX IF NOT EXISTS idx_clones_owner_active ON clones(owner_id, is_active);

-- Trigger for updated_at (reuse existing pattern from migrations)
-- or rely on GORM UpdatedAt.
```

GORM model sketch (`alita/db/models/clones.go`, following `connections.go` example):

```go
type CloneBot struct {
    ID         uint64    `gorm:"primaryKey;autoIncrement"`
    OwnerID    int64     `gorm:"column:owner_id;not null;index:idx_clones_owner_id"`
    BotID      int64     `gorm:"column:bot_id;not null;uniqueIndex"`
    BotUsername string   `gorm:"column:bot_username;not null"`
    BotTokenEnc string  `gorm:"column:bot_token_enc;not null"`
    IsActive   bool      `gorm:"column:is_active;not null;default:true"`
    CreatedAt  time.Time `gorm:"column:created_at"`
    UpdatedAt  time.Time `gorm:"column:updated_at"`
}
func (CloneBot) TableName() string { return "clones" }
```

Notes:
- `bot_id` from prefix before `:` is untrusted until `getMe` confirms; store authoritative `getMe.id`.
- `bot_token_enc` is `TEXT` holding `base64(nonce || ciphertext)`; decryption only in memory, never logged.
- `owner_id` BIGINT matches Telegram user_id (fits int64, see `config.OwnerId int64` precedent).
- Shared-table design per grilling lock: no per-clone schema, add `bot_id` column to domain tables later (separate migration per module).

### 2.2 Encryption choice — stdlib AES-GCM

**Recommended: `crypto/aes` + `cipher.NewGCM` (GCM, 32-byte key) — no new dependency.**

| Option | Pros | Cons | Verdict |
|---|---|---|---|
| `crypto/aes` + `cipher` (stdlib) AES-256-GCM | Zero deps, audited, in `go 1.26` stdlib, matches existing minimal-deps stance (`go.mod` has no `x/crypto` for this) | Requires 32-byte key management, nonce handling | **Choose** |
| `golang.org/x/crypto` (e.g. `chacha20poly1305`, `nacl/secretbox`) | Modern AEADs, libsodium-like | Adds `x/crypto` transitive dep, not currently in `go.mod`; ChaCha20Poly1305 needs same key discipline anyway | Avoid — unnecessary dep |
| `libsodium` via cgo | Strong, but cgo breaks distroless build | Heavy, platform-specific | Reject |
| Fernet / age | Higher-level but new dep | Overkill for single field | Reject |

Key management:
- Env `CLONE_TOKEN_ENC_KEY`: 32 bytes, supplied as **base64** (44 chars) or hex (64 chars). Loaded in `config.LoadConfig()`, validated `len==32`, fail-fast on missing/invalid when clone feature enabled. For dev/test, allow absent → clone routes return "cloning disabled".
- Use `aes.NewCipher(key)` → `cipher.NewGCM`. Per-encrypt: `nonce = 12 random bytes (crypto/rand)`, `ciphertext = gcm.Seal(nonce, nonce, plaintext, nil)`, store `base64.StdEncoding.EncodeToString(ciphertext)`. Decrypt: decode base64, split nonce/ciphertext, `gcm.Open`.
- Rotation of `CLONE_TOKEN_ENC_KEY`: out of scope for v1; document re-encrypt migration path (decrypt with old key, encrypt with new). Single active key keeps code minimal.

### 2.3 logredact integration

Existing pattern (`alita/config/config.go:497-523`, `alita/utils/logredact/logredact.go`):

- `logredact.Install(nil)` installs hook on std logger before config load; structural regex already redacts tokens in URLs.
- `logredact.RegisterSecret(cfg.BotToken, cfg.DatabaseURL, ...)` registers exact values (longest-first, ≥6 chars).
- **Clone extension**: after decrypting or receiving plaintext token (only in the narrow validation window), call `logredact.RegisterSecret(plaintextToken)` so any accidental log of that exact string is scrubbed for process lifetime. On boot, after loading clones table, register each decrypted token (or at least on first use). Plaintext must never be assigned to a log field.

Structural pattern already covers clone tokens: `\d{6,}:[A-Za-z0-9_-]{30,}` → `[REDACTED]`. No change needed; just reuse.

Config addition:

```go
// in Config struct
CloneTokenEncKey string // 32 bytes, from CLONE_TOKEN_ENC_KEY (base64/hex)
```

Validation mirrors `BotToken` required-when-enabled logic.

## 3. Ownership & Rotation

### 3.1 Ownership scope

- **Clone owner** = Telegram `user_id` who ran `/clone` (DM). Owner-scoped queries: `WHERE owner_id = ?`.
- `OWNER_ID` (global bot owner from `config.AppConfig.OwnerId`) is **not** the clone owner; global owner is for main bot admin ops, not clone management. Clone table's `owner_id` is authoritative.
- Chat admins have **no** rights over clones — clones are user-scoped, not chat-scoped (unlike `connections` which is user↔chat). `/myclones` lists only caller's rows. `/unclone <bot_username|bot_id>` checks `owner_id`.
- Enforcement at DB layer: unique `bot_id` prevents duplicate clone entries; optional per-user cap (1–2) enforced in application (count active rows for owner before insert) + global cap (count all active).

### 3.2 Rotation

- **User-initiated**: re-run `/clone <new_token>` with same `bot_id` (BotFather `/revoke` issues new suffix, same prefix id). Flow validates new token via `getMe`, re-encrypts, `UPDATE clones SET bot_token_enc=?, bot_username=?, is_active=true, updated_at=NOW() WHERE bot_id=? AND owner_id=?`. Old token immediately invalid; register new secret, old secret remains in registry (harmless).
- **Revoked externally**: user runs `/revoke` in BotFather. Clone's next `getMe` returns 401. Detection (see §3.3) flips `is_active=false`. User recovers by re-`/clone` with new token as above (reactivates).

### 3.3 Revocation detection

- **On-demand**: every clone start / periodic health check (e.g. on dispatcher error or cron every 10m) calls `getMe` with decrypted token.
  - `401 Unauthorized` → `UPDATE clones SET is_active=false WHERE bot_id=?`. Notify owner via main bot DM: "Your clone @X token was revoked. Re-run /clone with the new token from @BotFather."
  - `429` → backoff, do not mark inactive.
  - Other errors → log scrubbed error, keep active.
- Do **not** delete row on revoke — keep for audit / re-clone UX.
- GORM soft-delete not used; `is_active` boolean matches grilling spec.

### 3.4 Caps and cooldowns (context)

- Per-user cap 1–2 active clones, global cap N (to be locked in #805), 1h Redis cooldown — borrow `alita/utils/ratelimit/backup_ratelimit.go` pattern: `acquireOperation("clone:"+userID, 1*time.Hour)` using `cache.GetRedisClient()`. Reuse `gocache`/`redis` already in repo.

## 4. Threat Model Checklist

| # | Threat | Mitigation | Status |
|---|---|---|---|
| 1 | Token leaked in group chat history | DM-only gate (`chat.Type == private`), reject otherwise | Required |
| 2 | Token persists in Telegram message history | Best-effort `DeleteMessage` + user instruction fallback; warn user | Best-effort (API limitation) |
| 3 | Token in logs / crash dumps / error messages | Structural regex (`\d{6,}:[...]{30,}`) + `RegisterSecret(plain)` + `Scrub` hook on all levels; never `log.*(token)` | Reuse existing `logredact` |
| 4 | Token at rest in DB stolen (dump/S3 backup) | AES-256-GCM `bot_token_enc` with `CLONE_TOKEN_ENC_KEY` (32B, env base64); no plaintext column | Required |
| 5 | Token in transit (API validation) | HTTPS to `api.telegram.org` (TLS), do not log URL with token; use `API_SERVER` override correctly | Existing `resolveBotAPIURL` |
| 6 | Token replay / reuse by another user | `bot_id` UNIQUE + ownership check on `/unclone`; only owner can rotate | DB constraint |
| 7 | Revoked token still considered valid | `getMe` 401 → `is_active=false` + owner notification | Required |
| 8 | Stale webhook hijacks updates | `getWebhookInfo` → `deleteWebhook` on clone validation/start | Required |
| 9 | Enumeration of clones / IDs | Owner-scoped queries, no public list | Required |
| 10 | Brute-force token guessing | Not applicable (token is secret, not guessed); rate-limit `/clone` per user 1/h via Redis | Reuse `backup` ratelimit |
| 11 | `CLONE_TOKEN_ENC_KEY` in env leaked | Same scrub treatment as other secrets; document rotation procedure | Operational |
| 12 | Decrypted token lingers in memory | Decrypt only when needed (start/validate), zero after use if feasible; `RegisterSecret` keeps scrub coverage | Best-effort (Go GC) |
| 13 | Per-user/global caps bypassed (race) | `SELECT COUNT ... FOR UPDATE` or atomic Redis `INCR` + DB check inside transaction | Follow `backup_ratelimit` `acquireOperation` atomicity |
| 14 | Clone commands pollute main bot | Separate dispatcher per clone token, shared process; `setMyCommands` per clone isolates UX | Architecture (separate research ticket) |

### Not in scope (follow-up tickets)

- Per-clone `bot_id` column propagation to 17 domain tables + cache namespacing `alita:{module}:{bot_id}:{id}` — tracked in #801 sub-tickets.
- Webhook multiplexing per clone token vs single webhook domain — needs infra decision.
- Observability per clone (metrics labels, tracing `bot_id`).

## References

- `alita/config/config.go:115-253` — Config struct, `ValidateConfig`, `LoadConfig`, `init` with `logredact.Install/RegisterSecret`.
- `alita/utils/logredact/logredact.go:47-116` — structural regex, registry, `RegisterSecret`, `Scrub`.
- `main.go:124-144,458-467` — `gotgbot.NewBot` with `BaseBotClient`, `SetMyCommands`.
- `alita/utils/ratelimit/backup_ratelimit.go:36-96` — `acquireOperation` cooldown pattern to reuse for clone 1/h limit.
- `migrations/20260730030000_use_timestamptz_for_captcha_attempts.sql` — latest migration (base for next clone migration).
- `alita/db/models/connections.go` — GORM model example.
- Telegram Bot API: `GET https://api.telegram.org/bot<token>/getMe` → 200/401; `getWebhookInfo`, `deleteWebhook`, `setMyCommands`.

