package chats

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the chats row. Every write to the chats table
// invalidates the projections its change can stale; readers build keys with
// Key. chat_settings has no reader left (settings are read through chat);
// ChangeGroupLanguage still clears it, so it stays declared.
var (
	ChatProjection         = cache.Projection{Name: "chat"}
	ChatUsersProjection    = cache.Projection{Name: "chat_users"}
	ChatSettingsProjection = cache.Projection{Name: "chat_settings"}
	ChatLangProjection     = cache.Projection{Name: "chat_lang"}
)
