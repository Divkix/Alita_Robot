package federations

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the federations domain: the federation row, a
// chat's membership, a federation's admins, one ban, and a federation's
// subscriptions.
var (
	FedProjection       = cache.Projection{Name: "fed"}
	FedChatProjection   = cache.Projection{Name: "fed_chat"}
	FedAdminsProjection = cache.Projection{Name: "fed_admins"}
	FedBanProjection    = cache.Projection{Name: "fed_ban"}
	FedSubsProjection   = cache.Projection{Name: "fed_subs"}
)

// ChatKeys returns the cache keys a chat's federations backup import or clear
// invalidates; the fed-scoped projections are not chat-scoped.
var ChatKeys = cache.Keys(FedChatProjection)
