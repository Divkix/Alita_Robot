---
title: Database Schema
description: Complete reference of the PostgreSQL database schema
---
<!-- MANUALLY MAINTAINED: do not regenerate -->

# Database Schema

This page documents the complete PostgreSQL database schema for Alita Robot.

## Overview

- **Application Tables**: 37
- **Migration Metadata**: `schema_migrations`
- **Database Type**: PostgreSQL
- **ORM**: GORM
- **Migration Tool**: Custom SQL migration runner (`alita/db/migrations/runner.go`)
- **Migrations**: Timestamped forward SQL files using `YYYYMMDDHHMMSS_description.sql` naming

## Design Patterns

### Surrogate Key Pattern

All tables use an auto-incremented `id` field as the primary key (internal identifier), while external identifiers like `user_id` and `chat_id` (Telegram IDs) are stored with unique constraints.

**Benefits:**

- Decouples internal schema from external systems
- Provides stability if external IDs change
- Simplifies GORM operations with consistent integer primary keys
- Better performance for joins and indexing

### Chat Membership

Chat membership is managed via the JSONB `users` column on the `chats` table (an `Int64Array` of user IDs). The physical `chat_users` join table was dropped by migration and there is no corresponding GORM model.

## Tables

### `admin`

Stores admin settings per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `anon_admin` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Indexes

- `idx_admin_chat_id` (unique, on `chat_id`)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `ai_spam_settings`

Stores the per-chat opt-in switch for the AI spam filter. A missing row means the filter is off; `/aispam on` and `/aispam off` both upsert the row.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `enabled` | `BOOLEAN` | NO | `FALSE` | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `UNIQUE(chat_id)` — no separate index; the constraint index serves every lookup.

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

> **Note:** The flag lives in the database rather than Redis: it is the chat's moderation policy, and a cache flush must not silently stop the filter in a chat an admin enabled it for.

---

### `antiflood_settings`

Stores anti-flood configuration per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `flood_limit` | `BIGINT` | YES | `5` | CHECK (`flood_limit >= 0`) |
| `action` | `TEXT` | YES | `'mute'` | CHECK (`action IN ('mute','ban','kick','warn','tban','tmute')`) |
| `delete_antiflood_message` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `action` | `TEXT` | YES | `'warn'` | CHECK (`action IN ('warn','mute','ban','kick','tban','tmute','delete','none')`) |
| `reason` | `TEXT` | YES | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Indexes

- `uk_blacklists_chat_word` (unique constraint: `chat_id`, `word`)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `captcha_attempts`

Tracks active captcha verification attempts for users.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | — |
| `chat_id` | `BIGINT` | NO | — | — |
| `answer` | `VARCHAR(255)` | NO | — | — |
| `attempts` | `INTEGER` | YES | `0` | — |
| `message_id` | `BIGINT` | YES | — | — |
| `refresh_count` | `INTEGER` | YES | `0` | — |
| `expires_at` | `TIMESTAMPTZ` | NO | — | CHECK (`expires_at > created_at`) |
| `created_at` | `TIMESTAMPTZ` | YES | `CURRENT_TIMESTAMP` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `CURRENT_TIMESTAMP` | — |

#### Indexes

- `uk_captcha_user_chat` (unique index: `user_id`, `chat_id`; one attempt per user per chat)
- `idx_captcha_attempts_chat_id`

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
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `uk_captcha_muted_user_chat` (unique constraint: `user_id`, `chat_id`)
- `idx_captcha_muted_users_chat_id` (on `chat_id`)
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
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `enabled` | `BOOLEAN` | YES | `FALSE` | — |
| `captcha_mode` | `VARCHAR(10)` | YES | `'math'` | CHECK (`captcha_mode IN ('math','text')`) |
| `timeout` | `INTEGER` | YES | `2` | CHECK (`timeout BETWEEN 1 AND 10`) |
| `failure_action` | `VARCHAR(10)` | YES | `'kick'` | CHECK (`failure_action IN ('kick','ban','mute')`) |
| `max_attempts` | `INTEGER` | YES | `3` | CHECK (`max_attempts BETWEEN 1 AND 10`) |
| `created_at` | `TIMESTAMP` | YES | `CURRENT_TIMESTAMP` | — |
| `updated_at` | `TIMESTAMP` | YES | `CURRENT_TIMESTAMP` | — |

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
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Indexes

