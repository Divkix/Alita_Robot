---
title: Captcha Commands
description: Complete guide to Captcha module commands and features
---

# Captcha Commands

Protect your group from bots and spammers with CAPTCHA verification! Force new members to prove they're human by solving a simple challenge before they can send messages.

:::caution[Admin Permissions Required]
All commands in this module require admin permissions.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/captcha` | Enable or disable captcha verification. | No |
| `/captchamode` | Set captcha type (math problems or text recognition). | No |
| `/captchatime` | Set timeout in minutes (default: 2, range: 1-10). | No |
| `/captchaaction` | Set action for failed verification (default: kick). | No |
| `/captchamaxattempts` | Set maximum verification attempts (default: 3, range: 1-10). | No |
| `/captchapending` | View messages a user tried to send during captcha verification. | No |
| `/captchaclear` | Clear stored pending messages for a user. | No |

## Usage Examples

```text
/captcha on               # Enable captcha
/captchamode math         # Use math problems
/captchatime 3            # 3 minute timeout
/captchaaction ban        # Ban users who fail
/captchamaxattempts 5     # Allow 5 attempts
```

## How It Works

1. When captcha is enabled, new members are automatically muted upon joining
2. A captcha challenge is sent to the new member
3. The user must select the correct answer from the provided options
4. If successful, the user is unmuted and can participate in the group
5. If they fail or timeout, the configured action is taken (kick, ban, or mute)

## Captcha Types

| Type | Description |
|------|-------------|
| **Math** | Solve simple arithmetic problems (addition, subtraction, multiplication) |
| **Text** | Identify text shown in a distorted image |

## Failure Actions

| Action | Behavior |
|--------|----------|
| `kick` | Ban then immediately unban (allows user to rejoin) **(default)** |
| `ban` | Permanently ban the user |
| `mute` | Keep the user muted for 24 hours |

:::note[Pending Messages Feature]
When a user is completing the captcha, any messages they try to send are stored and deleted. Admins can view what messages a user tried to send using `/captchapending` and clear them with `/captchaclear`. This helps identify potential spam attempts before users complete verification.
:::

:::tip[Captcha + Greetings Integration]
When the Captcha module is enabled alongside the [Greetings module](/commands/greetings/), the welcome message is delayed until the user successfully completes the captcha.
:::

## Required Permissions

**Bot Permissions Required:**
- Ban users (for kick/ban actions)
- Restrict members (for muting during verification)
- Delete messages (for cleaning up captcha messages)
