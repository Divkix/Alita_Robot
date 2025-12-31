---
title: Database Schema
description: Complete reference of the PostgreSQL database schema
---

# 🗄️ Database Schema

This page documents the complete PostgreSQL database schema for Alita Robot.

## Overview

- **Total Tables**: 26
- **Database Type**: PostgreSQL
- **ORM**: GORM
- **Migration Tool**: golang-migrate

## Design Patterns

### Surrogate Key Pattern

All tables use an auto-incremented `id` field as the primary key (internal identifier), while external identifiers like `user_id` and `chat_id` (Telegram IDs) are stored with unique constraints.

**Benefits:**

- Decouples internal schema from external systems
- Provides stability if external IDs change
- Simplifies GORM operations with consistent integer primary keys
- Better performance for joins and indexing

## Tables

### `admin`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('admin_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `anon_admin` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_admin_settings_chat

#### Foreign Keys

- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)

### `antiflood_settings`

:::note[Application Default]
While the database schema shows a default of `5` for `flood_limit`, the application returns `0` (disabled) when no record exists for a chat. This means antiflood is **disabled by default** until explicitly configured.
:::

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('antiflood_settings_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `limit` | `BIGINT` | ✅ | `5` | — |
| `action` | `TEXT` | ✅ | `'mute'::text` | — |
| `mode` | `TEXT` | ✅ | `'mute'::text` | — |
| `delete_antiflood_message` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |
| `flood_limit` | `BIGINT` | ✅ | `5` | — |

#### Indexes

- idx_antiflood_chat_active
- idx_antiflood_chat_flood_active

### `blacklists`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('blacklists_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `word` | `TEXT` | ❌ | — | — |
| `action` | `TEXT` | ✅ | `'warn'::text` | — |
| `reason` | `TEXT` | ✅ | — | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_blacklists_chat_word_optimized

#### Foreign Keys

- chat_id -> chats(chat_id)
- user_id -> users(user_id)
- user_id -> users(user_id)

### `captcha_attempts`

Tracks active captcha verification attempts for users joining groups.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `SERIAL` | ❌ | — | PRIMARY KEY |
| `user_id` | `BIGINT` | ❌ | — | FK → users(user_id) |
| `chat_id` | `BIGINT` | ❌ | — | FK → chats(chat_id) |
| `answer` | `VARCHAR(255)` | ❌ | — | — |
| `attempts` | `INTEGER` | ✅ | `0` | — |
| `message_id` | `BIGINT` | ✅ | — | — |
| `refresh_count` | `INTEGER` | ✅ | `0` | — |
| `expires_at` | `TIMESTAMP` | ❌ | — | — |
| `created_at` | `TIMESTAMP` | ✅ | `CURRENT_TIMESTAMP` | — |
| `updated_at` | `TIMESTAMP` | ✅ | `CURRENT_TIMESTAMP` | — |

#### Indexes

- idx_captcha_user_chat (user_id, chat_id)
- idx_captcha_expires_at (expires_at)

#### Foreign Keys

- user_id → users(user_id) ON DELETE CASCADE
- chat_id → chats(chat_id) ON DELETE CASCADE

### `captcha_muted_users`

Tracks users who failed captcha with "mute" action and need to be automatically unmuted after a period.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | ❌ | — | PRIMARY KEY |
| `user_id` | `BIGINT` | ❌ | — | FK → users(user_id) |
| `chat_id` | `BIGINT` | ❌ | — | FK → chats(chat_id) |
| `unmute_at` | `TIMESTAMP` | ❌ | — | — |
| `created_at` | `TIMESTAMP` | ✅ | `NOW()` | — |

#### Indexes

- idx_captcha_muted_user_chat (user_id, chat_id)
- idx_captcha_unmute_at (unmute_at)

#### Foreign Keys

- user_id → users(user_id) ON DELETE CASCADE
- chat_id → chats(chat_id) ON DELETE CASCADE

### `captcha_settings`

Stores per-chat captcha configuration settings.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `SERIAL` | ❌ | — | PRIMARY KEY |
| `chat_id` | `BIGINT` | ❌ | — | UNIQUE, FK → chats(chat_id) |
| `enabled` | `BOOLEAN` | ✅ | `FALSE` | — |
| `captcha_mode` | `VARCHAR(10)` | ✅ | `'math'` | CHECK (math, text) |
| `timeout` | `INTEGER` | ✅ | `2` | CHECK (1-10 minutes) |
| `failure_action` | `VARCHAR(10)` | ✅ | `'kick'` | CHECK (kick, ban, mute) |
| `max_attempts` | `INTEGER` | ✅ | `3` | CHECK (1-10 attempts) |
| `created_at` | `TIMESTAMP` | ✅ | `CURRENT_TIMESTAMP` | — |
| `updated_at` | `TIMESTAMP` | ✅ | `CURRENT_TIMESTAMP` | — |

#### Indexes

- uk_captcha_settings_chat_id (chat_id) UNIQUE

#### Foreign Keys

- chat_id → chats(chat_id) ON DELETE CASCADE

### `channels`

Stores information about Telegram channels that interact with the bot, including channel metadata for lookup functionality.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('channels_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | UNIQUE |
| `channel_id` | `BIGINT` | ✅ | — | — |
| `channel_name` | `TEXT` | ✅ | — | Channel display name |
| `username` | `TEXT` | ✅ | — | Channel @username |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_channels_chat_update (chat_id)
- idx_channels_username (username) - for username lookups

#### Notes

The `channel_name` and `username` columns enable:
- Looking up channels by @username via `GetChannelIdByUserName()`
- Retrieving channel display names via `GetChannelInfoById()`

### `chat_users`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `chat_id` | `BIGINT` | ❌ | — | — |
| `user_id` | `BIGINT` | ❌ | — | — |

#### Indexes

- idx_chat_users_user_id
- idx_chat_users_chat_id

#### Foreign Keys

- chat_id -> chats(chat_id)
- user_id -> users(user_id)

### `chats`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('chats_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `chat_name` | `TEXT` | ✅ | — | — |
| `language` | `TEXT` | ✅ | — | — |
| `users` | `JSONB` | ✅ | — | — |
| `is_inactive` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_chats_chat_id_active
- idx_chats_covering
- idx_chats_users_gin
- idx_chats_inactive
- idx_chats_last_activity
- idx_chats_activity_status

#### Foreign Keys

- chat_id -> chats(chat_id)
- user_id -> users(user_id)
- chat_id -> chats(chat_id)
- user_id -> users(user_id)

### `connection`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('connection_id_seq'::regclass)` | — |
| `user_id` | `BIGINT` | ❌ | — | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `connected` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_connection_user_id
- idx_connection_chat_id

#### Foreign Keys

- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)

### `connection_settings`

Stores per-chat settings for the Connections module, controlling whether users can remotely connect to the chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('connection_settings_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | UNIQUE, FK → chats(chat_id) |
| `allow_connect` | `BOOLEAN` | ✅ | `true` | Controls if users can connect |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)

### `devs`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('devs_id_seq'::regclass)` | — |
| `user_id` | `BIGINT` | ❌ | — | — |
| `is_dev` | `BOOLEAN` | ✅ | `false` | — |
| `dev` | `BOOLEAN` | ✅ | `false` | — |
| `sudo` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)

