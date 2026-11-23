package cache

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func CacheKey(module string, ids ...any) string {
	var b strings.Builder
	b.Grow(32 + len(ids)*20)
	b.WriteString(cache.DataCachePrefix)
	b.WriteString(module)
	for _, id := range ids {
		b.WriteByte(':')
		switch v := id.(type) {
		case int64:
			b.WriteString(strconv.FormatInt(v, 10))
		case int:
			b.WriteString(strconv.Itoa(v))
		case string:
			b.WriteString(v)
		default:
			fmt.Fprint(&b, id)
		}
	}
	return b.String()
}

// A Projection is one cache key shape a domain reads, declared once beside the
// domain's repository. Write paths invalidate through it and readers build
// their key with Key, so the module name lives in exactly one place. ids are
// the same arguments CacheKey takes, in the order the key spells them.
type Projection struct {
	Name string
}

// Key returns the cache key for ids, e.g. projection.Key(chatID).
func (p Projection) Key(ids ...any) string {
	return CacheKey(p.Name, ids...)
}

// Keys returns the cache keys for ids, one per projection. Consumers that
// enumerate a chat's invalidations (backup import) use it; projections of one
// entity must share the id shape.
func Keys(projections ...Projection) func(ids ...any) []string {
	return func(ids ...any) []string {
		keys := make([]string, len(projections))
		for i, p := range projections {
			keys[i] = p.Key(ids...)
		}
		return keys
	}
}

// RegisterLocalSkipPrefix marks every key under prefix as too freshness-
// critical for the in-process layer: a value written on another replica (or
// outside DeleteCache) would otherwise be served stale for up to
// CACHE_LOCAL_TTL seconds. Domains call it from init in their projections.go;
// prefix is CacheKey(module)+":" so it matches the module's keys only.
func RegisterLocalSkipPrefix(prefix string) {
	localMu.Lock()
	skipLocalPrefixes = append(skipLocalPrefixes, prefix)
	localMu.Unlock()
}
