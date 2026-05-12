---
title: Bot Updates
description: Bot event handling and anonymous admin verification
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🤖 Bot Updates

This module handles bot lifecycle events and anonymous admin verification. It has no user-facing commands — all functionality is automatic or triggered by other modules.

## What This Module Does

### Bot Group Join Handling

When the bot is added to a group or channel:

1. **Non-supergroup groups**: The bot sends a message asking the group to be converted to a supergroup, then leaves.
2. **Channels**: The bot leaves immediately (channels are not supported).
3. **Supergroups**: The bot sends a welcome message and remains in the chat.

This handler runs at **group -1** (before all other handlers) to ensure the bot is in a valid chat before any other processing occurs.

### Admin Cache Auto-Update

When a chat member's admin status changes, this module automatically invalidates and reloads the admin cache for that chat. This ensures permission checks always use fresh data.

### Anonymous Admin Verification

When an anonymous admin (GroupAnonymousBot) sends a command, the bot cannot directly identify the real user. This module provides a verification flow:

1. Bot detects anonymous sender and stores the original message in cache (20s TTL)
2. Bot sends a "Verify Admin" button to the chat
3. Clicking admin is verified against the actual admin list
4. If verified, the original command is executed with the real user's context
5. If the button expires (20s), the command is silently dropped

**Supported commands for anonymous admin verification:**

- Admin: `/promote`, `/demote`, `/title`
- Bans: `/ban`, `/dban`, `/sban`, `/tban`, `/unban`, `/restrict`, `/unrestrict`
- Mutes: `/mute`, `/smute`, `/dmute`, `/tmute`, `/unmute`
- Pins: `/pin`, `/unpin`, `/permapin`, `/unpinall`
- Purges: `/purge`, `/del`
- Warns: `/warn`, `/swarn`, `/dwarn`

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `botupdates`

## Available Commands

This module has no user-facing commands. All functionality is automatic or triggered via callback buttons during anonymous admin verification.

## Technical Notes

**Handler Registration:**

| Handler | Type | Group | Filter | Return |
|---------|------|-------|--------|--------|
| `botJoinedGroup` | `MyChatMember` | -1 | Bot joined chat | `EndGroups` (leaves non-supergroups) |
| `adminCacheAutoUpdate` | `ChatMember` | 0 | Admin status change | `ContinueGroups` |
| `verifyAnonymousAdmin` | `CallbackQuery` | 0 | `anon_admin` prefix | `EndGroups` |

**Security Notes:**
- Anonymous admin callbacks are verified against the live admin list, not just the cache
- Callback data uses the versioned callback codec (`anon_admin|v1|...`) with legacy fallback support
- Cached anonymous admin messages expire after 20 seconds to prevent stale executions

## Related Pages

- [Admin Commands](/commands/admin/) - Commands that support anonymous admin verification
- [Handler Group Precedence](/architecture/handler-groups) - Where this module fits in the handler pipeline
- [Anonymous Admin](/architecture/anonymous-admin) - Deep dive into anonymous admin handling
