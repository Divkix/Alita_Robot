package chats

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the chats row. Every write to the chats table
// invalidates the projections its change can stale; readers build keys with
// Key.
var (
	ChatProjection      = cache.Projection{Name: "chat"}
	ChatUsersProjection = cache.Projection{Name: "chat_users"}
	ChatLangProjection  = cache.Projection{Name: "chat_lang"}
)
