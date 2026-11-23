package disabling

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projection of a chat's disabled-commands list.
var DisabledCommandsProjection = cache.Projection{Name: "disabled_cmds"}

// ChatKeys returns the cache keys a chat's disabling backup import or clear
// invalidates.
var ChatKeys = cache.Keys(DisabledCommandsProjection)
