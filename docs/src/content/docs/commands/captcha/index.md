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

Commands in this module are available to all users unless otherwise specified.