### `disable`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('disable_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `command` | `TEXT` | ❌ | — | — |
| `disabled` | `BOOLEAN` | ✅ | `true` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)

### `filters`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('filters_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `keyword` | `TEXT` | ❌ | — | — |
| `filter_reply` | `TEXT` | ✅ | — | — |
| `msgtype` | `BIGINT` | ✅ | — | — |
| `fileid` | `TEXT` | ✅ | — | — |
| `nonotif` | `BOOLEAN` | ✅ | `false` | — |
| `filter_buttons` | `JSONB` | ✅ | — | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_filters_chat_optimized

#### Foreign Keys

- chat_id -> chats(chat_id)

### `greetings`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('greetings_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `clean_service_settings` | `BOOLEAN` | ✅ | `false` | — |
| `welcome_clean_old` | `BOOLEAN` | ✅ | `false` | — |
| `welcome_last_msg_id` | `BIGINT` | ✅ | — | — |
| `welcome_enabled` | `BOOLEAN` | ✅ | `true` | — |
| `welcome_text` | `TEXT` | ✅ | — | — |
| `welcome_file_id` | `TEXT` | ✅ | — | — |
| `welcome_type` | `BIGINT` | ✅ | — | — |
| `welcome_btns` | `JSONB` | ✅ | — | — |
| `goodbye_clean_old` | `BOOLEAN` | ✅ | `false` | — |
| `goodbye_last_msg_id` | `BIGINT` | ✅ | — | — |
| `goodbye_enabled` | `BOOLEAN` | ✅ | `true` | — |
| `goodbye_text` | `TEXT` | ✅ | — | — |
| `goodbye_file_id` | `TEXT` | ✅ | — | — |
| `goodbye_type` | `BIGINT` | ✅ | — | — |
| `goodbye_btns` | `JSONB` | ✅ | — | — |
| `auto_approve` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_greetings_chat_enabled

#### Foreign Keys

- chat_id -> chats(chat_id)
- user_id -> users(user_id)
- chat_id -> chats(chat_id)

### `locks`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('locks_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `lock_type` | `TEXT` | ❌ | — | — |
| `locked` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_locks_chat_lock_lookup
- idx_locks_covering

