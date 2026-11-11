package modules

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const (
	antiraidJoinWindowSeconds = 60
	// antiraidStateGrace keeps an active raid's state this long past its
	// expiry; readers treat it as inactive (and delete it) from ExpiresAt on.
	antiraidStateGrace = 30 * time.Second
	antiraidStateKey   = "alita:antiraid:state" // format: state:chat_id
	antiraidJoinsKey   = "alita:antiraid:joins" // format: joins:chat_id (sorted set)
)

var (
	antiRaidModule = antiRaidStruct{
		moduleStruct: moduleStruct{moduleName: "AntiRaid"},
	}
	antiRaidStateMu sync.Mutex

	deleteRaidStateScript = redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("DEL", KEYS[1], KEYS[2])
			return 1
		end
		return 0
	`)
	replaceRaidStateScript = redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
			return 1
		end
		return 0
	`)
	disableRaidScript = redis.NewScript(`
		local existed = redis.call("DEL", KEYS[1])
		redis.call("DEL", KEYS[2])
		return existed
	`)
	trackJoinScript = redis.NewScript(`
		redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
		redis.call('EXPIRE', KEYS[1], ARGV[3])
		redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, ARGV[4])
		return redis.call('ZCARD', KEYS[1])
	`)
)

type antiRaidStruct struct {
	moduleStruct
}

type raidState struct {
	Active    bool  `json:"active"`
	StartedAt int64 `json:"started_at"`
	ExpiresAt int64 `json:"expires_at"`
}

func stateKey(chatID int64) string {
	return fmt.Sprintf("%s:%d", antiraidStateKey, chatID)
}

func joinsKey(chatID int64) string {
	return fmt.Sprintf("%s:%d", antiraidJoinsKey, chatID)
}

func trackJoin(chatID, userID int64) (count int, err error) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return 0, fmt.Errorf("cache not initialized")
	}
	now := time.Now().Unix()
	ctx := cache.Context
	member := fmt.Sprintf("%d:%d:%d", userID, now, time.Now().UnixNano())
	window := antiraidJoinWindowSeconds
	cutoff := now - int64(antiraidJoinWindowSeconds)
	res, err := trackJoinScript.Run(ctx, rdb, []string{joinsKey(chatID)}, now, member, window, cutoff).Int()
	if err != nil {
		return 0, err
	}
	return res, nil
}

func clearJoinTracking(chatID int64) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return
	}
	_ = rdb.Del(cache.Context, joinsKey(chatID)).Err()
}

// getRaidState reads Redis-only raid state. Without Redis, raids remain inactive and join
// tracking cannot run; there is no in-process fallback.
// The alita:antiraid: keys are outside alita:cache: and survive CLEAR_CACHE_ON_STARTUP.
func getRaidState(chatID int64) *raidState {
	m := cache.GetMarshal()
	if m == nil {
		return &raidState{Active: false}
	}
	var st raidState
	if _, err := m.Get(cache.Context, stateKey(chatID), &st); err != nil {
		return &raidState{Active: false}
	}
	return &st
}

func setRaidState(chatID int64, st *raidState) error {
	m := cache.GetMarshal()
	if m == nil {
		return fmt.Errorf("cache not initialized")
	}
	return m.Set(cache.Context, stateKey(chatID), st, store.WithExpiration(raidStateExpiration(st)))
}

func raidStateExpiration(st *raidState) time.Duration {
	expiration := 24 * time.Hour
	if st.Active {
		expiration = time.Until(time.Unix(st.ExpiresAt, 0)) + antiraidStateGrace
		if expiration <= 0 {
			expiration = antiraidStateGrace
		}
	}
	return expiration
}

func marshalRaidState(st *raidState) ([]byte, error) {
	data, err := msgpack.Marshal(st)
	if err != nil {
		return nil, fmt.Errorf("marshal anti-raid state: %w", err)
	}
	return data, nil
}

