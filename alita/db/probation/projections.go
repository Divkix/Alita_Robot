package probation

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the probation domain: the chat's settings row and
// one member's probation state. MemberKeyPrefix names the member projection's
// module for consumers that cannot import this package (alita/db/cache's
// local skip list).
var (
	SettingsProjection = cache.Projection{Name: "probation_settings"}
	MemberProjection   = cache.Projection{Name: "probation_member"}
)

// MemberKeyPrefix is the module name of MemberProjection.
const MemberKeyPrefix = "probation_member"
