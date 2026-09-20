package modules

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

const (
	// aispamHandlerGroup sits after the users tracker (-1), so parent chat and
	// user rows exist, and before antiflood (4), so model calls are not spent
	// on floods that are about to be punished.
	aispamHandlerGroup = 3

	aispamQueueCapacity = 256
	aispamWorkerCount   = 4

	// Jev is English-first, so only a message the model reads as English acts
	// on the lower bar. The message's own language decides, not the chat's
	// setting: an English chat still receives posts in other languages.
	aispamThresholdEnglish    = 0.8
	aispamThresholdNonEnglish = 0.9

	// aispamLanguageEnglish is the language answer that takes the lower bar.
	// Every other answer, including none at all, takes the higher one.
	aispamLanguageEnglish = "english"

	aispamBreakerFailures = 5

	aispamHistoryTexts         = 5
	aispamHistoryMaxTimestamps = 200
	aispamHistoryTextLen       = 400

	aispamMaxTitleLen       = 200
	aispamMaxDescriptionLen = 500
	aispamMaxRulesLen       = 1500
	aispamMaxMessageLen     = 1000
	aispamMaxLinkDomains    = 5

	aispamDescriptionMaxChats = 5000
	aispamBreakerMaxChats     = 10000
)

// Test-tunable so a breaker cooldown does not have to be waited out.
var (
	aispamBreakerCooldown = 5 * time.Minute
	aispamDescriptionTTL  = 10 * time.Minute
	// aispamHistoryWindow bounds the counted window and the key lifetime alike.
	aispamHistoryWindow = time.Hour
)

var (
	aispamChecked atomic.Int64
	aispamDeleted atomic.Int64
	aispamKept    atomic.Int64
	aispamShed    atomic.Int64
	aispamFailed  atomic.Int64
)

// GetAISpamStats reports the AI spam filter's counters. checked counts the
// checks that produced a verdict; shed and failed count the ones that did not.
func GetAISpamStats() (checked, deleted, kept, shed, failed int64) {
	return aispamChecked.Load(), aispamDeleted.Load(), aispamKept.Load(), aispamShed.Load(), aispamFailed.Load()
}

var aispamModule = moduleStruct{
	moduleName:   "AISpam",
	handlerGroup: aispamHandlerGroup,
}

var aispamDesc = helpers.CommandDescriptor{
	Name:        "aispam",
	Group:       aispamModule.handlerGroup,
	Disableable: true,
	RequiredChecks: []helpers.CheckFunc{
		helpers.CheckDisabled("aispam"),
		helpers.RequireGroup(),
		helpers.RequireUserAdmin(),
	},
}

// aispamTask carries the cheap facts gathered on the update path. Everything
// that costs a database or network read is resolved by the worker instead.
type aispamTask struct {
	bot         *gotgbot.Bot
	chatID      int64
	messageID   int64
	userID      int64
	userName    string
	text        string
	hasLink     bool
	linkDomains []string
	isForward   bool
	isReply     bool
	title       string
	language    string
}

func (t aispamTask) state(recent []string, lastHour int) aispamState {
	return aispamState{
		Chat: aispamChatState{
			Title:       aispamTruncate(t.title, aispamMaxTitleLen),
			Description: aispamTruncate(aispamChatDescription(t.bot, t.chatID), aispamMaxDescriptionLen),
			Rules:       aispamTruncate(aispamPlainText(rules.GetChatRulesInfo(t.chatID).Rules), aispamMaxRulesLen),
			Language:    t.language,
		},
		Sender: aispamSenderState{
			FirstSeenDaysAgo: aispamFirstSeenDaysAgo(t.userID),
			MessagesLastHour: lastHour,
			WarnCount:        aispamWarnCount(t.userID, t.chatID),
		},
		Message: aispamMessageState{
			Text:        aispamTruncate(t.text, aispamMaxMessageLen),
			HasLink:     t.hasLink,
			LinkDomains: t.linkDomains,
			IsForward:   t.isForward,
			IsReply:     t.isReply,
		},
		RecentFromSender: recent,
	}
}

