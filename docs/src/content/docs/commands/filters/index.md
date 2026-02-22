---
title: Filters Commands
description: Complete guide to Filters module commands and features
---

# Filters Commands

Filters are case insensitive; every time someone says your trigger words, Alita will reply something else! This can be used to create your own custom commands if desired.

:::caution[Admin Permissions Required]
Admin commands require admin with "Change Group Info" permission. `/stopall` requires chat owner. `/filters` is available to all users.
:::

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/filter` | Every time someone says the trigger, the bot will reply with the given text. For multiple word filters, quote the trigger. | No |
| `/addfilter` | Add a filter. Alias for /filter. | No |
| `/filters` | List all chat filters. | Yes |
| `/stop` | Stop the bot from replying to a trigger. | No |
| `/removefilter` | Stop a filter. Alias for /stop. | No |
| `/rmfilter` | Stop a filter. Alias for /stop. | No |
| `/stopall` | Stop ALL filters in the current chat. This action cannot be undone. | No |

## Usage Examples

```text
/filter hello Hello there! How are you?
/filter "hello friend" Hello back! Long time no see!
/filters                                          # List all filters
/stop hello                                       # Remove a filter
```

:::tip[Admin-Only Filters]
Set a filter that can only be triggered by admins:

```text
/filter example This filter won't trigger for normal users {admin}
```
:::

:::tip[Media Filters]
Reply to any media (photo, video, document, sticker, etc.) with `/filter trigger` to create a filter that sends that media when triggered.
:::

## Advanced Features

### Random Responses

Use `%%%` to separate multiple responses. One will be picked randomly:

```text
/filter hello Hi there!%%%Hello!%%%Hey, how are you?
```

### Noformat Mode

Admins can view the raw filter content (including formatting codes) by adding `noformat` after the trigger:

```text
hello noformat
```

This is useful for debugging filters or seeing button configurations.

### Filter Buttons

You can add inline buttons to your filters:

```text
/filter hello Hello! Check out these links:
[Button 1](buttonurl:https://example.com)
[Button 2](buttonurl:https://example2.com)
```

## Module Aliases

This module can be accessed using the following aliases:
`filter`

## Required Permissions

**Bot Permissions Required:**
- Delete messages (for admin-only filter enforcement)

:::note[Filter Limits]
Maximum 150 filters per chat. Filter keywords cannot exceed 100 characters.
:::
