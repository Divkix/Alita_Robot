---
title: Approvals Commands
description: Complete guide to Approvals module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Approvals Commands

**👤 Approvals**

Grant trusted members immunity from automated anti-spam measures without making them admins.

**Admin Commands:**

- `/approve <reply/username/mention/userid> [reason]`
- `/unapprove <reply/username/mention/userid>`
- `/approval <reply/username/mention/userid>`
- `/approved`: List all approved users
- `/unapproveall`: Remove all approvals (creator only)

Approved users are exempt from: antiflood, blacklists, locks, CAPTCHA, and antispam.


## Module Aliases

This module can be accessed using the following aliases:

- `approval`
- `approve`
- `unapprove`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/approval` | Show approval status or help for approvals | ❌ |
| `/approve` | Approve a user | ❌ |
| `/approved` | List all approved users | ❌ |
| `/unapprove` | Remove approval for a specified user | ❌ |
| `/unapproveall` | Remove all approvals (creator only) | ❌ |

## Usage Examples

### Basic Usage

```text
/approval
/approve
/approved
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module require administrator or admin-level permissions.