// checkAISpam is the group 3 watcher. Every message is either carved out here,
// before anything is enqueued, or handed to the worker pool. It always returns
// ContinueGroups: AI moderation is never allowed to consume an update.
func (m moduleStruct) checkAISpam(b *gotgbot.Bot, ctx *ext.Context) error {
	if !config.AppConfig.EnableAISpam || config.AppConfig.TypeSafeAPIKey == "" {
		return ext.ContinueGroups
	}

	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	text := aispamMessageText(msg)
	if !aispamEligible(b.Username, chat, msg, ctx.EffectiveSender, user, text) {
		return ext.ContinueGroups
	}
	if !aispam.IsAISpamEnabled(chat.Id) || aispamBreakerPaused(chat.Id) {
		return ext.ContinueGroups
	}
	if aispamCarvedOut(b, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	domains := aispamLinkDomains(msg, text)

	aispamEnqueue(aispamTask{
		bot:         b,
		chatID:      chat.Id,
		messageID:   msg.MessageId,
		userID:      user.Id,
		userName:    formatting.GetFullName(user.FirstName, user.LastName),
		text:        text,
		hasLink:     len(domains) > 0,
		linkDomains: domains,
		isForward:   msg.ForwardOrigin != nil,
		isReply:     msg.ReplyToMessage != nil,
		title:       chat.Title,
		language:    lang.GetLanguage(ctx),
	})

	return ext.ContinueGroups
}

// aispamEligible reports whether the update is even a candidate: a group text
// message from a real user. Channel posts, anonymous admins, bots, media with
// no text and this bot's own commands are not conversation the model can judge.
func aispamEligible(botUsername string, chat *gotgbot.Chat, msg *gotgbot.Message, sender *gotgbot.Sender, user *gotgbot.User, text string) bool {
	if chat == nil || msg == nil || sender == nil {
		return false
	}
	if chat.Type != "group" && chat.Type != "supergroup" {
		return false
	}
	if !sender.IsUser() || sender.IsAnonymousAdmin() || sender.IsAnonymousChannel() || sender.IsLinkedChannel() {
		return false
	}
	if user == nil || user.IsBot {
		return false
	}
	return text != "" && !aispamIsCommandForBot(text, botUsername)
}

// aispamIsCommandForBot reports whether the message is one of this bot's own
// commands: a bare /command, or /command@thisbot. A slash-prefixed message
// aimed at another bot is not ours, and is still judged.
func aispamIsCommandForBot(text, botUsername string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}

	name, addressedTo, addressed := strings.Cut(fields[0], "@")
	if len(name) < 2 || name[0] != '/' {
		return false
	}
	if !addressed {
		return true
	}
	return botUsername != "" && strings.EqualFold(addressedTo, botUsername)
}

// aispamCarvedOut reports the carve-outs that need a lookup: members still
// solving captcha belong to the captcha module, and admins and approved users
// are trusted by definition.
func aispamCarvedOut(b *gotgbot.Bot, chatID, userID int64) bool {
	if attempt, err := captcha.GetCaptchaAttempt(userID, chatID); err == nil && attempt != nil {
		return true
	}
	return chat_status.IsUserAdmin(b, chatID, userID) || chat_status.IsApproved(b, chatID, userID)
}

func aispamMessageText(msg *gotgbot.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Text != "" {
		return msg.Text
	}
	return msg.Caption
}

// processAISpam runs one check to completion. Failures fail open: a timeout,
// rate limit, server error or unparseable answer produces no verdict and no
// action, and counts towards the chat's breaker.
func processAISpam(task aispamTask) {
	// The window is read and written here rather than on the update path, so a
	// Redis round trip never sits between a message arriving and the bot
	// answering it. A message is recorded even when the check that follows
	// fails: the next check still sees it.
	recent, lastHour := observeAISpamSender(task.chatID, task.userID, task.messageID, aispamTruncate(task.text, aispamHistoryTextLen), time.Now())

	verdict, err := aispamJevDecide(context.Background(), config.AppConfig.TypeSafeAPIKey, task.state(recent, lastHour))
	if err != nil {
		aispamFailed.Add(1)
		log.WithFields(log.Fields{
			"chat_id":    task.chatID,
			"user_id":    task.userID,
			"message_id": task.messageID,
			"error":      err,
		}).Warn("[AISpam] check failed, no action taken")

		if aispamBreakerRecordFailure(task.chatID) {
			aispamNotifyBreaker(task.bot, task.chatID, task.language)
		}
		return
	}
	aispamBreakerRecordSuccess(task.chatID)
	aispamChecked.Add(1)

	// The message's own language picks the bar, not the chat's setting: Jev is
	// English-first, and an English chat still receives posts written in other
	// languages. An answer the module cannot read takes the higher bar.
	threshold := aispamThresholdNonEnglish
	if verdict.Language == aispamLanguageEnglish {
		threshold = aispamThresholdEnglish
	}
	deleting := verdict.DeleteProbability >= threshold

	log.WithFields(log.Fields{
		"chat_id":            task.chatID,
		"user_id":            task.userID,
		"message_id":         task.messageID,
		"decision":           aispamDecisionName(deleting),
		"probability":        verdict.DeleteProbability,
		"threshold":          threshold,
		"category":           verdict.Category,
		"message_language":   verdict.Language,
		"model":              verdict.Model,
		"input_tokens":       verdict.InputTokens,
		"output_tokens":      verdict.OutputTokens,
		"messages_last_hour": lastHour,
	}).Info("[AISpam] verdict")

	if !deleting {
		aispamKept.Add(1)
		return
	}

	aispamDeleted.Add(1)
	// Tolerates an already-deleted message, so racing another watcher is fine.
	if err := helpers.DeleteMessageWithErrorHandling(task.bot, task.chatID, task.messageID); err != nil {
		log.WithFields(log.Fields{
			"chat_id":    task.chatID,
			"message_id": task.messageID,
			"error":      err,
		}).Warn("[AISpam] failed to delete a message the model flagged as spam")
	}

	aispamMirrorDeletion(task, verdict)
}