- `idx_channels_username`

#### Foreign Keys

> **Note:** All foreign key constraints on this table have been dropped by migrations (`20260117104821_fix_invalid_channels_fk_constraint.sql`, `20260117120000_drop_channels_chat_fk.sql`). The `chat_id` column stores the channel's own Telegram ID for identification.

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
| `is_inactive` | `BOOLEAN` | YES | `false` | — |
| `last_activity` | `TIMESTAMPTZ` | YES | `CURRENT_TIMESTAMP` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Indexes

- `uk_chats_chat_id` (unique constraint on `chat_id`)
- `idx_chats_covering` (on `chat_id` INCLUDE `language`, `is_inactive`)
- `idx_chats_activity_status`

---

### `connection`

Per-user connection state (a user has at most one connected chat).

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | UNIQUE |
| `chat_id` | `BIGINT` | NO | — | — |
| `connected` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Indexes

- `uk_connection_user_id` (unique constraint on `user_id`; one saved connection per user)
- `idx_connection_chat_id` (on `chat_id`; backs the foreign key)

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
| `allow_connect` | `BOOLEAN` | YES | `true` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `is_dev` | `BOOLEAN` | YES | `false` | — |
| `sudo` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `disabled` | `BOOLEAN` | YES | `true` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `federations`

Federation definitions; one per owner.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `fed_id` | `VARCHAR(36)` | NO | — | — |
| `owner_id` | `BIGINT` | NO | — | — |
| `name` | `VARCHAR(64)` | NO | — | — |
| `require_reason` | `BOOLEAN` | NO | `FALSE` | — |
| `notify_owner` | `BOOLEAN` | NO | `TRUE` | — |
| `log_chat_id` | `BIGINT` | NO | `0` | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `idx_federations_fed_id` (unique, on `fed_id`)
- `idx_federations_owner_id` (unique, on `owner_id`)

#### Foreign Keys

