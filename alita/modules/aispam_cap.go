package modules

import (
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

//
// Daily cap on Jev checks (AISPAM_DAILY_LIMIT), so a spam wave cannot run up
// the bill. A check past the cap is not made: like a shed check, the message
// stays unchecked and nothing is deleted.
//

// aispamDailyKeyTTL keeps a day's counter a day past its own, for a replica
// whose clock is a little behind, and lets Redis drop it after.
const aispamDailyKeyTTL = 48 * time.Hour

// aispamDailyNow is the cap's clock; tests move it to the next UTC day.
var aispamDailyNow = time.Now

// aispamDailyScript counts one check against the day's counter and returns the
// new count. The first check of the day sets the key's lifetime, in the same
// call, so a counter never outlives its TTL.
var aispamDailyScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return n
`)

func aispamDailyKey(day string) string {
	return "alita:aispam:daily:" + day
}

// aispamLocalDaily counts the day's checks in this process when Redis is not
// configured or a Redis call fails. That count is per replica, so without
// Redis the bot as a whole can make up to the limit times its replica count.
var aispamLocalDaily struct {
	mu     sync.Mutex
	day    string
	count  int64
	warned string // the day the limit was last reported, so it logs once a day
}

// aispamClaimDailyCheck reports whether one more Jev check fits under
// AISPAM_DAILY_LIMIT for the current UTC day, counting it when the cap is
// set. With no cap it always does and counts nothing.
func aispamClaimDailyCheck() bool {
	limit := int64(config.AppConfig.AISpamDailyLimit)
	if limit <= 0 {
		return true
	}

	day := aispamDailyNow().UTC().Format(time.DateOnly)
	if aispamCountDailyCheck(day) <= limit {
		return true
	}

	aispamLocalDaily.mu.Lock()
	first := aispamLocalDaily.warned != day
	aispamLocalDaily.warned = day
	aispamLocalDaily.mu.Unlock()
	if first {
		log.WithFields(log.Fields{"limit": limit, "day": day}).
			Warn("[AISpam] AISPAM_DAILY_LIMIT reached, no AI spam checks until the next UTC day")
	}
	return false
}

// aispamCountDailyCheck adds one check to the day's count and returns it:
// the count shared through Redis, or this process's own without it.
func aispamCountDailyCheck(day string) int64 {
	if rdb := cache.GetRedisClient(); rdb != nil {
		count, err := aispamDailyScript.Run(cache.Context, rdb, []string{aispamDailyKey(day)}, int(aispamDailyKeyTTL.Seconds())).Int64()
		if err == nil {
			return count
		}
		log.WithError(err).Debug("[AISpam] daily counter unavailable, counting in this process")
	}

	aispamLocalDaily.mu.Lock()
	defer aispamLocalDaily.mu.Unlock()
	if aispamLocalDaily.day != day {
		aispamLocalDaily.day = day
		aispamLocalDaily.count = 0
	}
	aispamLocalDaily.count++
	return aispamLocalDaily.count
}
