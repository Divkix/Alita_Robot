// Package stats keeps per-chat activity counters in process memory and writes
// them to chat_daily_stats, chat_hourly_stats and chat_member_daily_stats in
// one batched, transactional upsert every 60 seconds.
//
// Counting never touches the database: one sync.Map lookup and one atomic add
// per message, join or leave, plus one sharded map increment per message from
// a real user (members.go). A crash therefore loses at most the events since
// the last flush (under a minute). That is a deliberate trade: a database
// write per message would make chat_daily_stats the busiest table in the
// database, and the digest only needs approximate numbers.
//
// No Redis is involved, so a cache-less deployment still counts.
package stats

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

const (
	dayLayout     = "2006-01-02"
	flushInterval = time.Minute
)

type metric uint8

const (
	metricMessages metric = iota
	metricJoins
	metricLeaves
)

// statKey identifies one counter: a chat, a UTC day and hour, and one metric.
// The daily rows sum a day's hours; the hourly rows keep messages per hour.
type statKey struct {
	chatID int64
	day    string
	hour   int16
	metric metric
}

var (
	// counters holds *atomic.Int64 values. The hot path loads by key; only a
	// missed key creates one.
	counters sync.Map

	// timeNow is the clock the counters use. Tests replace it to pin the day
	// key; production never writes it after startup.
	timeNow = time.Now

	// flushMu serialises flushes so the ticker, shutdown and /digest now
	// cannot drain and write the same deltas twice.
	flushMu sync.Mutex

	flusherMu     sync.Mutex
	flusherCancel context.CancelFunc
	flusherWG     sync.WaitGroup
)

// Day formats t as the UTC day key stored in chat_daily_stats.
func Day(t time.Time) string {
	return t.UTC().Format(dayLayout)
}

// hourOf returns t's UTC hour, the hour key stored in chat_hourly_stats.
func hourOf(t time.Time) int16 {
	return int16(t.UTC().Hour())
}

func counter(key statKey) *atomic.Int64 {
	if value, ok := counters.Load(key); ok {
		return value.(*atomic.Int64)
	}
	value, _ := counters.LoadOrStore(key, new(atomic.Int64))
	return value.(*atomic.Int64)
}

func record(chatID int64, m metric) {
	now := timeNow()
	counter(statKey{chatID: chatID, day: Day(now), hour: hourOf(now), metric: m}).Add(1)
}

// RecordMessage counts one group message.
func RecordMessage(chatID int64) {
	record(chatID, metricMessages)
}

// RecordJoin counts one membership change into the group.
func RecordJoin(chatID int64) {
	record(chatID, metricJoins)
}

// RecordLeave counts one membership change out of the group.
func RecordLeave(chatID int64) {
	record(chatID, metricLeaves)
}

// Flush drains the counters and writes them as one additive upsert. When the
// write fails the drained deltas are put back, so the next flush retries them
// instead of losing them.
func Flush() error {
	flushMu.Lock()
	defer flushMu.Unlock()

	now := timeNow()
	if dropped := memberDropped.Swap(0); dropped > 0 {
		log.Warnf("[Stats] %d member messages were not counted: more than %d members posted in one flush interval",
			dropped, memberCounterLimit)
	}

	deltas := drain()
	memberDeltas := drainMembers()
	if len(deltas) == 0 && len(memberDeltas) == 0 {
		return nil
	}

	batch := chatstats.Batch{
		Daily:   rowsFromDeltas(deltas),
		Hourly:  hourlyRowsFromDeltas(deltas),
		Members: memberRowsFromDeltas(memberDeltas),
	}
	if err := chatstats.Add(batch); err != nil {
		for key, delta := range deltas {
			counter(key).Add(delta)
		}
		restoreMembers(memberDeltas)
		return err
	}

	pruneBefore(Day(now), hourOf(now))
	return nil
}

// drain swaps every counter to zero and returns the nonzero deltas.
func drain() map[statKey]int64 {
	deltas := make(map[statKey]int64)
	counters.Range(func(key, value any) bool {
		if delta := value.(*atomic.Int64).Swap(0); delta != 0 {
			deltas[key.(statKey)] = delta
		}
		return true
	})
	return deltas
}

// rowsFromDeltas folds the per-hour, per-metric deltas into one row per chat
// and day.
func rowsFromDeltas(deltas map[statKey]int64) []models.ChatDailyStat {
	type rowKey struct {
		chatID int64
		day    string
	}

	order := make([]*models.ChatDailyStat, 0, len(deltas))
	byRow := make(map[rowKey]*models.ChatDailyStat, len(deltas))
	for key, delta := range deltas {
		rk := rowKey{chatID: key.chatID, day: key.day}
		row, ok := byRow[rk]
		if !ok {
			row = &models.ChatDailyStat{ChatID: key.chatID, Day: key.day}
			byRow[rk] = row
			order = append(order, row)
		}
		switch key.metric {
		case metricMessages:
			row.Messages += delta
		case metricJoins:
			row.Joins += delta
		case metricLeaves:
			row.Leaves += delta
		}
	}

	rows := make([]models.ChatDailyStat, 0, len(order))
	for _, row := range order {
		rows = append(rows, *row)
	}
	return rows
}

// hourlyRowsFromDeltas keeps the message deltas, one row per chat, day and
// hour. Joins and leaves have no hourly table.
func hourlyRowsFromDeltas(deltas map[statKey]int64) []models.ChatHourlyStat {
	rows := make([]models.ChatHourlyStat, 0, len(deltas))
	for key, delta := range deltas {
		if key.metric != metricMessages {
			continue
		}
		rows = append(rows, models.ChatHourlyStat{
			ChatID:   key.chatID,
			Day:      key.day,
			Hour:     key.hour,
			Messages: delta,
		})
	}
	return rows
}

// pruneBefore drops drained counters for hours before the current one, so the
// map does not keep one entry per chat and metric per hour forever. An event
// for a past hour that races the delete can be lost; that is inside the same
// at-most-one-minute trade the package documents.
func pruneBefore(today string, hour int16) {
	counters.Range(func(key, value any) bool {
		k := key.(statKey)
		past := k.day < today || (k.day == today && k.hour < hour)
		if past && value.(*atomic.Int64).Load() == 0 {
			counters.CompareAndDelete(key, value)
		}
		return true
	})
}

// StartFlusher starts the 60-second flush loop. Call it once; a second call
// does nothing.
func StartFlusher() {
	flusherMu.Lock()
	defer flusherMu.Unlock()
	if flusherCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	flusherCancel = cancel
	flusherWG.Add(1)
	go func() {
		defer flusherWG.Done()
		defer error_handling.RecoverFromPanic("statsFlusher", "stats")

		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				func() {
					defer error_handling.RecoverFromPanic("statsFlush", "stats")
					if err := Flush(); err != nil {
						log.Errorf("[Stats] daily counter flush failed: %v", err)
					}
				}()
			}
		}
	}()
}

// StopFlusher stops the flush loop and flushes the counters it still holds.
// It is idempotent: a second call only flushes whatever is pending.
func StopFlusher() error {
	flusherMu.Lock()
	cancel := flusherCancel
	flusherCancel = nil
	flusherMu.Unlock()

	if cancel != nil {
		cancel()
		flusherWG.Wait()
	}
	return Flush()
}
