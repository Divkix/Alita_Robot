---
title: Captcha Commands
description: Complete guide to Captcha module commands and features
---

# Captcha Commands

Protect your group from bots and spammers with CAPTCHA verification!

Force new members to prove they're human by solving a simple challenge before they can send messages.

## Captcha Types

- **Math**: Solve simple arithmetic problems (addition, subtraction, multiplication)
- **Text**: Identify text shown in a distorted image

## How It Works

1. When captcha is enabled, new members are automatically muted upon joining
2. A captcha challenge is sent to the new member
3. The user must select the correct answer from the provided options
4. If successful, the user is unmuted and can participate in the group
5. If they fail or timeout, the configured action is taken (kick, ban, or mute)

## Admin Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/captcha <on/off>` | Enable or disable captcha verification | No |
| `/captchamode <math/text>` | Set captcha type (math problems or text recognition) | No |
| `/captchatime <1-10>` | Set timeout in minutes (default: 2) | No |
| `/captchaaction <kick/ban/mute>` | Set action for failed verification (default: kick) | No |
| `/captchamaxattempts <1-10>` | Set maximum verification attempts (default: 3) | No |
| `/captchapending <user>` | View pending messages a user tried to send before verification | No |
| `/captchaclear <user>` | Clear stored pending messages for a user | No |

## Usage Examples

### Enable Captcha

```
/captcha on
```

### Configure Captcha Settings

```
/captchamode math
/captchatime 5
/captchaaction ban
/captchamaxattempts 3
```

### View Current Settings

```
/captcha
```

### Manage Pending Messages

```
/captchapending @username
/captchaclear @username
```

## Pending Messages Feature

When a user is completing the captcha, any messages they try to send are stored and deleted. Admins can:

- View what messages a user tried to send using `/captchapending`
- Clear stored messages using `/captchaclear`

This helps identify potential spam attempts before users complete verification.

## Required Permissions

All captcha commands require **admin permissions** to use. The bot also requires admin permissions with the ability to restrict members.

### Bot Requirements

- Ban users (for kick/ban actions)
- Restrict members (for muting during verification)
- Delete messages (for cleaning up captcha messages)

## Failure Actions

| Action | Behavior |
|--------|----------|
| `kick` | Ban then immediately unban (allows user to rejoin) |
| `ban` | Permanently ban the user |
| `mute` | Keep the user muted for 24 hours |
