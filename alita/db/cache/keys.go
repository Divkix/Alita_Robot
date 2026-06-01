package cache

import (
	"fmt"
	"strconv"
	"strings"
)

// CacheKey generates a cache key with the alita prefix and any number of ID segments.
func CacheKey(module string, ids ...any) string {
	var b strings.Builder
	b.Grow(32 + len(ids)*20)
	b.WriteString("alita:")
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
