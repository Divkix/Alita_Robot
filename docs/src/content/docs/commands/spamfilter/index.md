---
title: Spamfilter Commands
description: Complete guide to Spamfilter module commands and features
---

# 📦 Spamfilter Commands

**🧹 Spam Filter**

An opt-in filter that learns from the spam your admins remove. Reply /spam to a spam message to delete it, ban its sender and save the text as one of this chat's spam samples. With the filter on, a new message that closely matches a sample is caught, even with a changed link, added emoji or look-alike letters.

A text also counts in every chat that uses the filter once two unrelated chats (different owners, no admin in common) have flagged it.

Turning the filter on starts a 7-day dry run: matches are only reported to the log channel, each with a Not spam button, and nothing is deleted. Admins, approved users and members solving a captcha are never checked.


**Admin commands:**

- `/spam`: Reply to a message to delete it, ban its sender and save the text as a spam sample.

- `/spamfilter on/off/dryrun`: Turn the filter on (matches are deleted once the 7-day dry run is over), off, or to dry run only (matches are reported, never deleted). Without an option, show the filter's state.
Use /spamfilter threshold `0.5-1` to set how similar a message must be to a sample: 0.85 by default; lower catches more and makes more mistakes.

- `/spamsamples`: List the newest spam samples with their IDs.

- `/forgetspam ID`: Delete a spam sample.


Samples are kept for 90 days, at most 200 per chat. Unbanning a user banned with /spam, or pressing Not spam in the log channel, marks the text as not spam in this chat.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/forgetspam` | Delete a spam sample. | ❌ |
| `/spam` | Reply to a message to delete it, ban its sender and save the text as a spam sample. | ❌ |
| `/spamfilter` | Turn the filter on (matches are deleted once the 7-day dry run is over), off, or to dry run only (matches are reported, never deleted). Without an option, show the filter's state. | ❌ |
| `/spamsamples` | List the newest spam samples with their IDs. | ❌ |

## Usage Examples

### Basic Usage

```text
/forgetspam
/spam
/spamfilter
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.

