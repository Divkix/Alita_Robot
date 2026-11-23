package rules

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's rules settings row.
var RulesProjection = cache.Projection{Name: "rules"}

// ChatKeys returns the cache keys a chat's rules backup import or clear
// invalidates.
var ChatKeys = cache.Keys(RulesProjection)
