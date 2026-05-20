package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/redis/go-redis/v9"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

const (
	antiraidJoinWindowSeconds = 60
	antiraidPollInterval      = 30 * time.Second
	antiraidStateKey          = "alita:antiraid:state" // format: state:chat_id
	antiraidJoinsKey          = "alita:antiraid:joins" // format: joins:chat_id (sorted set)
)

var (
	antiRaidModule = antiRaidStruct{
		moduleStruct: moduleStruct{moduleName: "AntiRaid", handlerGroup: -5},
	}
	antiRaidMu     sync.Mutex // package-level mutex for state mutations
	antiRaidCtx    context.Context
	antiRaidCancel context.CancelFunc
)

type antiRaidStruct struct {
	moduleStruct
}

// raidState stores the serialized raid state in Redis.
type raidState struct {
	Active      bool    `json:"active"`
	StartedAt   int64   `json:"started_at"`   // unix seconds
	ExpiresAt   int64   `json:"expires_at"`   // unix seconds
	BannedUsers []int64 `json:"banned_users"` // user IDs banned during this raid
}

// StartAntiRaidExpiryPoller starts the background expiry poller after cache is available.
func StartAntiRaidExpiryPoller() {
	if !cache.IsRedisAvailable() {
		log.Warn("[AntiRaid] Redis not available, skipping expiry poller start")
		return
	}
	if antiRaidCancel != nil {
		// Already started
		return
	}
	antiRaidCtx, antiRaidCancel = context.WithCancel(context.Background())
	go func() {
		defer error_handling.RecoverFromPanic("antiRaidExpiryPoller", "antiraid")
		antiRaidModule.expiryPoller(antiRaidCtx)
	}()
}

// StopAntiRaidExpiryPoller stops the background expiry poller.
func StopAntiRaidExpiryPoller() {
	if antiRaidCancel != nil {
		antiRaidCancel()
		antiRaidCancel = nil
	}
}

func stateKey(chatID int64) string {
	return fmt.Sprintf("%s:%d", antiraidStateKey, chatID)
}

func joinsKey(chatID int64) string {
	return fmt.Sprintf("%s:%d", antiraidJoinsKey, chatID)
}

func trackJoin(chatID, userID int64) (count int, err error) {
	if !cache.IsRedisAvailable() {
		return 0, fmt.Errorf("cache not initialized")
	}
	now := time.Now().Unix()
	ctx := cache.Context
	rdb := cache.GetRedisClient()
	_, err = rdb.ZAdd(ctx, joinsKey(chatID), redis.Z{Score: float64(now), Member: strconv.FormatInt(userID, 10)}).Result()
	if err != nil {
		return 0, err
	}
	_, err = rdb.ZRemRangeByScore(ctx, joinsKey(chatID), "0", strconv.FormatInt(now-int64(antiraidJoinWindowSeconds), 10)).Result()
	if err != nil {
		log.WithError(err).Warnf("[AntiRaid] ZRemRangeByScore failed on joinsKey %d", chatID)
	}
	rawCount, err := rdb.ZCard(ctx, joinsKey(chatID)).Result()
	return int(rawCount), err
}

func clearJoinTracking(chatID int64) {
	if !cache.IsRedisAvailable() {
		return
	}
	ctx := cache.Context
	rdb := cache.GetRedisClient()
	_ = rdb.Del(ctx, joinsKey(chatID)).Err()
}

func getRaidState(chatID int64) *raidState {
	if cache.Marshal == nil {
		return &raidState{Active: false}
	}
	var st raidState
	if _, err := cache.Marshal.Get(cache.Context, stateKey(chatID), &st); err != nil {
		return &raidState{Active: false}
	}
	if st.Active && time.Now().Unix() > st.ExpiresAt {
		st.Active = false
	}
	return &st
}

func setRaidState(chatID int64, st *raidState) error {
	if cache.Marshal == nil {
		return fmt.Errorf("cache not initialized")
	}
	return cache.Marshal.Set(cache.Context, stateKey(chatID), st, store.WithExpiration(24*time.Hour))
}

