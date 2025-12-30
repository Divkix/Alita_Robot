package modules

import (
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"
)

var (
	antiSpamMutex sync.Mutex
	antiSpamMap   = make(map[spamKey]*antiSpamInfo)
)

func init() {
	go antiSpamCleanupLoop()
}

// antiSpamCleanupLoop periodically removes expired entries to prevent memory leaks.
func antiSpamCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		antiSpamMutex.Lock()
		now := time.Now()
		for key, info := range antiSpamMap {
			allExpired := true
			for _, level := range info.Levels {
				if now.Sub(level.CurrTime) < level.Expiry*2 {
					allExpired = false
					break
				}
			}
			if allExpired {
				delete(antiSpamMap, key)
			}
		}
		antiSpamMutex.Unlock()
	}
}

// checkSpammed evaluates if a user in a chat has exceeded spam detection levels.
// Returns true if any configured spam threshold has been violated.
func checkSpammed(key spamKey, levels []antiSpamLevel) bool {
	antiSpamMutex.Lock()
	defer antiSpamMutex.Unlock()

	info, ok := antiSpamMap[key]
	if !ok {
		// Initialize with current time and count=1 (first message counts)
		info = &antiSpamInfo{
			Levels: make([]antiSpamLevel, len(levels)),
		}
		for i, lvl := range levels {
			info.Levels[i] = antiSpamLevel{
				Count:    1,
				Limit:    lvl.Limit,
				CurrTime: time.Now(),
				Expiry:   lvl.Expiry,
				Spammed:  false,
			}
		}
		antiSpamMap[key] = info
		return false
	}

	var spammed bool
	for i := range info.Levels {
		level := &info.Levels[i]

		// Reset if window expired
		if time.Since(level.CurrTime) >= level.Expiry {
			level.CurrTime = time.Now()
			level.Count = 0
			level.Spammed = false
		}

		level.Count++
		if level.Count >= level.Limit {
			level.Spammed = true
			spammed = true
		}
	}

	return spammed
}

// spamCheck performs spam detection for a specific user in a chat.
// Checks against a default threshold of 18 messages per second.
func spamCheck(key spamKey) bool {
	return checkSpammed(key, []antiSpamLevel{
		{
			Limit:  18,
			Expiry: time.Second,
		},
	})
}

// LoadAntispam registers the antispam message handler with the dispatcher.
// Sets up spam detection monitoring for all incoming messages.
func LoadAntispam(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(
			message.All,
			func(bot *gotgbot.Bot, ctx *ext.Context) error {
				// Skip if no user (channel posts, etc.)
				if ctx.EffectiveUser == nil {
					return ext.ContinueGroups
				}

				key := spamKey{
					chatId: ctx.EffectiveChat.Id,
					userId: ctx.EffectiveUser.Id,
				}

				if spamCheck(key) {
					log.Debugf("[Antispam] Rate limited user=%d chat=%d",
						ctx.EffectiveUser.Id, ctx.EffectiveChat.Id)
					return ext.EndGroups
				}
				return ext.ContinueGroups
			},
		), -2,
	)
}
