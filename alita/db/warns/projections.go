package warns

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the warns domain: one user's warn entries in a
// chat, and the chat's warn settings. Warn writes invalidate both because a
// first warn also creates the settings row.
var (
	WarnsProjection        = cache.Projection{Name: "warns"}
	WarnSettingsProjection = cache.Projection{Name: "warn_settings"}
)

// InvalidateWarnCaches clears both projections for userID in chatID.
func InvalidateWarnCaches(userID, chatID int64) {
	cache.DeleteCache(WarnsProjection.Key(userID, chatID))
	cache.DeleteCache(WarnSettingsProjection.Key(chatID))
}

// InvalidateWarnSettingsCache clears the chat's settings projection.
func InvalidateWarnSettingsCache(chatID int64) {
	cache.DeleteCache(WarnSettingsProjection.Key(chatID))
}

// ChatKeys returns the cache keys a chat's warns backup import or clear
// invalidates: the settings projection plus the per-user warns projection
// for each of userIDs. Pass the union of the user ids the write removed and
// the ids it restored.
func ChatKeys(chatID int64, userIDs ...int64) []string {
	keys := []string{WarnSettingsProjection.Key(chatID)}
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		keys = append(keys, WarnsProjection.Key(userID, chatID))
	}
	return keys
}
