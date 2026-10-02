-- Filters and notes imported on 2025-08-05 kept their media type but lost the
-- Telegram file ID, so every trigger logs "Empty FileID" and the sender falls
-- back to the stored text. Record them as the text replies they already are.
--
-- Rows with neither a file ID nor text have nothing to send and are left for
-- an explicit cleanup decision. Type 1 is db.TEXT.

UPDATE filters
SET msgtype = 1, updated_at = NOW()
WHERE msgtype <> 1
  AND COALESCE(fileid, '') = ''
  AND BTRIM(COALESCE(filter_reply, '')) <> '';

UPDATE notes
SET msg_type = 1, updated_at = NOW()
WHERE msg_type <> 1
  AND COALESCE(file_id, '') = ''
  AND BTRIM(COALESCE(note_content, '')) <> '';