func (a *antiRaidStruct) expiryPoller(ctx context.Context) {
	ticker := time.NewTicker(antiraidPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.checkExpiredRaids(ctx)
		case <-ctx.Done():
			log.Info("AntiRaid expiry poller shutting down gracefully")
			return
		}
	}
}

func (a *antiRaidStruct) checkExpiredRaids(ctx context.Context) {
	if !cache.IsRedisAvailable() {
		return
	}
	rdb := cache.GetRedisClient()

	iter := rdb.Scan(ctx, 0, fmt.Sprintf("%s:*", antiraidStateKey), 0).Iterator()
	for iter.Next(ctx) {
		k := iter.Val()
		parts := strings.Split(k, ":")
		if len(parts) < 4 {
			continue
		}
		chatID, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if chatID == 0 {
			continue
		}
		st := getRaidState(chatID)
		if st.Active && time.Now().Unix() > st.ExpiresAt {
			st.Active = false
			_ = setRaidState(chatID, st)
			clearJoinTracking(chatID)
			log.Infof("[AntiRaid] Raid expired for chat %d (auto-expiry)", chatID)
		}
	}
	if err := iter.Err(); err != nil {
		log.WithError(err).Warn("[AntiRaid] SCAN iteration failed in checkExpiredRaids")
	}
}

func (a *antiRaidStruct) isRaidActive(chatID int64) bool {
	st := getRaidState(chatID)
	if !st.Active {
		return false
	}
	if time.Now().Unix() > st.ExpiresAt {
		st.Active = false
		_ = setRaidState(chatID, st)
		return false
	}
	return true
}

func (a *antiRaidStruct) enableRaid(chatID int64, durationSeconds int) {
	st := &raidState{
		Active:      true,
		StartedAt:   time.Now().Unix(),
		ExpiresAt:   time.Now().Unix() + int64(durationSeconds),
		BannedUsers: []int64{},
	}
	_ = setRaidState(chatID, st)
	clearJoinTracking(chatID)
}

func (a *antiRaidStruct) disableRaid(chatID int64) bool {
	st := getRaidState(chatID)
	if !st.Active {
		return false
	}
	st.Active = false
	_ = setRaidState(chatID, st)
	clearJoinTracking(chatID)
	return true
}

func (a *antiRaidStruct) onJoin(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if chat == nil {
		return ext.ContinueGroups
	}
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.ContinueGroups
	}

	if !chat_status.IsBotAdmin(bot, ctx, chat) {
		return ext.ContinueGroups
	}
	if !chat_status.CanBotRestrict(bot, ctx, chat, false) {
		return ext.ContinueGroups
	}

	settings := db.GetAntiRaidSettings(chat.Id)
	isActive := a.isRaidActive(chat.Id)

	for _, member := range msg.NewChatMembers {
		if member.Id == bot.Id {
			continue
		}
		if chat_status.IsApproved(bot, chat.Id, member.Id) {
			continue
		}
		if chat_status.IsUserAdmin(bot, chat.Id, member.Id) {
			continue
		}

		if isActive {
			_, err := chat.BanMember(bot, member.Id, &gotgbot.BanChatMemberOpts{
				UntilDate: time.Now().Unix() + int64(settings.RaidActionTime),
			})
			if err != nil {
				log.WithError(err).Warnf("[AntiRaid] Failed to ban user %d in chat %d", member.Id, chat.Id)
			} else {
				antiRaidMu.Lock()
				st := getRaidState(chat.Id)
				st.BannedUsers = append(st.BannedUsers, member.Id)
				_ = setRaidState(chat.Id, st)
				antiRaidMu.Unlock()
			}
			continue
		}

		if settings.AutoAntiRaidThreshold <= 0 {
			continue
		}

		count, err := trackJoin(chat.Id, member.Id)
		if err != nil {
			log.WithError(err).Warnf("[AntiRaid] Failed to track join for chat %d", chat.Id)
			continue
		}

		if count >= settings.AutoAntiRaidThreshold {
			a.enableRaid(chat.Id, settings.RaidTime)
			log.Infof("[AntiRaid] Auto-triggered raid in chat %d (joins=%d >= threshold=%d)", chat.Id, count, settings.AutoAntiRaidThreshold)

			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("antiraid_auto_triggered", i18n.TranslationParams{"count": strconv.Itoa(count)})
			_, _ = chat.SendMessage(bot, text, helpers.Shtml())

			_, err := chat.BanMember(bot, member.Id, &gotgbot.BanChatMemberOpts{
				UntilDate: time.Now().Unix() + int64(settings.RaidActionTime),
			})
			if err != nil {
				log.WithError(err).Warnf("[AntiRaid] Failed to ban user %d in chat %d", member.Id, chat.Id)
			} else {
				antiRaidMu.Lock()
				st := getRaidState(chat.Id)
				st.BannedUsers = append(st.BannedUsers, member.Id)
				_ = setRaidState(chat.Id, st)
				antiRaidMu.Unlock()
			}
		}
	}

	return ext.ContinueGroups
}

