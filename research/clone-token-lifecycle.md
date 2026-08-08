# Research: Telegram Token Lifecycle — Validation, Encryption at Rest, Ownership (#804)

> Branch: `research/clone-token-lifecycle` · Part of #801 · Grilling 2026-08-08 locked (same-process shared tables + `bot_id`/`owner_id`, namespaced cache `alita:{module}:{bot_id}:{id}`, per-user cap 1–2 + global cap + 1h Redis cooldown, DM-only `/clone` with delete + `getMe`, minimal lifecycle + revoked detection) · 2026-08-08  
> Scope: `alita/config/config.go` (`BotToken`, `BotVersion "2.21.3"`, `RegisterSecret`, REQUIRED env), `alita/utils/logredact/*` (`RegisterSecret`, `Scrub`, hook), `alita/utils/helpers/*` + `alita/utils/chat_status/access.go` (`RequirePrivate`), `main.go` (`gotgbot.NewBot` + `BaseBotClient` + `resolveBotAPIURL` + flag handling), `alita/db/models/*` (`TableName`, JSONB, GORM), `migrations/*.sql` (naming `20260808000000_*`, SHA-256 immutability), `alita/utils/cache/*` (key `alita:{module}:{id}`), `alita/utils/ratelimit/backup_ratelimit.go` (`acquireOperation` 1h cooldown), `sample.env` (`CLONE_TOKEN_ENC_KEY` placeholder)  
> References: Telegram Bot API `getMe` / `getWebhookInfo` / `deleteWebhook` / `setMyCommands`; `ctx7` `/paulsonoflars/gotgbot` if deeper `NewBot` internals needed — but stdlib `crypto/aes` + `cipher.NewGCM` is chosen on minimal-deps grounds (AGENTS §21) without new library.

## 1. Validation Flow — Ephemeral `NewBot` + `getMe`, No Persistence Before 200

### 1.1 Token format

BotFather tokens are `^\d+:[A-Za-z0-9_-]+$` (numeric `bot_id` prefix, colon, opaque suffix). The repo already redacts this shape structurally:

```go
// alita/utils/logredact/logredact.go:56-58
regexp.MustCompile(`\d{6,}:[A-Za-z0-9_-]{30,}`)
```

For validation we use a **lighter pre-check** before any network call (same family, relaxed length):

