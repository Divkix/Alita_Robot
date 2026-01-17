-- Drop the erroneous foreign key constraint on channels.channel_id
-- The channel_id column stores the channel's own Telegram ID, not a reference to another chat
ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_channel;