#### Foreign Keys

- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)

### `notes`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('notes_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `note_name` | `TEXT` | ❌ | — | — |
| `note_content` | `TEXT` | ✅ | — | — |
| `file_id` | `TEXT` | ✅ | — | — |
| `msg_type` | `BIGINT` | ✅ | — | — |
| `buttons` | `JSONB` | ✅ | — | — |
| `admin_only` | `BOOLEAN` | ✅ | `false` | — |
| `private_only` | `BOOLEAN` | ✅ | `false` | — |
| `group_only` | `BOOLEAN` | ✅ | `false` | — |
| `web_preview` | `BOOLEAN` | ✅ | `true` | — |
| `is_protected` | `BOOLEAN` | ✅ | `false` | — |
| `no_notif` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_notes_chat_name

#### Foreign Keys

- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)
- user_id -> users(user_id)

### `notes_settings`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('notes_settings_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `private` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)

### `pins`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('pins_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `msg_id` | `BIGINT` | ✅ | — | — |
| `clean_linked` | `BOOLEAN` | ✅ | `false` | — |
| `anti_channel_pin` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_pins_chat

### `report_chat_settings`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('report_chat_settings_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `enabled` | `BOOLEAN` | ✅ | `true` | — |
| `status` | `BOOLEAN` | ✅ | `true` | — |
| `blocked_list` | `JSONB` | ✅ | — | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)
- user_id -> users(user_id)

### `report_user_settings`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('report_user_settings_id_seq'::regclass)` | — |
| `user_id` | `BIGINT` | ❌ | — | — |
| `enabled` | `BOOLEAN` | ✅ | `true` | — |
| `status` | `BOOLEAN` | ✅ | `true` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)
- channel_id -> chats(chat_id)
- user_id -> users(user_id)

### `rules`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('rules_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `rules` | `TEXT` | ✅ | — | — |
| `rules_btn` | `TEXT` | ✅ | — | — |
| `private` | `BOOLEAN` | ✅ | `false` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)

### `stored_messages`

Stores messages sent by users before completing captcha verification. These messages are deleted from the chat but stored for admin review.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | ❌ | — | PRIMARY KEY |
| `user_id` | `BIGINT` | ❌ | — | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `message_type` | `INTEGER` | ❌ | `1` | 1=TEXT, 2=STICKER, etc. |
| `content` | `TEXT` | ✅ | — | Text content of message |
| `file_id` | `TEXT` | ✅ | — | Telegram file ID for media |
| `caption` | `TEXT` | ✅ | — | Media caption if any |
| `attempt_id` | `BIGINT` | ❌ | — | FK → captcha_attempts(id) |
| `created_at` | `TIMESTAMP` | ✅ | `NOW()` | — |

#### Indexes

- idx_stored_user_chat (user_id, chat_id)
- idx_stored_attempt (attempt_id)

#### Foreign Keys

- attempt_id → captcha_attempts(id) ON DELETE CASCADE

### `users`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('users_id_seq'::regclass)` | — |
| `user_id` | `BIGINT` | ❌ | — | — |
| `username` | `TEXT` | ✅ | — | — |
| `name` | `TEXT` | ✅ | — | — |
| `language` | `TEXT` | ✅ | `'en'::text` | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_users_user_id_active
- idx_users_covering
- idx_users_last_activity

#### Foreign Keys

- user_id -> users(user_id)
- chat_id -> chats(chat_id)

### `warns_settings`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('warns_settings_id_seq'::regclass)` | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `warn_limit` | `BIGINT` | ✅ | `3` | — |
| `warn_mode` | `TEXT` | ✅ | — | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Foreign Keys

- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)

### `warns_users`

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | ❌ | `nextval('warns_users_id_seq'::regclass)` | — |
| `user_id` | `BIGINT` | ❌ | — | — |
| `chat_id` | `BIGINT` | ❌ | — | — |
| `num_warns` | `BIGINT` | ✅ | `0` | — |
| `warns` | `JSONB` | ✅ | — | — |
| `created_at` | `TIMESTAMP` | ✅ | — | — |
| `updated_at` | `TIMESTAMP` | ✅ | — | — |

#### Indexes

- idx_warns_users_user_id
- idx_warns_users_chat_id
- idx_warns_users_composite

#### Foreign Keys

- chat_id -> chats(chat_id)
- chat_id -> chats(chat_id)

## Entity Relationships

### Core Entities

- **Users**: Telegram users who interact with the bot
- **Chats**: Telegram groups/channels managed by the bot
- **ChatUsers**: Join table linking users to chats

### Relationship Patterns

- User ↔ Chat: Many-to-many through `chat_users`
- Chat → Settings: One-to-one (module-specific settings)
- Chat → Content: One-to-many (filters, notes, rules, etc.)
