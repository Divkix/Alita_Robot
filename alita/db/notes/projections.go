package notes

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of a chat's notes: the settings row and the note-name
// list. Note writes invalidate the list; the private-notes switch invalidates
// settings.
var (
	SettingsProjection = cache.Projection{Name: "notes_settings"}
	ListProjection     = cache.Projection{Name: "notes_list"}
)

func invalidateNotesCache(chatID int64) {
	cache.DeleteCache(ListProjection.Key(chatID))
}

// ChatKeys returns the cache keys a chat's notes backup import or clear
// invalidates.
var ChatKeys = cache.Keys(SettingsProjection, ListProjection)