func aispamDecisionName(deleting bool) string {
	if deleting {
		return "delete"
	}
	return "keep"
}

// aispamNotifyBreaker tells the chat once that checks have paused, so a broken
// key is visible without reading logs. It posts in the chat because the Bot API
// has no admin-only channel, and a DM per admin is N calls that silently fail
// for admins who block the bot.
func aispamNotifyBreaker(b *gotgbot.Bot, chatID int64, language string) {
	text, _ := i18n.MustNewTranslator(language).GetString("aispam_breaker_notice")
	if text == "" {
		return
	}
	if _, err := helpers.SendMessageWithErrorHandling(b, chatID, text, formatting.Shtml()); err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "error": err}).Debug("[AISpam] breaker notice failed")
	}
}

func aispamMirrorDeletion(task aispamTask, verdict *aispamVerdict) {
	dump := config.AppConfig.MessageDump
	if dump == 0 {
		return
	}

	text := fmt.Sprintf(
		"<b>AI spam filter: message deleted</b>\n"+
			"<b>Chat:</b> %s (<code>%d</code>)\n"+
			"<b>Sender:</b> %s (<code>%d</code>)\n"+
			"<b>Probability:</b> %.2f · category: %s\n"+
			"<b>Model:</b> %s · tokens %d/%d\n"+
			"<b>Message:</b>\n%s",
		formatting.HtmlEscape(task.title), task.chatID,
		formatting.HtmlEscape(task.userName), task.userID,
		verdict.DeleteProbability, formatting.HtmlEscape(verdict.Category),
		formatting.HtmlEscape(verdict.Model), verdict.InputTokens, verdict.OutputTokens,
		formatting.HtmlEscape(aispamTruncate(task.text, aispamMaxMessageLen)),
	)

	if _, err := helpers.SendMessageWithErrorHandling(task.bot, dump, text, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
	}); err != nil {
		log.WithFields(log.Fields{"chat_id": task.chatID, "error": err}).Debug("[AISpam] audit mirror failed")
	}
}

//
// Queue and workers
//

var (
	aispamWorkersOnce sync.Once
	aispamQueue       chan aispamTask
	aispamInFlight    sync.WaitGroup
	// aispamDraining is set before the shutdown drain so a late update cannot
	// Add to the WaitGroup after Wait has seen zero.
	aispamDraining atomic.Bool
)

func aispamEnsureWorkers() {
	aispamWorkersOnce.Do(func() {
		// ponytail: fixed pool. A provider brownout sheds checks instead of
		// growing goroutines; size it only if shedding shows up in production.
		aispamQueue = make(chan aispamTask, aispamQueueCapacity)
		for range aispamWorkerCount {
			go aispamWorker()
		}
	})
}

func aispamWorker() {
	defer error_handling.RecoverFromPanic("aispamWorker", "aispam")

	for task := range aispamQueue {
		func() {
			defer aispamInFlight.Done()
			defer error_handling.RecoverFromPanic("processAISpam", "aispam")
			processAISpam(task)
		}()
	}
}

