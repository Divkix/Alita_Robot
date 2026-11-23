package connections

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the connections domain: the chat's connect
// preference and one user's connection state.
var (
	ConnectionChatProjection = cache.Projection{Name: "connection_chat"}
	ConnectionProjection     = cache.Projection{Name: "connection"}
)

// ChatKeys returns the cache keys a chat's connections backup import or clear
// invalidates; the per-user connection projection is not chat-scoped.
var ChatKeys = cache.Keys(ConnectionChatProjection)
