// Package reputation answers whether several distinct chats recently banned or
// kicked a user, using moderation_events as the shared signal.
//
// The signal is a rolling window, not a score: when a human unbans the user
// inside the same window in a chat that banned or kicked them earlier in that
// window, the signal clears. A no-op /unban in a chat that never removed the
// user clears nothing. Lookups fail open — a database error is logged and
// reported as not listed, so a join is never blocked by it — and a verdict is
// cached per user for ten minutes. Nothing invalidates
// that key on write or on the retention sweep; the next lookup after the TTL
// sees the new window, which is the only refresh the feature needs.
package reputation

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
)

const (
	// cacheTTL is how long a verdict may be served. The retention sweep only
	// deletes old events, so it invalidates nothing: the key expires here.
	cacheTTL = 10 * time.Minute
	// cacheModule is the CacheKey module name of a user's verdict.
	cacheModule = "reputation"
)

// now is the clock the window is measured from; tests replace it.
var now = time.Now

// signal is the cached part of a verdict: the raw aggregate, with the
// configured threshold applied after the cache. A config change therefore
// never mixes verdicts computed under different thresholds.
type signal struct {
	Chats    int
	Unbanned bool
}

// Lookup reports whether userID was banned or kicked by at least
// REPUTATION_MIN_CHATS distinct chats inside the last REPUTATION_WINDOW_DAYS
// days, and whether no human unban reversed one of those removals. An unban
// only reverses a ban or kick the same chat recorded earlier in the window; an
// unban in a chat that never removed the user does not clear the signal.
//
// It fails open. With ENABLE_REPUTATION off it returns not listed without
// touching the database; a database error is logged at warn level and returned
// as not listed, with the error, so a caller can log it and still let the join
// through.
func Lookup(ctx context.Context, userID int64) (listed bool, chats int, err error) {
	cfg := config.AppConfig
	if cfg == nil || !cfg.EnableReputation {
		return false, 0, nil
	}
	minChats := cfg.ReputationMinChats
	if minChats <= 0 {
		minChats = config.DefaultReputationMinChats
	}
	windowDays := cfg.ReputationWindowDays
	if windowDays <= 0 {
		windowDays = config.DefaultReputationWindowDays
	}
	since := now().UTC().Add(-time.Duration(windowDays) * 24 * time.Hour)

	verdict, err := dbcache.GetFromCacheOrLoad(ctx, dbcache.CacheKey(cacheModule, userID), cacheTTL,
		func(context.Context) (signal, error) {
			count, unbanned, loadErr := moderation.ReputationSignal(userID, since)
			return signal{Chats: count, Unbanned: unbanned}, loadErr
		})
	if err != nil {
		log.WithError(err).WithField("user_id", userID).
			Warn("[Reputation] lookup failed; treating the user as not listed")
		return false, 0, err
	}
	return verdict.Chats >= minChats && !verdict.Unbanned, verdict.Chats, nil
}