// enqueue hands a check to the pool, shedding it when the queue is full or the
// process is draining. A shed check means "not checked", which is acceptable
// because antiflood still owns floods and a raid must not push past the
// provider's request rate.
func aispamEnqueue(task aispamTask) bool {
	aispamEnsureWorkers()

	// The counter is bumped before the drain flag is read. A check counted here
	// is therefore either waited for by DrainAISpamChecks, or observes the flag
	// and is shed without any work: nothing can start after the drain returns.
	aispamInFlight.Add(1)
	if aispamDraining.Load() {
		aispamInFlight.Done()
		aispamShed.Add(1)
		return false
	}

	select {
	case aispamQueue <- task:
		return true
	default:
		aispamInFlight.Done()
		aispamShed.Add(1)
		log.WithFields(log.Fields{
			"chat_id": task.chatID,
			"user_id": task.userID,
		}).Debug("[AISpam] queue full, check shed")
		return false
	}
}

// aispamDrainTimeout bounds the shutdown drain: a full queue of slow checks can
// outlast the shutdown budget, and exiting with work left behind beats blocking
// the process.
const aispamDrainTimeout = 30 * time.Second

// DrainAISpamChecks stops accepting new checks and waits for the enqueued ones
// to finish, so callers observe deletions without sleeping.
func DrainAISpamChecks() {
	aispamDraining.Store(true)
	defer aispamDraining.Store(false)

	done := make(chan struct{})
	go func() {
		aispamInFlight.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(aispamDrainTimeout):
		log.Warn("[AISpam] shutdown drain timed out with checks still in flight")
	}
}

//
// Per-chat circuit breaker
//

type aispamBreaker struct {
	failures   int
	pausedTill time.Time
	notified   bool
}

var aispamBreakers = struct {
	mu sync.Mutex
	m  map[int64]*aispamBreaker
}{m: make(map[int64]*aispamBreaker)}

func aispamBreakerPaused(chatID int64) bool {
	aispamBreakers.mu.Lock()
	defer aispamBreakers.mu.Unlock()

	breaker, ok := aispamBreakers.m[chatID]
	if !ok {
		return false
	}
	return !breaker.pausedTill.IsZero() && time.Now().Before(breaker.pausedTill)
}

// aispamBreakerRecordFailure counts one failed check and reports whether this
// failure tripped the breaker. It returns true exactly once per trip, so the
// chat gets one notice. The stored enabled flag is never touched: a provider
// outage must not silently disable a chat's protection.
func aispamBreakerRecordFailure(chatID int64) bool {
	now := time.Now()

	aispamBreakers.mu.Lock()
	defer aispamBreakers.mu.Unlock()

	breaker, ok := aispamBreakers.m[chatID]
	if !ok {
		if len(aispamBreakers.m) >= aispamBreakerMaxChats {
			return false
		}
		breaker = &aispamBreaker{}
		aispamBreakers.m[chatID] = breaker
	}
	if !breaker.pausedTill.IsZero() && !now.Before(breaker.pausedTill) {
		breaker.failures = 0
		breaker.pausedTill = time.Time{}
		breaker.notified = false
	}

	breaker.failures++
	if breaker.failures < aispamBreakerFailures || breaker.notified {
		return false
	}

	breaker.failures = 0
	breaker.pausedTill = now.Add(aispamBreakerCooldown)
	breaker.notified = true
	return true
}

func aispamBreakerRecordSuccess(chatID int64) {
	aispamBreakers.mu.Lock()
	defer aispamBreakers.mu.Unlock()

	delete(aispamBreakers.m, chatID)
}

//
// Sender history: a bounded rolling window per sender, kept in Redis so every
// replica sees the same one. Both keys expire on their own, and nothing else
// about a message is stored anywhere.
//

// aispamHistoryScript reads a sender's window and records this message in one
// atomic round trip, so two replicas cannot interleave a read with a write. It
// answers with the number of messages already inside the window, followed by
// the recorded texts oldest first.
//
// ponytail: the count saturates at aispamHistoryMaxTimestamps, so a 200+/hour
// poster reads as 200. It is a profile hint, and antiflood is what punishes
// volume.
var aispamHistoryScript = redis.NewScript(`
local texts = redis.call('LRANGE', KEYS[2], 0, -1)
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, ARGV[3])
local prior = redis.call('ZCARD', KEYS[1])
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
redis.call('ZREMRANGEBYRANK', KEYS[1], 0, -tonumber(ARGV[6]) - 1)
redis.call('EXPIRE', KEYS[1], ARGV[4])
if ARGV[5] ~= '' then
	redis.call('RPUSH', KEYS[2], ARGV[5])
	redis.call('LTRIM', KEYS[2], -tonumber(ARGV[7]), -1)
end
redis.call('EXPIRE', KEYS[2], ARGV[4])
local out = {prior}
for i = 1, #texts do out[#out + 1] = texts[i] end
return out
`)