func deleteRaidStateIfUnchanged(chatID int64, expected *raidState) (bool, error) {
	expectedJSON, err := marshalRaidState(expected)
	if err != nil {
		return false, err
	}
	if rdb := cache.GetRedisClient(); rdb != nil {
		deleted, err := deleteRaidStateScript.Run(
			cache.Context,
			rdb,
			[]string{stateKey(chatID), joinsKey(chatID)},
			expectedJSON,
		).Int()
		return deleted == 1, err
	}

	antiRaidStateMu.Lock()
	defer antiRaidStateMu.Unlock()
	current := getRaidState(chatID)
	if *current != *expected {
		return false, nil
	}
	m := cache.GetMarshal()
	if m == nil {
		return false, fmt.Errorf("cache not initialized")
	}
	if err := m.Delete(cache.Context, stateKey(chatID)); err != nil {
		return false, err
	}
	return true, nil
}

func replaceRaidStateIfUnchanged(chatID int64, expected, replacement *raidState) (bool, error) {
	expectedJSON, err := marshalRaidState(expected)
	if err != nil {
		return false, err
	}
	replacementJSON, err := marshalRaidState(replacement)
	if err != nil {
		return false, err
	}
	if rdb := cache.GetRedisClient(); rdb != nil {
		replaced, err := replaceRaidStateScript.Run(
			cache.Context,
			rdb,
			[]string{stateKey(chatID)},
			expectedJSON,
			replacementJSON,
			raidStateExpiration(replacement).Milliseconds(),
		).Int()
		return replaced == 1, err
	}

	antiRaidStateMu.Lock()
	defer antiRaidStateMu.Unlock()
	current := getRaidState(chatID)
	if *current != *expected {
		return false, nil
	}
	return true, setRaidState(chatID, replacement)
}

func (a *antiRaidStruct) isRaidActive(chatID int64) bool {
	st := getRaidState(chatID)
	if !st.Active {
		return false
	}
	if time.Now().Unix() > st.ExpiresAt {
		deleted, err := deleteRaidStateIfUnchanged(chatID, st)
		if err != nil {
			log.WithError(err).Warnf("[AntiRaid] Failed to expire raid for chat %d", chatID)
			return false
		}
		if !deleted {
			st = getRaidState(chatID)
			return st.Active && time.Now().Unix() <= st.ExpiresAt
		}
		return false
	}
	return true
}

func (a *antiRaidStruct) enableRaid(chatID int64, durationSeconds int) (bool, error) {
	if err := chatsettings.RaidTime.Validate(durationSeconds); err != nil {
		return false, err
	}
	now := time.Now().Unix()
	st := &raidState{
		Active:    true,
		StartedAt: now,
		ExpiresAt: now + int64(durationSeconds),
	}
	if rdb := cache.GetRedisClient(); rdb != nil {
		current := getRaidState(chatID)
		if current.Active && current.ExpiresAt > now {
			return false, nil
		}
		if _, err := deleteRaidStateIfUnchanged(chatID, current); err != nil {
			return false, err
		}
		payload, err := marshalRaidState(st)
		if err != nil {
			return false, err
		}
		enabled, err := rdb.SetNX(cache.Context, stateKey(chatID), payload, raidStateExpiration(st)).Result()
		if err != nil || !enabled {
			return enabled, err
		}
		clearJoinTracking(chatID)
		return true, nil
	}

	antiRaidStateMu.Lock()
	defer antiRaidStateMu.Unlock()
	current := getRaidState(chatID)
	if current.Active && current.ExpiresAt > now {
		return false, nil
	}
	if err := setRaidState(chatID, st); err != nil {
		return false, err
	}
	clearJoinTracking(chatID)
	return true, nil
}

func (a *antiRaidStruct) disableRaid(chatID int64) (bool, error) {
	if rdb := cache.GetRedisClient(); rdb != nil {
		deleted, err := disableRaidScript.Run(
			cache.Context,
			rdb,
			[]string{stateKey(chatID), joinsKey(chatID)},
		).Int()
		if err != nil {
			log.WithError(err).Warnf("[AntiRaid] Failed to disable raid for chat %d", chatID)
			return false, err
		}
		return deleted == 1, nil
	}

	antiRaidStateMu.Lock()
	defer antiRaidStateMu.Unlock()
	st := getRaidState(chatID)
	if !st.Active || time.Now().Unix() > st.ExpiresAt {
		return false, nil
	}
	m := cache.GetMarshal()
	if m == nil {
		return false, fmt.Errorf("cache not initialized")
	}
	if err := m.Delete(cache.Context, stateKey(chatID)); err != nil {
		log.WithError(err).Warnf("[AntiRaid] Failed to disable raid for chat %d", chatID)
		return false, err
	}
	clearJoinTracking(chatID)
	return true, nil
}

