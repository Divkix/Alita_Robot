package ephemeral

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's ephemeral-replies settings row.
var SettingsProjection = cache.Projection{Name: "ephemeral_settings"}
