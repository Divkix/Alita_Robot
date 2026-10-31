package modules

import (
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var warnsModule = moduleStruct{moduleName: "Warns"}

// Serializes limit-punish + reset per (chat,user): two concurrent limit hits must
// not both punish and then wipe each other's freshly added warn with ResetUserWarns.
// ponytail: fixed 32 shards; resize only if warn-punish contention shows up.
var warnPunishShards [32]sync.Mutex

func warnPunishLock(chatID, userID int64) *sync.Mutex {
	return &warnPunishShards[uint64(chatID*31+userID)%uint64(len(warnPunishShards))]
}

// defaultWarnActionTime applies to tban/tmute rows without a usable duration,
// such as backups taken before warn_action_time existed. Telegram treats a
// missing or out-of-range until_date as forever, which a warn must never be.
const defaultWarnActionTime = 60 * 60

func isTimedWarnMode(mode string) bool {
	return mode == "tban" || mode == "tmute"
}

func warnActionTime(settings *models.WarnSettings) int {
	if !models.ValidWarnActionTime(settings.WarnActionTime) {
		return defaultWarnActionTime
	}
	return settings.WarnActionTime
}

// warnModeLabel renders the mode for display, with the duration for tban/tmute.
func warnModeLabel(settings *models.WarnSettings) string {
	if isTimedWarnMode(settings.WarnMode) {
		return settings.WarnMode + " " + formatDuration(warnActionTime(settings))
	}
	return settings.WarnMode
}

func (moduleStruct) setWarnMode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := resolveAdminChat(b, ctx, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var replyText string

	if len(args) > 0 {
		mode := strings.ToLower(args[0])
		modeKeys := map[string]string{
			"ban":   "warns_mode_updated_ban",
			"kick":  "warns_mode_updated_kick",
			"mute":  "warns_mode_updated_mute",
			"tban":  "warns_mode_updated_tban",
			"tmute": "warns_mode_updated_tmute",
		}
		if key, ok := modeKeys[mode]; ok {
			actionTime := 0
			if isTimedWarnMode(mode) {
				seconds := extraction.ExtractDuration(b, ctx, strings.Join(args[1:], " "))
				if seconds == -1 {
					return ext.EndGroups
				}
				actionTime = int(seconds)
			}
			if err := warns.SetWarnMode(chat.Id, mode, actionTime); err != nil {
				log.Errorf("[Warns] SetWarnMode failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			replyText, _ = tr.GetString(key)
			if isTimedWarnMode(mode) {
				replyText = fmt.Sprintf(replyText, formatDuration(actionTime))
			}
		} else {
			temp, _ := tr.GetString("warns_mode_unknown")
			replyText = fmt.Sprintf(temp, html.EscapeString(args[0]))
		}
	} else {
		replyText, _ = tr.GetString("warns_specify_action")
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// warnExpiryLabel renders the chat's warn expiry for display.
func warnExpiryLabel(tr *i18n.Translator, settings *models.WarnSettings) string {
	if settings.WarnExpiry > 0 {
		return formatDuration(settings.WarnExpiry)
	}
	label, _ := tr.GetString("warns_expiry_never")
	return label
}

// setWarnTime sets how long a warn counts: /setwarntime 30d, or off.
func (moduleStruct) setWarnTime(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := resolveAdminChat(b, ctx, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var replyText string
	switch {
	case len(args) == 0:
		settings := warns.GetWarnSettingContext(tracing.UpdateContext(ctx), chat.Id)
		if settings.WarnExpiry > 0 {
			temp, _ := tr.GetString("warns_expiry_current")
			replyText = fmt.Sprintf(temp, formatDuration(settings.WarnExpiry))
		} else {
			replyText, _ = tr.GetString("warns_expiry_current_off")
		}
	case slices.Contains([]string{"off", "no", "false", "0"}, strings.ToLower(args[0])) && len(args) == 1:
		if err := warns.SetWarnExpiry(chat.Id, 0); err != nil {
			log.Errorf("[Warns] SetWarnExpiry failed for chat %d: %v", chat.Id, err)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(b, errText, formatting.Shtml())
			return ext.EndGroups
		}
		replyText, _ = tr.GetString("warns_expiry_off")
	default:
		seconds := extraction.ExtractDuration(b, ctx, strings.Join(args, " "))
		if seconds == -1 {
			return ext.EndGroups
		}
		if err := warns.SetWarnExpiry(chat.Id, int(seconds)); err != nil {
			log.Errorf("[Warns] SetWarnExpiry failed for chat %d: %v", chat.Id, err)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = msg.Reply(b, errText, formatting.Shtml())
			return ext.EndGroups
		}
		temp, _ := tr.GetString("warns_expiry_set")
		replyText = fmt.Sprintf(temp, formatDuration(int(seconds)))
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// warnIssuer reports who issues a warn from ctx. An anonymous admin arrives
// either directly, when the chat allows anonymous admins and the sender is the
// group itself, or replayed after the confirmation button: the confirming
// admin is then the effective user and the message keeps its original
// GroupAnonymousBot sender.
func warnIssuer(ctx *ext.Context) models.WarnEntry {
	var issuer models.WarnEntry
	if ctx.EffectiveUser != nil {
		issuer.WarnedBy = ctx.EffectiveUser.Id
	}
	if ctx.EffectiveSender != nil && ctx.EffectiveSender.IsAnonymousAdmin() {
		issuer.Anonymous = true
	}
	if msg := ctx.EffectiveMessage; msg != nil && msg.From != nil && msg.From.Id == chat_status.GroupAnonymousBotID {
		issuer.Anonymous = true
	}
	return issuer
}

// maxShownReasonLen caps a reason as shown, after escaping, in UTF-16 units.
// With the issuer and time its line stays under Telegram's message length, so
// SplitMessage breaks only between lines, never inside an entity or a tag.
const maxShownReasonLen = 3000

// shownReason escapes reason and shortens it, at a whole escaped character,
// to at most maxShownReasonLen UTF-16 units, ending with "…" when shortened.
func shownReason(reason string, escape func(string) string) string {
	if out := escape(reason); formatting.UTF16Len(out) <= maxShownReasonLen {
		return out
	}
	var sb strings.Builder
	n := 0
	for _, r := range reason {
		part := escape(string(r))
		k := formatting.UTF16Len(part)
		if n+k > maxShownReasonLen-1 {
			break
		}
		sb.WriteString(part)
		n += k
	}
	return sb.String() + "…"
}

// warnEntryLines renders one "\n - reason (issuer, time)" line per entry. An
// entry without a time predates entries and shows its reason only. escape is
// applied to the reason and the issuer name.
func warnEntryLines(b *gotgbot.Bot, tr *i18n.Translator, entries []models.WarnEntry, escape func(string) string) string {
	metaFormat, _ := tr.GetString("warns_entry_meta")
	var sb strings.Builder
	for _, e := range entries {
		line := shownReason(e.Reason, escape)
		if e.WarnedAt != nil && metaFormat != "" {
			line = fmt.Sprintf(metaFormat, line, escape(warnIssuerLabel(b, tr, e)), e.WarnedAt.UTC().Format("2006-01-02 15:04 UTC"))
		}
		fmt.Fprintf(&sb, "\n - %s", line)
	}
	return sb.String()
}

func warnIssuerLabel(b *gotgbot.Bot, tr *i18n.Translator, e models.WarnEntry) string {
	key := ""
	switch {
	case e.Anonymous:
		key = "warns_issuer_anonymous"
	case e.WarnedBy == b.Id || e.WarnedBy == models.WarnedByBot:
		key = "warns_issuer_auto"
	case e.WarnedBy == 0:
		key = "warns_issuer_unknown"
	}
	if key != "" {
		label, _ := tr.GetString(key)
		return label
	}
	if u, err := user.GetUserBasicInfoCached(e.WarnedBy); err == nil && u != nil {
		if u.Name != "" {
			return u.Name
		}
		if u.UserName != "" {
			return "@" + u.UserName
		}
	}
	return strconv.FormatInt(e.WarnedBy, 10)
}

// logWarnRemoval posts an admin's removal of userID's warns under the warns
// log category. Call it only after the removal was written.
func logWarnRemoval(b *gotgbot.Bot, chat *gotgbot.Chat, admin *gotgbot.User, tag string, userID int64) {
	name := strconv.FormatInt(userID, 10)
	if u, err := user.GetUserBasicInfoCached(userID); err == nil && u != nil && u.Name != "" {
		name = u.Name
	}
	actionlog.Warn(b, chat, admin, tag, userID, name)
}

// punishAtWarnLimit applies the chat's warn mode to u, who just reached the
// warn limit, and returns the reply. punished is false for an unknown mode.
// A ban is recorded as issued by bannedBy (0 for the bot) for reason, the
// reason of the warn that reached the limit; banRecordID is its record (0
// when the mode bans nobody or the record write failed).
func punishAtWarnLimit(b *gotgbot.Bot, chat *gotgbot.Chat, tr *i18n.Translator, warnrc *models.WarnSettings, u gotgbot.User, numWarns int, bannedBy int64, reason string) (reply string, punished bool, banRecordID int64, err error) {
	userId := u.Id
	switch warnrc.WarnMode {
	case "kick":
		err = kickMember(b, chat.Id, userId)
		temp, _ := tr.GetString("warns_limit_kicked")
		reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, formatting.MentionHtml(u.Id, u.FirstName))
		if err != nil {
			log.Errorf("[warn] warnlimit: kick (%d) - %s", userId, err)
			return "", false, 0, err
		}
		punished = true
	case "mute":
		_, err = chat.RestrictMember(b, userId,
			MutedPermissions,
			nil,
		)
		temp, _ := tr.GetString("warns_limit_muted")
		reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, formatting.MentionHtml(u.Id, u.FirstName))
		if err != nil {
			log.Errorf("[warn] warnlimit: mute (%d) - %s", userId, err)
			return "", false, 0, err
		}
		punished = true
	case "ban":
		_, err = chat.BanMember(b, userId, nil)
		temp, _ := tr.GetString("warns_limit_banned")
		reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, formatting.MentionHtml(u.Id, u.FirstName))
		if err != nil {
			log.Errorf("[warn] warnlimit: ban (%d) - %s", userId, err)
			return "", false, 0, err
		}
		banRecordID = recordBan(banRecord{chatID: chat.Id, userID: userId, bannedBy: bannedBy, source: models.BanSourceWarn, reason: reason})
		punished = true
	case "tban", "tmute":
		actionTime := warnActionTime(warnrc)
		untilDate := time.Now().Unix() + int64(actionTime)
		key := "warns_limit_tbanned"
		if warnrc.WarnMode == "tban" {
			_, err = chat.BanMember(b, userId, &gotgbot.BanChatMemberOpts{UntilDate: untilDate})
		} else {
			key = "warns_limit_tmuted"
			_, err = chat.RestrictMember(b, userId, MutedPermissions, &gotgbot.RestrictChatMemberOpts{UntilDate: untilDate})
		}
		if err != nil {
			log.Errorf("[warn] warnlimit: %s (%d) - %s", warnrc.WarnMode, userId, err)
			return "", false, 0, err
		}
		if warnrc.WarnMode == "tban" {
			banRecordID = recordBan(banRecord{
				chatID: chat.Id, userID: userId, bannedBy: bannedBy,
				source: models.BanSourceWarn, reason: reason, untilUnix: untilDate,
			})
		}
		temp, _ := tr.GetString(key)
		reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, formatting.MentionHtml(u.Id, u.FirstName), formatDuration(actionTime))
		punished = true
	default:
		log.Warnf("[Warns] Unknown warn mode: %s", warnrc.WarnMode)
	}
	return reply, punished, banRecordID, nil
}

// warnThisUser warns userId; issuer carries who issues the warn (warnIssuer
// for admin commands, the bot itself for automatic warns).
func (moduleStruct) warnThisUser(b *gotgbot.Bot, ctx *ext.Context, userId int64, reason, warnType string, issuer models.WarnEntry) (err error) {
	var (
		reply    string
		keyboard gotgbot.InlineKeyboardMarkup
	)

	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	removeWarnText, _ := tr.GetString("warns_remove_button")
	rulesButtonText, _ := tr.GetString("common_rules_button_emoji")

	if chat_status.IsUserAdmin(b, chat.Id, userId) {
		text, _ := tr.GetString("warns_admin_warning_error")
		_, err = msg.Reply(b, text, nil)
		return err
	}

	if !chat_status.IsValidUserId(userId) || chat_status.IsSystemUser(userId) {
		return ext.EndGroups
	}
	chatMember, err := b.GetChatMemberWithContext(tracing.UpdateContext(ctx), chat.Id, userId, nil)
	if error_handling.IsMissingChatMemberError(err) {
		replyHTML(b, msg, trS(tr, "common_user_not_in_chat"))
		return ext.EndGroups
	}
	if err != nil {
		return err
	}

	u := chatMember.MergeChatMember().User
	mu := warnPunishLock(chat.Id, userId)
	mu.Lock()
	defer mu.Unlock()
	warnrc := warns.GetWarnSettingContext(tracing.UpdateContext(ctx), chat.Id)
	issuer.Reason = reason
	numWarns, entries, err := warns.WarnUser(userId, chat.Id, issuer)
	if err != nil {
		text, _ := tr.GetString("common_settings_save_failed")
		_, sendErr := msg.Reply(b, text, formatting.Shtml())
		if sendErr != nil {
			return sendErr
		}
		return ext.EndGroups
	}

	switch warnType {
	case "dwarn":
		if msg.ReplyToMessage != nil {
			if _, deleteErr := msg.ReplyToMessage.Delete(b, nil); deleteErr != nil {
				log.Errorf("[Warns] Failed to delete message: %v", deleteErr)
			}
		}
	case "swarn":
		// From PM there is no command message in the group to delete.
		if !inPM(msg) {
			_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		}
	}

	// logWarn posts an admin's warn under the warns log category. The bot's
	// own warns (blacklists) are no admin's: they log under automated, and
	// only at the limit.
	// banRecordID is the record of a ban the warn limit made, for the
	// entry's Unban button.
	var banRecordID int64
	logWarn := func(tag string, extra ...actionlog.Field) {
		if issuer.WarnedBy == b.Id {
			return
		}
		fields := append([]actionlog.Field{{Name: "Warns", Value: fmt.Sprintf("%d/%d", numWarns, warnrc.WarnLimit)}}, extra...)
		fields = append(fields, actionlog.Field{Name: "Reason", Value: shownReason(reason, func(s string) string { return s })})
		actionlog.WarnWithMarkup(b, chat, unbanLogMarkup(chat.Id, banRecordID), ctx.EffectiveUser, tag, u.Id, u.FirstName, fields...)
	}

	if numWarns >= warnrc.WarnLimit {
		var punished bool
		// The bot's own warns (blacklists) make a ban no admin issued.
		bannedBy := issuer.WarnedBy
		if bannedBy == b.Id {
			bannedBy = 0
		}
		reply, punished, banRecordID, err = punishAtWarnLimit(b, chat, tr, warnrc, u, numWarns, bannedBy, reason)
		if err != nil {
			// The warn stays written though the limit's action failed.
			logWarn("WARN")
			return err
		}

		if !punished {
			// An unknown warn mode punishes nobody: the warn alone was made.
			logWarn("WARN")
		} else {
			// Only the bot's own warns (blacklists) are automated; an admin's
			// warn at the limit is a human action.
			if issuer.WarnedBy == b.Id {
				actionlog.AutomatedWithMarkup(b, chat, unbanLogMarkup(chat.Id, banRecordID), "WARNLIMIT", u.Id, u.FirstName,
					actionlog.Field{Name: "Warns", Value: fmt.Sprintf("%d/%d", numWarns, warnrc.WarnLimit)},
					actionlog.Field{Name: "Action", Value: warnModeLabel(warnrc)},
					actionlog.Field{Name: "By", Value: "blacklist"},
				)
			}
			logWarn("WARNLIMIT", actionlog.Field{Name: "Action", Value: warnModeLabel(warnrc)})
			if _, resetErr := warns.ResetUserWarns(userId, chat.Id); resetErr != nil {
				return resetErr
			}
		}
		reply += warnEntryLines(b, tr, entries, html.EscapeString)
	} else {
		logWarn("WARN")
		rules := rules.GetChatRulesInfo(chat.Id)
		removeWarnData := moderationButtonData("rmWarn", msg, chat, map[string]string{"u": fmt.Sprint(u.Id)})
		if len(rules.Rules) >= 1 {
			keyboard = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         removeWarnText,
							CallbackData: removeWarnData,
						},
						{
							Text: rulesButtonText,
							Url:  fmt.Sprintf("t.me/%s?start=rules_%d", b.Username, chat.Id),
						},
					},
				},
			}
		} else {
			keyboard = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         removeWarnText,
							CallbackData: removeWarnData,
						},
					},
				},
			}
		}

		temp, _ := tr.GetString("warns_user_warning")
		reply = fmt.Sprintf(temp, formatting.MentionHtml(u.Id, u.FirstName), numWarns, warnrc.WarnLimit)

		if reason != "" {
			temp, _ := tr.GetString("warns_warning_reason")
			reply += fmt.Sprintf(temp, shownReason(reason, html.EscapeString))
		}
	}
	// The list at the warn limit can pass Telegram's length limit; only the
	// first part replies to the command and carries the buttons. The reply
	// goes where the command came from: from PM, to the PM.
	for i, part := range formatting.SplitMessage(withGroupLine(tr, msg, chat, reply)) {
		opts := &gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		}
		if i == 0 {
			opts.ReplyParameters = &gotgbot.ReplyParameters{
				MessageId:                msg.MessageId,
				AllowSendingWithoutReply: true,
			}
			opts.ReplyMarkup = &keyboard
		}
		if _, err = b.SendMessage(msg.Chat.Id, part, opts); err != nil {
			log.Errorf("[warn] sendMessage (%d) - %s", userId, err)
			return err
		}
	}

	return ext.EndGroups
}

