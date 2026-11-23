package captcha

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the captcha domain: the chat's settings row and
// the per-chat "has pending attempts" flag attempt creation invalidates.
var (
	SettingsProjection = cache.Projection{Name: "captcha_settings"}
	PendingProjection  = cache.Projection{Name: "captcha_pending"}
)
