-- Add federation tables for cross-chat ban sharing.
CREATE TABLE IF NOT EXISTS federations (
    id BIGSERIAL PRIMARY KEY,
    fed_id TEXT NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL,
    owner_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(owner_id),
    CONSTRAINT chk_federation_name_non_empty CHECK (LENGTH(TRIM(name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_federations_fed_id ON federations(fed_id);
CREATE INDEX IF NOT EXISTS idx_federations_owner_id ON federations(owner_id);

CREATE TABLE IF NOT EXISTS federation_admins (
    id BIGSERIAL PRIMARY KEY,
    federation_id BIGINT NOT NULL REFERENCES federations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    promoted_by BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(federation_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_federation_admins_federation_id ON federation_admins(federation_id);
CREATE INDEX IF NOT EXISTS idx_federation_admins_user_id ON federation_admins(user_id);

CREATE TABLE IF NOT EXISTS federation_chats (
    id BIGSERIAL PRIMARY KEY,
    federation_id BIGINT NOT NULL REFERENCES federations(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL UNIQUE,
    joined_by BIGINT NOT NULL,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    quiet BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_federation_chats_federation_id ON federation_chats(federation_id);
CREATE INDEX IF NOT EXISTS idx_federation_chats_chat_id ON federation_chats(chat_id);

CREATE TABLE IF NOT EXISTS federation_bans (
    id BIGSERIAL PRIMARY KEY,
    federation_id BIGINT NOT NULL REFERENCES federations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    reason TEXT,
    banned_by BIGINT NOT NULL,
    banned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(federation_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_federation_bans_federation_id ON federation_bans(federation_id);
CREATE INDEX IF NOT EXISTS idx_federation_bans_user_id ON federation_bans(user_id);

CREATE TABLE IF NOT EXISTS federation_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    federation_id BIGINT NOT NULL REFERENCES federations(id) ON DELETE CASCADE,
    subscribed_to_federation_id BIGINT NOT NULL REFERENCES federations(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(federation_id, subscribed_to_federation_id),
    CHECK (federation_id != subscribed_to_federation_id)
);

CREATE INDEX IF NOT EXISTS idx_federation_subscriptions_federation_id ON federation_subscriptions(federation_id);
CREATE INDEX IF NOT EXISTS idx_federation_subscriptions_subscribed_to ON federation_subscriptions(subscribed_to_federation_id);

CREATE TABLE IF NOT EXISTS federation_settings (
    id BIGSERIAL PRIMARY KEY,
    federation_id BIGINT NOT NULL REFERENCES federations(id) ON DELETE CASCADE,
    require_reason BOOLEAN NOT NULL DEFAULT FALSE,
    notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    log_chat_id BIGINT,
    UNIQUE(federation_id)
);

CREATE INDEX IF NOT EXISTS idx_federation_settings_federation_id ON federation_settings(federation_id);

-- Add foreign keys to chats table when available (self-managed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_federation_chats_chat') THEN
        ALTER TABLE federation_chats DROP CONSTRAINT fk_federation_chats_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE federation_chats
        ADD CONSTRAINT fk_federation_chats_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name = 'fk_federation_settings_log_chat') THEN
        ALTER TABLE federation_settings DROP CONSTRAINT fk_federation_settings_log_chat;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE federation_settings
        ADD CONSTRAINT fk_federation_settings_log_chat
        FOREIGN KEY (log_chat_id) REFERENCES chats(chat_id) ON DELETE SET NULL ON UPDATE CASCADE;
    END IF;
END $$;
