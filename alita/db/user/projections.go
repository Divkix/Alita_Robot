package user

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the users row: the basic-info snapshot and the
// language preference alita/db/lang reads and writes.
var (
	UserProjection     = cache.Projection{Name: "user"}
	UserLangProjection = cache.Projection{Name: "user_lang"}
)