```go
var cloneTokenRe = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]+$`)
if !cloneTokenRe.MatchString(token) {
    // reply: "That doesn't look like a bot token. Copy it from @BotFather → /mybots → API Token."
    return
}
```

Reject empty/whitespace, trim first. Log only `bot_id` prefix (`strings.Cut(token, ":")`) or `getMe.id`; never log the suffix.

### 1.2 DM-only gate

Per grilling lock `/clone` is **DM-only**. Enforce via existing helper that already covers this concern:

```go
// alita/utils/chat_status/access.go:206-215
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
    chat = extractChatFromContext(ctx, chat)
    if chat == nil { return false }
    return chat.Type == "private"
}
```

Handler skeleton (pipeline or manual):

```go
if !chat_status.RequirePrivate(b, ctx, nil) {
    _, _ = b.SendMessage(ctx.EffectiveChat.Id, "Please use /clone in private chat with me.", nil)
    return ext.EndGroups
}
```

Group/supergroup/channel invocations are rejected without ever inspecting the token argument.

### 1.3 Delete after receipt

On receipt, **best-effort delete** the user's message to reduce exposure in chat history:

```go
// alita/utils/helpers/telegram_helpers.go:12-27 — DeleteMessageWithErrorHandling already exists
_ = helpers.DeleteMessageWithErrorHandling(b, ctx.EffectiveChat.Id, ctx.EffectiveMessage.MessageId)
```

**Telegram limitation**: Bot API `deleteMessage` can generally delete only messages sent by the bot (or where bot has delete rights). In a DM the bot cannot delete the user's token message via API — the call returns `400 Bad Request`. So the delete is best-effort; on failure send a follow-up:

> "I couldn't delete your message — please delete it yourself. I never log tokens."

This preserves the grilling-lock threat model while documenting the platform constraint (also noted in `research/clone-token-security` predecessor branch).

### 1.4 Ephemeral validation — `getMe` before persist

No DB write happens until Telegram confirms the token is live. Two equivalent paths; both use `main.go`'s `newBotAPITransport`/`resolveBotAPIURL` so `API_SERVER` override is respected:

**Option A — raw `getMe` without storing a `Bot`:**

```go
transport := newBotAPITransport(config.AppConfig.HTTPMaxIdleConns, config.AppConfig.HTTPMaxIdleConnsPerHost)
apiURL := resolveBotAPIURL(config.AppConfig.ApiServer) // "https://api.telegram.org" default
client := &http.Client{Transport: transport, Timeout: constants.LongTimeout}
_ = client // used to construct gotgbot.BaseBotClient below
```

**Option B — ephemeral `gotgbot.NewBot` (preferred, one-liner):**

```go
cloneBot, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
    BotClient: &gotgbot.BaseBotClient{
        Client: http.Client{Transport: transport, Timeout: constants.LongTimeout},
        DefaultRequestOpts: &gotgbot.RequestOpts{
            Timeout: constants.LongTimeout,
            APIURL:  resolveBotAPIURL(config.AppConfig.ApiServer),
        },
    },
})
if err != nil {
    // err.Error() may contain "unauthorized" for 401 — scrub before logging
    log.WithError(errors.New(logredact.Scrub(err.Error()))).Warn("clone token validation failed")
    // map Telegram error_code:
    // 401 Unauthorized → "Invalid or revoked token. Get a fresh one from @BotFather."
    // 429 Too Many Requests → "Telegram is busy, try again in a minute."
    // 5xx / network → "Telegram is temporarily unavailable, try again."
    return
}
botID := cloneBot.Id        // authoritative, from getMe.id
botUsername := cloneBot.User.Username
isBot := cloneBot.User.IsBot // must be true (getMe confirms)
```

`gotgbot.NewBot` internally calls `getMe` (`ext/bot.go`) — on 401 it returns error with `Unauthorized`. This satisfies:

- **No persistence before 200**: token touches memory and the outbound HTTPS call only. On failure, nothing is written, no `RegisterSecret` for that token (optional — see §2.3).
- **Upstream truth**: `bot_id` used everywhere else is `getMe.id`, not the untrusted numeric prefix before `:`.
- **No raw log**: error is scrubbed; `logredact.Scrub` structural pattern already redacts any token-shaped substring, and `RegisterSecret` for the main token is already installed via `config.init` (`alita/config/config.go:517-523`).

### 1.5 Webhook hygiene

If webhook mode is enabled for clones, stale webhooks would steal updates:

```go
info, _ := cloneBot.GetWebhookInfo(nil)
if info != nil && info.Url != "" {
    _, _ = cloneBot.DeleteWebhook(&gotgbot.DeleteWebhookOpts{DropPendingUpdates: boolPtr(true)})
}
_ = cloneBot.SetMyCommands( /* clone-scoped commands, e.g. via postInit pattern main.go:458 */ )
```

This is idempotent and safe in polling mode — just ensures long polling can start.

## 2. Encryption at Rest

### 2.1 Decision — stdlib `crypto/aes` + `crypto/cipher` AES-256-GCM, no new deps

| Option | Verdict | Reason |
|---|---|---|
| **`crypto/aes` + `cipher.NewGCM` (AES-256-GCM)** | **Choose** | Zero new deps, stdlib in Go 1.26, audited, matches `AGENTS §21` minimal-deps stance (`go.mod` carries no `x/crypto` for this). |
| `golang.org/x/crypto` (ChaCha20-Poly1305, `nacl/secretbox`) | Avoid | Adds `x/crypto` transitive dep for same key-management discipline; no repo precedent. |
| `libsodium` via cgo | Reject | cgo breaks distroless build, platform-specific, heavy. |

### 2.2 Key `CLONE_TOKEN_ENC_KEY` — 32 bytes, base64 or hex

New env, documented in `sample.env`:

```ini
# Clone token encryption — 32 bytes, base64 (44 chars) or hex (64 chars)
# Generate: openssl rand -base64 32  or  python3 -c "import os,base64;print(base64.b64encode(os.urandom(32)).decode())"
# CLONE_TOKEN_ENC_KEY=
```

Loading (extend `alita/config/config.go` `Config` + `LoadConfig`/`ValidateConfig`/`setDefaults`):

```go
type Config struct {
    // ... existing
    CloneTokenEncKey string `env:"CLONE_TOKEN_ENC_KEY"` // raw 32 bytes after decode, keep string form + parsed bytes
    cloneTokenEncKeyBytes [32]byte // unexported, parsed
}

