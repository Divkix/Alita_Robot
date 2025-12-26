---
title: Filters Commands
description: Complete guide to Filters module commands and features
---

# 🔍 Filters Commands

Filters are case insensitive; every time someone says your trigger words, Alita will reply something else! This can be used to create your commands, if desired.

Commands:
- /filter <trigger> <reply>: Every time someone says trigger, the bot will reply with sentence. For multiple word filters, quote the trigger.
- /filters: List all chat filters.
- /stop <trigger>: Stop the bot from replying to trigger.
- /stopall: Stop ALL filters in the current chat. This action cannot be undone.

Examples:
- Set a filter:
-> /filter hello Hello there! How are you?
- Set a multiword filter:
-> /filter hello friend Hello back! Long time no see!
- Set a filter that can only be used by admins:
-> /filter example This filter won't  happen if a normal user says it {admin}
- To save a file, image, gif, or any other attachment, simply reply to the file with:
-> /filter trigger

## Module Aliases

This module can be accessed using the following aliases:

- `filter`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addfilter` | No description available | ❌ |
| `/filter` | Every time someone says trigger, the bot will reply with sentence. For multiple word filters, quote the trigger. | ❌ |
| `/filters` | List all chat filters. | ✅ |
| `/removefilter` | No description available | ❌ |
| `/rmfilter` | No description available | ❌ |
| `/stop` | Stop the bot from replying to trigger. | ❌ |
| `/stopall` | Stop ALL filters in the current chat. This action cannot be undone. | ❌ |

## Usage Examples

### Basic Usage

```
/addfilter
/filter
/filters
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.
