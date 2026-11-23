package spamfilter

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of the spam filter: a chat's settings row, its
// labelled samples, and the shared corpus every chat's spam rows feed (the
// shared key carries no chat id).
var (
	SettingsProjection = cache.Projection{Name: "spam_filter_settings"}
	SamplesProjection  = cache.Projection{Name: "spam_samples"}
	SharedProjection   = cache.Projection{Name: "spam_shared_corpus"}
)
