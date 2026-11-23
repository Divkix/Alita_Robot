package antiflood

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's antiflood settings row.
var SettingsProjection = cache.Projection{Name: "antiflood"}

// ChatKeys returns the cache keys a chat's antiflood backup import or clear
// invalidates.
var ChatKeys = cache.Keys(SettingsProjection)
