package probation

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the probation domain: the chat's settings row and
// one member's probation state.
var (
	SettingsProjection = cache.Projection{Name: "probation_settings"}
	MemberProjection   = cache.Projection{Name: "probation_member"}
)

func init() {
	// A probation row gates a new member's first messages the same way the
	// captcha pending flag does, so it skips the in-process layer too.
	cache.RegisterLocalSkipPrefix(MemberProjection.Key() + ":")
}