- `owner_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `federation_admins`

Federation admins other than the owner.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `fed_id` | `VARCHAR(36)` | NO | — | — |
| `user_id` | `BIGINT` | NO | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `idx_fed_admins_fed_user` (unique: `fed_id`, `user_id`)
- `idx_federation_admins_user_id` (on `user_id`)

#### Foreign Keys

- `fed_id` → `federations(fed_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `federation_bans`

Users banned across a federation.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `fed_id` | `VARCHAR(36)` | NO | — | — |
| `user_id` | `BIGINT` | NO | — | — |
| `reason` | `TEXT` | NO | `''` | — |
| `banned_by` | `BIGINT` | NO | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `idx_fed_bans_fed_user` (unique: `fed_id`, `user_id`)
- `idx_federation_bans_user_id` (on `user_id`)

#### Foreign Keys

- `fed_id` → `federations(fed_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `federation_chats`

Chats that have joined a federation.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `fed_id` | `VARCHAR(36)` | NO | — | — |
| `chat_id` | `BIGINT` | NO | — | — |
| `quiet` | `BOOLEAN` | NO | `FALSE` | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `idx_federation_chats_chat_id` (unique, on `chat_id`; a chat joins at most one federation)
- `idx_federation_chats_fed_id` (on `fed_id`)

#### Foreign Keys

- `fed_id` → `federations(fed_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `federation_subs`

Federation-to-federation ban subscriptions.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `fed_id` | `VARCHAR(36)` | NO | — | — |
| `subscribed_fed_id` | `VARCHAR(36)` | NO | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `idx_fed_subs_pair` (unique: `fed_id`, `subscribed_fed_id`)
- `idx_federation_subs_subscribed_fed_id` (on `subscribed_fed_id`)

#### Foreign Keys

- `fed_id` → `federations(fed_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `subscribed_fed_id` → `federations(fed_id)` ON DELETE CASCADE ON UPDATE CASCADE

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
| `nonotif` | `BOOLEAN` | YES | `false` | — |
| `filter_buttons` | `JSONB` | YES | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `clean_service_settings` | `BOOLEAN` | YES | `false` | — |
| `welcome_clean_old` | `BOOLEAN` | YES | `false` | — |
| `welcome_last_msg_id` | `BIGINT` | YES | — | — |
| `welcome_enabled` | `BOOLEAN` | YES | `true` | — |
| `welcome_text` | `TEXT` | YES | — | — |
| `welcome_file_id` | `TEXT` | YES | — | — |
| `welcome_type` | `BIGINT` | YES | — (GORM writes 1 on insert) | — |
| `welcome_btns` | `JSONB` | YES | — | — |
| `goodbye_clean_old` | `BOOLEAN` | YES | `false` | — |
| `goodbye_last_msg_id` | `BIGINT` | YES | — | — |
| `goodbye_enabled` | `BOOLEAN` | YES | `true` | — |
| `goodbye_text` | `TEXT` | YES | — | — |
| `goodbye_file_id` | `TEXT` | YES | — | — |
| `goodbye_type` | `BIGINT` | YES | — (GORM writes 1 on insert) | — |
| `goodbye_btns` | `JSONB` | YES | — | — |
| `auto_approve` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `locked` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `log_channels`

Per-chat admin log channel and enabled categories.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | — |
| `log_channel_id` | `BIGINT` | NO | — | — |
| `cat_settings` | `BOOLEAN` | NO | `TRUE` | — |
| `cat_admin` | `BOOLEAN` | NO | `TRUE` | — |
| `cat_user` | `BOOLEAN` | NO | `TRUE` | — |
| `cat_automated` | `BOOLEAN` | NO | `TRUE` | — |
| `cat_reports` | `BOOLEAN` | NO | `TRUE` | — |
| `cat_other` | `BOOLEAN` | NO | `TRUE` | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `idx_log_channels_chat_id` (unique, on `chat_id`)

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
| `admin_only` | `BOOLEAN` | YES | `false` | — |
| `private_only` | `BOOLEAN` | YES | `false` | — |
| `group_only` | `BOOLEAN` | YES | `false` | — |
| `web_preview` | `BOOLEAN` | YES | `true` | — |
| `is_protected` | `BOOLEAN` | YES | `false` | — |
| `no_notif` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `private` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `clean_linked` | `BOOLEAN` | YES | `false` | — |
| `anti_channel_pin` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `reactions`

Stores per-chat keyword-to-emoji reaction mappings.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGSERIAL` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `chat_id`, `keyword`) |
| `keyword` | `TEXT` | NO | — | UNIQUE (composite: `chat_id`, `keyword`) |
| `emoji` | `TEXT` | NO | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- Unique composite index on (`chat_id`, `keyword`)

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
| `enabled` | `BOOLEAN` | YES | `true` | — |
| `status` | `BOOLEAN` | YES | `true` | — |
| `blocked_list` | `JSONB` | YES | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `enabled` | `BOOLEAN` | YES | `true` | — |
| `status` | `BOOLEAN` | YES | `true` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `private` | `BOOLEAN` | YES | `false` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

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
| `language` | `TEXT` | YES | `'en'` | — |
| `last_activity` | `TIMESTAMPTZ` | YES | `CURRENT_TIMESTAMP` | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Indexes

- `uk_users_user_id` (unique constraint on `user_id`)
- `idx_users_user_name` (on `username`)
- `idx_users_last_activity`

---

### `approved_users`

Users approved for a chat (immune to anti-spam).

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE (composite: `user_id`, `chat_id`) |
| `user_id` | `BIGINT` | NO | — | UNIQUE (composite: `user_id`, `chat_id`) |
| `reason` | `TEXT` | YES | `''` | — |
| `approved_by` | `BIGINT` | NO | `0` | — |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `idx_approved_users_chat_id`

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `antiraid_settings`

Anti-raid configuration per chat.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `raid_time` | `INT` | NO | `21600` | CHECK (`raid_time >= 0`) |
| `raid_action_time` | `INT` | NO | `3600` | CHECK (`raid_action_time >= 0`) |
| `auto_antiraid_threshold` | `INT` | NO | `0` | CHECK (`auto_antiraid_threshold >= 0`) |
| `created_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |
| `updated_at` | `TIMESTAMPTZ` | YES | `NOW()` | — |

#### Indexes

- `antiraid_settings_chat_id_key` (unique constraint on `chat_id`)

#### Foreign Keys

- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `warns_settings`

Warning system settings per chat. `warn_action_time` is the tban/tmute duration in seconds; `warn_expiry` is how long a warn counts, in seconds, with `0` meaning warns never expire. Two CHECKs cover `warn_mode`: `chk_warn_mode` (NOT VALID) also accepts `''`, but the older validated `chk_warns_mode` does not, so PostgreSQL rejects an empty string.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `chat_id` | `BIGINT` | NO | — | UNIQUE |
| `warn_limit` | `BIGINT` | YES | `3` | CHECK (`warn_limit > 0`) |
| `warn_mode` | `TEXT` | YES | — | CHECK (`warn_mode IS NULL OR warn_mode IN ('ban','kick','mute','tban','tmute')`) |
| `warn_action_time` | `INTEGER` | NO | `0` | CHECK (`warn_action_time >= 0`) |
| `warn_expiry` | `INTEGER` | NO | `0` | CHECK (`warn_expiry >= 0`) |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

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
| `num_warns` | `BIGINT` | YES | `0` | CHECK (`num_warns >= 0`) |
| `warns` | `JSONB` | YES | — | — |
| `created_at` | `TIMESTAMPTZ` | YES | — | — |
| `updated_at` | `TIMESTAMPTZ` | YES | — | — |

#### Indexes

- Unique composite index on (`user_id`, `chat_id`) (`uk_warns_users_user_chat`)
- `idx_warns_users_chat_id`

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

---

### `warn_entries`

One row per warn: its reason, who issued it and when. `warns_users` keeps the count and the `warns` reasons array, which the bot still writes for older releases.

#### Columns

| Column | Type | Nullable | Default | Constraints |
|--------|------|----------|---------|-------------|
| `id` | `BIGINT` | NO | auto-increment | PRIMARY KEY |
| `user_id` | `BIGINT` | NO | — | — |
| `chat_id` | `BIGINT` | NO | — | — |
| `reason` | `TEXT` | NO | `''` | — |
| `warned_by` | `BIGINT` | NO | `0` | — |
| `anonymous` | `BOOLEAN` | NO | `FALSE` | — |
| `warned_at` | `TIMESTAMPTZ` | YES | — | — |

`warned_by = 0` with a `NULL` `warned_at` marks a warn issued before this table existed. `anonymous` hides the issuer, an anonymous admin; backups export `warned_by = 0` for those rows. Backups export a warn the bot issued itself as `warned_by = -1` (`models.WarnedByBot`), because the bot's own ID means nothing to another bot account, and importing such a backup stores `-1` unchanged. `/warns` shows these warns, and warns whose `warned_by` is the bot's own ID, as automatic.

#### Indexes

- `idx_warn_entries_chat_user` (on `chat_id`, `user_id`, `id`)
- `idx_warn_entries_user_id` (on `user_id`)

#### Foreign Keys

- `user_id` → `users(user_id)` ON DELETE CASCADE ON UPDATE CASCADE
- `chat_id` → `chats(chat_id)` ON DELETE CASCADE ON UPDATE CASCADE

There is no foreign key to `warns_users`: the bot deletes a user's entries in the same transaction as their `warns_users` row.

## Entity Relationships

### Core Entities

- **Users**: Telegram users who interact with the bot
- **Chats**: Telegram groups/channels managed by the bot
- **Chat Users**: Managed via JSONB `users` array on the `chats` table (not a physical join table)

### Relationship Patterns

- User ↔ Chat: Many-to-many through JSONB `users` field on `chats`
- Chat → Settings: One-to-one (module-specific settings like `warns_settings`, `antiflood_settings`, `pins`)
- Chat → Content: One-to-many (`filters`, `notes`, `blacklists`, `reactions`)
- User → Chat Warnings: One-to-many through `warns_users`, one `warn_entries` row per warn
- Chat → Captcha: One-to-one (`captcha_settings`) with one-to-many attempts (`captcha_attempts`)
- User → Federation: one-to-one ownership (`federations.owner_id` is unique)
- Federation → Members: one-to-many (`federation_admins`, `federation_bans`, `federation_chats`), plus federation-to-federation subscriptions via `federation_subs`
- Chat → Federation: at most one (`federation_chats.chat_id` is unique)
- Chat → Log channel: one-to-one (`log_channels`)
