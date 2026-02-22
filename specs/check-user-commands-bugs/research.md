# Research: Bug Hunting in User-Facing Command Handlers

**Date:** 2026-02-22
**Goal:** Catalog all user-invocable functions in `alita/modules/` and identify bugs in user-facing commands.
**Confidence:** HIGH -- every module file was read, all handler registrations were cataloged, all critical bug patterns were searched.

---

## 1. Module Load Order & Command Registry

25 modules loaded in `alita/main.go:107-138`. Help module loads last via `defer`.

### Complete Command Registry (120+ commands)

| Module | Commands | Disableable | Callbacks | Watchers |
|--------|----------|-------------|-----------|----------|
| **help** | /start, /help, /donate, /about | None | helpq, configuration, about | None |
| **admin** | /promote, /demote, /invitelink, /title, /adminlist, /anonadmin, /admincache, /clearadmincache | adminlist | None | None |
| **bans** | /ban, /sban, /tban, /dban, /unban, /kick, /dkick, /kickme, /restrict, /unrestrict | None | restrict, unrestrict | None |
| **mute** | /mute, /smute, /tmute, /dmute, /unmute | None | None | None |
| **purges** | /del, /purge, /purgefrom, /purgeto | None | deleteMsg | None |
| **warns** | /warn, /swarn, /dwarn, /resetwarns, /resetwarn, /rmwarn, /unwarn, /warns, /setwarnlimit, /setwarnmode, /resetallwarns, /warnings | warns | rmAllChatWarns, rmWarn | None |
| **filters** | /filter, /addfilter, /stop, /rmfilter, /removefilter, /filters, /stopall | filters | rmAllFilters, filters_overwrite | Group 8 (filter matching) |
| **notes** | /save, /addnote, /clear, /rmnote, /notes, /saved, /clearall, /get, /privnote, /privatenotes | notes, get | rmAllNotes, notes.overwrite | #notename watcher |
| **blacklists** | /blacklists, /addblacklist, /blacklist, /rmblacklist, /blaction, /blacklistaction, /remallbl, /rmallbl | blacklists | rmAllBlacklist | Blacklist enforcement |
| **greetings** | /welcome, /setwelcome, /resetwelcome, /goodbye, /setgoodbye, /resetgoodbye, /cleanwelcome, /cleangoodbye, /cleanservice, /autoapprove | None | join_request | ChatMember join/leave |
| **captcha** | /captcha, /captchamode, /captchatime, /captchaaction, /captchamaxattempts, /captchapending, /captchaclear | None | captcha_verify, captcha_refresh | Group -10 (pending msgs) |
| **connections** | /connect, /disconnect, /connection, /reconnect, /allowconnect | None | connbtns | None |
| **disabling** | /disable, /disableable, /disabled, /disabledel, /enable | disabled | None | None |
| **rules** | /rules, /setrules, /resetrules, /clearrules, /privaterules, /rulesbutton, /rulesbtn, /clearrulesbutton, /clearrulesbtn, /resetrulesbutton, /resetrulesbtn | rules | None | None |
| **antiflood** | /setflood, /setfloodmode, /delflood, /flood | flood | None | Flood detection |
| **locks** | /lock, /unlock, /locktypes, /locks | locktypes, locks | lockAction | Perm + restriction watchers |
| **language** | /lang | None | change_language | None |
| **reports** | /report, /reports | report | report | @admin/@admins watcher |
| **pins** | /unpin, /unpinall, /pin, /pinned, /antichannelpin, /permapin, /cleanlinked | None | unpinallbtn | Channel pin watcher |
| **misc** | /stat, /id, /tell, /ping, /info, /tr, /removebotkeyboard | stat, id, ping, info, tr | None | None |
| **devs** | /stats, /addsudo, /adddev, /remsudo, /remdev, /teamusers, /chatinfo, /chatlist, /leavechat | None | None | None |
| **formatting** | /markdownhelp, /formatting | None | formatting | None |
| **bot_updates** | (no user commands) | None | anon_admin, alita:anonAdmin: | ChatMember tracking |
| **antispam** | (no user commands) | None | None | Group 4 (gban enforcement) |
| **users** | (no user commands) | None | None | logUsers on ALL messages |

---

## 2. Dependency Chain Summary

