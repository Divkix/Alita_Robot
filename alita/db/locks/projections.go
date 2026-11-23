package locks

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's lock map: one entry per chat covering
// every lock type.
var LocksProjection = cache.Projection{Name: "locks_map"}
