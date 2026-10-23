// Package membersnapshot records each active chat's Telegram member count
// once per UTC day into chat_member_counts, the history behind the panel's
// member growth chart. Telegram gives bots no member history, so the series
// starts on the first day this runs.
package membersnapshot

import (
	"context"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/jobs"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

const (
	// defaultInterval is how often a tick looks for chats without today's
	// count. Only the first tick of a day (and chats first active later that
	// day) finds work; the rest run one indexed query.
	defaultInterval = 10 * time.Minute

	// defaultPace is the pause between two getChatMemberCount calls, so a
	// tick never spends more than about 10 calls a second of the bot's API
	// budget.
	defaultPace = 100 * time.Millisecond

	// tickLimit caps the chats one tick measures. At defaultPace that is
	// under 500 s, inside one interval; a larger install catches up over the
	// next ticks of the same day.
	tickLimit = 5000
)

// Snapshotter measures each chat that had activity yesterday or today (UTC)
// once per UTC day.
//
// The chat_member_counts row is the claim: a replica inserts it with no count
// and calls Telegram only when its insert won, so two replicas ticking at the
// same minute make one call per chat and day. A transient failure deletes the
// claim and the next tick retries; a permanent one (the bot left, the chat is
// gone) keeps it, so the chat is not retried until the next day. A crash
// between the claim and the write leaves that day without a count.
type Snapshotter struct {
	bot      *gotgbot.Bot
	interval time.Duration
	pace     time.Duration
	// now is the clock a tick reads. Tests replace it to pin the day;
	// production never writes it after construction.
	now func() time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// New returns a snapshotter for bot. It does not start ticking.
func New(bot *gotgbot.Bot) *Snapshotter {
	return &Snapshotter{bot: bot, interval: defaultInterval, pace: defaultPace, now: time.Now}
}

// Start begins the ticker. Calling it twice does nothing.
func (s *Snapshotter) Start() {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer error_handling.RecoverFromPanic("memberSnapshotter", "membersnapshot")

			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					jobs.Run(ctx, "memberSnapshotterTick", func(ctx context.Context) error {
						s.runDue(ctx, s.now())
						return nil
					})
				}
			}
		}()
	})
}

// Stop stops the ticker and waits for a running tick, which stops between two
// chats (an in-flight call is cancelled and its claim released). It is
// idempotent and safe on a snapshotter that was never started.
func (s *Snapshotter) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}

// runDue measures every chat due a count for now's UTC day.
func (s *Snapshotter) runDue(ctx context.Context, now time.Time) {
	day := stats.Day(now)
	since := stats.Day(now.AddDate(0, 0, -1))
	chatIDs, err := chatstats.ListChatsDueMemberCount(since, day, tickLimit)
	if err != nil {
		log.Errorf("[MemberSnapshot] could not list chats due a member count: %v", err)
		return
	}

	for i, chatID := range chatIDs {
		if ctx.Err() != nil {
			return
		}
		if i > 0 && !s.wait(ctx) {
			return
		}
		s.snapshot(ctx, chatID, day)
	}
}

// wait sleeps for the pace and reports false when ctx ended first.
func (s *Snapshotter) wait(ctx context.Context) bool {
	if s.pace <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(s.pace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// snapshot claims, measures and stores one chat's count for day.
func (s *Snapshotter) snapshot(ctx context.Context, chatID int64, day string) {
	fields := log.Fields{"chat_id": chatID, "day": day}
	won, err := chatstats.ClaimMemberCount(chatID, day)
	if err != nil {
		log.WithFields(fields).WithError(err).Error("[MemberSnapshot] could not claim the member count")
		return
	}
	if !won {
		return
	}

	count, err := s.bot.GetChatMemberCountWithContext(ctx, chatID, nil)
	if err != nil {
		if helpers.IsPermanentSendError(err) {
			log.WithFields(fields).WithError(err).Debug("[MemberSnapshot] chat cannot be measured; not retrying today")
			return
		}
		log.WithFields(fields).WithError(err).Warn("[MemberSnapshot] could not read the member count")
		if releaseErr := chatstats.ReleaseMemberCount(chatID, day); releaseErr != nil {
			log.WithFields(fields).WithError(releaseErr).Error("[MemberSnapshot] could not release the member count claim")
		}
		return
	}

	if err := chatstats.SetMemberCount(chatID, day, count); err != nil {
		log.WithFields(fields).WithError(err).Error("[MemberSnapshot] could not store the member count")
		// Release so the next tick retries instead of leaving the day empty.
		if releaseErr := chatstats.ReleaseMemberCount(chatID, day); releaseErr != nil {
			log.WithFields(fields).WithError(releaseErr).Error("[MemberSnapshot] could not release the member count claim")
		}
	}
}
