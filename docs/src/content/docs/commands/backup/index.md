---
title: Backup Commands
description: Complete guide to Backup module commands and features
---

# 📦 Backup Commands

**📦 Backup & Restore**

Backup and restore your group settings using these commands:

**User Commands:**
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

Commands in this module are available to all users unless otherwise specified.