func (a *antiRaidStruct) setRaidDuration(chatID int64, durationSeconds int) error {
	if err := chatsettings.RaidTime.Validate(durationSeconds); err != nil {
		return err
	}
	for range 5 {
		current := getRaidState(chatID)
		if current.Active && time.Now().Unix() <= current.ExpiresAt {
			replacement := *current
			replacement.ExpiresAt = time.Now().Unix() + int64(durationSeconds)
			replaced, err := replaceRaidStateIfUnchanged(chatID, current, &replacement)
			if err != nil {
				return err
			}
			if replaced {
				return nil
			}
			continue
		}
		enabled, err := a.enableRaid(chatID, durationSeconds)
		if err != nil {
			return err
		}
		if enabled {
			return nil
		}
	}
	return fmt.Errorf("anti-raid state changed concurrently")
}

// banRaidMember bans userID for actionTime seconds and reports whether the
// ban went through.
func banRaidMember(bot *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64, actionTime int) bool {
	reqCtx := tracing.UpdateContext(ctx)
	// A zero duration would be a permanent ban in the module; raid never bans for good.
	if actionTime == 0 {
		log.WithContext(reqCtx).Warnf("[AntiRaid] Invalid raid action time %d; refusing permanent ban for user %d in chat %d", actionTime, userID, chat.Id)
		return false
	}
	result, err := punishment.Apply(reqCtx, bot, punishment.Request{
		ChatID: chat.Id, Target: punishment.Target{ID: userID}, Mode: punishment.Ban,
		Duration: time.Duration(actionTime) * time.Second, Source: models.BanSourceAntiraid,
	})
	if err != nil {
		log.WithContext(reqCtx).WithError(err).Warnf("[AntiRaid] Failed to ban user %d in chat %d", userID, chat.Id)
		return false
	}
	if !result.Applied {
		log.WithContext(reqCtx).Warnf("[AntiRaid] Invalid raid action time %d; refusing permanent ban for user %d in chat %d: %s", actionTime, userID, chat.Id, result.Refusal)
		return false
	}
	return true
}

// raidApplies reports whether antiraid acts on userID joining the chat in
// ctx: a group the bot can ban in, and a newcomer who is not the bot, an
// approved user or an admin.
func raidApplies(bot *gotgbot.Bot, ctx *ext.Context, userID int64) bool {
	chat := ctx.EffectiveChat
	if chat == nil || (chat.Type != "group" && chat.Type != "supergroup") || userID == bot.Id {
		return false
	}
	if !chat_status.IsBotAdmin(bot, ctx, chat) {
		return false
	}
	if !chat_status.CanBotRestrict(bot, ctx, chat) {
		log.WithContext(tracing.UpdateContext(ctx)).WithFields(log.Fields{"chatId": chat.Id}).Debug("Antiraid action skipped: bot lacks restrict permissions")
		return false
	}
	return !chat_status.IsApproved(bot, chat.Id, userID) && !chat_status.IsUserAdmin(bot, chat.Id, userID)
}

