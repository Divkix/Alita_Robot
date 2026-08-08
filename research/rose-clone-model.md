# Research: How MissRose Custom Clones Work — Findings (#802)

> Branch: `research/rose-clone-model` · Source: https://missrose.org/custom-instances/ + https://believablebots.io · 2026-08-08  
> Ticket: #802 (Part of #801)

## Observed — Verbatim Sources

### 1. `missrose.org/custom-instances/` — Tier Feature Table

Source snapshot 2026-08-08 via direct fetch. Header links point to `believablebots.io` (marketing) and `t.me/RoseClone_bot` (clone factory bot).

| Feature | Free | Rose Core | Rose Premium |
|---|---|---|---|
| Filters / Notes / Blocklist entries | 200 / 1000 / 200 | 500 / 2000 / 500 | 500 / 2000 / 500 |
| Number of groups (based on tier) | N/A | 5 / 10 / 20 | 50 |
| Custom name & profile picture | ❌ | ✅ | ✅ |
| [Echo commands](https://missrose.org/docs/echo/) | ❌ | ✅ | ✅ |
| [Broadcast announcements](https://missrose.org/docs/echo/#broadcast) | ❌ | ✅ | ✅ |
| [Scheduled/repeated messages](https://missrose.org/docs/notes/#repeated-notes-custom-clones) | ❌ | ✅ | ✅ |
| [Blocklist spam names & usernames](https://missrose.org/docs/anti-spam/blocklists/advanced/#blocklisting-names-custom-clones) | ❌ | ✅ | ✅ |
| [Auto-clean old bot messages](https://missrose.org/docs/cleaning/clean-message/) | ❌ | ✅ | ✅ |
| [Filter commands autofill](https://missrose.org/docs/filters/#command-suggestions-custom-clones) | ❌ | ✅ | ✅ |
| Priority support | ❌ | ❌ | ✅ |
| Dedicated infrastructure | ❌ | ❌ | ✅ |

Notes on table:
- "Number of groups (based on tier)" renders as `N/A` for Free, `5 / 10 / 20` for Core, `50` for Premium. Core's three buckets are unexplained on-page — inferred to map to Core sub-tiers (see believablebots.io pricing section below). Free `N/A` means the free path is the public `@MissRose_bot`, not a clone.
- Limits row is per-feature-type caps, not a single pool: the slash-separated triple is Filters / Notes / Blocklist entries.

### 2. `believablebots.io` — Marketing Site

**Hero** (Hugo 0.118.2):
> "Protect your telegram communities from spam! After 5 years of moderating groups and fighting spam across Telegram, we're finally making it possible for users to get their own, fully customised, versions of Rose. With over 770 million users across 17 million chats, Rose has more than proven her worth."

**Feature blocks** (four illustrated cards):
1. **Spam fighting** — "Use all of Rose's tried-and-tested features to protect your group from spam, misinformation, and antisocial behaviour."
2. **Customisation** — "Customise your bot to match your community! Change the bot's name and picture according to your needs."
3. **Exclusive features** — "Enable exclusive new features to enhance your group's interactions! Send scheduled reminders, speak as your bot, and more!"
4. **Increased limits** — "Get increased notes, filters, and blocklists with your very own bot!"

**Pricing section** (`#pricing` anchor, CTA all to `t.me/RoseClone_bot`):

- **Free**: "Completely free! Blocklists, CAPTCHAs, AntiRaid, Locks - all the core features are available. Extensive online documentation. Fair usage limits apply; limited notes, filters, blocklists, etc. A public support group. [Public Rose](https://t.me/MissRose_bot)" — i.e. Free is *not* a clone; it's the shared public bot.
- **Core — $10/mo (VAT not included)**: "Your own branded bot · Customisable name and profile picture · All free tier features included · Increased limits on notes, filters, and blocklists · Exclusive features: echo, broadcasts, repeated messages. [Create your bot now!](https://t.me/RoseClone_bot) [Terms & Conditions](/subscriptions-terms-and-conditions.pdf)"
- **Premium — $50/mo (VAT not included)**: "Everything in Core, plus: Can be used in [more groups](https://missrose.org/custom-instances/) · Dedicated infrastructure for enhanced performance · Priority support · Perfect for high-traffic communities. [Create your premium bot now!](https://t.me/RoseClone_bot) [Terms & Conditions](/subscriptions-terms-and-conditions.pdf)"

Payment/Legal (T&C PDF, `subscriptions-terms-and-conditions.pdf`, updated 2023-09-09): third-party `Paddle` for billing, monthly/quarterly/annual terms, VAT on top, 7-day refund window minus admin. Not directly infra-relevant but confirms subscription backend is standard SaaS, not Telegram-native.

### 3. `@RoseClone_bot` Flow (as documented + inferred)

- Single entry point referenced from both sites: `t.me/RoseClone_bot` ("Want your own Rose bot with your branding and premium features? You're in the right place!" CTA on missrose page).
- Docs imply the Telegram onboarding: user supplies a BotFather token → Rose validates via `getMe` (standard Bot API; bot checks token authenticity and captures `bot_id`/username) → provisions a clone with branding (name/avatar set via `setMyName`/`setUserProfilePhotos` equivalent). No public doc details rate limits on this flow; limits are post-provisioning (groups count + per-type caps).
- Lifecycle docs do not publicise `/unclone`/`/myclones` equivalents; offboarding is cancellation via subscription (see T&C 2.2), i.e. managed outside Telegram. Revoked-token handling is not documented publicly; likely detected server-side on poll failure.

### 4. 17-Module Context (`alita/db/backup/types.go`)

Alita's exportable domain (BackupFormat + per-module backup structs, today `BackupFormatVersion = "1.1"`):

Constants (18 defined, spec calls it "17-module" — likely counts without `admin` or pre-reactions):

```go
BackupModuleAdmin       = "admin"
BackupModuleAntiflood   = "antiflood"
BackupModuleAntiraid    = "antiraid"
BackupModuleApprovals   = "approvals"
BackupModuleBlacklists  = "blacklists"
BackupModuleCaptcha     = "captcha"
BackupModuleConnections = "connections"
BackupModuleDisabling   = "disabling"
BackupModuleFilters     = "filters"
BackupModuleGreetings   = "greetings"
BackupModuleLocks       = "locks"
BackupModuleNotes       = "notes"
BackupModulePins        = "pins"
BackupModuleReactions   = "reactions"
BackupModuleReports     = "reports"
BackupModuleRules       = "rules"
BackupModuleWarns       = "warns"
```

These are the domain tables that a clone model must scope per-`bot_id` (shared-tables design). See Inferred Architecture below — this maps 1:1 to the "17 modules" referenced in grilling notes.

---

## Inferred Architecture

### Same-process vs separate infra — what Rose does

- **Public Rose and Free = same process, same DB, same infra.** `Free` is not a clone; it's the single `@MissRose_bot` instance serving 17M chats. This is the baseline.
- **Core clones: very likely same process, shared DB, logically isolated.** Cues:
  - Same feature binary (echo/broadcast/scheduled etc. are feature flags, not separate builds).
  - Group cap `5 / 10 / 20` suggests a single control plane dispatching per-`bot_id` and gating by subscription row — cheap to enforce in-process (middleware check on `chat_id` × `bot_id`).
  - No marketing claim of isolation for Core — only Premium advertises "Dedicated infrastructure". If Core were container-per-clone, Premium's differentiator would be meaningless.
  - Operationally: 770M users / 17M chats × potentially thousands of clones would be unmanageable as one-container-per-clone at $10/mo.
- **Premium clones: dedicated infrastructure.** The table's `Dedicated infrastructure: ❌/❌/✅` plus believablebots copy "Dedicated infrastructure for enhanced performance" strongly implies at least dedicated workers / DB shard / queue for Premium bots (perhaps a separate polling shard or reserved webhook workers). Priority support is bundled only here, consistent with an SLO-bearing tier.
- **DB sharing model inferred**: shared Postgres with per-row `bot_id` (and `owner_id` on the clone registry) — every domain table gains a `bot_id` column. Alternative (schema-per-clone or DB-per-clone) contradicts the Core economics and would make the single `@RoseClone_bot` factory far more complex for little benefit. Cache (Redis) is namespaced: expected key pattern is `{prefix}:{module}:{bot_id}:{id}` or equivalent — otherwise clones would read each other's filters/notes.
- **Bot API multiplexing**: one control plane needs N `Bot` instances (one `token`/`bot_id` each). Whether via polling (N long-polls) or webhooks (one domain per token path — `/{bot_token}/webhook` multiplexing), the dispatcher must fan-in updates and route handlers with `bot_id` in context. This is the same seam Alita adopts (clones branch design).
- **Not observable from static docs**: exact cache prefix, migration story for adding `bot_id`, whether webhook vs polling, how revoked tokens are reaped (cron vs lazy on poll error), anti-abuse caps on clone creation. Those are implementation choices Rose does not publish.

### Confidence

- High: tier-gating model, limits, exclusive-feature set, branding entitlement.
- High: Free = shared public bot; Premium = isolated infra.
- Medium: Core = same-process shared-DB (inferred by elimination + economics; no Rose eng blog confirms it).
- Low: physical deployment (polling vs webhook multiplex, Redis layout specifics).

---

## What Alita Copies vs Intentionally Diverges

### Copies (pattern-level, not paywall)

- **Single clone-factory bot UX** — a bot like `@RoseClone_bot` (here: DM to the main Alita bot, `DM-only /clone <bot_token>`) that validates via `getMe` before provisioning. Rose proves users understand "give RoseClone_bot your BotFather token".
- **Per-clone branding capability** — even though imitation is free, the plumbing to set name/avatar per `bot_id` is worth retaining.
- **Operational limits as quotas** — the *shape* of Rose's gating (per-type caps + group cap) is reused as per-user clone cap + global cap + per-group feature caps namespaced by `bot_id`. See grilling lock below.
- **Same-process shared-DB with `bot_id`/`owner_id` columns** — the inferred Core-architecture is elevated to Alita's universal architecture (no Premium shard).
- **Namespaced cache keys** — `alita:{module}:{bot_id}:{id}` (grilling-locked) mirrors the inferred Rose `:{bot_id}:` namespacing.
- **Token validation via `getMe`** and lifecycle hygiene (delete trigger message, encrypt token AES-GCM, `logredact.RegisterSecret`).
- **Minimal lifecycle + revocation detection** — `/clone` + `/myclones` + `/unclone` + rotation by re-`/clone` + `getMe 401 → inactive` (leaner than Rose's subscription dashboard).

### Intentionally Diverges (product decisions, grilling-locked 2026-08-08)

| Dimension | Rose (observed) | Alita (decided) | Rationale |
|---|---|---|---|
| **Pricing** | Free = public bot only; clones are $10/$50 (paid SaaS via Paddle) | **All clones free** — any Telegram user can clone | Product thesis: free full-feature clones drive adoption; monetisation out of scope (#801). |
| **Feature tiering** | Echo / broadcast / scheduled / blocklist-advanced / autoclean / autofill are Core-or-higher only; limits doubles on Core/Premium | **Full feature parity** — clones get every Alita module (all ~17) with no paywall | Simplicity + parity; gating would be novel complexity with no business mandate. |
| **Infra tiers** | Core = shared; Premium = dedicated | **Single tier, always shared-process** | No dedicated infra to operate; keeps deploys to one binary + migrations. |
| **Group cap** | `N/A` / `5 / 10 / 20` / `50` | **Per-user clone cap 1–2 + global cap** (numerics finalised in #805), not per-group count caps | Abuse control without Rose-style group-metering complexity. |
| **Rate limit** | Subscription-managed, undocumented cooldown | **1h Redis cooldown** per user (borrows `backup` ratelimit pattern) | Cheap abuse brake; aligns with existing codebase pattern. |
| **Lifecycle scope** | Subscription dashboard + T&C cancellation | **In-Telegram only**: `/clone`, `/myclones`, `/unclone`, rotate via re-`/clone` | Minimal surface; no external billing system. |
| **Token handling** | Not documented publicly | **DM-only, delete message, never log raw, AES-GCM + `logredact`** | Security invariant stronger than observable Rose behaviour. |
| **Process/DB model rollout** | Not documented | **Shared tables + `bot_id`/`owner_id` per domain table + namespaced cache; single-dispatcher seam** | Locked in grilling (#806); avoids schema-per-clone migration burden. |

Grilling invariant restated: reuse of existing patterns is required where available (singleflight, gocache, backup cooldown, migration SHA-256 immutability, gotgbot dispatcher).

---

## Open Questions

1. **Rose Core sub-tier mapping** — what do `5 / 10 / 20` groups precisely correspond to? Monthly vs quarterly pricing granularity, or legacy tier names? Would clarify whether Alita's per-user cap of `1–2` is conservatively tight vs Rose's paid allowance.
2. **Revoked-token reaping interval / UX** — does Rose lazily mark inactive on next poll failure or run a sweeper? Alita plans `getMe 401 → inactive`; cadence (on-demand vs periodic cron) is still to lock.
3. **Webhook vs polling for clones** — if Alita uses polling, N concurrent `getUpdates` loops need a supervisor/fan-in; if webhook, needs path-per-token multiplexing and a single domain. Migration story for existing single-bot deployments is flagged "not yet specified" in #801.
4. **Per-module `bot_id` migration replay safety** — adding `bot_id` columns to 17 tables on a live DB with 770M-user-scale chat rows needs a zero-downtime story; should confirm existing `migrations/*.sql` SHA-256 immutability convention handles additive columns idempotently.
5. **Observability per clone** — per-`bot_id` metrics labels / tracing span attribute `bot_id` / health fan-in aggregation not yet specified (graduable ticket per #801).
6. **Branding write-back limits** — Telegram `setMyName`/`setMyProfilePhotos` rate limits are not published alongside Rose docs; free unlimited renames could hit Bot API quotas.
7. **Exclusivity enforcement for broadcast/echo/scheduled equivalents** — Alita has no premium features to gate, but should confirm none of the 17 modules accidentally inherit a "premium-only" code path from a Rose-inspired port.
8. **`Free` N/A semantics** — confirmed `Free` = public bot, but believablebots wording "Fair usage limits apply; limited notes, filters, blocklists, etc" suggests public Rose also has lower default caps than clones. Whether Alita's main bot should keep its current caps vs unify with clone caps is open.

---

## Sources

- https://missrose.org/custom-instances/ — tier table + factory CTA + feature deep-links.
- https://believablebots.io/ — hero, feature cards, pricing ($10 Core / $50 Premium), T&C/Paddle billing.
- https://believablebots.io/subscriptions-terms-and-conditions.pdf — 2023-09-09, billing terms.
- `alita/db/backup/types.go` — 18 `BackupModule*` constants (spec shorthand "17 modules"), `BackupFormat` v1.1, `AllExportableModules()`.

## Next Pointers

- Remaining wayfinder tickets: #803 (gotgbot multi-bot), #804 (token security) run in parallel per Step 5.
- Design decisions that consume this doc: #805/#806/#807 grilling threads (already locked same-process/shared-DB/free-parity direction).
