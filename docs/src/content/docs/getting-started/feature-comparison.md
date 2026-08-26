---
title: Alita vs Miss Rose
description: Feature-by-feature map of Alita Robot against Miss Rose (missrose.org) — what we have, what is partial, and what we do not have.
---

<!-- MANUALLY MAINTAINED: do not regenerate -->

# Alita vs Miss Rose

This page maps [Miss Rose](https://missrose.org/) (`@MissRose_bot`) against Alita. Rose is the de-facto Telegram group-management bot; Alita already covers the same core Marie/Rose command set, with gaps in federations, logging, lock/blocklist depth, and a few anti-spam extras.

**How to read the status column**

| Status | Meaning |
|--------|---------|
| **Have** | Alita has an equivalent users can rely on |
| **Partial** | Same feature area exists, but Rose is deeper or Alita behaves differently |
| **Missing** | Alita has no equivalent |
| **Alita-only** | Alita has this; public Rose does not (or only ships it on paid custom clones) |

Rose source: public docs at [missrose.org/docs](https://missrose.org/docs/) (reviewed August 2026). Alita source: modules, locales, and command docs in this repo.

---

## Snapshot

| | Miss Rose (public) | Alita |
|---|--------------------|-------|
| Model | Hosted SaaS (`@MissRose_bot`); paid **custom clones** unlock extras | Open-source, self-hostable, 7 locales including help |
| Command surface | Large; extra clone-only commands | 29 modules, 158 commands |
| Languages | **31** locales; `/start` and `/help` stay English | **7** locales (`en`, `es`, `fr`, `hi`, `ru`, `pt`, `id`); help is translated |
| Biggest Rose-only systems | Federations, admin log channels, forum topics | — |
| Biggest Alita-only systems | — | Keyword reactions, in-group captcha + pending-message capture, `/tr`, self-host + observability |

Core group management (bans/mutes/warns, notes/filters/rules, welcomes, connections, backup, antiraid, approvals, reports, pins) is largely in place. The remaining work is **depth inside existing modules** plus **three missing subsystems**: federations, log channels, and forum topics.

---

## 1. Moderation actions

| Feature | Miss Rose | Alita | Status |
|---------|-----------|-------|--------|
| Ban / unban | `/ban`, `/unban` | Same | Have |
| Timed ban | `/tban` (`m/h/d/w`) | Same | Have |
| Delete + ban | `/dban` | Same | Have |
| Silent ban | `/sban` | Same | Have |
| Kick | `/kick` | Same | Have |
| Delete + kick | `/dkick` | Same | Have |
| Silent kick | `/skick` | No | Missing |
| Mute / unmute | `/mute`, `/unmute` | Same | Have |
| Timed mute | `/tmute` | Same | Have |
| Delete + mute | `/dmute` | Same | Have |
| Silent mute | `/smute` | Same | Have |
| Self-kick | `/kickme` (disableable) | Same | Have |
| Restrict picker | — | `/restrict` / `/unrestrict` inline keyboard | Alita-only |
| Warns | `/warn`, `/dwarn`, `/swarn`, `/warns`, `/rmwarn`, `/resetwarn`, limit + mode | Same command set | Have |
| Warn modes | kick / ban / mute / **tban / tmute** | kick / ban / mute only | Partial |
| Warn limit | Configurable (default 3) | Same (default 3, then mute) | Have |
| Anonymous admin verify | Button + `/anonadmin` | Same (20s cache, prove-admin button) | Have |
| Promote / demote / title | Maps Telegram perms; cannot grant `addAdmins` | `/promote`, `/demote`, `/title` | Have |
| Admin list / cache refresh | `/adminlist`, force-refresh cache | `/adminlist`, `/admincache`, `/clearadmincache` | Have |
| Admin error spam toggle | Disable “you need to be admin” replies | No dedicated toggle | Missing |
| Invite link | Present | `/invitelink` | Have |

---

## 2. Anti-spam

### Locks

| Feature | Miss Rose | Alita | Status |
|---------|-----------|-------|--------|
| Lock / unlock / list | `/lock`, `/unlock`, `/locks`, `/locktypes` | Same | Have |
| Category locks | `all` plus many single types | `all`, `media`, `messages`, `other`, `previews` | Partial |
| Lock warnings | `/lockwarns on/off` | No | Missing |
| Per-lock action mode | `/lock … ### {ban\|kick\|mute\|tban\|tmute}` | Delete only | Missing |
| Allowlists | `/allowlist`, `/rmallowlist`, `/rmallowlistall` for url/button/invitelink/inline/anonchannel/command/sticker/emoji/forward | No | Missing |

**Lock types Rose has that Alita does not:** `album`, `button`, `cashtag`, `checklist`, `cjk`, `command`, `cyrillic`, `email`, `emoji`, `emojicustom`, `emojigame`, `emojionly`, `externalreply`, `forwarduser`, `forwardbot`, `forwardchannel`, `forwardstory`, `guestbot`, `inline`, `invitelink`, `botlink`, `outsidereaction`, `phone`, `poll`, `reaction`, `spoiler`, `stickeranimated`, `stickerpremium`, `text`, `zalgo`.

**Lock types both have (names may differ):** `all`, `anonchannel`, `audio`, `bots`/`bot`, `comment`/`comments`, `contact`, `document`, `forward`, `game`, `gif`, `location`, `photo`, `rtl`, `sticker`, `url`, `video`, `videonote`, `voice`.

Alita-only named locks: `media`, `messages`, `other`, `previews`.

### Blocklists (Alita: blacklists)

| Feature | Miss Rose | Alita | Status |
|---------|-----------|-------|--------|
| Add / remove / list | `/addblocklist`, `/rmblocklist`, `/blocklist` | `/addblacklist`, `/rmblacklist`, `/blacklists` | Have |
| Default action | `/blocklistmode` nothing/warn/kick/ban/tban/mute/tmute | `/blaction` none/warn/mute/kick/ban (default **warn**) | Partial |
| Don't delete the message | `/blocklistdelete off` | Always deletes | Missing |
| Per-entry action | `{ban}`, `{tban 2w}`, `{skick}`, … | Chat-wide action only | Missing |
| Glob modifiers | `?`, `*`, `**`, escaping | Aho-Corasick substring; no globs | Missing |
| Unicode / space normalize | Accent folding, collapsed spaces | Case-insensitive only | Missing |
| Prefix / exact entries | `prefix:`, `exact:` | No | Missing |
| Lookalike entries | `lookalike:` | No | Missing |
| Sticker / emoji packs | `stickerpack:` | No | Missing |
| File / extension | `file:docs.pdf`, `file:*.pdf` | No | Missing |
| Forward / inline sources | `forward:`, `inline:` | Covered only by the `forward` lock | Partial |
| Bulk add | Bracketed comma lists | One trigger at a time | Missing |
| Default reason | `/setblocklistreason` | Generated / none | Missing |
| Name / username lists | Clone-only `name:`, `username:` | No | Missing (clone-only on Rose) |
| Clear all | Owner command | `/remallbl` (owner) | Have |

### Flood, raid, CAPTCHA, reports, approvals

| Feature | Miss Rose | Alita | Status |
|---------|-----------|-------|--------|
| Consecutive flood | `/setflood N` | Same (3–100, or off) | Have |
| Timed flood window | `/setfloodtimer` (N messages in duration) | Consecutive only | Missing |
| Flood modes | kick/ban/mute/**tban/tmute** | kick/ban/mute | Partial |
| Delete flood messages | Default last-over-limit; `/clearflood` deletes the whole burst | `/delflood` on/off | Partial |
| AntiRaid | `/antiraid`, `/raidtime` (default 6h), `/raidactiontime` (default 1h), `/autoantiraid` | Same commands and defaults | Have |
| CAPTCHA on join | Requires **welcome on**; button on welcome | Independent of welcome; mutes then challenges in-group | Partial |
| CAPTCHA modes | `button`, `text`, `math`, `text2` (char pick in PM) | `math`, `text` (image, in-group) | Partial |
| Join-request CAPTCHA | Auto-PM captcha, then accept the request | `/autoapprove` accept-all; no captcha-gated join requests | Partial |
| CAPTCHA kick / timeout | `/captchakick`, `/captchakicktime` | `/captchatime` + `/captchaaction` kick/ban/mute | Have |
| CAPTCHA rules gate | `/captcharules` | No | Missing |
| CAPTCHA auto-unmute | `/captchamutetime` | No (timeout takes the failure action) | Missing |
| CAPTCHA button text | `/setcaptchatext` | No | Missing |
| Max attempts / refresh | Not documented the same way | `/captchamaxattempts`, refresh with cooldown | Alita-only |
| Pending-message capture | — | `/captchapending`, `/captchaclear` | Alita-only |
| Reports | `/report`, `@admin`, `/reports on/off` | Same + per-admin PM toggle | Have |
| Report action buttons | Notify admins | Kick / ban / delete / resolved / jump-to-message | Alita-only |
| Block reporters | — | `/reports block\|unblock\|showblocklist` | Alita-only |
| Approvals | `/approve` (reason), `/unapprove`, `/approval`, `/approved`, `/unapproveall` | Same, including optional reason | Have |
| Approval exemptions | Immune to flood, blocklists, locks | Also captcha + antispam telemetry | Have |

---

## 3. Content: notes, filters, rules, greetings, pins

| Feature | Miss Rose | Alita | Status |
|---------|-----------|-------|--------|
| Notes save/get/`#name` | `/save`, `/get`, `#note`, `/notes`, `/clear` | Same (`/saved` alias) | Have |
| Media notes | Reply-save | Same | Have |
| Private notes (chat-wide) | `/privatenotes` | Same | Have |
| Per-note `{private}` / `{noprivate}` / `{admin}` | Yes | Yes | Have |
| `{protect}` / `{nonotif}` / `{preview}` | Yes | Yes (`{preview}` enables link preview) | Have |
| `{mediaspoiler}` | Yes | No | Missing |
| Note → note buttons | `[name](buttonurl://#othernote)` menus | URL buttons only; `#note` buttons dropped | Missing |
| Repeated / scheduled notes | Clone-only `{repeat 6h}` | No | Missing (clone-only on Rose) |
| Filters | `/filter`, `/filters`, `/stop` | Same + overwrite confirm, 150 cap | Have |
| Random variants | `%%%` | Same | Have |
| Filter `{admin}` | Admin-only trigger | Documented and parsed | Have |
| Prefix / exact filters | Yes | Substring Aho-Corasick only | Missing |
| `{user}` / `{replytag}` / `{allow_bot}` | Yes | No | Missing |
| `{command}` bot-menu filters | Clone-only | No | Missing (clone-only on Rose) |
| Rules | `/rules`, `/setrules`, `/resetrules`, `/privaterules`, `/setrulesbutton` | Same (`/rulesbtn`) | Have |
| `{rules}` filling | Button in notes/filters/welcomes | Same + `{rules:up\|same}` placement | Have |
| Welcome / goodbye | `/welcome`, `/setwelcome`, `/resetwelcome`, goodbye twins | Same | Have |
| Welcome `noformat` | Yes | Yes | Have |
| Clean welcome | `/cleanwelcome` (also 5-minute expiry) | `/cleanwelcome` / `/cleangoodbye` on new join | Partial |
| CAPTCHA tied to welcome | Required | Captcha can run without welcome | Alita-only |
| Pin / unpin / unpinall | Yes | Same | Have |
| Loud pin | `/pin loud` | `/pin loud\|notify\|violent` | Have |
| Permapin | `/permapin` | Same (text, buttons, media) | Have |
| `/pinned` | Yes | Same | Have |
| Anti-channel-pin | `/antichannelpin` | Same | Have |
| Clean linked channel | `/cleanlinked` | Same | Have |

**Shared fillings:** `{first}`, `{last}`, `{fullname}`, `{username}`, `{mention}`, `{id}`, `{chatname}`, `{rules}`.

**Alita extra filling:** `{count}` (member count).

**Markdown / buttons:** both support markdown, URL buttons, and `:same` for same-row buttons. Rose also ships a [web button generator](https://missrose.org/docs/formatting/button-generator/); Alita does not.

---

## 4. Cleaning, disabling, connections, backup, languages

| Feature | Miss Rose | Alita | Status |
|---------|-----------|-------|--------|
| Clean all service messages | `/cleanservice on/off` | Same | Have |
| Typed service clean | `/cleanservice join leave pin …` | All-or-nothing | Missing |
| Clean commands | `/cleancommand` admin/user/other | `/disabledel` only for **disabled** commands | Partial |
| Clean bot action/filter/note msgs | Clone-only `/cleanmsg` | No | Missing (clone-only on Rose) |
| Disable commands | `/disable`, `/enable`, `/disableable`, `/disabled` | Same | Have |
| Delete disabled cmds | Yes | `/disabledel` | Have |
| Disable for admins too | `/disableadmin` | Admins always bypass | Missing |
| Connections | `/connect`, `/disconnect`, `/connection` | Same | Have |
| Reconnect | Last **5** chats; `/reconnect` / empty `/connect` | Last **1** chat (`/reconnect`) | Partial |
| Allow connect | Chat setting | `/allowconnect` | Have |
| Connect from group | Button → PM | Same deep-link button | Have |
| Export / import JSON | `/export`, `/import` (creator), module subset | Same, 17 modules, format `1.1` | Have |
| Reset settings | `/reset` (creator, confirm) | Same + per-module reset | Have |
| Languages | 31 locales; help stays English | 7 locales; **help is translated**; Crowdin | Partial |
| Boolean on/off | on/yes/true, off/no/false | Same pattern in toggles | Have |
| Duration syntax | `Xm/Xh/Xd/Xw` | Same + raw seconds on antiraid | Have |

---

## 5. Missing Rose subsystems

These are not “a thinner version of a module we already have” — they are whole products Alita does not ship.

### Federations — Missing

Rose’s largest unique feature. A federation is a shared ban list with its own owner and admins, subscribed by many chats.

Documented pieces (none exist in Alita):

- Create / rename / delete: `/newfed`, `/renamefed`, `/delfed` (PM only; one fed per owner)
- Subscribe chats, fed admins, fed info / subs
- `/fban` / unban — **active** (ban immediately in chats where the user was seen) then **passive** (ban on join/speak)
- Sub-federations
- Fed log channel (`/setfedlog`) and owner PM notifications
- Quiet fed (hide “banned in fed X” messages)

There is no Alita equivalent. Approvals, antiraid, and per-chat bans do not sync across chats.

### Admin log channels — Missing

Rose `/log` posts bot actions to a channel, with categories:

`settings`, `admin`, `user`, `automated`, `reports`, `other`

Commands: `/log`, `/nolog`, `/logcategories`. Alita has no chat-facing log channel. Operators only get process metrics (`/metrics`, `/db_metrics`, OTel).

### Forum topics — Missing

Rose: `/actiontopic`, `/setactiontopic`, `/newtopic`, `/renametopic`, `/closetopic`, `/reopentopic`, `/deletetopic`. Settings are forum-wide; action topic is where welcomes go.

Alita has no topic commands. It will still answer in forum groups as a normal chat, without topic-aware routing.

---

## 6. What Alita has that public Rose does not

| Area | Alita |
|------|-------|
| License / deploy | Open source, Docker/distroless, webhooks, self-host |
| Observability | Prometheus `/metrics`, `/db_metrics`, OpenTelemetry tracing, `/health` |
| Keyword reactions | `/addreaction` — auto-react to keywords with Telegram emoji |
| Translation | `/tr <lang>` |
| Ping / stats | `/ping` (API RTT, send, overhead), `/stat` |
| Echo via bot | `/tell` (admin, reply) — Rose `/echo` is **clone-only** |
| Captcha | In-group math/text images, refresh, max attempts, pending-message store |
| Join requests | `/autoapprove` (accept all) |
| Reports | Inline kick/ban/delete/resolved + reporter blocklist |
| Restrict UI | `/restrict` / `/unrestrict` keyboards |
| Fillings | `{count}`, `{rules:up\|same}` |
| Dev / owner | `/devs` diagnostics (not a Rose user feature) |
| Help i18n | Translated help in all 7 locales (Rose keeps help in English) |

Rose **custom clones** (paid) add `/echo`, `/broadcast`, `/cleanmsg`, repeated notes, `{command}` filters, and name/username blocklists. Those are not gaps versus the public bot.

---

## 7. What to do next (gap list)

Priority is “how much Rose users will miss this,” not implementation size.

### Highest — whole subsystems

1. **Federations** — `/newfed`, chat subscribe, `/fban`, fed admins, fed log. This is the usual reason communities stay on Rose.
2. **Admin log channels** — `/log` with the six Rose categories.
3. **Forum topics** — action topic + create/rename/close/delete.

### High — depth in modules we already have

4. **Locks** — remaining lock types (especially `invitelink`, `command`, `inline`, `poll`, `emoji*`, `zalgo`, `cjk`, split forwards), `/lockwarns`, per-lock `{mode}`, allowlists.
5. **Blocklists** — globs (`?` `*` `**`), per-entry `{ban}`/`{tban}`, `prefix:`/`exact:`, sticker/file/forward/inline entries, optional non-delete.
6. **Antiflood** — `/setfloodtimer` (N in a time window) and `tban`/`tmute` modes.
7. **CAPTCHA** — join-request flow (PM captcha then approve), `button` mode, `/captcharules`.
8. **Filters** — prefix and exact match; `{replytag}` / `{allow_bot}`.

### Medium

9. **Languages** — more locales (Rose 31 vs Alita 7). Help translation is already ahead.
10. **Connections** — remember last 5 chats, not only the previous one.
11. **Typed `/cleanservice`** and `/cleancommand`.
12. **Warn / flood / blacklist timed modes** (`tban`/`tmute`) for automated actions.
13. **`/skick`**, **`{mediaspoiler}`**, **note-to-note buttons** (`buttonurl://#note`).
14. **`/disableadmin`**, admin-error quiet mode.

### Low / skip unless requested

15. Rose clone-only: `/echo` (Alita has `/tell`), `/broadcast`, `/cleanmsg`, repeated notes, `{command}` filters, `name:`/`username:` blocklists, web button generator.
16. Fun extras such as Rose `/runs`.

---

## 8. Module crosswalk

| Rose docs section | Alita module | Status |
|-------------------|--------------|--------|
| [Getting started](https://missrose.org/docs/getting-started/) | Hosted bot + [self-hosting](/self-hosting/) | Have |
| [Command usage](https://missrose.org/docs/basics/command-usage/) | Shared toggle/duration parsing | Have |
| [Languages](https://missrose.org/docs/basics/languages/) | `languages` | Partial (7 vs 31) |
| [Rules](https://missrose.org/docs/basics/rules/) | `rules` | Have |
| [Connections](https://missrose.org/docs/connections/) | `connections` | Partial (1 vs 5 recents) |
| [Locks](https://missrose.org/docs/anti-spam/locks/) | `locks` | Partial |
| [Blocklists](https://missrose.org/docs/anti-spam/blocklists/) | `blacklists` | Partial |
| [Antiflood](https://missrose.org/docs/anti-spam/antiflood/) | `antiflood` | Partial |
| [AntiRaid](https://missrose.org/docs/anti-spam/antiraid/) | `antiraid` | Have |
| [CAPTCHA](https://missrose.org/docs/anti-spam/captchas/) | `captcha` | Partial |
| [Restrictions](https://missrose.org/docs/moderation/restrictions/) | `bans`, `mutes` | Partial (`/skick`) |
| [Warnings](https://missrose.org/docs/moderation/warnings/) | `warns` | Partial (no tban/tmute mode) |
| [Admins](https://missrose.org/docs/moderation/admins/) | `admin` | Partial (no admin-error toggle) |
| [Approvals](https://missrose.org/docs/moderation/approvals/) | `approvals` | Have |
| [Reports](https://missrose.org/docs/moderation/reports/) | `reports` | Have (Alita ahead on buttons) |
| [Pins](https://missrose.org/docs/moderation/pins/) | `pins` | Have |
| [Disabling](https://missrose.org/docs/moderation/disabling/) | `disabling` | Partial (no `/disableadmin`) |
| [Log channels](https://missrose.org/docs/moderation/log-channels/) | — | Missing |
| [Notes](https://missrose.org/docs/notes/) | `notes` | Partial (no `#note` buttons / mediaspoiler) |
| [Filters](https://missrose.org/docs/filters/) | `filters` | Partial |
| [Welcomes](https://missrose.org/docs/greetings/) | `greetings` | Have |
| [Formatting](https://missrose.org/docs/formatting/) | `formatting` | Partial |
| [Clean service](https://missrose.org/docs/cleaning/clean-service/) | `greetings` `/cleanservice` | Partial |
| [Clean command](https://missrose.org/docs/cleaning/clean-command/) | `/disabledel` only | Partial |
| [Exports](https://missrose.org/docs/exports/) | `backup` | Have |
| [Federations](https://missrose.org/docs/federations/) | — | Missing |
| [Topics](https://missrose.org/docs/topics/) | — | Missing |
| [Echo](https://missrose.org/docs/echo/) (clones) | `/tell` | Alita-only vs public Rose |
| — | `reactions` | Alita-only |
| — | `misc` `/tr` `/ping` `/stat` | Alita-only |
| — | `antispam` (telemetry rate limiter) | Alita-only (not a CAS/Spamwatch ban list) |

---

## 9. Keeping this page honest

When you add a Rose-like feature, update the matching row here in the same change. Rose docs live at [missrose.org/docs](https://missrose.org/docs/); Alita command pages live under [Commands](/commands/).
