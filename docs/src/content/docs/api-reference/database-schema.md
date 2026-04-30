---
title: Database Schema
description: Complete reference of the PostgreSQL database schema
---

# Database Schema

This page documents the complete PostgreSQL database schema for Alita Robot.

## Overview

- **Total Tables**: 27
- **Database Type**: PostgreSQL
- **ORM**: GORM
- **Migration Tool**: Custom SQL migration runner (`alita/db/migrations.go`)
- **Migrations**: 27 files using `YYYYMMDDHHMMSS_description.sql` naming (e.g., `20250805200527_initial_migration.sql`)

## Design Patterns

### Surrogate Key Pattern

All tables use an auto-incremented `id` field as the primary key (internal identifier), while external identifiers like `user_id` and `chat_id` (Telegram IDs) are stored with unique constraints.

**Benefits:**

- Decouples internal schema from external systems
- Provides stability if external IDs change
- Simplifies GORM operations with consistent integer primary keys
- Better performance for joins and indexing

### Chat Membership

Chat membership is managed via the JSONB `users` column on the `chats` table (an `Int64Array` of user IDs). The `ChatUser` GORM model exists in code for type safety but the physical `chat_users` join table has been dropped by migration.

## Tables

### `admin`

Stores admin settings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `anon_admin` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_admin_settings_chat`

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `antiflood_settings`

Stores anti-flood configuration per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `flood_limit` | `BIGINT` | NO | `5` | CHECK (`flood_limit >= 0`) |
| `action` | `TEXT` | NO | `'mute'` | CHECK (`action IN ('mute','ban','kick','warn','tban','tmute')`) |
| `delete_antiflood_message` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

> **Note:** Previous `mode` and `limit` columns were dropped by migrations (`20260420120000_consolidate_duplicate_fields.sql`, `20250814100000_fix_antiflood_column_duplication.sql`). Only `flood_limit` is used.

#### Indexes

- `idx_antiflood_chat_flood_active` (conditional: `flood_limit > 0`)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `blacklists`

Stores blacklisted words and their actions per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | — |
| `word` | `TEXT` | NO | — | — |
| `action` | `TEXT` | NO | `'warn'` | CHECK (`action IN ('warn','mute','ban','kick','tban','tmute','delete','none')`) |
| `reason` | `TEXT` | YES | — | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_blacklist_chat_word` (composite: `chat_id`, `word`)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `captcha_attempts`

Tracks active captcha verification attempts for users.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `SERIAL` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | — |
| `chat_id` | `BIGINT` | NO | — | — |
| `answer` | `VARCHAR(255)` | NO | — | — |
| `attempts` | `INTEGER` | NO | `0` | — |
| `message_id` | `BIGINT` | YES | — | — |
| `refresh_count` | `INTEGER` | NO | `0` | — |
| `expires_at` | `TIMESTAMP` | NO | — | CHECK (`expires_at > created_at`) |
| `created_at` | `TIMESTAMP` | NO | `CURRENT_TIMESTAMP` | — |
| `updated_at` | `TIMESTAMP` | NO | `CURRENT_TIMESTAMP` | — |

#### Indexes

- `idx_captcha_user_chat` (composite: `user_id`, `chat_id`)
- `idx_captcha_attempts_chat_id`
- `idx_captcha_expires_at` (dropped by migration `20250808120328`; may not exist)

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE
- `chat_id` → `chats(chat_id)` ON DELETE CASCADE

---

### `captcha_muted_users`

Tracks users who failed captcha with mute action, for automatic un-mute scheduling.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | — |
| `chat_id` | `BIGINT` | NO | — | — |
| `unmute_at` | `TIMESTAMPTZ` | NO | — | — |
| `created_at` | `TIMESTAMPTZ` | NO | `NOW()` | — |

#### Indexes

- `idx_captcha_muted_user_chat` (composite: `user_id`, `chat_id`)
- `idx_captcha_unmute_at`

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE
- `chat_id` → `chats(chat_id)` ON DELETE CASCADE

---

### `captcha_settings`

Stores captcha configuration per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `SERIAL` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `enabled` | `BOOLEAN` | NO | `FALSE` | — |
| `captcha_mode` | `VARCHAR(10)` | NO | `'math'` | CHECK (`captcha_mode IN ('math','text')`) |
| `timeout` | `INTEGER` | NO | `2` | CHECK (`timeout BETWEEN 1 AND 10`) |
| `failure_action` | `VARCHAR(10)` | NO | `'kick'` | CHECK (`failure_action IN ('kick','ban','mute')`) |
| `max_attempts` | `INTEGER` | NO | `3` | CHECK (`max_attempts BETWEEN 1 AND 10`) |
| `created_at` | `TIMESTAMP` | NO | `CURRENT_TIMESTAMP` | — |
| `updated_at` | `TIMESTAMP` | NO | `CURRENT_TIMESTAMP` | — |

