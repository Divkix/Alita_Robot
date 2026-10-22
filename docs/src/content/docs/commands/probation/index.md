---
title: Probation Commands
description: Complete guide to Probation module commands and features
---

# 📦 Probation Commands

**🕑 Probation**

An opt-in probation for new members: someone who joins after an admin turns it on can't post links or forwards (and, if you choose, media) until they have sent a few real messages or a set time has passed. Their blocked messages are deleted; they are not muted.

Members who were in the group before probation was turned on are never on probation. Admins, approved users and bots are skipped.


### Admin commands:

- `/probation`: Show the probation settings.

- `/probation <on/off>`: Turn probation on or off for members who join from now on.

- `/probation time <duration>`: How long probation lasts, from 1h to 30d. The default is 24h.

- `/probation messages <number>`: How many messages end probation early, from 1 to 100. The default is 5.

- `/probation minlen <number>`: The shortest message, in characters, that counts. The default is 20. A repeated message counts once.

- `/probation block <links,forwards,media>`: What is deleted during probation. The default is links and forwards.

- `/probation notice <on/off>`: Tell the member once, in the group, why a message was removed. Off by default.


Probation is off in every chat until an admin turns it on. Messages are counted only when the bot has Redis; without it, probation ends when its time is up. The bot needs the right to delete messages.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/probation` | Show the probation settings. | ❌ |

## Usage Examples

### Basic Usage

```text
/probation
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.