// countJoin counts userID's join towards the chat's auto-antiraid threshold
// and switches the raid on when the join reaches it. The switch writes one log
// entry and posts one notice: raiders banned during it get none of their own,
// as a raid can be hundreds of joins.
func (a *antiRaidStruct) countJoin(bot *gotgbot.Bot, ctx *ext.Context, userID int64) {
	chat := ctx.EffectiveChat
	if chat == nil {
		return
	}
	// Settings and raid state first: they are cached, the bot's rights may not be.
	settings := chatsettings.ReadAntiRaid(chat.Id)
	if settings.AutoAntiRaidThreshold <= 0 || a.isRaidActive(chat.Id) || !raidApplies(bot, ctx, userID) {
		return
	}
	count, err := trackJoin(chat.Id, userID)
	if err != nil {
		log.WithContext(tracing.UpdateContext(ctx)).WithError(err).Warnf("[AntiRaid] Failed to track join for chat %d", chat.Id)
		return
	}
	if count < settings.AutoAntiRaidThreshold {
		return
	}
	enabled, err := a.enableRaid(chat.Id, settings.RaidTime)
	if err != nil {
		log.WithContext(tracing.UpdateContext(ctx)).WithError(err).Warnf("[AntiRaid] Failed to auto-enable raid in chat %d", chat.Id)
		return
	}
	if !enabled {
		// Another join switched it on first and reported it.
		return
	}
	log.WithContext(tracing.UpdateContext(ctx)).Infof("[AntiRaid] Auto-triggered raid in chat %d (joins=%d >= threshold=%d)", chat.Id, count, settings.AutoAntiRaidThreshold)
	actionlog.Automated(bot, chat, "ANTIRAID", 0, "",
		actionlog.Field{Name: "Joins", Value: strconv.Itoa(count)},
		actionlog.Field{Name: "Threshold", Value: strconv.Itoa(settings.AutoAntiRaidThreshold)},
		actionlog.Field{Name: "Duration", Value: formatDuration(settings.RaidTime)},
		actionlog.Field{Name: "Joiners banned for", Value: formatDuration(settings.RaidActionTime)},
	)
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("antiraid_auto_triggered", i18n.TranslationParams{"count": strconv.Itoa(count)})
	_, _ = chat.SendMessage(bot, text, formatting.Shtml())
}

// antiRaidStage is membership intake's raid guard: while a raid is on it bans
// the newcomer for the raid action time. A newcomer the chat's federation
// banned is left to that permanent ban, which a timed ban could shorten. A
// failed ban lets the member on to the next stage.
func (a *antiRaidStruct) antiRaidStage(bot *gotgbot.Bot, ctx *ext.Context, member gotgbot.User) StageAnswer {
	chat := ctx.EffectiveChat
	if chat == nil || !a.isRaidActive(chat.Id) || !raidApplies(bot, ctx, member.Id) || fedBannedInChat(ctx, chat.Id, member.Id) {
		return StageNotApplicable
	}
	if !banRaidMember(bot, ctx, chat, member.Id, chatsettings.ReadAntiRaid(chat.Id).RaidActionTime) {
		return StageFailed
	}
	return StageRemoved
}

