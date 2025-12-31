---
title: Captcha Commands
description: Complete guide to Captcha module commands and features
---

# 🔐 Captcha Commands

Protect your group from bots and spammers with CAPTCHA verification!

Force new members to prove they're human by solving a simple challenge before they can send messages.

*Captcha Types:*
× Math: Solve simple arithmetic problems
× Text: Identify text shown in an image

*Admin Commands:*
× /captcha `<on/off>`: Enable or disable captcha verification
× /captchamode `<math/text>`: Set captcha type (math problems or text recognition)
× /captchatime `<1-10>`: Set timeout in minutes (default: 2)
× /captchaaction `<kick/ban/mute>`: Set action for failed verification (default: kick)
× /captchaattempts `<1-10>`: Set maximum verification attempts (default: 3)

When enabled, new members are automatically muted until they complete the captcha.
If they fail or timeout, the configured action is taken.

**How It Works:**
1. When captcha is enabled, new members are automatically muted upon joining
2. A captcha challenge is sent to the new member
3. The user must select the correct answer from the provided options
4. If successful, the user is unmuted and can participate in the group
5. If they fail or timeout, the configured action is taken (kick, ban, or mute)

**Captcha Types:**
- **Math**: Solve simple arithmetic problems (addition, subtraction, multiplication)
- **Text**: Identify text shown in a distorted image

**Pending Messages Feature:**
When a user is completing the captcha, any messages they try to send are stored and deleted. Admins can:
- View what messages a user tried to send using `/captchapending`
- Clear stored messages using `/captchaclear`

This helps identify potential spam attempts before users complete verification.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/captcha` | Enable or disable captcha verification | ❌ |
| `/captchaaction` | Set action for failed verification (default: kick) | ❌ |
| `/captchaclear` | No description available | ❌ |
| `/captchamaxattempts` | No description available | ❌ |
| `/captchamode` | Set captcha type (math problems or text recognition) | ❌ |
| `/captchapending` | No description available | ❌ |
| `/captchatime` | Set timeout in minutes (default: 2) | ❌ |

## Usage Examples

### Basic Usage

```
/captcha
/captchaaction
/captchaclear
```

For detailed command usage, refer to the commands table above.

## Required Permissions

**Bot Requirements:**
- Ban users (for kick/ban actions)
- Restrict members (for muting during verification)
- Delete messages (for cleaning up captcha messages)

**Failure Actions:**
- `kick` - Ban then immediately unban (allows user to rejoin)
- `ban` - Permanently ban the user
- `mute` - Keep the user muted for 24 hours
