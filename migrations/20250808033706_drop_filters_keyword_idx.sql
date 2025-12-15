-- Drop the filters_keyword_idx index if it exists
-- This index is not needed as all queries use both chat_id and keyword
-- The existing composite index idx_filters_chat_keyword already covers these queries
DROP INDEX IF EXISTS public.filters_keyword_idx;
