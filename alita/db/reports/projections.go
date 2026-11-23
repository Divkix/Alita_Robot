package reports

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's report settings row.
var ReportSettingsProjection = cache.Projection{Name: "report_settings"}

// ChatKeys returns the cache keys a chat's reports backup import or clear
// invalidates.
var ChatKeys = cache.Keys(ReportSettingsProjection)
