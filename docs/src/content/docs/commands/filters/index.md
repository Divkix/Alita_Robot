---
title: Filters Commands
description: Complete guide to Filters module commands and features
---

# 🔍 Filters Commands

Filters are case insensitive; every time someone says your trigger words, Alita will reply something else! This can be used to create your commands, if desired.

## Quick Start

Commands:
- `/filter <trigger> <reply>`: Every time someone says trigger, the bot will reply with sentence. For multiple word filters, quote the trigger.
- `/filters`: List all chat filters.
- `/stop <trigger>`: Stop the bot from replying to trigger.
- `/stopall`: Stop ALL filters in the current chat. This action cannot be undone.

Examples:
- Set a filter:
  → `/filter hello Hello there! How are you?`
- Set a multiword filter:
  → `/filter "hello friend" Hello back! Long time no see!`
- Set a filter that can only be used by admins:
  → `/filter example This filter won't happen if a normal user says it {admin}`
- To save a file, image, gif, or any other attachment, simply reply to the file with:
  → `/filter trigger`

## Module Aliases

This module can be accessed using the following aliases:

- `filter`

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/addfilter` | Alias for `/filter` | ❌ |
| `/filter` | Every time someone says trigger, the bot will reply with sentence. For multiple word filters, quote the trigger. | ❌ |
| `/filters` | List all chat filters. | ✅ |
| `/removefilter` | Alias for `/stop` | ❌ |
| `/rmfilter` | Alias for `/stop` | ❌ |
| `/stop` | Stop the bot from replying to trigger. | ❌ |
| `/stopall` | Stop ALL filters in the current chat. This action cannot be undone. | ❌ |

## Required Permissions

| Command | Required Permission |
|---------|---------------------|
| `/filter`, `/addfilter` | Admin with Change Info permission |
| `/stop`, `/rmfilter`, `/removefilter` | Admin with Change Info permission |
| `/stopall` | Chat Owner only |
| `/filters` | Available to all users |

## Limits

- Maximum 150 filters per chat
- Maximum 100 characters per filter keyword
- Filter keywords are stored in lowercase

## Advanced Features

### Random Responses

Use `%%%` to separate multiple responses. One will be picked randomly:

```
/filter hello Hi there!%%%Hello!%%%Hey, how are you?
```

### Media Filters

Reply to any media (photo, video, document, sticker, etc.) with `/filter trigger` to create a filter that sends that media.

### Noformat Mode

Admins can view the raw filter content (including formatting codes) by adding `noformat` after the trigger:

```
hello noformat
```

This is useful for debugging filters or seeing button configurations.

### Filter Buttons

You can add inline buttons to your filters using the button syntax:

```
/filter hello Hello! Check out these links:
[Button 1](buttonurl:https://example.com)
[Button 2](buttonurl:https://example2.com)
```

## Technical Details

### How Filters Work

1. When a message is received, the bot fetches all filters for the chat using an optimized cached query
2. The Aho-Corasick algorithm efficiently matches all filter keywords against the message text
3. The first matching filter triggers a response
4. Filters support text, media, and button responses

### Overwrite Confirmation

When you try to add a filter that already exists, the bot will ask for confirmation before overwriting. The confirmation expires after 5 minutes.

### Cache Behavior

- Filter lists are cached for 15 minutes per chat
- Cache is automatically invalidated when filters are added or removed

## Usage Examples

### Basic Usage

```
/filter hello Hello there! Welcome to the group!
/filters
/stop hello
```

### Multi-word Triggers

```
/filter "good morning" Good morning to you too! ☀️
/stop "good morning"
```

### Filter with Buttons

```
/filter rules Please read our rules:
[Rules](buttonurl:https://example.com/rules)
[FAQ](buttonurl:https://example.com/faq)
```

For detailed command usage, refer to the commands table above.