func (a *antiRaidStruct) antiraid(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(bot, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := ctx.Args()[1:]

	if len(args) == 0 {
		settings := db.GetAntiRaidSettings(chat.Id)
		isActive := a.isRaidActive(chat.Id)
		st := getRaidState(chat.Id)
		var text string
		if isActive {
			text, _ = tr.GetString("antiraid_active_status", i18n.TranslationParams{
				"raid_time":      formatDuration(settings.RaidTime),
				"action_time":    formatDuration(settings.RaidActionTime),
				"auto_threshold": strconv.Itoa(settings.AutoAntiRaidThreshold),
				"expires_in":     formatDuration(int(int64(st.ExpiresAt) - time.Now().Unix())),
			})
		} else {
			text, _ = tr.GetString("antiraid_inactive_status", i18n.TranslationParams{
				"raid_time":      formatDuration(settings.RaidTime),
				"action_time":    formatDuration(settings.RaidActionTime),
				"auto_threshold": strconv.Itoa(settings.AutoAntiRaidThreshold),
			})
		}

		var kb [][]gotgbot.InlineKeyboardButton
		if isActive {
			disableText, _ := tr.GetString("antiraid_btn_disable")
			kb = append(kb, []gotgbot.InlineKeyboardButton{{
				Text:         disableText,
				CallbackData: encodeCallbackData("antiraid", map[string]string{"a": "off"}, "antiraid.off"),
			}})
		} else {
			enableText, _ := tr.GetString("antiraid_btn_enable")
			kb = append(kb, []gotgbot.InlineKeyboardButton{{
				Text:         enableText,
				CallbackData: encodeCallbackData("antiraid", map[string]string{"a": "on"}, "antiraid.on"),
			}})
		}

		_, _ = msg.Reply(bot, text, &gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: kb},
		})
		return ext.EndGroups
	}

	arg := strings.ToLower(args[0])
	switch arg {
	case "on":
		if a.isRaidActive(chat.Id) {
			text, _ := tr.GetString("antiraid_already_active")
			_, _ = msg.Reply(bot, text, helpers.Shtml())
			return ext.EndGroups
		}
		settings := db.GetAntiRaidSettings(chat.Id)
		a.enableRaid(chat.Id, settings.RaidTime)
		text, _ := tr.GetString("antiraid_enabled", i18n.TranslationParams{"duration": formatDuration(settings.RaidTime)})
		_, _ = msg.Reply(bot, text, helpers.Shtml())

	case "off":
		if !a.disableRaid(chat.Id) {
			text, _ := tr.GetString("antiraid_not_active")
			_, _ = msg.Reply(bot, text, helpers.Shtml())
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_disabled")
		_, _ = msg.Reply(bot, text, helpers.Shtml())

	default:
		dur, ok := parseDuration(arg)
		if !ok {
			text, _ := tr.GetString("antiraid_invalid_duration")
			_, _ = msg.Reply(bot, text, helpers.Shtml())
			return ext.EndGroups
		}
		if a.isRaidActive(chat.Id) {
			st := getRaidState(chat.Id)
			st.ExpiresAt = time.Now().Unix() + int64(dur)
			_ = setRaidState(chat.Id, st)
		} else {
			a.enableRaid(chat.Id, dur)
		}
		text, _ := tr.GetString("antiraid_enabled", i18n.TranslationParams{"duration": formatDuration(dur)})
		_, _ = msg.Reply(bot, text, helpers.Shtml())
	}

	return ext.EndGroups
}

