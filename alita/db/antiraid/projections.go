package antiraid

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's anti-raid settings.
var SettingsProjection = cache.Projection{Name: "antiraid"}

// ChatKeys returns the cache keys a chat's antiraid backup import or clear
// invalidates.
var ChatKeys = cache.Keys(SettingsProjection)
