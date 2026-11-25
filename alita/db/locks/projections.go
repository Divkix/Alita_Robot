package locks

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of a chat's locks: the lock map and its allowlist.
var LocksProjection = cache.Projection{Name: "locks_map"}
var LockAllowlistProjection = cache.Projection{Name: "lock_allowlist"}

// ChatKeys returns the cache keys a chat's locks backup import or clear
// invalidates.
var ChatKeys = cache.Keys(LocksProjection, LockAllowlistProjection)
