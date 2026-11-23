package reputation

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a user's cross-chat reputation verdict. Nothing
// writes it: the verdict derives from moderation data and the 10 minute TTL
// bounds staleness.
var ReputationProjection = cache.Projection{Name: "reputation"}