func (m moduleStruct) dispatchWarn(b *gotgbot.Bot, ctx *ext.Context, warnType string) error {
	mc, err := buildModerationCtx(&warnsModule, b, ctx)
	if err != nil {
		return ext.EndGroups
	}

	mc.Command = warnType
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := mc.Tr

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		if mc.sentInPM() {
			// From PM, show how to name the group and the user.
			err = mc.replyCommandHelp(text)
		} else {
			_, err = msg.Reply(b, text, formatting.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !chat_status.IsUserInChat(b, chat, userId) {
		return ext.EndGroups
	}
	var warnusr int64
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.Id == userId {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, warnType, warnIssuer(ctx))
}

func (m moduleStruct) warnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.dispatchWarn(b, ctx, "warn")
}

func (m moduleStruct) sWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.dispatchWarn(b, ctx, "swarn")
}

func (m moduleStruct) dWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.dispatchWarn(b, ctx, "dwarn")
}

func (moduleStruct) warnings(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	warnrc := warns.GetWarnSettingContext(tracing.UpdateContext(ctx), chat.Id)
	temp, _ := tr.GetString("warns_settings_display")
	text := fmt.Sprintf(temp, warnrc.WarnLimit, warnModeLabel(warnrc), warnExpiryLabel(tr, warnrc))
	_, err := msg.Reply(b, withGroupLine(tr, msg, chat, text), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (moduleStruct) warns(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		// The name was not found and extraction has already said so.
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		var err error
		if inPM(msg) {
			// From PM, show how to name the group and the user.
			err = replyCommandHelp(b, msg, tr, "warns", text)
		} else {
			_, err = msg.Reply(b, text, formatting.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	numWarns, entries := warns.GetWarnsContext(tracing.UpdateContext(ctx), userId, chat.Id)
	text := ""

	if numWarns != 0 {
		warnrc := warns.GetWarnSettingContext(tracing.UpdateContext(ctx), chat.Id)
		if len(entries) > 0 {
			temp, _ := tr.GetString("warns_user_warnings_list")
			text = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit)
			// Plain-text reply: reasons and names are sent as they are.
			text += warnEntryLines(b, tr, entries, func(s string) string { return s })
			msgs := formatting.SplitMessage(confirmText(ctx, tr, text, chat))
			for _, msgText := range msgs {
				_, err := msg.Reply(b, msgText, nil)
				if err != nil {
					log.Error(err)
					return err
				}
			}
		} else {
			temp, _ := tr.GetString("warns_user_warnings_no_reasons")
			_, err := msg.Reply(b, confirmText(ctx, tr, fmt.Sprintf(temp, numWarns, warnrc.WarnLimit), chat), nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		text, _ := tr.GetString("warns_user_no_warnings")
		_, err := msg.Reply(b, confirmText(ctx, tr, text, chat), nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

func (moduleStruct) rmWarnButton(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// A button sent from PM names its group; the presser and the bot are
	// checked there.
	decoded, decodedOK := decodeCallbackData(query.Data, "rmWarn")
	chat, chatOK := buttonChat(ctx, decoded)
	if chatOK && chat == nil {
		// Telegram sent no message with the press: no chat to act or answer in.
		return ext.EndGroups
	}
	if chatOK && !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if chatOK && !chat_status.RequireBotAdmin(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	userMatch := ""
	if decodedOK {
		userMatch, _ = decoded.Field("u")
	}
	if !chatOK || chat == nil || userMatch == "" {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	userId, parseErr := strconv.ParseInt(userMatch, 10, 64)
	if parseErr != nil {
		log.Errorf("[Warns] Failed to parse user ID from callback: %v", parseErr)
		return answerInvalidCallback(b, ctx, query)
	}
	var replyText string

	removed, removeErr := warns.RemoveWarn(userId, chat.Id)
	if removeErr != nil {
		replyText, _ = tr.GetString("error_generic")
	} else if removed {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
		logWarnRemoval(b, chat, user, "RMWARN", userId)
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: replyText})
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyText, ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) setWarnLimit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := resolveAdminChat(b, ctx, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var replyText string

	if len(args) == 0 {
		replyText, _ = tr.GetString("warns_limit_set_help")
	} else {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			temp, _ := tr.GetString("warns_invalid_number")
			replyText = fmt.Sprintf(temp, html.EscapeString(args[0]))
		} else {
			if num < 1 || num > 100 {
				replyText, _ = tr.GetString("warns_limit_range_error")
			} else {
				if err := warns.SetWarnLimit(chat.Id, num); err != nil {
					log.Errorf("[Warns] SetWarnLimit failed for chat %d: %v", chat.Id, err)
					errText, _ := tr.GetString("common_settings_save_failed")
					_, _ = msg.Reply(b, errText, formatting.Shtml())
					return ext.EndGroups
				}
				temp, _ := tr.GetString("warns_limit_updated")
				replyText = fmt.Sprintf(temp, num)
			}
		}
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) resetWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		var err error
		if inPM(msg) {
			// From PM, show how to name the group and the user.
			err = replyCommandHelp(b, msg, tr, "resetwarns", text)
		} else {
			_, err = msg.Reply(b, text, formatting.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	removed, resetErr := warns.ResetUserWarns(userId, chat.Id)
	var text string
	if resetErr != nil {
		text, _ = tr.GetString("error_generic")
	} else if removed {
		text, _ = tr.GetString("warns_reset_success")
		logWarnRemoval(b, chat, user, "RESETWARNS", userId)
	} else {
		text, _ = tr.GetString("warns_no_warns_to_remove")
	}
	_, err := msg.Reply(b, withGroupLine(tr, msg, chat, text), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) resetAllWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	chat := groupOnlyChat(b, ctx)
	if chat == nil {
		return ext.EndGroups
	}
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	warnrc := warns.GetAllChatWarns(chat.Id)
	if warnrc == 0 {
		text, _ := tr.GetString("warns_no_users_warned")
		_, err := msg.Reply(b, text, formatting.Shtml())
		return err
	}

	if chat_status.RequireUserOwner(b, ctx, chat, user.Id) {
		text, _ := tr.GetString("warns_reset_all_confirm")
		text = confirmText(ctx, tr, text, chat)
		_, err := msg.Reply(b, text,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: confirmKeyboard(tr, "rmAllChatWarns", chat),
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

func (moduleStruct) warnsButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	decoded, _ := decodeCallbackData(query.Data, "rmAllChatWarns")
	chat := ownerConfirmChat(b, ctx, query, decoded)
	if chat == nil {
		return ext.EndGroups
	}

	response, _ := decoded.Field("a")
	if response == "" {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	var replyText string

	switch response {
	case "yes":
		if err := warns.ResetAllChatWarns(chat.Id); err == nil {
			helpText, _ = tr.GetString("warns_reset_all_success")
			replyText, _ = tr.GetString("warns_reset_all_final")
			actionlog.Warn(b, chat, &query.From, "RESETALLWARNS", 0, "")
		} else {
			helpText, _ = tr.GetString("error_generic")
			replyText = helpText
		}
	case "no":
		helpText, _ = tr.GetString("warns_reset_all_cancelled")
		replyText = helpText
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyText})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: helpText,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) removeWarn(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		var err error
		if inPM(msg) {
			// From PM, show how to name the group and the user.
			err = replyCommandHelp(b, msg, tr, "rmwarn", text)
		} else {
			_, err = msg.Reply(b, text, formatting.Shtml())
		}
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	var replyText string
	removed, removeErr := warns.RemoveWarn(userId, chat.Id)
	if removeErr != nil {
		replyText, _ = tr.GetString("error_generic")
	} else if removed {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
		logWarnRemoval(b, chat, user, "RMWARN", userId)
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	_, err := msg.Reply(b, withGroupLine(tr, msg, chat, replyText), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// From PM, the commands that act on a user named after the group take the
// group as their first argument (or from the connection), as the bans
// commands do. /dwarn needs a reply to a group message, so it stays
// group-only.
var (
	warnDesc        = helpers.CommandDescriptor{Name: "warn", PMGroupArg: helpers.PMGroupArgOwn}
	swarnDesc       = helpers.CommandDescriptor{Name: "swarn", PMGroupArg: helpers.PMGroupArgOwn}
	dwarnDesc       = helpers.CommandDescriptor{Name: "dwarn"}
	resetWarnsDesc  = helpers.CommandDescriptor{Name: "resetwarns", Aliases: []string{"resetwarn"}, PMGroupArg: helpers.PMGroupArgOwn}
	removeWarnDesc  = helpers.CommandDescriptor{Name: "rmwarn", Aliases: []string{"unwarn"}, PMGroupArg: helpers.PMGroupArgOwn}
	warnsStatusDesc = helpers.CommandDescriptor{Name: "warns", Disableable: true, PMGroupArg: helpers.PMGroupArgOwn}
	warningsDesc    = helpers.CommandDescriptor{Name: "warnings", PMGroupArg: helpers.PMGroupArgNone}
)

// warnsChatCheck points a /warns sent from PM at the group named or
// connected. Any member may run /warns in the group, so a member may from PM:
// the group's admins are not required. A reply in PM names no user. Disabled
// in that group, /warns answers a member who is no admin there as it does in
// the group: with nothing.
func warnsChatCheck(c *helpers.CommandContext) bool {
	if !inPM(c.Ctx.EffectiveMessage) {
		return true
	}
	chat := chat_status.IsUserConnected(c.Bot, c.Ctx, false, false)
	if chat == nil {
		return false
	}
	c.Ctx.EffectiveChat = chat
	c.Chat = chat
	if disabling.IsCommandDisabled(chat.Id, "warns") && !chat_status.IsUserAdmin(c.Bot, chat.Id, c.User.Id) {
		return false
	}
	dropPMReply(c)
	return true
}

// adminBotUserChecks are the checks of a command an admin runs with the bot
// an admin; chat checks (pmTargetChatCheck) run before the group's.
func adminBotUserChecks(cmd string, chat ...helpers.CheckFunc) []helpers.CheckFunc {
	return append(append([]helpers.CheckFunc{helpers.CheckDisabled(cmd)}, chat...),
		helpers.RequireGroup(),
		helpers.RequireBotAdmin(),
		helpers.RequireUserAdmin(),
	)
}

func initWarnDescs() {
	warnDesc.RequiredChecks = restrictChecks("warn", pmTargetChatCheck)
	swarnDesc.RequiredChecks = deleteRestrictChecks("swarn", pmTargetChatCheck)
	dwarnDesc.RequiredChecks = deleteRestrictChecks("dwarn")
	resetWarnsDesc.RequiredChecks = adminBotUserChecks("resetwarns", pmTargetChatCheck)
	removeWarnDesc.RequiredChecks = adminBotUserChecks("rmwarn", pmTargetChatCheck)
	warnsStatusDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("warns"), warnsChatCheck}
	warningsDesc.RequiredChecks = adminBotUserChecks("warnings", groupOnlyChatCheck)
}

func LoadWarns(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(warnsModule.moduleName, true)
	initWarnDescs()

	helpers.WrapCommand(dispatcher, warnDesc, pipelineHandler(warnsModule.warnUser))
	helpers.WrapCommand(dispatcher, swarnDesc, pipelineHandler(warnsModule.sWarnUser))
	helpers.WrapCommand(dispatcher, dwarnDesc, pipelineHandler(warnsModule.dWarnUser))
	helpers.WrapCommand(dispatcher, resetWarnsDesc, pipelineHandler(warnsModule.resetWarns))
	helpers.WrapCommand(dispatcher, removeWarnDesc, pipelineHandler(warnsModule.removeWarn))
	helpers.WrapCommand(dispatcher, warnsStatusDesc, pipelineHandler(warnsModule.warns))
	for _, cfg := range []struct {
		name    string
		arg     helpers.PMGroupArg
		handler func(*gotgbot.Bot, *ext.Context) error
	}{
		{"setwarnlimit", helpers.PMGroupArgOwn, warnsModule.setWarnLimit},
		{"setwarnmode", helpers.PMGroupArgOwn, warnsModule.setWarnMode},
		{"setwarntime", helpers.PMGroupArgOwn, warnsModule.setWarnTime},
		{"resetallwarns", helpers.PMGroupArgNone, warnsModule.resetAllWarns},
	} {
		desc := helpers.CommandDescriptor{
			Name:           cfg.name,
			RequiredChecks: []helpers.CheckFunc{helpers.CheckDisabled(cfg.name)},
			PMGroupArg:     cfg.arg,
		}
		helpers.WrapCommand(dispatcher, desc, pipelineHandler(cfg.handler))
	}
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllChatWarns"), warnsModule.warnsButtonHandler))
	helpers.WrapCommand(dispatcher, warningsDesc, pipelineHandler(warnsModule.warnings))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmWarn"), warnsModule.rmWarnButton))
}

func init() {
	RegisterLegacyModule("Warns", 200, LoadWarns)
	initWarnDescs()
	RegisterAnonymousAdminHandler("warn", anonPipelineHandler(warnDesc, warnsModule.warnUser))
	RegisterAnonymousAdminHandler("swarn", anonPipelineHandler(swarnDesc, warnsModule.sWarnUser))
	RegisterAnonymousAdminHandler("dwarn", anonPipelineHandler(dwarnDesc, warnsModule.dWarnUser))
}