func (a *antiRaidStruct) raidTime(bot *gotgbot.Bot, ctx *ext.Context) error {
	return a.raidTimeSetter(bot, ctx, true)
}

func (a *antiRaidStruct) raidActionTime(bot *gotgbot.Bot, ctx *ext.Context) error {
	return a.raidTimeSetter(bot, ctx, false)
}

//nolint:dupl // Similar patterns by design: raidTime vs raidActionTime commands.
func (a *antiRaidStruct) raidTimeSetter(bot *gotgbot.Bot, ctx *ext.Context, isRaidTime bool) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(bot, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := ctx.Args()[1:]
	if len(args) == 0 {
		text := ""
		if isRaidTime {
			text, _ = tr.GetString("antiraid_raidtime_usage")
		} else {
			text, _ = tr.GetString("antiraid_raidactiontime_usage")
		}
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return ext.EndGroups
	}

	dur, ok := parseDuration(args[0])
	if !ok {
		text, _ := tr.GetString("antiraid_invalid_duration")
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return ext.EndGroups
	}
	if dur == 0 {
		text, _ := tr.GetString("antiraid_duration_must_be_positive")
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return ext.EndGroups
	}

	text := ""
	if isRaidTime {
		settings := db.GetAntiRaidSettings(chat.Id)
		if settings.RaidTime == dur {
			text, _ = tr.GetString("antiraid_raidtime_no_change", i18n.TranslationParams{"duration": formatDuration(dur)})
			_, _ = msg.Reply(bot, text, helpers.Shtml())
			return ext.EndGroups
		}
		err := db.SetRaidTime(chat.Id, dur)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] SetRaidTime failed for chat %d", chat.Id)
			return ext.EndGroups
		}
		text, _ = tr.GetString("antiraid_raidtime_set", i18n.TranslationParams{"duration": formatDuration(dur)})
	} else {
		settings := db.GetAntiRaidSettings(chat.Id)
		if settings.RaidActionTime == dur {
			text, _ = tr.GetString("antiraid_raidactiontime_no_change", i18n.TranslationParams{"duration": formatDuration(dur)})
			_, _ = msg.Reply(bot, text, helpers.Shtml())
			return ext.EndGroups
		}
		err := db.SetRaidActionTime(chat.Id, dur)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] SetRaidActionTime failed for chat %d", chat.Id)
			return ext.EndGroups
		}
		text, _ = tr.GetString("antiraid_raidactiontime_set", i18n.TranslationParams{"duration": formatDuration(dur)})
	}
	_, _ = msg.Reply(bot, text, helpers.Shtml())
	return ext.EndGroups
}

func (a *antiRaidStruct) autoAntiRaid(bot *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(bot, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := ctx.Args()[1:]
	if len(args) == 0 {
		settings := db.GetAntiRaidSettings(chat.Id)
		var text string
		if settings.AutoAntiRaidThreshold > 0 {
			text, _ = tr.GetString("antiraid_auto_enabled", i18n.TranslationParams{"threshold": strconv.Itoa(settings.AutoAntiRaidThreshold)})
		} else {
			text, _ = tr.GetString("antiraid_auto_disabled")
		}
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return ext.EndGroups
	}

	arg := strings.ToLower(args[0])
	if arg == "off" {
		err := db.SetAutoAntiRaidThreshold(chat.Id, 0)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] SetAutoAntiRaidThreshold(0) failed for chat %d", chat.Id)
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_auto_disabled")
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return ext.EndGroups
	}

	threshold, err := strconv.Atoi(arg)
	if err != nil || threshold <= 0 {
		text, _ := tr.GetString("antiraid_invalid_threshold")
		_, _ = msg.Reply(bot, text, helpers.Shtml())
		return ext.EndGroups
	}
	if err := db.SetAutoAntiRaidThreshold(chat.Id, threshold); err != nil {
		log.WithError(err).Errorf("[AntiRaid] SetAutoAntiRaidThreshold(%d) failed for chat %d", threshold, chat.Id)
		return ext.EndGroups
	}

	text, _ := tr.GetString("antiraid_auto_enabled", i18n.TranslationParams{"threshold": strconv.Itoa(threshold)})
	_, _ = msg.Reply(bot, text, helpers.Shtml())
	return ext.EndGroups
}