#### Indexes

- `uk_captcha_settings_chat_id` (UNIQUE)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE

---

### `channels`

Stores channel metadata and linked channel relationships.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `channel_id` | `BIGINT` | YES | — | — |
| `channel_name` | `TEXT` | YES | — | — |
| `username` | `TEXT` | YES | — | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_channels_chat_update`
- `idx_channels_username`

#### Foreign Keys

> **Note:** All foreign key constraints on this table have been dropped by migrations (`20260117104821_fix_invalid_channels_fk_constraint.sql`, `20260117120000_drop_channels_chat_fk.sql`). The `chat_id` column stores the channel's own Telegram ID for identification.

---

### `chat_users`

Junction table for many-to-many relationship between chats and users.

> **Note:** The physical `chat_users` table has been dropped by migration (`20250814100001_drop_unused_chat_users_table.sql`). Chat membership is now managed exclusively via the JSONB `users` column on the `chats` table. The `ChatUser` GORM model exists in code for type safety only.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `chat_id` | `BIGINT` | NO | — | PRIMARY KEY (composite) |
| `user_id` | `BIGINT` | NO | — | PRIMARY KEY (composite) |

---

### `chats`

Main table storing chat/group information.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `chat_name` | `TEXT` | YES | — | — |
| `language` | `TEXT` | YES | — | — |
| `users` | `JSONB` | YES | — | — |
| `is_inactive` | `BOOLEAN` | NO | `false` | — |
| `last_activity` | `TIMESTAMP` | YES | — | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_chats_chat_id_active`
- `idx_chats_covering`
- `idx_chats_users_gin`
- `idx_chats_inactive`
- `idx_chats_last_activity`
- `idx_chats_activity_status`

---

### `connection`

User-to-chat connection state.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | UNIQUE (composite: `user_id`, `chat_id`) |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `user_id`, `chat_id`) |
| `connected` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_connection_user_id`
- `idx_connection_chat_id`

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `connection_settings`

Chat-level connection configuration.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `enabled` | `BOOLEAN` | NO | `true` | — |
| `allow_connect` | `BOOLEAN` | NO | `true` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

> **Note:** The `enabled` column was dropped by migration `20251231131415` as duplicate of `allow_connect`. It remains defined in the GORM model but may not exist in the physical database schema.

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `devs`

Bot developers and sudo users.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | UNIQUE |
| `is_dev` | `BOOLEAN` | NO | `false` | — |
| `dev` | `BOOLEAN` | NO | `false` | — |
| `sudo` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

> **Note:** The `dev` column was dropped by migration `20260420120000` (consolidated into `is_dev`). It remains defined in the GORM model for backward compatibility but may not exist in the physical database schema.

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `disable`

Per-command disable state for chats.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `chat_id`, `command`) |
| `command` | `TEXT` | NO | — | UNIQUE (composite: `chat_id`, `command`) |
| `disabled` | `BOOLEAN` | NO | `true` | — |
| `delete_commands` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `disable_chat_settings`

Chat-level disable configuration for command deletion behavior.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `delete_commands` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

---

### `filters`

Custom keyword filters per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `chat_id`, `keyword`) |
| `keyword` | `TEXT` | NO | — | UNIQUE (composite: `chat_id`, `keyword`) |
| `filter_reply` | `TEXT` | YES | — | — |
| `msgtype` | `BIGINT` | YES | — | — |
| `fileid` | `TEXT` | YES | — | — |
| `nonotif` | `BOOLEAN` | NO | `false` | — |
| `filter_buttons` | `JSONB` | YES | — | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_filters_chat_keyword` (composite: `chat_id`, `keyword`)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `greetings`

Welcome and goodbye message settings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `clean_service_settings` | `BOOLEAN` | NO | `false` | — |
| `welcome_clean_old` | `BOOLEAN` | NO | `false` | — |
| `welcome_last_msg_id` | `BIGINT` | YES | — | — |
| `welcome_enabled` | `BOOLEAN` | NO | `true` | — |
| `welcome_text` | `TEXT` | YES | — | — |
| `welcome_file_id` | `TEXT` | YES | — | — |
| `welcome_type` | `BIGINT` | NO | `1` | — |
| `welcome_btns` | `JSONB` | YES | — | — |
| `goodbye_clean_old` | `BOOLEAN` | NO | `false` | — |
| `goodbye_last_msg_id` | `BIGINT` | YES | — | — |
| `goodbye_enabled` | `BOOLEAN` | NO | `true` | — |
| `goodbye_text` | `TEXT` | YES | — | — |
| `goodbye_file_id` | `TEXT` | YES | — | — |
| `goodbye_type` | `BIGINT` | NO | `1` | — |
| `goodbye_btns` | `JSONB` | YES | — | — |
| `auto_approve` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_greetings_chat_enabled`

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `locks`

Locked permissions per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `chat_id`, `lock_type`) |
| `lock_type` | `TEXT` | NO | — | UNIQUE (composite: `chat_id`, `lock_type`) |
| `locked` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_locks_chat_lock_lookup`
- `idx_locks_covering`

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `notes`

