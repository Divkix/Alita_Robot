package logchannels

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's log channel row.
var LogChannelProjection = cache.Projection{Name: "log_channel"}

// ChatKeys returns the cache keys a chat's log channels backup import or
// clear invalidates.
var ChatKeys = cache.Keys(LogChannelProjection)
