package greetings

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's greeting settings row.
var SettingsProjection = cache.Projection{Name: "greetings"}

// ChatKeys returns the cache keys a chat's greetings backup import or clear
// invalidates.
var ChatKeys = cache.Keys(SettingsProjection)
