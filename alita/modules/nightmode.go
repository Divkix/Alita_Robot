package modules

import (
	"maps"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
)

// nightModeNow is the clock night mode reads. Tests pin it; production never
// writes it after startup.
var nightModeNow = time.Now

// nightLockExcluded are lock types night mode cannot apply: "bots" bans added
// bots on the ChatMember path, not on the delete path night mode runs on.
var nightLockExcluded = map[string]bool{"bots": true}

// withNightLocks returns chatLocks with the chat's night lock types added
// while its night window is open in the chat's timezone, and chatLocks itself
// otherwise. It never writes to chatLocks, the cached map: the merge is a
// copy. A chat without night mode costs one cached read.
func withNightLocks(chatID int64, chatLocks map[string]bool) map[string]bool {
	settings, err := nightmode.Get(chatID)
	if err != nil {
		log.Errorf("[NightMode] read night mode: %v - %d", err, chatID)
		return chatLocks
	}
	if !settings.Enabled || !nightmode.ActiveAt(settings, nightModeNow().In(chattz.Location(chatID))) {
		return chatLocks
	}
	types := settings.Types()
	merged := make(map[string]bool, len(chatLocks)+len(types))
	maps.Copy(merged, chatLocks)
	for _, lockType := range types {
		if !nightLockExcluded[lockType] {
			merged[lockType] = true
		}
	}
	return merged
}
