---
title: Anonymous Admin Verification
description: How Alita Robot handles admin commands sent via Telegram's anonymous admin mode.
---

# Anonymous Admin Verification

When a group admin enables "Send as Group" (anonymous admin mode), Telegram sends their messages as `GroupAnonymousBot` (ID `1087968824`). The bot cannot identify which admin sent the command. Alita Robot provides two modes to handle this: automatic trust (AnonAdmin ON) or keyboard verification (AnonAdmin OFF, the default).

## AnonAdmin Modes

### AnonAdmin Mode ON (`/anonadmin on`)

When enabled via admin settings, the bot treats all anonymous admin messages as legitimate admin commands. The command executes immediately without verification.

:::caution[Security trade-off]
AnonAdmin ON is faster but any admin can execute commands without individual accountability. There is no audit trail of which specific admin performed an action.
:::

### AnonAdmin Mode OFF (default)

When AnonAdmin mode is off (the default), the bot uses keyboard verification:

1. The bot stores the original message in Redis with a 20-second TTL.
2. An inline keyboard with a "Prove Admin" button is sent to the chat.
3. Any chat admin can click the button to verify and execute the command.
4. After 20 seconds the cached message is evicted. Nothing runs on a timer: an admin who clicks after that sees the keyboard message edited to "Button expired".

## Verification Flow

The following sequence diagram shows the complete verification process for both modes:

```mermaid
sequenceDiagram
    participant A as Anonymous Admin
    participant B as Bot (Alita)
    participant C as Redis Cache
    participant D as Any Chat Admin

    A->>B: /ban @user (via GroupAnonymousBot)
    B->>B: sender.IsAnonymousAdmin() == true
    alt AnonAdmin mode ON
        B->>B: Treat as admin, execute command
    else AnonAdmin mode OFF (default)
        B->>C: Store original message (TTL: 20s)
        B->>A: Send "Prove Admin" inline keyboard
        D->>B: Clicks "Prove Admin" button
        B->>B: IsUserAdmin(clicker) check
        alt Clicker is NOT admin
            B->>D: Toast: "Need to be admin"
        else Clicker is admin, clicked within 20s
            B->>C: Retrieve original message
            B->>B: Delete keyboard message
            B->>B: Execute original /ban command as clicker
        else Clicker is admin, clicked after 20s
            B->>C: Retrieve original message (cache miss)
            B->>A: Edit keyboard message: "Button expired"
        end
    end
```

## Technical Details

- **Cache key format:** `alita:anonAdmin:{chatId}:{msgId}` with 20-second TTL
- **Callback data encoding:** Versioned codec format `anon_admin|v1|c={chatId}&m={msgId}` only; unversioned or malformed payloads are rejected with "Invalid request." (`alita:anonAdmin:{chatId}:{msgId}` is the Redis cache key above, not a callback format)
- **Command re-dispatch:** After verification, the bot sets `ctx.EffectiveMessage` to the original cached message, clears `SenderChat` (prevents re-triggering anonymous admin detection), replaces `ctx.Update` with a message-only update for the original message (dropping the `CallbackQuery`), sets the effective user to the clicker, extracts the command from the message text or caption, and dispatches through `HandleAnonymousAdmin`, which looks it up in the registry filled by `RegisterAnonymousAdminHandler` (`alita/modules/anonymous_admin_router.go`)
- **Non-admin click handling:** Shows a toast notification ("Need to be admin") without error; does not consume the keyboard

## Supported Commands

The following commands support anonymous admin verification. Only these commands are registered with `RegisterAnonymousAdminHandler`:

| Module | Commands |
|--------|----------|
| Admin | `/promote`, `/demote`, `/title` |
| Bans | `/ban`, `/dban`, `/sban`, `/tban`, `/unban`, `/kick`, `/dkick`, `/skick`, `/restrict`, `/unrestrict` |
| Mutes | `/mute`, `/smute`, `/dmute`, `/tmute`, `/unmute` |
| Pins | `/pin`, `/unpin`, `/permapin`, `/unpinall` |
| Purges | `/purge`, `/del`, `/purgefrom`, `/purgeto` |
| Warns | `/warn`, `/swarn`, `/dwarn` |

## Troubleshooting

:::tip[Common issues]
- **"Button expired"** -- An admin clicked the button after the 20-second cache entry expired. The admin must re-send the command.
- **"Need to be admin"** -- A non-admin user clicked the "Prove Admin" button. Only actual chat admins can verify.
- **Commands not executing in anonymous mode** -- The command may not be in the supported commands list above. Only commands registered with `RegisterAnonymousAdminHandler` are supported.
:::

## Related Pages

- [Handler Group Precedence](/architecture/handler-groups) -- How message watchers interact with command processing
- [Request Flow](/architecture/request-flow) -- Full update processing pipeline
