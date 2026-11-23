package spamdbsettings

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's spam-database settings row.
var SettingsProjection = cache.Projection{Name: "spam_db_settings"}
