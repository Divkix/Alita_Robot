-- Drop the fk_channels_chat foreign key constraint on channels table
--
-- The channels.chat_id column stores the channel's own Telegram ID for identification,
-- not a reference to a parent chat. The table is used for channel metadata storage
-- (tracking channel usernames and names), not for linked channel relationships.
--
-- This constraint was incorrectly requiring channels.chat_id to exist in chats.chat_id,
-- but Telegram channels are not stored in the chats table (only groups/supergroups).
--
-- Error being fixed:
-- ERROR: insert or update on table "channels" violates foreign key constraint "fk_channels_chat" (SQLSTATE 23503)

ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_chat;