// aispamMessagesKey holds the window's timestamps. Its members are message ids,
// so a redelivered update is counted once rather than twice.
func aispamMessagesKey(chatID, userID int64) string {
	return fmt.Sprintf("alita:aispam:msgs:%d:%d", chatID, userID)
}

func aispamTextsKey(chatID, userID int64) string {
	return fmt.Sprintf("alita:aispam:texts:%d:%d", chatID, userID)
}

// observeAISpamSender returns the sender's prior texts and how many messages
// they posted in the last hour, this one included, then records this message.
// Losing Redis costs the window and not the check: the message is still judged,
// with no history and a count of one.
func observeAISpamSender(chatID, userID, messageID int64, text string, now time.Time) ([]string, int) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return nil, 1
	}

	millis := now.UnixMilli()
	recorded, err := aispamHistoryScript.Run(
		cache.Context,
		rdb,
		[]string{aispamMessagesKey(chatID, userID), aispamTextsKey(chatID, userID)},
		millis,
		strconv.FormatInt(messageID, 10),
		millis-aispamHistoryWindow.Milliseconds(),
		int(aispamHistoryWindow.Seconds()),
		text,
		aispamHistoryMaxTimestamps,
		aispamHistoryTexts,
	).Result()
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "error": err}).Debug("[AISpam] sender history unavailable")
		return nil, 1
	}

	values, ok := recorded.([]any)
	if !ok || len(values) == 0 {
		return nil, 1
	}
	prior, _ := values[0].(int64)

	recent := make([]string, 0, len(values)-1)
	for _, value := range values[1:] {
		if priorText, ok := value.(string); ok {
			recent = append(recent, priorText)
		}
	}
	return recent, int(prior) + 1
}

//
// Chat description cache: the only reason a check calls the Telegram API, and
// only once per chat per TTL.
//

type aispamChatDescriptionEntry struct {
	description string
	expires     time.Time
}

var aispamDescriptions = struct {
	mu      sync.Mutex
	entries map[int64]aispamChatDescriptionEntry
}{entries: make(map[int64]aispamChatDescriptionEntry)}

func aispamChatDescription(b *gotgbot.Bot, chatID int64) string {
	now := time.Now()

	aispamDescriptions.mu.Lock()
	entry, ok := aispamDescriptions.entries[chatID]
	aispamDescriptions.mu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.description
	}

	description := ""
	if chat, err := b.GetChat(chatID, nil); err == nil && chat != nil {
		description = chat.Description
	} else if err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "error": err}).Debug("[AISpam] chat description unavailable")
	}

	aispamDescriptions.mu.Lock()
	// A failed lookup is cached too: an empty description for one TTL beats
	// re-asking on every message.
	if _, present := aispamDescriptions.entries[chatID]; present || len(aispamDescriptions.entries) < aispamDescriptionMaxChats {
		aispamDescriptions.entries[chatID] = aispamChatDescriptionEntry{
			description: description,
			expires:     now.Add(aispamDescriptionTTL),
		}
	}
	aispamDescriptions.mu.Unlock()

	return description
}

func aispamPruneDescriptions(now time.Time) {
	aispamDescriptions.mu.Lock()
	defer aispamDescriptions.mu.Unlock()

	for chatID, entry := range aispamDescriptions.entries {
		if now.After(entry.expires) {
			delete(aispamDescriptions.entries, chatID)
		}
	}
}

//
// Helpers
//