### Core Dependencies
- **gotgbot v2.0.0-rc.33**: Telegram Bot API 9.1. Pre-release candidate. `ctx.EffectiveSender` can be nil for channel posts. `GetChat` returns `ChatFullInfo` (requires `.ToChat()`).
- **GORM v1.31.1**: ORM with surrogate key pattern. Known preload regression (#7686) but project doesn't use Preload.
- **Redis + gocache v4.2.3**: Cache with singleflight stampede protection. 15+ cache key namespaces with TTLs 20sec-1hr.

### Key Utility Packages
- **Permission System (chat_status)**: 20+ functions. Admin caching with 30-min TTL. Anonymous admin handling with 20-sec verification cache.
- **i18n System**: Singleton LocaleManager. 4 locales (en, es, fr, hi) + config.yml. Supports named `{key}` and legacy `%s` params.
- **Extraction Utils**: `ExtractUserAndText` returns inconsistent error values (-1 vs 0). Numeric IDs trusted without validation.
- **Callback Codec**: 64-byte limit. Silent fallback to legacy dot-notation on encoding failures.

---

## 3. Bugs Found

### Category 1: Nil Pointer Panics (HIGH SEVERITY)

**Bug 1: users.go:76-79 -- logUsers handler panics on channel posts**
- `ctx.EffectiveSender` used without nil check in handler that runs on EVERY message
- `user.IsAnonymousChannel()` panics if sender is nil
- This is the most critical bug: crashes the entire message processing pipeline

**Bug 2: purges.go:149,335,419 -- ctx.EffectiveUser.Id without nil check**
- Three callback handlers (`purgeToCallback`, `purgeFromCallback`, `purgeCallback`) call `RequireUser()` but then use `ctx.EffectiveUser.Id` instead of the returned `user.Id`
- If `ctx.EffectiveUser` is nil, this panics

**Bug 3: misc.go:277-284 -- info command panics on channel messages**
- `ctx.EffectiveSender` used without nil check
- `sender.Id()` panics if sender is nil

**Bug 4: misc.go:79 -- echomsg handler msg.From nil risk**
- `msg.From.Id` accessed without nil check
- `msg.From` can be nil for channel-forwarded messages in edge cases

### Category 2: DB Layer Design Flaw -- 36+ Void DB Functions (MEDIUM SEVERITY)

The following DB write functions have **void return types** -- they log errors internally but return nothing. Users see success messages even when DB writes fail.

**Affected DB functions (by file):**
- `greetings_db.go`: SetWelcomeText, SetWelcomeToggle, SetGoodbyeText, SetGoodbyeToggle, SetShouldCleanService, SetShouldAutoApprove, SetCleanWelcomeSetting, SetCleanGoodbyeSetting, SetCleanWelcomeMsgId, SetCleanGoodbyeMsgId (10 functions)
- `antiflood_db.go`: SetFlood, SetFloodMode, SetFloodMsgDel (3 functions)
- `lang_db.go`: ChangeUserLanguage, ChangeGroupLanguage (2 functions)
- `pin_db.go`: SetAntiChannelPin, SetCleanLinked (2 functions)
- `warns_db.go`: SetWarnLimit, SetWarnMode (2 functions)
- `filters_db.go`: AddFilter, RemoveFilter (2 functions -- need verification)

**Correctly-designed counterexamples**: `db.SetCaptchaEnabled`, `db.AddSudo`, `db.DisableCMD`, `db.ToggleDel` -- these all return `error`.

### Category 3: Fire-and-Forget Goroutine DB Operations (LOW SEVERITY)

- `helpers.go:260`: `go db.ConnectId(user.Id, cochat.Id)` -- error lost
- `connections.go:110-119`: `go db.ToggleAllowConnect()` patterns
- `connections.go:170-173`: `go db.ConnectId()` pattern
- `connections.go:326-329`: `go db.DisconnectId()` pattern

### Category 4: i18n Config.yml Not Loadable (MEDIUM SEVERITY)

`config.yml` is explicitly skipped during locale loading, but `helpers.go` calls `MustNewTranslator("config")` to look up `alt_names` mappings. This silently falls back to "en" which has no `alt_names`, meaning **alternative module names are NOT being resolved** in the help system.

### Category 5: Extraction Utility Inconsistencies (MEDIUM SEVERITY)

- `ExtractUserAndText` returns -1 for username-not-found but 0 for other failures
- Numeric user IDs are trusted without validation (no check for negative IDs or zero)
- UUID detection in args can cause unexpected early returns

### Category 6: HtmlEscape Incomplete (LOW SEVERITY)

`HtmlEscape()` only escapes `&`, `<`, `>` but not `"` or `'`. Used only for message text (not attributes), so actual injection risk is low.

### Category 7: IsUserConnected Mutates Context (MEDIUM SEVERITY)

`IsUserConnected()` at helpers.go:307 sets `ctx.EffectiveChat = chat`. Handlers after this call see the connected chat, not the original. Any code that reads `ctx.EffectiveChat` after `IsUserConnected` must be aware of this mutation.

### Category 8: Miscellaneous

- `misc.go:461`: `time.Sleep(1 * time.Second)` blocks handler goroutine in `removeBotKeyboard`
- `devs.go:117`: `os.WriteFile("chatlist.txt")` race condition if two users invoke `/chatlist` simultaneously
- `captcha.go:240-245`: `secureIntn` has infinite retry loop on `crand.Int()` error (no backoff/max retries)
- Callback codec `EncodeOrFallback` silently falls back to dot-notation; fallback could also exceed 64 bytes

---

## 4. What's NOT Broken (Verified Clean)

- **No double-answer callback bugs**: All callback handlers return `ext.EndGroups` immediately when `RequireUserAdmin` fails
- **Entity completeness**: `buildModerationMatchText()` correctly checks both `msg.Entities` AND `msg.CaptionEntities`
- **Correct nil protection in most modules**: admin.go, bans.go, mute.go, notes.go, captcha.go, devs.go, disabling.go, language.go all use `RequireUser()` correctly
- **Correct nil sender in watchers**: antiflood.go, antispam.go, reports.go, blacklists.go, locks.go all check for nil sender

---

## 5. Test Infrastructure

- **0% handler coverage**: All 7 test files test ONLY utility functions -- zero command/callback/watcher handlers are tested
- **CI quality gates are non-blocking**: gosec (-no-fail), govulncheck (continue-on-error), golangci-lint (--issues-exit-code 0)
- **Coverage threshold**: 15% (effectively meaningless)
- **golangci-lint**: Only `godox` and `dupl` linters enabled. No `errcheck`, `nilaway`, or `staticcheck`
- **Pre-existing lint warnings**: 63 dupl, 4 godox (documented in MEMORY.md)

---

## 6. Bug Priority Matrix

| # | Bug | Severity | Impact | Files |
|---|-----|----------|--------|-------|
| 1 | users.go nil sender panic | HIGH | Crashes ALL message processing | users.go:76-79 |
| 2 | purges.go nil EffectiveUser | HIGH | Crashes purge callbacks | purges.go:149,335,419 |
| 3 | misc.go info nil sender | MEDIUM | Crashes /info command | misc.go:277-284 |
| 4 | misc.go echomsg nil From | MEDIUM | Crashes /tell command | misc.go:79 |
| 5 | 36+ void DB functions | MEDIUM | Silent write failures | 6 db files, 6 module files |
| 6 | i18n config.yml not loadable | MEDIUM | Alt module names broken | i18n/loader.go, modules/helpers.go |
| 7 | Extraction inconsistencies | MEDIUM | Wrong user targeting | extraction/extraction.go |
| 8 | IsUserConnected ctx mutation | MEDIUM | Unexpected chat context | helpers/helpers.go:307 |
| 9 | Fire-and-forget DB goroutines | LOW | Silent error loss | connections.go, helpers.go |
| 10 | HtmlEscape incomplete | LOW | Theoretical attribute injection | helpers/helpers.go |
| 11 | chatlist.txt race condition | LOW | File write race | devs.go:117 |
| 12 | secureIntn infinite loop | LOW | Theoretical infinite loop | captcha.go:240-245 |
| 13 | time.Sleep blocks goroutine | LOW | Goroutine pool waste | misc.go:461 |

---

## 7. Open Questions

- [ ] Does `config.yml` actually contain `alt_names` mappings? If so, the help system's module name resolution is broken.
- [ ] Are there handlers that access `ctx.EffectiveChat` after calling `IsUserConnected` without being aware of the mutation?
- [ ] Is the `google/uuid` dependency in extraction actively used or legacy?
- [ ] Do `db.AddFilter` and `db.RemoveFilter` return errors? Need verification.
- [ ] What is the actual test coverage percentage?
