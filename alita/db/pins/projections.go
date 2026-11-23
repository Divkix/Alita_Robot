package pins

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's pin settings row.
var PinSettingsProjection = cache.Projection{Name: "pins"}

// ChatKeys returns the cache keys a chat's pins backup import or clear
// invalidates.
var ChatKeys = cache.Keys(PinSettingsProjection)