func aispamFirstSeenDaysAgo(userID int64) int {
	record, err := user.GetUserBasicInfoCached(userID)
	if err != nil || record == nil || record.CreatedAt.IsZero() {
		return 0
	}
	// first-seen-by-the-bot is the honest proxy: the Bot API exposes neither
	// account creation nor the member's join date.
	days := int(time.Since(record.CreatedAt).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

func aispamWarnCount(userID, chatID int64) int {
	count, _ := warns.GetWarnsContext(context.Background(), userID, chatID)
	return count
}

var (
	aispamURLPattern = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s<>"]+|(?i)\bt\.me/[^\s<>"]+`)
	aispamTagPattern = regexp.MustCompile(`<[^>]*>`)
	aispamSpaceRun   = regexp.MustCompile(`\s+`)
)

// aispamLinkDomains collects the domains a message points at, from both the
// text and the link entities Telegram attached to it.
func aispamLinkDomains(msg *gotgbot.Message, text string) []string {
	entities := msg.Entities
	if msg.Text == "" {
		entities = msg.CaptionEntities
	}

	raw := aispamURLPattern.FindAllString(text, -1)
	raw = append(raw, extractEntityURLs(text, entities)...)

	var (
		domains []string
		seen    = make(map[string]struct{}, len(raw))
	)
	for _, candidate := range raw {
		host := aispamHost(candidate)
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		domains = append(domains, host)
		if len(domains) == aispamMaxLinkDomains {
			break
		}
	}
	return domains
}

func aispamHost(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), ".,;:!?)]}")
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("https://" + trimmed)
		if err != nil || parsed.Host == "" {
			return ""
		}
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if host == "" || !strings.Contains(host, ".") {
		return ""
	}
	return host
}

// aispamPlainText drops the HTML the bot stores rules and greetings in: tags
// are material irrelevant to the question, and Jev loses accuracy as state
// fills with them.
func aispamPlainText(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSpace(aispamSpaceRun.ReplaceAllString(html.UnescapeString(aispamTagPattern.ReplaceAllString(s, " ")), " "))
}

func aispamTruncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

// aispam is the /aispam command: a bare call reports status and what the model
// is sent, an argument turns the filter on or off. Enabling refuses when the
// operator has not configured a key or has pulled the global kill switch,
// rather than leaving an admin believing their chat is protected.
func (m moduleStruct) aispam(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	enabled := aispam.IsAISpamEnabled(c.Chat.Id)

	if len(args) == 0 {
		return aispamReply(c, aispamStatusText(c.Tr, enabled))
	}

	want, ok := parseToggleArg(args[0])
	if !ok {
		text, _ := c.Tr.GetString("aispam_invalid_option")
		return aispamReply(c, text)
	}

	if want {
		if refusal, refused := aispamEnableRefusal(c.Tr); refused {
			return aispamReply(c, refusal)
		}
	}

	if err := aispam.SetAISpamEnabled(c.Chat.Id, want); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Error("[AISpam] failed to save the setting")
		text, _ := c.Tr.GetString("common_settings_save_failed")
		if replyErr := aispamReply(c, text); replyErr != nil {
			return replyErr
		}
		return err
	}

	key := "aispam_disabled_success"
	if want {
		key = "aispam_enabled_success"
	}
	text, _ := c.Tr.GetString(key)
	return aispamReply(c, text)
}

func aispamStatusText(tr *i18n.Translator, enabled bool) string {
	lineKey := "aispam_status_disabled"
	if enabled {
		lineKey = "aispam_status_enabled"
	}
	line, _ := tr.GetString(lineKey)

	parts := []string{line}
	if config.AppConfig.TypeSafeAPIKey == "" {
		warning, _ := tr.GetString("aispam_status_key_missing")
		parts = append(parts, warning)
	} else if !config.AppConfig.EnableAISpam {
		warning, _ := tr.GetString("aispam_status_globally_disabled")
		parts = append(parts, warning)
	}
	notice, _ := tr.GetString("aispam_data_notice")
	parts = append(parts, notice)

	return strings.Join(parts, "\n\n")
}

func aispamEnableRefusal(tr *i18n.Translator) (string, bool) {
	if config.AppConfig.TypeSafeAPIKey == "" {
		text, _ := tr.GetString("aispam_no_key")
		return text, true
	}
	if !config.AppConfig.EnableAISpam {
		text, _ := tr.GetString("aispam_globally_disabled")
		return text, true
	}
	return "", false
}

func aispamReply(c *helpers.CommandContext, text string) error {
	_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
	return err
}

//
// Setup
//

func aispamCleanupLoop() {
	defer error_handling.RecoverFromPanic("aispamCleanupLoop", "aispam")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for now := range ticker.C {
		func() {
			defer error_handling.RecoverFromPanic("aispamCleanupTick", "aispam")
			aispamCleanupOnce(now)
		}()
	}
}

func aispamCleanupOnce(now time.Time) {
	aispamPruneDescriptions(now)
}

func LoadAISpam(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(aispamModule.moduleName, true)
	aispamEnsureWorkers()

	helpers.WrapCommand(dispatcher, aispamDesc, aispamModule.aispam)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, aispamModule.checkAISpam),
		aispamModule.handlerGroup,
	)
}

func init() {
	RegisterLegacyModule("AISpam", 12, LoadAISpam)
	go aispamCleanupLoop()
}
