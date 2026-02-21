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

**Advanced Features:**

**Random Responses:**
Use `%%%` to separate multiple responses. One will be picked randomly:
`/filter hello Hi there!%%%Hello!%%%Hey, how are you?`

**Media Filters:**
Reply to any media (photo, video, document, sticker, etc.) with `/filter trigger` to create a filter that sends that media.

**Noformat Mode:**
Admins can view the raw filter content (including formatting codes) by adding `noformat` after the trigger:
`hello noformat`
This is useful for debugging filters or seeing button configurations.

**Filter Buttons:**
You can add inline buttons to your filters:
`/filter hello Hello! Check out these links:
[Button 1](buttonurl:https://example.com)
[Button 2](buttonurl:https://example2.com)`


## Module Aliases

This module can be accessed using the following aliases:

- `filter`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addfilter` | Add a filter. Alias for /filter. | ❌ |
| `/filter` | Every time someone says trigger, the bot will reply with sentence. For multiple word filters, quote the trigger. | ❌ |
| `/filters` | List all chat filters. | ✅ |
| `/removefilter` | Stop a filter. Alias for /stop. | ❌ |
| `/rmfilter` | Stop a filter. Alias for /stop. | ❌ |
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

Admin commands require admin with 'Change Group Info' permission. `/stopall` requires chat owner. `/filters` is available to all users.

## Technical Notes

**Limits:** Maximum 150 filters per chat. Filter keywords cannot exceed 100 characters.
