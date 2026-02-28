---
plan: 02-02
phase: 02-api-reference-and-command-documentation
status: complete
started: 2026-02-27
completed: 2026-02-27
requirements_completed: [DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-07]
---

## Summary

Rewrote the API reference command table with the new 5-column format and added all 13 missing commands. Updated callbacks.md with the versioned codec format.

## What Was Built

- **commands.md full rewrite**: New 5-column format (Command|Description|Permission|Disableable|Aliases), 142 total commands across 25 modules
- **13 previously missing commands added**: help (start, help, donate, about) and devs (stats, addsudo, adddev, remsudo, remdev, chatinfo, chatlist, leavechat, teamusers)
- **Permission column**: Values match handler guards (Everyone, Admin, Owner, Dev/Owner, Team, User/Admin)
- **Alias notation**: All 8 MultiCommand aliases use "Alias of /primary" format
- **callbacks.md codec update**: Replaced old dot-notation with versioned format (namespace|v1|url-encoded-fields), Go code example, backward compatibility note

## Key Decisions

- Module sections organized by category (Administration, Moderation, Content Management, User Tools, Bot Management, Developer, Internal)
- Antispam and Users sections have no tables (0 commands) -- note text with link to module page instead
- 17 commands marked disableable (verified against canonical inventory)

## Commits

1. `docs(02-02): rewrite command reference with 142 commands and update callback codec` (28e578c)

## key-files

### modified
- docs/src/content/docs/api-reference/commands.md
- docs/src/content/docs/api-reference/callbacks.md

## Self-Check: PASSED

- [x] commands.md contains 142 commands in new 5-column format
- [x] All 13 previously missing commands present
- [x] 8 alias rows with "Alias of /primary" notation
- [x] 17 commands marked disableable
- [x] Permission column matches handler guards
- [x] callbacks.md documents versioned codec format
- [x] Backward compatibility note for legacy dot-notation
