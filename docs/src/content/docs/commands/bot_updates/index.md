---
title: Bot Updates
description: Bot event handling and anonymous admin verification
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 🤖 Bot Updates

This module handles bot lifecycle events and anonymous admin verification. It has no user-facing commands — all functionality is automatic or triggered by other modules.

## What This Module Does

### Bot Group Join Handling

When the bot is added to a chat:

1. **Non-supergroup groups**: The bot sends a message asking the group to be converted to a supergroup, then leaves.
2. **Channels**: The join is ignored. The bot stays in the channel without posting, so the channel can serve as a log channel (see `/setlog`).
3. **Supergroups**: The bot sends a welcome message and remains in the chat.

This handler runs at **group -1**, after the captcha (-10), antiraid (-5) and admin-cache (-2) handlers and before the default command group (0), so the bot leaves unsupported chats before command handlers run.

### Admin Cache Auto-Update

When a chat member's admin status changes, this module automatically invalidates and reloads the admin cache for that chat. This ensures permission checks always use fresh data. This runs for `ChatMember` updates (group 0) and for the bot's own `MyChatMember` updates (group -2).

### Anonymous Admin Verification

When an anonymous admin (GroupAnonymousBot) sends a command, the bot cannot directly identify the real user. This module provides a verification flow:

1. Bot detects anonymous sender and stores the original message in cache (20s TTL)
2. Bot sends a "Click to prove admin" button to the chat
3. The clicking user is checked against the chat's admin list (admin cache, with a Telegram lookup on a cache miss)
4. If verified, the original command is executed with the real user's context
5. If an admin presses the button after the 20s cache entry has expired, the prompt is replaced with the "This button has expired, Please use the command again." message (`bot_updates_button_expired`) and the command does not run; the admin must send it again

**Supported commands for anonymous admin verification:**

- Admin: `/promote`, `/demote`, `/title`, `/setgtitle`, `/setgpic`, `/setgdesc`
- Bans: `/ban`, `/dban`, `/sban`, `/tban`, `/unban`, `/kick`, `/dkick`, `/skick`, `/restrict`, `/unrestrict`
- Mutes: `/mute`, `/smute`, `/dmute`, `/tmute`, `/unmute`
- Pins: `/pin`, `/unpin`, `/permapin`, `/unpinall`
- Purges: `/purge`, `/del`, `/purgefrom`, `/purgeto`
- Warns: `/warn`, `/swarn`, `/dwarn`

## Available Commands

This module has no user-facing commands. All functionality is automatic or triggered via callback buttons during anonymous admin verification.

## Technical Notes

**Handler Registration:**

| Handler | Type | Group | Filter | Return |
|---------|------|-------|--------|--------|
| `adminCacheAutoUpdate` | `MyChatMember` | -2 | Bot's own admin status change | `ContinueGroups` |
| `botJoinedGroup` | `MyChatMember` | -1 | Bot joined chat | `EndGroups` for private chats and basic groups (leaves basic groups); `ContinueGroups` for supergroups |
| `adminCacheAutoUpdate` | `ChatMember` | 0 | Admin status change | `ContinueGroups` |
| `verifyAnonymousAdmin` | `CallbackQuery` | 0 | `anon_admin` prefix | `EndGroups` |

**Security Notes:**
- Anonymous admin callbacks re-check that the clicking user is an admin of the chat encoded in the button (via `chat_status.IsUserAdmin`): the admin cache answers when it holds the chat's admin list, and Telegram is queried only when it does not. `adminCacheAutoUpdate` invalidates and reloads that cache on admin status changes
- Callback data uses the versioned callback codec (`anon_admin|v1|c={chatId}&m={msgId}`); any other payload is rejected with "Invalid request."
- Cached anonymous admin messages expire after 20 seconds to prevent stale executions

## Related Pages

- [Admin Commands](/commands/admin/) - Commands that support anonymous admin verification
- [Handler Group Precedence](/architecture/handler-groups) - Where this module fits in the handler pipeline
- [Anonymous Admin](/architecture/anonymous-admin) - Deep dive into anonymous admin handling