func loadCloneEncKey(raw string) ([32]byte, error) {
    var key [32]byte
    if raw == "" { return key, errors.New("CLONE_TOKEN_ENC_KEY is required when clone feature enabled") }
    // Try base64 (std, no padding strict), then hex
    if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
        copy(key[:], b); return key, nil
    }
    if b, err := base64.RawStdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
        copy(key[:], b); return key, nil
    }
    if b, err := hex.DecodeString(strings.TrimSpace(raw)); err == nil && len(b) == 32 {
        copy(key[:], b); return key, nil
    }
    return key, fmt.Errorf("CLONE_TOKEN_ENC_KEY must be 32 bytes as base64 (44 chars) or hex (64 chars)")
}
```

Validation mirrors `BOT_TOKEN` pattern (`config.go:197-209`): if clone feature is enabled, fail fast when key missing/invalid. For dev without clones, allow absent but have clone handlers return "cloning disabled".

Register the key itself as a secret:

```go
// in config.init after LoadConfig, alongside existing RegisterSecret call config.go:517-523
logredact.RegisterSecret(
    cfg.BotToken,
    cfg.DatabaseURL,
    cfg.RedisPassword,
    cfg.WebhookSecret,
    cfg.MetricsAuthToken,
    cfg.CloneTokenEncKey, // exact base64/hex form
)
```

### 2.3 Encrypt before DB insert, decrypt on load; `RegisterSecret` on both raw and ciphertext forms

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    b64 "encoding/base64"
    "io"
)

func encryptToken(plain string, key [32]byte) (string, error) {
    block, err := aes.NewCipher(key[:]); if err != nil { return "", err }
    gcm, err := cipher.NewGCM(block); if err != nil { return "", err }
    nonce := make([]byte, gcm.NonceSize()) // 12
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return "", err }
    ct := gcm.Seal(nonce, nonce, []byte(plain), nil) // nonce || ciphertext
    // Register plaintext so any accidental log of it is scrubbed for process lifetime
    logredact.RegisterSecret(plain)
    return b64.StdEncoding.EncodeToString(ct), nil
}

func decryptToken(enc string, key [32]byte) (string, error) {
    raw, err := b64.StdEncoding.DecodeString(enc); if err != nil { return "", err }
    block, err := aes.NewCipher(key[:]); if err != nil { return "", err }
    gcm, err := cipher.NewGCM(block); if err != nil { return "", err }
    if len(raw) < gcm.NonceSize() { return "", errors.New("ciphertext too short") }
    nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
    pt, err := gcm.Open(nil, nonce, ct, nil); if err != nil { return "", err }
    plain := string(pt)
    logredact.RegisterSecret(plain) // also scrub after decrypt
    return plain, nil
}
```

Usage:

```go
enc, err := encryptToken(token, config.AppConfig.CloneEncKeyBytes())
if err != nil { /* log scrubbed */ return }
// persist enc, never plain
db.Create(&models.CloneBot{OwnerID: userID, BotID: botID, BotUsername: botUsername, BotTokenEnc: enc})
```

On clone boot (process restart or manager start):

```go
plain, err := decryptToken(row.BotTokenEnc, config.AppConfig.CloneEncKeyBytes())
if err != nil { log.WithError(errors.New(logredact.Scrub(err.Error()))).Error("clone decrypt failed"); continue }
// RegisterSecret already called inside decryptToken
b, err := gotgbot.NewBot(plain, &gotgbot.BotOpts{BotClient: ...}) // validates again
```

`logredact.RegisterSecret` is process-lifetime, longest-first, ≥6 chars (`logredact.go:91-116`). Structural regex (`\d{6,}:[A-Za-z0-9_-]{30,}`) is a safety net even without registration — but explicit registration of the exact plaintext token is the AGENTS security invariant and covers tokens logged outside URL form.

**Never log raw**: no `log.Infof("%s", token)`, no `fmt.Errorf("token %s invalid", token)`. If an error from `gotgbot` wraps the URL `https://api.telegram.org/bot<token>/getMe`, the hook's `Scrub` rewrites it before emission (`logredact.go:129-155`).

### 2.4 `config.go` secret registration snippet (extends existing `init`)

```go
// alita/config/config.go:474-534 — init already does:
//   logredact.Install(nil)
//   cfg, _ := LoadConfig()
//   logredact.RegisterSecret(cfg.BotToken, cfg.DatabaseURL, cfg.RedisPassword, cfg.WebhookSecret, cfg.MetricsAuthToken)
// Add clone key to that same call (see §2.2). Clone plaintext tokens are registered dynamically at
// encryptToken/decryptToken time, not here.
```

