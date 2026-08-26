package modules

import (
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// spamKey is a composite key for rate limiting per user per chat
type spamKey struct {
	chatId int64
	userId int64
}

const (
	antiSpamLimit  = 18
	antiSpamWindow = time.Second
)

// antiSpamInfo tracks a user's current fixed spam window.
type antiSpamInfo struct {
	Count       int
	WindowStart time.Time
}

type spamShard struct {
	mu sync.Mutex
	m  map[spamKey]*antiSpamInfo
}

var antiSpamShards [16]spamShard

func shardFor(key spamKey) *spamShard {
	// Mix both chat and user to spread users in the same chat across shards.
	hash := uint64(key.chatId)*31 + uint64(key.userId)
	return &antiSpamShards[hash%16]
}

func init() {
	for i := range antiSpamShards {
		antiSpamShards[i].m = make(map[spamKey]*antiSpamInfo)
	}
	RegisterLegacyModule("Antispam", 10, LoadAntispam)
	go antiSpamCleanupLoop()
}

// antiSpamCleanupLoop periodically removes expired entries to prevent memory leaks.
func antiSpamCleanupLoop() {
	defer error_handling.RecoverFromPanic("antiSpamCleanupLoop", "antispam")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		func() {
			defer error_handling.RecoverFromPanic("antiSpamCleanupTick", "antispam")
			cleanupExpiredAntiSpam(time.Now())
		}()
	}
}

func cleanupExpiredAntiSpam(now time.Time) {
	for i := range antiSpamShards {
		func() {
			shard := &antiSpamShards[i]
			shard.mu.Lock()
			defer shard.mu.Unlock()
			for key, info := range shard.m {
				if info == nil || now.Sub(info.WindowStart) >= 2*antiSpamWindow {
					delete(shard.m, key)
				}
			}
		}()
	}
}

// spamCheck performs spam detection for a specific user in a chat.
// The eighteenth message within one second is spam.
func spamCheck(key spamKey) bool {
	shard := shardFor(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	now := time.Now()
	info, ok := shard.m[key]
	if !ok || info == nil {
		info = &antiSpamInfo{Count: 1, WindowStart: now}
		shard.m[key] = info
		return false
	}

	if now.Sub(info.WindowStart) >= antiSpamWindow {
		info.WindowStart = now
		info.Count = 0
	}

	info.Count++
	return info.Count >= antiSpamLimit
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
				// Skip approved users (immune to anti-spam)
				if chat_status.IsApproved(bot, ctx.EffectiveChat.Id, ctx.EffectiveUser.Id) {
					return ext.ContinueGroups
				}
				// Skip admins: every other anti-abuse module exempts admins, and
				// silently dropping an admin's messages (including legitimate
				// command bursts) is worse than letting them through.
				if chat_status.IsUserAdmin(bot, ctx.EffectiveChat.Id, ctx.EffectiveUser.Id) {
					return ext.ContinueGroups
				}

				key := spamKey{
					chatId: ctx.EffectiveChat.Id,
					userId: ctx.EffectiveUser.Id,
				}

				if spamCheck(key) {
					log.Debugf("[Antispam] Rate limited user=%d chat=%d",
						ctx.EffectiveUser.Id, ctx.EffectiveChat.Id)
				}
				return ext.ContinueGroups
			},
		), -2,
	)
}
