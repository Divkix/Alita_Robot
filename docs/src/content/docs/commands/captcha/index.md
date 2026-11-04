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
× /captchajoin `<on/off>`: Verify join requests in private messages (default: off; no argument shows its state)
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

## Join-request verification in PM

Use `/captchajoin on` for groups that require approval to join. This is independent
of `/captcha on`: either check can be enabled on its own. `/captchajoin` shows the
current state; `/captchajoin off` restores the existing auto-approval or admin-button flow.
The `/captcha` status and `/settings` summary also show this setting.

- Applicants receive a private captcha naming the group. It uses `/captchamode`,
  `/captchatime` and `/captchamaxattempts`, with a **5-minute maximum timeout**.
- A correct answer approves the request even when `/autoapprove` is off. Users
  on the group's approved list are approved immediately without a challenge.
- Running out of time or attempts declines the request; `/captchaaction` does not
  apply. The applicant is not kicked, banned or muted for failing a join captcha.
- When `/spamdb` is enabled, its `ban` action bans and declines a listed applicant;
  `mute` declines them; `log` logs the match and still sends the challenge.
- If the bot cannot send the PM, it falls back to `/autoapprove` or the usual
  Approve / Decline / Ban buttons. A failed PM never declines the request.
- Admin buttons are shown only for that fallback. Captcha and spam-database
  outcomes are sent to the configured log channel.
- After approval, the normal welcome still appears. A recent successful join
  captcha skips the regular captcha, so the user does not solve it twice.

Telegram allows the bot to contact applicants for only five minutes, until their
request is processed, and only if another admin has not already contacted them.
The bot needs the **Invite users** permission to receive, approve and decline
requests. Join-request verification does not require restricting members.



## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/captcha` | Toggle captcha verification for new members | ❌ |
| `/captchaaction` | Set action for failed captcha attempts | ❌ |
| `/captchaclear` | Clear pending captcha messages | ❌ |
| `/captchajoin` | Toggle private captcha for join requests; no argument shows its state | ❌ |
| `/captchamaxattempts` | Set maximum captcha verification attempts | ❌ |
| `/captchamode` | Set captcha verification mode | ❌ |
| `/captchapending` | View a pending user's stored messages | ❌ |
| `/captchatime` | Set time limit for captcha verification | ❌ |

## Usage Examples

### Basic Usage

```
/captcha
/captchaaction
/captchaclear
/captchajoin on
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
- Still-valid attempts: the existing challenge is retained and its remaining timeout is rescheduled
- Incomplete attempts that never received a Telegram message are removed and the user's chat permissions are restored

Messages intercepted while verification is pending are deleted. After a
successful answer, the bot sends a summary of their types and count; it does
not replay their original contents.

**Failure Actions:**
- `kick` - Remove the user while allowing them to rejoin
- `ban` - Permanently ban the user
- `mute` - Keep the user muted for 24 hours
