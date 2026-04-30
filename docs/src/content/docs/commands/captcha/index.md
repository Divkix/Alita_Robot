---
title: Captcha Commands
description: Complete guide to Captcha module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

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
× /captchamaxattempts `<1-10>`: Set maximum verification attempts (default: 3)

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
| `/captcha` | Toggle captcha verification for new members | ❌ |
| `/captchaaction` | Set action for failed captcha attempts | ❌ |
| `/captchaclear` | Clear pending captcha messages | ❌ |
| `/captchamaxattempts` | Set maximum captcha verification attempts | ❌ |
| `/captchamode` | Set captcha verification mode | ❌ |
| `/captchapending` | View pending captcha verifications | ❌ |
| `/captchatime` | Set time limit for captcha verification | ❌ |

## Usage Examples

### Basic Usage

```
/captcha
/captchaaction
/captchaclear
```

For detailed command usage, refer to the commands table above.

## Module Aliases

> These are help-menu module names, not command aliases.

This module can be accessed using the following aliases:

- `captcha`

## Required Permissions

**Bot Requirements:**
- Ban users (for kick/ban actions)
- Restrict members (for muting during verification)
- Delete messages (for cleaning up captcha messages)

**Captcha Refresh:**
- Available for image-based captchas (math image and text modes)
- Users can refresh up to **3 times** with a **5-second cooldown** between refreshes
- Refreshes generate a new captcha image without resetting the attempt counter

**Orphaned Captcha Recovery:**
On bot restart, any pending captcha attempts are automatically processed:
- Expired attempts: failure action is applied (kick/ban/mute)
- Still-valid attempts: captcha message is deleted, user must rejoin to get a new challenge

**Failure Actions:**
- `kick` - Ban then immediately unban (allows user to rejoin)
- `ban` - Permanently ban the user
- `mute` - Keep the user muted for 24 hours
