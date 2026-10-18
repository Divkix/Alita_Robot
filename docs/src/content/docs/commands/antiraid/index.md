---
title: Antiraid Commands
description: Complete guide to Antiraid module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Antiraid Commands

Protect your group from spam-join attacks with AntiRaid.
## Admin Commands:
/antiraid: Show current state + enable/disable buttons.
/antiraid on: Enable raid mode for configured time.
/antiraid <duration>: Enable for custom duration (e.g. 90s, 30m, 3h, 1w). A unit (s, m, h, d or w) is required.
/antiraid off: Disable raid mode immediately.
/raidtime <time>: Set raid duration (default: 6h). Supports s, m, h, d, w (e.g. 90s, 30m, 6h, 1d, 1w); a bare number is rejected.
/raidactiontime <time>: Set temp-ban duration (default: 1h). Supports s, m, h, d, w (e.g. 90s, 30m, 1h, 1d); a bare number is rejected.
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

```text
/antiraid
/autoantiraid
/raidactiontime
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module require **admin permissions** in the group.