Structural `placeholder` is `[REDACTED]` (`logredact.go:35`). Hook scrubs `entry.Message` + every string field on all levels (`logredact.go:159-184`), installed once via `sync.Once` for the std logger (`logredact.go:187-204`).

### 2.5 Rotation — `CLONE_TOKEN_ENC_KEY`

- **Token re-encrypt on rotation**: decrypt with old key, encrypt with new key, `UPDATE clones SET bot_token_enc = ?`. Single active key for v1 (minimal), document migration path; no dual-key envelope needed until operational need arises.
- **`CLONE_TOKEN_ENC_KEY` itself** is a secret — treat like `DATABASE_URL` (scrubbed, env-only, never committed).

## 3. Ownership

### 3.1 `owner_id` — Telegram `user_id` who ran `/clone`

Per grilling lock, clone ownership is **user-scoped, not chat-scoped** (unlike `connections` which is `user_id ↔ chat_id`):

- `owner_id BIGINT NOT NULL` = `ctx.EffectiveUser.Id` at `/clone` time (Telegram user_id, `int64` — same domain as `config.OwnerId int64` and `users.user_id`).
- `OWNER_ID` (global bot owner from `config.AppConfig.OwnerId`, `env:"OWNER_ID" required,min=1` — `config.go:125,275,201`) is **not** the clone owner. It governs main bot admin ops; clone rows are authorized by their own `owner_id`.
- Chat admins have no rights over clones.

### 3.2 Table + FK + scopes

**Migration naming**: latest is `20260730030000_use_timestamptz_for_captcha_attempts.sql`; next clone migration must be `20260808000000_add_clones_table.sql` (timestamp after grilling, SHA-256 immutable per `migrations/*.sql` convention).

```sql
-- 20260808000000_add_clones_table.sql
CREATE TABLE IF NOT EXISTS clones (
    id             BIGSERIAL PRIMARY KEY,
    owner_id       BIGINT      NOT NULL,        -- Telegram user_id of cloner (FK, see below)
    bot_id         BIGINT      NOT NULL UNIQUE, -- getMe.id, authoritative
    bot_username   TEXT        NOT NULL,
    bot_token_enc  TEXT        NOT NULL,        -- base64(nonce || AES-GCM ciphertext)
    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clones_owner_id ON clones(owner_id);
CREATE INDEX IF NOT EXISTS idx_clones_bot_id ON clones(bot_id);
CREATE INDEX IF NOT EXISTS idx_clones_owner_active ON clones(owner_id, is_active);
-- Optional FK to users.user_id (Telegram user_id), DEFERRABLE or no FK if users table is sparse:
-- ALTER TABLE clones ADD CONSTRAINT fk_clones_owner FOREIGN KEY (owner_id) REFERENCES users(user_id) ON DELETE CASCADE;
```

GORM model (follow `alita/db/models/notes.go:24-44`, `connections.go`, `channels.go` — `TableName()` + `gorm:"column:..."` + `json:"..."`):

```go
// alita/db/models/clones.go
package models

import "time"

type CloneBot struct {
    ID          uint64    `gorm:"primaryKey;autoIncrement" json:"-"`
    OwnerID     int64     `gorm:"column:owner_id;not null;index:idx_clones_owner_id" json:"owner_id"`
    BotID       int64     `gorm:"column:bot_id;not null;uniqueIndex" json:"bot_id"`
    BotUsername string    `gorm:"column:bot_username;not null" json:"bot_username"`
    BotTokenEnc string    `gorm:"column:bot_token_enc;not null" json:"-"`
    IsActive    bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
    CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
    UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CloneBot) TableName() string { return "clones" }
```

**FK choice**: `owner_id` FK to `users(user_id)` is semantically strongest but `users` may not contain every Telegram user until first interaction. Two options: (a) no FK, enforce at app layer (simpler, avoids insert race); (b) `REFERENCES users(user_id) ON DELETE CASCADE` with `DEFERRABLE` and upsert user on `/clone`. Document trade-off; recommend (a) for v1 to avoid migration ordering friction, add FK later if `users` becomes canonical identity table. `chats` is not the owner domain — clones are not chat-bound.

### 3.3 Owner-scoped queries

All read/write paths filter by `owner_id`:

```go
// list
db.Where("owner_id = ?", callerUserID).Find(&clones)
// stop / rotate — also check bot_id match to avoid IDOR
db.Where("owner_id = ? AND bot_id = ?", callerUserID, botID).First(&clone)
db.Model(&clone).Update("is_active", false) // /unclone
db.Model(&clone).Updates(map[string]any{"bot_token_enc": newEnc, "bot_username": newUsername, "is_active": true}) // re-/clone rotation
```