func (a *antiRaidStruct) antiraid(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chat := ctx.EffectiveChat

	args := ctx.Args()[1:]

	if len(args) == 0 {
		settings := chatsettings.ReadAntiRaid(chat.Id)
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
				CallbackData: encodeCallbackData("antiraid", map[string]string{"a": "off", "c": strconv.FormatInt(chat.Id, 10)}),
			}})
		} else {
			enableText, _ := tr.GetString("antiraid_btn_enable")
			kb = append(kb, []gotgbot.InlineKeyboardButton{{
				Text:         enableText,
				CallbackData: encodeCallbackData("antiraid", map[string]string{"a": "on", "c": strconv.FormatInt(chat.Id, 10)}),
			}})
		}

		_, _ = msg.Reply(bot, text, &gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
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
		settings := chatsettings.ReadAntiRaid(chat.Id)
		enabled, err := a.enableRaid(chat.Id, settings.RaidTime)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] Failed to enable raid in chat %d", chat.Id)
			text, _ := tr.GetString("error_generic")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		if !enabled {
			text, _ := tr.GetString("antiraid_already_active")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_enabled", i18n.TranslationParams{"duration": formatDuration(settings.RaidTime)})
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		logRaidMode(bot, chat, ctx.EffectiveUser, true, settings.RaidTime)

	case "off":
		disabled, err := a.disableRaid(chat.Id)
		if err != nil {
			text, _ := tr.GetString("error_generic")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		if !disabled {
			text, _ := tr.GetString("antiraid_not_active")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_disabled")
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		logRaidMode(bot, chat, ctx.EffectiveUser, false, 0)

	default:
		dur, ok := parseDuration(arg)
		if !ok {
			text, _ := tr.GetString("antiraid_invalid_duration")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		if err := a.setRaidDuration(chat.Id, dur); err != nil {
			log.WithError(err).Errorf("[AntiRaid] Failed to set raid duration in chat %d", chat.Id)
			text, _ := tr.GetString("error_generic")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_enabled", i18n.TranslationParams{"duration": formatDuration(dur)})
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		logRaidMode(bot, chat, ctx.EffectiveUser, true, dur)
	}

	return ext.EndGroups
}

// logRaidMode posts an admin switching raid mode on (for duration seconds)
// or off under the settings log category.
func logRaidMode(bot *gotgbot.Bot, chat *gotgbot.Chat, admin *gotgbot.User, on bool, duration int) {
	state, shown := "off", ""
	if on {
		state, shown = "on", formatDuration(duration)
	}
	actionlog.SettingChange(bot, chat, admin, "ANTIRAID",
		actionlog.Field{Name: "Raid mode", Value: state},
		actionlog.Field{Name: "Duration", Value: shown},
	)
}

func (a *antiRaidStruct) raidTime(bot *gotgbot.Bot, ctx *ext.Context) error {
	return a.raidTimeSetter(bot, ctx, true)
}

func (a *antiRaidStruct) raidActionTime(bot *gotgbot.Bot, ctx *ext.Context) error {
	return a.raidTimeSetter(bot, ctx, false)
}

//nolint:dupl // Similar patterns by design: raidTime vs raidActionTime commands.
func (a *antiRaidStruct) raidTimeSetter(bot *gotgbot.Bot, ctx *ext.Context, isRaidTime bool) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chat := ctx.EffectiveChat

	args := ctx.Args()[1:]
	if len(args) == 0 {
		text := ""
		if isRaidTime {
			text, _ = tr.GetString("antiraid_raidtime_usage")
		} else {
			text, _ = tr.GetString("antiraid_raidactiontime_usage")
		}
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	dur, ok := parseDuration(args[0])
	if ok && !isRaidTime {
		ok = chatsettings.RaidActionTime.Validate(dur) == nil
	}
	if !ok {
		text, _ := tr.GetString("antiraid_invalid_duration")
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	text := ""
	if isRaidTime {
		settings := chatsettings.ReadAntiRaid(chat.Id)
		if settings.RaidTime == dur {
			text, _ = tr.GetString("antiraid_raidtime_no_change", i18n.TranslationParams{"duration": formatDuration(dur)})
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		err := chatsettings.RaidTime.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, dur)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] SetRaidTime failed for chat %d", chat.Id)
			text, _ = tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ = tr.GetString("antiraid_raidtime_set", i18n.TranslationParams{"duration": formatDuration(dur)})
	} else {
		settings := chatsettings.ReadAntiRaid(chat.Id)
		if settings.RaidActionTime == dur {
			text, _ = tr.GetString("antiraid_raidactiontime_no_change", i18n.TranslationParams{"duration": formatDuration(dur)})
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		err := chatsettings.RaidActionTime.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, dur)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] SetRaidActionTime failed for chat %d", chat.Id)
			text, _ = tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ = tr.GetString("antiraid_raidactiontime_set", i18n.TranslationParams{"duration": formatDuration(dur)})
	}
	_, _ = msg.Reply(bot, text, formatting.Shtml())
	return ext.EndGroups
}

func (a *antiRaidStruct) autoAntiRaid(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chat := ctx.EffectiveChat

	args := ctx.Args()[1:]
	if len(args) == 0 {
		settings := chatsettings.ReadAntiRaid(chat.Id)
		var text string
		if settings.AutoAntiRaidThreshold > 0 {
			text, _ = tr.GetString("antiraid_auto_enabled", i18n.TranslationParams{"threshold": strconv.Itoa(settings.AutoAntiRaidThreshold)})
		} else {
			text, _ = tr.GetString("antiraid_auto_disabled")
		}
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	arg := strings.ToLower(args[0])
	if arg == "off" {
		err := chatsettings.AutoAntiRaidThreshold.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, 0)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] SetAutoAntiRaidThreshold(0) failed for chat %d", chat.Id)
			text, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(bot, text, formatting.Shtml())
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_auto_disabled")
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	threshold, err := strconv.Atoi(arg)
	if err != nil || threshold == 0 || chatsettings.AutoAntiRaidThreshold.Validate(threshold) != nil {
		text, _ := tr.GetString("antiraid_invalid_threshold")
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	if err := chatsettings.AutoAntiRaidThreshold.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, threshold); err != nil {
		log.WithError(err).Errorf("[AntiRaid] SetAutoAntiRaidThreshold(%d) failed for chat %d", threshold, chat.Id)
		text, _ := tr.GetString("common_settings_save_failed")
		_, _ = msg.Reply(bot, text, formatting.Shtml())
		return ext.EndGroups
	}

	text, _ := tr.GetString("antiraid_auto_enabled", i18n.TranslationParams{"threshold": strconv.Itoa(threshold)})
	_, _ = msg.Reply(bot, text, formatting.Shtml())
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
	shownIn := msg.GetChat()
	chat := &shownIn
	chatID := chat.Id
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if msg.GetChat().Type == "private" {
		// The status message was sent in PM: act on the group it showed.
		chat = callbackAdminChat(bot, ctx, query, decoded, true)
		if chat == nil {
			return ext.EndGroups
		}
		chatID = chat.Id
	} else if !chat_status.IsUserAdmin(bot, chatID, query.From.Id) {
		text, _ := tr.GetString("connections_is_user_connected_user_not_admin")
		_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{
			Text: text,
		})
		return ext.EndGroups
	}

	switch action {
	case "on":
		settings := chatsettings.ReadAntiRaid(chatID)
		enabled, err := a.enableRaid(chatID, settings.RaidTime)
		if err != nil {
			log.WithError(err).Errorf("[AntiRaid] Failed to enable raid in chat %d", chatID)
			text, _ := tr.GetString("error_generic")
			_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		if !enabled {
			text, _ := tr.GetString("antiraid_already_active")
			_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_enabled", i18n.TranslationParams{"duration": formatDuration(settings.RaidTime)})
		_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		_, _, _ = msg.EditText(bot, &gotgbot.EditMessageTextOpts{Text: formatting.ToTelegramHTML(text), ParseMode: formatting.HTML})
		logRaidMode(bot, chat, &query.From, true, settings.RaidTime)
	case "off":
		disabled, err := a.disableRaid(chatID)
		if err != nil {
			text, _ := tr.GetString("error_generic")
			_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		if !disabled {
			text, _ := tr.GetString("antiraid_not_active")
			_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		text, _ := tr.GetString("antiraid_disabled")
		_, _ = bot.AnswerCallbackQuery(query.Id, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		_, _, _ = msg.EditText(bot, &gotgbot.EditMessageTextOpts{Text: formatting.ToTelegramHTML(text), ParseMode: formatting.HTML})
		logRaidMode(bot, chat, &query.From, false, 0)
	default:
		return answerInvalidCallback(bot, ctx, query)
	}

	return ext.EndGroups
}

func parseDuration(input string) (seconds int, ok bool) {
	input = strings.TrimSpace(strings.ToLower(input))
	if len(input) == 0 {
		return 0, false
	}

	var duration int64
	unit := input[len(input)-1]
	if unit >= '0' && unit <= '9' {
		return 0, false
	}
	num, err := strconv.ParseInt(input[:len(input)-1], 10, 64)
	if err != nil || num < 0 {
		return 0, false
	}
	var multiplier int64
	switch unit {
	case 's':
		multiplier = 1
	case 'm':
		multiplier = 60
	case 'h':
		multiplier = 60 * 60
	case 'd':
		multiplier = 24 * 60 * 60
	case 'w':
		multiplier = 7 * 24 * 60 * 60
	default:
		return 0, false
	}
	if num > math.MaxInt64/multiplier {
		return 0, false
	}
	duration = num * multiplier

	if duration > int64(math.MaxInt) || chatsettings.RaidTime.Validate(int(duration)) != nil {
		return 0, false
	}
	return int(duration), true
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
	SetModuleEnabled(antiRaidModule.moduleName, true)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "antiraid", PMGroupArg: helpers.PMGroupArgOwn, Disableable: true,
		RequiredChecks: []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired)},
	}, pipelineHandler(antiRaidModule.antiraid))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "raidtime", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: declaredSettingChecks(chatsettings.RaidTime),
	}, pipelineHandler(antiRaidModule.raidTime))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "raidactiontime", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: declaredSettingChecks(chatsettings.RaidActionTime),
	}, pipelineHandler(antiRaidModule.raidActionTime))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "autoantiraid", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: declaredSettingChecks(chatsettings.AutoAntiRaidThreshold),
	}, pipelineHandler(antiRaidModule.autoAntiRaid))

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("antiraid"), antiRaidModule.callbackHandler))
}

func init() {
	RegisterLegacyModule("AntiRaid", 230, LoadAntiRaid)
}
