package filters

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of a chat's filters: the keyword list and the
// matcher payload. Every write invalidates both.
var (
	ListProjection      = cache.Projection{Name: "filter_list"}
	OptimizedProjection = cache.Projection{Name: "filters_optimized"}
)

func invalidateFilterCaches(chatID int64) {
	cache.DeleteCache(ListProjection.Key(chatID))
	cache.DeleteCache(OptimizedProjection.Key(chatID))
}
