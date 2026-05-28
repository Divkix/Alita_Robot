---
title: Antiraid Commands
description: Complete guide to Antiraid module commands and features
---

# 📦 Antiraid Commands

Protect your group from spam-join attacks with AntiRaid.
### Admin Commands:
/antiraid: Show current state + enable/disable buttons.
/antiraid on: Enable raid mode for configured time.
/antiraid <duration>: Enable for custom duration (e.g. 3h, 30m).
/antiraid off: Disable raid mode immediately.
/raidtime <time>: Set raid duration (default: 6h).
/raidactiontime <time>: Set temp-ban duration (default: 1h).
/autoantiraid <N>: Auto-enable if N+ joins/min.
/autoantiraid off: Disable auto-trigger.


## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/antiraid` | Show current state + enable/disable buttons. | ✅ |
| `/autoantiraid` | Auto-enable if N+ joins/min. | ❌ |
| `/raidactiontime` | Set temp-ban duration (default: 1h). | ❌ |
| `/raidtime` | Set raid duration (default: 6h). | ❌ |

## Usage Examples

### Basic Usage

```
/antiraid
/autoantiraid
/raidactiontime
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.

