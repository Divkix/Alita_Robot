package approvals

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's approved-users list.
var ApprovalsProjection = cache.Projection{Name: "approvals"}

// ChatKeys returns the cache keys a chat's approvals backup import or clear
// invalidates.
var ChatKeys = cache.Keys(ApprovalsProjection)