func (a *antiRaidStruct) callbackHandler(bot *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.ContinueGroups
	}
	if query == nil {
		return ext.ContinueGroups
	}

	action := ""
	data := query.Data
	decoded, ok := decodeCallbackData(data, "antiraid")
	if !ok {
		log.Warnf("[AntiRaid] Ignoring malformed callback data: %s", data)
		return ext.ContinueGroups
	}
	action, _ = decoded.Field("a")

	msg := query.Message
	if msg == nil {
		return ext.ContinueGroups
	}
	chatID := msg.GetChat().Id

	if !chat_status.IsUserAdmin(bot, chatID, query.From.Id) {
		_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{
			Text: "You're not an admin!",
		})
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	switch action {
	case "on":
		if a.isRaidActive(chatID) {
			text, _ := tr.GetString("antiraid_already_active")
			_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		settings := db.GetAntiRaidSettings(chatID)
		a.enableRaid(chatID, settings.RaidTime)
		text, _ := tr.GetString("antiraid_enabled", i18n.TranslationParams{"duration": formatDuration(settings.RaidTime)})
		_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		_, _, _ = msg.EditText(bot, tgmd2html.MD2HTMLV2(text), &gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
		})
	case "off":
		if !a.disableRaid(chatID) {
			text, _ := tr.GetString("antiraid_not_active")
			_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_disabled")
		_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		_, _, _ = msg.EditText(bot, tgmd2html.MD2HTMLV2(text), &gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
		})
	}

	return ext.EndGroups
}

func parseDuration(input string) (seconds int, ok bool) {
	input = strings.TrimSpace(strings.ToLower(input))
	if len(input) == 0 {
		return 0, false
	}
	numStr := input[:len(input)-1]
	unit := input[len(input)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil || num < 0 {
		return 0, false
	}
	switch unit {
	case 'm':
		return num * 60, true
	case 'h':
		return num * 60 * 60, true
	case 'd':
		return num * 24 * 60 * 60, true
	case 'w':
		return num * 7 * 24 * 60 * 60, true
	default:
		raw, err := strconv.Atoi(input)
		if err != nil {
			return 0, false
		}
		return raw, true
	}
}

func formatDuration(seconds int) string {
	if seconds >= 604800 && seconds%604800 == 0 {
		return fmt.Sprintf("%dw", seconds/604800)
	}
	if seconds >= 86400 && seconds%86400 == 0 {
		return fmt.Sprintf("%dd", seconds/86400)
	}
	if seconds >= 3600 && seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds >= 60 && seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func LoadAntiRaid(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store(antiRaidModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("antiraid", antiRaidModule.antiraid))
	dispatcher.AddHandler(handlers.NewCommand("raidtime", antiRaidModule.raidTime))
	dispatcher.AddHandler(handlers.NewCommand("raidactiontime", antiRaidModule.raidActionTime))
	dispatcher.AddHandler(handlers.NewCommand("autoantiraid", antiRaidModule.autoAntiRaid))

	dispatcher.AddHandler(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return msg.NewChatMembers != nil
			},
			antiRaidModule.onJoin,
		),
	)

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("antiraid"), antiRaidModule.callbackHandler))

	helpers.AddCmdToDisableable("antiraid")

	StartAntiRaidExpiryPoller()
}

func init() {
	RegisterLegacyModule("AntiRaid", 230, LoadAntiRaid)
}