Saved notes/tags per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `chat_id`, `note_name`) |
| `note_name` | `TEXT` | NO | — | UNIQUE (composite: `chat_id`, `note_name`) |
| `note_content` | `TEXT` | YES | — | — |
| `file_id` | `TEXT` | YES | — | — |
| `msg_type` | `BIGINT` | YES | — | — |
| `buttons` | `JSONB` | YES | — | — |
| `admin_only` | `BOOLEAN` | NO | `false` | — |
| `private_only` | `BOOLEAN` | NO | `false` | — |
| `group_only` | `BOOLEAN` | NO | `false` | — |
| `web_preview` | `BOOLEAN` | NO | `true` | — |
| `is_protected` | `BOOLEAN` | NO | `false` | — |
| `no_notif` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_notes_chat_name` (composite: `chat_id`, `note_name`)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `notes_settings`

Note settings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `private` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `pins`

Pinned message settings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `msg_id` | `BIGINT` | YES | — | — |
| `clean_linked` | `BOOLEAN` | NO | `false` | — |
| `anti_channel_pin` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_pins_chat`

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `report_chat_settings`

Report settings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `enabled` | `BOOLEAN` | NO | `true` | — |
| `status` | `BOOLEAN` | NO | `true` | — |
| `blocked_list` | `JSONB` | YES | — | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `report_user_settings`

Report settings per user.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | UNIQUE |
| `enabled` | `BOOLEAN` | NO | `true` | — |
| `status` | `BOOLEAN` | NO | `true` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `rules`

Chat rules text.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `rules` | `TEXT` | YES | — | — |
| `rules_btn` | `TEXT` | YES | — | — |
| `private` | `BOOLEAN` | NO | `false` | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `stored_messages`

Stores messages sent by users before completing captcha verification.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | — |
| `chat_id` | `BIGINT` | NO | — | — |
| `message_type` | `INTEGER` | NO | `1` | — |
| `content` | `TEXT` | YES | — | — |
| `file_id` | `TEXT` | YES | — | — |
| `caption` | `TEXT` | YES | — | — |
| `attempt_id` | `BIGINT` | NO | — | — |
| `created_at` | `TIMESTAMPTZ` | NO | `NOW()` | — |

#### Indexes

- `idx_stored_user_chat` (composite: `user_id`, `chat_id`)
- `idx_stored_attempt`

#### Foreign Keys

- `attempt_id` → `captcha_attempts(id)` ON DELETE CASCADE

---

### `users`

Main table storing user information.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | UNIQUE |
| `username` | `TEXT` | YES | — | INDEXED |
| `name` | `TEXT` | YES | — | — |
| `language` | `TEXT` | NO | `'en'` | — |
| `last_activity` | `TIMESTAMP` | YES | — | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_users_user_id_active`
- `idx_users_covering`
- `idx_users_last_activity`

---

### `warns_settings`

Warning system settings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `warn_limit` | `BIGINT` | NO | `3` | CHECK (`warn_limit > 0`) |
| `warn_mode` | `TEXT` | YES | — | CHECK (`warn_mode IS NULL OR warn_mode = '' OR warn_mode IN ('ban','kick','mute','tban','tmute')`) |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `warns_users`

User warnings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | UNIQUE (composite: `user_id`, `chat_id`) |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `user_id`, `chat_id`) |
| `num_warns` | `BIGINT` | NO | `0` | CHECK (`num_warns >= 0`) |
| `warns` | `JSONB` | YES | — | — |
| `created_at` | `TIMESTAMP` | YES | — | — |
| `updated_at` | `TIMESTAMP` | YES | — | — |

#### Indexes

- `idx_warns_users_user_id`
- `idx_warns_users_chat_id`
- `idx_warns_users_composite`

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

## Entity Relationships

### Core Entities

- **Users**: Telegram users who interact with the bot
- **Chats**: Telegram groups/channels managed by the bot
- **Chat Users**: Managed via JSONB `users` array on the `chats` table (not a physical join table)

### Relationship Patterns

- User ↔ Chat: Many-to-many through JSONB `users` field on `chats`
- Chat → Settings: One-to-one (module-specific settings like `warns_settings`, `antiflood_settings`, `pins`)
- Chat → Content: One-to-many (`filters`, `notes`, `blacklists`)
- User → Chat Warnings: One-to-many through `warns_users`
- Chat → Captcha: One-to-one (`captcha_settings`) with one-to-many attempts (`captcha_attempts`)