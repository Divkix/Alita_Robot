package stats

import (
	"sync"
	"sync/atomic"

	"github.com/divkix/Alita_Robot/alita/db/models"
)

// memberShardCount spreads the hot path over several locks, so one busy chat
// does not serialise every other chat's messages.
const memberShardCount = 32

// memberCounterLimit caps the distinct (chat, user, day) counters one replica
// holds between two flushes. Each flush empties the map, so the cap bounds
// memory per minute, not per day: past it, a user not already counted in this
// minute is not counted until the next flush. The check is not atomic across
// shards, so the map can overshoot by a few entries; it is a memory bound, not
// an exact number. Tests lower it; production never writes it.
var memberCounterLimit int64 = 100_000

// memberKey identifies one per-member counter: a chat, a user and a UTC day.
type memberKey struct {
	chatID int64
	userID int64
	day    string
}

// memberShard is one lock and the counters it guards. A flush swaps counts
// for nil under the lock, so a drained key is gone at once and no increment
// can land on a counter the flush already read.
type memberShard struct {
	mu     sync.Mutex
	counts map[memberKey]int64
}

var (
	memberShards [memberShardCount]memberShard

	// memberKeys is the number of keys across all shards, for the cap.
	memberKeys atomic.Int64

	// memberDropped counts messages not counted because of the cap since the
	// last flush; the flush logs it.
	memberDropped atomic.Int64
)

func shardFor(chatID, userID int64) *memberShard {
	h := uint64(chatID)*31 + uint64(userID)
	return &memberShards[h%memberShardCount]
}

// RecordMemberMessage counts one group message by a real user. Callers skip
// channel senders and anonymous admins: they have no user to rank.
func RecordMemberMessage(chatID, userID int64) {
	if userID <= 0 {
		return
	}
	key := memberKey{chatID: chatID, userID: userID, day: Day(timeNow())}
	shard := shardFor(chatID, userID)

	shard.mu.Lock()
	defer shard.mu.Unlock()
	if _, ok := shard.counts[key]; ok {
		shard.counts[key]++
		return
	}
	if memberKeys.Load() >= memberCounterLimit {
		memberDropped.Add(1)
		return
	}
	if shard.counts == nil {
		shard.counts = make(map[memberKey]int64)
	}
	shard.counts[key] = 1
	memberKeys.Add(1)
}

// drainMembers empties every shard and returns what they held.
func drainMembers() map[memberKey]int64 {
	deltas := make(map[memberKey]int64)
	for i := range memberShards {
		shard := &memberShards[i]
		shard.mu.Lock()
		counts := shard.counts
		shard.counts = nil
		shard.mu.Unlock()

		memberKeys.Add(-int64(len(counts)))
		for key, delta := range counts {
			deltas[key] = delta
		}
	}
	return deltas
}

// restoreMembers adds drained deltas back after a failed write. It ignores the
// cap: the counts were already taken, and dropping them would lose them.
func restoreMembers(deltas map[memberKey]int64) {
	for key, delta := range deltas {
		shard := shardFor(key.chatID, key.userID)
		shard.mu.Lock()
		if shard.counts == nil {
			shard.counts = make(map[memberKey]int64)
		}
		if _, ok := shard.counts[key]; !ok {
			memberKeys.Add(1)
		}
		shard.counts[key] += delta
		shard.mu.Unlock()
	}
}

// memberRowsFromDeltas turns the drained counters into rows.
func memberRowsFromDeltas(deltas map[memberKey]int64) []models.ChatMemberDailyStat {
	rows := make([]models.ChatMemberDailyStat, 0, len(deltas))
	for key, delta := range deltas {
		rows = append(rows, models.ChatMemberDailyStat{
			ChatID:   key.chatID,
			Day:      key.day,
			UserID:   key.userID,
			Messages: delta,
		})
	}
	return rows
}
