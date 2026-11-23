package admin

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's admin settings row.
var AdminSettingsProjection = cache.Projection{Name: "admin"}

// ChatKeys returns the cache keys a chat's admin backup import or clear
// invalidates.
var ChatKeys = cache.Keys(AdminSettingsProjection)
