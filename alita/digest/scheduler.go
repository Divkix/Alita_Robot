package digest

import (
	"context"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/panel"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// Scheduler sends each opted-in chat the previous week's digest once.
//
// A tick lists the chats whose digest is due, claims each send with a
// conditional update on digest_settings.last_sent_at, and sends only when the
// claim won. Because the claim is the database row, two replicas ticking at
// the same minute send one message, not two, with no Redis involved. A failed
// send restores the claim and the next tick retries it, unless the failure is
// permanent (the bot was removed, the chat is gone or the bot may not send
// there): then the week counts as done and the failure is logged once.
type Scheduler struct {
	bot      *gotgbot.Bot
	interval time.Duration
	// now is the clock a tick reads. Tests replace it to pin the week
	// boundary and the claim instant; production never writes it after
	// construction.
	now func() time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewScheduler returns a scheduler for bot. It does not start ticking.
func NewScheduler(bot *gotgbot.Bot) *Scheduler {
	interval := config.DefaultDigestInterval
	if config.AppConfig != nil && config.AppConfig.DigestInterval > 0 {
		interval = config.AppConfig.DigestInterval
	}
	return &Scheduler{bot: bot, interval: interval, now: time.Now}
}

// Start begins the ticker. Calling it twice does nothing.
func (s *Scheduler) Start() {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer error_handling.RecoverFromPanic("digestScheduler", "digest")

			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					func() {
						defer error_handling.RecoverFromPanic("digestSchedulerTick", "digest")
						s.runDue(s.now())
					}()
				}
			}
		}()
	})
}

// Stop stops the ticker and waits for a running tick. It is idempotent and
// safe on a scheduler that was never started.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}

// runDue sends the digest for the week that just ended to every chat due one.
// The completed week is [WeekStart(now)-7d, WeekStart(now)); a chat is due when
// its digest is on and last_sent_at is nil or before that boundary.
func (s *Scheduler) runDue(now time.Time) {
	if config.AppConfig == nil || !config.AppConfig.EnableDigest {
		return
	}

	boundary := WeekStart(now)
	due, err := digestsettings.ListDue(boundary)
	if err != nil {
		log.Errorf("[Digest] could not list chats due a digest: %v", err)
		return
	}
	for i := range due {
		s.send(&due[i], boundary)
	}
}

// send claims and delivers one chat's digest. A lost claim means another
// replica already sent it this week.
func (s *Scheduler) send(settings *models.DigestSettings, boundary time.Time) {
	claimedAt := s.now().UTC()
	won, err := digestsettings.Claim(settings.ChatID, settings.LastSentAt, claimedAt)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": settings.ChatID, "error": err}).
			Error("[Digest] could not claim the weekly digest")
		return
	}
	if !won {
		return
	}

	if err := s.deliver(settings, boundary.AddDate(0, 0, -7), boundary); err != nil {
		// A permanent failure cannot be fixed by retrying: keep the claim so
		// the week counts as done and the next week starts clean.
		if helpers.IsPermanentSendError(err) {
			log.WithFields(log.Fields{"chat_id": settings.ChatID, "error": err}).
				Warn("[Digest] week's digest is undeliverable; not retrying")
			return
		}
		log.WithFields(log.Fields{"chat_id": settings.ChatID, "error": err}).
			Error("[Digest] could not send the weekly digest")
		if restoreErr := digestsettings.Restore(settings.ChatID, claimedAt, settings.LastSentAt); restoreErr != nil {
			log.WithFields(log.Fields{"chat_id": settings.ChatID, "error": restoreErr}).
				Error("[Digest] could not restore the digest claim")
		}
	}
}

// deliver reads the two weeks, renders the message in the chat's language and
// sends it to the resolved destination.
func (s *Scheduler) deliver(settings *models.DigestSettings, from, to time.Time) error {
	current, err := ReadStats(settings.ChatID, from, to)
	if err != nil {
		return err
	}
	previous, err := ReadStats(settings.ChatID, from.AddDate(0, 0, -7), from)
	if err != nil {
		return err
	}

	logChannel := logchannels.Get(settings.ChatID)
	hasLogChannel := logChannel != nil && logChannel.LogChannelID != 0
	target := settings.ChatID
	if ResolveDestination(settings.Destination, hasLogChannel) == digestsettings.DestinationLog {
		target = logChannel.LogChannelID
	}

	tr := i18n.MustNewTranslator(lang.GetGroupLanguage(context.Background(), settings.ChatID))
	dashboardURL := ""
	if config.AppConfig != nil && config.AppConfig.PanelEnabled {
		dashboardURL = panel.EntryURL(s.bot.Username, config.AppConfig.PanelMiniAppShortName, settings.ChatID)
	}
	text, keyboard := Build(tr, current, previous, from, dashboardURL)

	opts := formatting.Shtml()
	if keyboard != nil {
		opts.ReplyMarkup = keyboard
	}
	_, err = s.bot.SendMessage(target, text, opts)
	return err
}
