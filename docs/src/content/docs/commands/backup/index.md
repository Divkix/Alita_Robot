---
title: Backup Commands
description: Complete guide to Backup module commands and features
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# 📦 Backup Commands

**📦 Backup & Restore**

Backup and restore your group settings using these commands:

**Admin Commands:**
- /export - Export all group settings to a JSON file
- /export notes filters rules - Export specific modules only

**Creator Commands:**
- /import - Reply to a backup file to restore settings
- /import notes filters - Import only specific modules
- /reset - Reset all settings to default
- /reset warnings locks - Reset specific modules only

**Rate Limits:**
- Export: 1 per 5 minutes
- Import: 1 per 10 minutes
- Reset: 1 per hour


## Module Aliases

This module can be accessed using the following aliases:

- `export`
- `import`
- `reset`

## Supported Data

Backups cover 17 modules: admin, antiflood, antiraid, approvals, blacklists,
captcha, connections, disabling, filters, greetings, locks, notes, pins,
reactions, reports, rules, and warns. Import and reset are transactional: if one
selected module fails, no selected module is changed.

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/export` | Export all group settings to a JSON file | ✅ |
| `/import` | Reply to a backup file to restore settings | ✅ |
| `/reset` | Reset all settings to default | ❌ |

## Usage Examples

### Basic Usage

```
/export
/import
/reset
```

For detailed command usage, refer to the commands table above.

## Required Permissions

- `/export` — Requires **Admin** (`RequireUserAdmin` + `RequireBotAdmin`).
  Rate limited: 1 per 5 minutes.
- `/import` — Requires **Group Creator** (`RequireUserOwner`).
  Rate limited: 1 per 10 minutes.
- `/reset` — Requires **Group Creator** (`RequireUserOwner`).
  Rate limited: 1 per hour.

## Confirmation Flow

Both `/import` and `/reset` require explicit confirmation before executing:

1. After parsing the backup file (or reset modules), the bot shows a message
   listing which modules will be affected.
2. An **inline keyboard** with Confirm/Cancel buttons is attached:
   - **Confirm** — Executes the operation immediately.
   - **Cancel** — Cleans up pending state and aborts.
3. Only the group creator can click either button; non-creators see an alert.

## Version Compatibility

The current backup format is `1.1`. Imports also accept legacy `1.0` files while
preserving notes settings and user warning rows that `1.0` did not export.
Unsupported versions are rejected before the confirmation step.
