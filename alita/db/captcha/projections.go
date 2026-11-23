package captcha

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the captcha domain: the chat's settings row and
// the per-chat "has pending attempts" flag attempt creation invalidates.
var (
	SettingsProjection = cache.Projection{Name: "captcha_settings"}
	PendingProjection  = cache.Projection{Name: "captcha_pending"}
)

func init() {
	// The pending flag gates whether a new member's messages are held; a
	// replica must see a flag set elsewhere immediately, so it skips the
	// in-process layer.
	cache.RegisterLocalSkipPrefix(PendingProjection.Key() + ":")
}

// ChatKeys returns the cache keys a chat's captcha backup import or clear
// invalidates; pending attempts are operational state, not backup data.
var ChatKeys = cache.Keys(SettingsProjection)
