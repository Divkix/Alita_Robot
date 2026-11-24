package blacklists

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's blacklist entries.
var BlacklistProjection = cache.Projection{Name: "blacklist"}

// ChatKeys returns the cache keys a chat's blacklists backup import or clear
// invalidates.
var ChatKeys = cache.Keys(BlacklistProjection)