`/myclones` lists only caller's rows. `/unclone <bot_username|bot_id>` and rotation via re-`/clone <new_token>` both require the `owner_id` match; `OWNER_ID` bearer does not bypass it (unless explicitly granted as super-admin — out of scope for grilling lock).

### 3.4 Caps

Per-user cap (1–2 active `is_active=true` rows per `owner_id`) + global cap enforced in app transaction + Redis 1/h cooldown reuse `alita/utils/ratelimit/backup_ratelimit.go:118-152` `acquireOperation("clone:<user_id>", 1*time.Hour)` pattern (`cache.GetRedisClient()` + `gocache`). Numeric caps locked in #805.

## 4. Revocation Detection & Rotation

### 4.1 BotFather `/revoke` invalidates the suffix

`/revoke` in `@BotFather` issues a new `bot_id:suffix` pair (same numeric `bot_id` prefix, new suffix). The old token becomes `401 Unauthorized` immediately.

### 4.2 Detection — `getMe` `401 Unauthorized` → mark inactive

Every clone is validated at **two points**:

1. **On process boot / manager start**: decrypt → `gotgbot.NewBot` (which calls `getMe`). On 401 → `UPDATE clones SET is_active=false, updated_at=NOW() WHERE bot_id=?`.
2. **Periodic poll or on-update failure**: either a cron (e.g. every 10m) that calls `cloneBot.GetMe(nil)` per active clone, or the clone's own polling loop surfacing 401 during `getUpdates` (gotgbot returns `Unauthorized` on invalid token). On 401 → same `is_active=false`.

Notify owner via main bot DM (main `Bot` sends to `owner_id`):

> "Your clone @<username> token was revoked (401). Get a new token from @BotFather → /mybots → API Token, then run /clone <new_token> here to restore it."

Do **not** delete the row — keep for audit and re-clone UX. `is_active` boolean matches grilling spec (no soft-delete).

Non-401 errors (`429`, `5xx`, network) are transient: backoff, keep `is_active=true`, log scrubbed error.

### 4.3 Rotation via re-`/clone <new_token>` with same `owner_id`

User pastes the new token (same `bot_id` prefix). Flow:

```
validate token (getMe 200) → encrypt(new_token) → RegisterSecret(new_token)
→ SELECT ... WHERE bot_id=? FOR UPDATE
→ if row exists AND owner_id == caller: UPDATE bot_token_enc=?, bot_username=?, is_active=true
→ else if row exists but owner mismatch: reject ("This clone belongs to another user.")
→ else if no row: INSERT (new clone, subject to caps)
→ restart clone dispatcher/updater with new token
```

Old token's `RegisterSecret` entry remains (harmless — extra scrubbing). Single `CLONE_TOKEN_ENC_KEY` encrypts the new token; key rotation procedure (decrypt old, encrypt new) is documented separately.

## 5. Threat-Model Checklist

