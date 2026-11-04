ALTER TABLE captcha_settings
    ADD COLUMN IF NOT EXISTS join_requests BOOLEAN NOT NULL DEFAULT FALSE;

-- Applicants are not members and may not have a users row yet. Keeping their
-- challenges separate also makes rollback safe for the member captcha worker.
CREATE TABLE IF NOT EXISTS join_request_captchas (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    user_chat_id BIGINT NOT NULL,
    first_name TEXT NOT NULL,
    answer VARCHAR(255) NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    message_id BIGINT NOT NULL DEFAULT 0,
    refresh_count INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_join_request_captcha_chat_user UNIQUE (chat_id, user_id),
    CONSTRAINT chk_join_request_captcha_expires_at CHECK (expires_at > created_at),
    CONSTRAINT fk_join_request_captchas_chat FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_join_request_captchas_expires_at ON join_request_captchas(expires_at);

DROP TRIGGER IF EXISTS update_join_request_captchas_updated_at ON join_request_captchas;
CREATE TRIGGER update_join_request_captchas_updated_at
    BEFORE UPDATE ON join_request_captchas
    FOR EACH ROW
    EXECUTE FUNCTION update_captcha_updated_at();
