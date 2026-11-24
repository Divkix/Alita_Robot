package locks

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's lock map: one entry per chat covering
// every lock type.
var LocksProjection = cache.Projection{Name: "locks_map"}

// ChatKeys returns the cache keys a chat's locks backup import or clear
// invalidates.
var ChatKeys = cache.Keys(LocksProjection)