| # | Threat | Mitigation | Pattern | Status |
|---|---|---|---|---|
| 1 | Token leaked in group chat history | DM-only gate `chat.Type == "private"` (`chat_status.RequirePrivate`) — reject otherwise, never echo token | `alita/utils/chat_status/access.go:206` | **Required** |
| 2 | Token persists in Telegram message history | Best-effort `DeleteMessage` (`helpers.DeleteMessageWithErrorHandling`) + fallback "please delete" instruction; warn user | `alita/utils/helpers/telegram_helpers.go:12` | Best-effort (Bot API can't delete user's DM) |
| 3 | Token in logs / crash dumps | Structural regex `\d{6,}:[A-Za-z0-9_-]{30,}` → `[REDACTED]` + `RegisterSecret(plain)` + `Scrub` hook on all levels; never `log.*(token)` | `alita/utils/logredact/logredact.go:47-184`, `alita/config/config.go:517` | Reuse existing |
| 4 | Token at rest stolen (DB dump / backup) | AES-256-GCM `bot_token_enc` (`base64(nonce||ct)`) with `CLONE_TOKEN_ENC_KEY` 32B env; no plaintext column | `crypto/aes`, `cipher.NewGCM`, §2 | **Required** |
| 5 | Token in transit (API validation) | HTTPS to `api.telegram.org` (TLS) via shared `newBotAPITransport`; `API_SERVER` override via `resolveBotAPIURL` | `main.go:355-390` | Existing |
| 6 | Token replay / reuse by another user | `bot_id` UNIQUE + `owner_id` check on `/unclone`/rotate; only owner can rotate | `clones` migration, §3.3 | DB constraint |
| 7 | Revoked token still considered valid | `getMe` 401 → `is_active=false` + owner DM notification; re-clone restores | §4.2 | **Required** |
| 8 | Stale webhook hijacks updates | `getWebhookInfo` → `deleteWebhook(drop_pending_updates=true)` on validation/start; idempotent | Telegram Bot API | **Required** |
| 9 | Clone enumeration | Owner-scoped queries `WHERE owner_id=?`; no public list | §3.3 | **Required** |
| 10 | Brute-force token guessing | Token is high-entropy secret (not guessed); rate-limit `/clone` 1/h per user via Redis `acquireOperation("clone:<uid>", 1h)` borrowing backup pattern | `alita/utils/ratelimit/backup_ratelimit.go:118` | Reuse pattern |
| 11 | `CLONE_TOKEN_ENC_KEY` leaked (env dump) | Same scrub treatment (`RegisterSecret`) as other secrets; document key rotation | `alita/config/config.go:517` | Operational |
| 12 | Decrypted token lingers in memory | Decrypt only when needed (boot/validate), zero after use if feasible; `RegisterSecret` keeps scrub coverage | §2.3 | Best-effort (Go GC) |
| 13 | Per-user/global caps raced (concurrent `/clone`) | `SELECT COUNT ... FOR UPDATE` or Redis atomic `acquireOperation` inside transaction | `backup_ratelimit.go` atomicity | Follow pattern |
| 14 | Clone commands pollute main bot | Separate `Dispatcher`/`Updater` per clone token (M1), shared process; `setMyCommands` per clone | `research/gotgbot-multi-bot` (§2.3) | Architecture (separate ticket) |
| 15 | `getMe` retries leak token in logs | Scrub every error via `logredact.Scrub(err.Error())` before logging; structural pattern covers URL form | `logredact.go:129` | **Required** |

### Out of scope (follow-up tickets per #801)

- Per-clone `bot_id` column propagation to 17 domain tables + cache namespacing `alita:{module}:{bot_id}:{id}` — tracked in #801 sub-tickets (see #806).
- Webhook multiplexing per clone token vs single webhook domain — infra decision.
- Observability per clone (metrics labels, tracing `bot_id`).

## 6. References

- `alita/config/config.go:115-253` — `Config` struct, `ValidateConfig`, `LoadConfig`, `init` with `logredact.Install`/`RegisterSecret`, `BotVersion "2.21.3"` hard-coded default, `OWNER_ID` `validate:"required,min=1"`.
- `alita/utils/logredact/logredact.go:47-116,129-204` — structural regex, registry `RegisterSecret`/`Scrub`, `hook` + `Install(nil)` with `sync.Once`.
- `alita/utils/chat_status/access.go:206-215` — `RequirePrivate` DM-only gate.
- `alita/utils/helpers/telegram_helpers.go:12-27` — `DeleteMessageWithErrorHandling` best-effort delete.
- `main.go:124-144,355-402,458-467` — `gotgbot.NewBot` with `BaseBotClient`, `newBotAPITransport`, `resolveBotAPIURL`, `resolveBotUsername`, `postInit` `SetMyCommands`.
- `main.go:44-74` — `isCliModeActive` / flag handling (`--version`, `--health`) precedent for env handling.
- `alita/utils/ratelimit/backup_ratelimit.go:36-96,118-152` — `acquireOperation` / `canOperate` 1h cooldown pattern to reuse for clone rate-limit.
- `alita/utils/cache/adminCache.go:19, cache/restrictedCache.go:22` — key format `alita:{module}:{id}` (`alita:adminCache:%d`, `alita:restricted:%d`) to namespace as `alita:{module}:{bot_id}:{id}` for clones.
- `alita/db/models/*.go` (`notes.go:24-44`, `connections.go`, `filters.go`) — GORM model pattern `TableName()`, column tags, JSONB `ButtonArray`/`StringArray` via `types.go`.
- `migrations/20260730030000_use_timestamptz_for_captcha_attempts.sql` — latest migration (base for `20260808000000_add_clones_table.sql`).
- `sample.env` — env placeholder list (add `CLONE_TOKEN_ENC_KEY`).
- Telegram Bot API: `GET https://api.telegram.org/bot<token>/getMe` → 200/401; `getWebhookInfo`, `deleteWebhook`, `setMyCommands`.
