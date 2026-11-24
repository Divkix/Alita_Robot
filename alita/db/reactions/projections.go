package reactions

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's reactions map.
var ReactionsProjection = cache.Projection{Name: "reactions"}

// ChatKeys returns the cache keys a chat's reactions backup import or clear
// invalidates.
var ChatKeys = cache.Keys(ReactionsProjection)
