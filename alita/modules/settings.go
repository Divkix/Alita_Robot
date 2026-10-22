package modules

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const (
	settingsCandidateLimit = 50
	settingsMaxButtons     = 20
)

// settingsCandidateChats lists groups the user has posted in. It is a variable
// because the query needs Postgres jsonb, which the SQLite test database lacks.
var settingsCandidateChats = chats.ListChatsWithUser

// settingsPickerLiveBudget bounds the picker's Telegram admin checks for
// groups whose admin list is not cached. settingsPickerCooldown is the pause
// between one user's picker builds; it outlasts the budget, so one user never
// runs two builds at once. Variables so tests can drive them without sleeping.
var (
	settingsPickerLiveBudget = 3 * time.Second
	settingsPickerCooldown   = 5 * time.Second
	settingsPickerNow        = time.Now
)

// settingsPickerWait reserves a picker build for userID. While the previous
// build's pause lasts it returns wait = true and the notice to show instead.
func settingsPickerWait(tr *i18n.Translator, userID int64) (notice string, wait bool) {
	allowed, remaining := ratelimit.AcquireSettingsPicker(userID, settingsPickerCooldown)
	if allowed {
		return "", false
	}
	seconds := max(int(math.Ceil(remaining.Seconds())), 1)
	notice, _ = tr.GetString("settings_picker_wait", i18n.TranslationParams{"seconds": strconv.Itoa(seconds)})
	return notice, true
}

// settings handles /settings. In PM it shows the group picker; in a group it
// links the admin to the summary in PM so nothing else is posted there.
func (m moduleStruct) settings(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if chat.Type == "private" {
		if notice, wait := settingsPickerWait(tr, user.Id); wait {
			if _, err := msg.Reply(b, notice, formatting.Shtml()); err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		text, kb := m.settingsPicker(b, ctx, tr, user.Id)
		if _, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	text, _ := tr.GetString("settings_open_in_pm")
	button, _ := tr.GetString("settings_open_in_pm_button")
	kb := gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
		Text: button,
		Url:  fmt.Sprintf("https://t.me/%s?start=settings_%d", b.Username, chat.Id),
	}}}}
	if _, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// settingsPickerEntry is one group button of the picker.
type settingsPickerEntry struct {
	ChatID int64
	Title  string
}

// settingsPickerListKey stores a user's last picker for the pause, so
// "Another group" inside it shows that list again instead of a notice.
func settingsPickerListKey(userID int64) string {
	return fmt.Sprintf("%ssettings_picker:%d", cache.DataCachePrefix, userID)
}

// settingsPicker lists the connected group first, then recently active groups
// where the user is an admin: those with a cached admin list, then the rest
// while settingsPickerLiveBudget lasts. The list is kept for the pause.
func (moduleStruct) settingsPicker(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, userID int64) (string, gotgbot.InlineKeyboardMarkup) {
	entries := settingsPickerEntries(b, ctx, userID)
	storeSettingsPickerEntries(userID, entries)
	return settingsPickerView(tr, userID, entries)
}

// adminGroupsForUser lists the groups userID administers for a PM command
// that needs a group, as text rather than buttons. It shares the picker's
// pause and budget: during the pause it returns the list the last build kept,
// or, when there is none, the notice to show instead (wait is not empty).
func adminGroupsForUser(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, userID int64) (entries []settingsPickerEntry, wait string) {
	if notice, paused := settingsPickerWait(tr, userID); paused {
		if stored, ok := storedSettingsPickerEntries(userID); ok {
			return stored, ""
		}
		return nil, notice
	}
	entries = settingsPickerEntries(b, ctx, userID)
	storeSettingsPickerEntries(userID, entries)
	return entries, ""
}

// storeSettingsPickerEntries keeps a user's list for the pause.
func storeSettingsPickerEntries(userID int64, entries []settingsPickerEntry) {
	if m := cache.GetMarshal(); m != nil && settingsPickerCooldown > 0 {
		if err := m.Set(cache.Context, settingsPickerListKey(userID), entries, store.WithExpiration(settingsPickerCooldown)); err != nil {
			log.Debugf("[Settings] store picker for user %d: %v", userID, err)
		}
	}
}

// storedSettingsPickerEntries returns the list kept for userID during the
// pause; ok is false when there is none.
func storedSettingsPickerEntries(userID int64) ([]settingsPickerEntry, bool) {
	m := cache.GetMarshal()
	if m == nil {
		return nil, false
	}
	stored, err := m.Get(cache.Context, settingsPickerListKey(userID), new([]settingsPickerEntry))
	entries, isList := stored.(*[]settingsPickerEntry)
	if err != nil || !isList || entries == nil {
		return nil, false
	}
	return *entries, true
}

// storedSettingsPicker renders the list settingsPicker kept for userID during
// the pause; ok is false when there is none.
func storedSettingsPicker(tr *i18n.Translator, userID int64) (text string, kb gotgbot.InlineKeyboardMarkup, ok bool) {
	entries, ok := storedSettingsPickerEntries(userID)
	if !ok {
		return "", kb, false
	}
	text, kb = settingsPickerView(tr, userID, entries)
	return text, kb, true
}

// settingsPickerView marks the group the user is connected to now and puts it
// first, so a list shown again after a pick shows the new connection.
func settingsPickerView(tr *i18n.Translator, userID int64, entries []settingsPickerEntry) (string, gotgbot.InlineKeyboardMarkup) {
	if len(entries) == 0 {
		text, _ := tr.GetString("settings_picker_empty")
		return text, gotgbot.InlineKeyboardMarkup{}
	}
	var connectedID int64
	if conn := connections.Connection(userID); conn.Connected {
		connectedID = conn.ChatId
	}
	rows := make([][]gotgbot.InlineKeyboardButton, 0, len(entries))
	for _, entry := range entries {
		title := entry.Title
		if strings.TrimSpace(title) == "" {
			title = strconv.FormatInt(entry.ChatID, 10)
		}
		row := []gotgbot.InlineKeyboardButton{{
			Text:         title,
			CallbackData: encodeCallbackData("settings", map[string]string{"a": "p", "c": strconv.FormatInt(entry.ChatID, 10)}),
		}}
		if entry.ChatID == connectedID {
			row[0].Text = "✅ " + title
			rows = append([][]gotgbot.InlineKeyboardButton{row}, rows...)
			continue
		}
		rows = append(rows, row)
	}
	text, _ := tr.GetString("settings_picker_text")
	return text, gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func settingsPickerEntries(b *gotgbot.Bot, ctx *ext.Context, userID int64) []settingsPickerEntry {
	return settingsPickerEntriesFrom(b, ctx, userID, settingsCandidates(ctx, userID))
}

// settingsCandidates scans for the groups userID posted in most recently.
// Callers hold the picker's pause, which bounds the scans to one per user per
// pause.
func settingsCandidates(ctx *ext.Context, userID int64) []models.Chat {
	candidates, err := settingsCandidateChats(tracing.UpdateContext(ctx), userID, settingsCandidateLimit)
	if err != nil {
		log.Warnf("[Settings] candidate chats for user %d: %v", userID, err)
	}
	return candidates
}

// settingsPickerEntriesFrom builds the picker's list from candidates, the
// groups settingsCandidates found.
func settingsPickerEntriesFrom(b *gotgbot.Bot, ctx *ext.Context, userID int64, candidates []models.Chat) []settingsPickerEntry {
	deadline := settingsPickerNow().Add(settingsPickerLiveBudget)
	// The deadline also bounds each check's Telegram calls and its wait for a
	// shared admin-list load, so one hung group cannot hold the picker past it.
	checkCtx, cancel := context.WithDeadline(tracing.UpdateContext(ctx), deadline)
	defer cancel()
	var entries []settingsPickerEntry
	seen := map[int64]bool{}
	addChat := func(chatID int64, title string) {
		seen[chatID] = true
		entries = append(entries, settingsPickerEntry{ChatID: chatID, Title: title})
	}

	// The connected group stays listed when the budget cuts its check: the user
	// chose it, and tapping it re-checks their rights. A panicking admin-list
	// load leaves it out like any group, since tapping it would panic again.
	if conn := connections.Connection(userID); conn.Connected && conn.ChatId != 0 {
		isAdmin, err := chat_status.IsUserAdminWithContext(checkCtx, b, conn.ChatId, userID)
		// Checked either way, so the candidate loop does not check it again.
		seen[conn.ChatId] = true
		if isAdmin || (err != nil && !settingsPickerLoadPanicked(err)) {
			title := ""
			if info, err := chats.GetChatBasicInfoCached(conn.ChatId); err == nil && info != nil {
				title = info.ChatName
			}
			addChat(conn.ChatId, title)
		}
	}

	// Cached admin lists answer without Telegram, so they go first.
	var cold []models.Chat
	for _, candidate := range candidates {
		if len(entries) >= settingsMaxButtons {
			break
		}
		if seen[candidate.ChatId] {
			continue
		}
		isAdmin, known := chat_status.IsUserAdminCached(candidate.ChatId, userID)
		if !known {
			cold = append(cold, candidate)
			continue
		}
		if isAdmin {
			addChat(candidate.ChatId, candidate.ChatName)
		}
	}
	// Each cold check is up to four Telegram calls (getChat, the bot's own
	// getChatMember, getChatAdministrators with retries, the user's
	// getChatMember). A check the deadline cuts shows nothing and caches
	// nothing, so the next press checks that group again; an admin-list load it
	// stopped waiting for finishes in the background and warms the cache.
	// connectForSettings re-checks admin rights when a group is picked.
	for _, candidate := range cold {
		if len(entries) >= settingsMaxButtons || !settingsPickerNow().Before(deadline) {
			break
		}
		isAdmin, err := chat_status.IsUserAdminWithContext(checkCtx, b, candidate.ChatId, userID)
		settingsPickerLoadPanicked(err)
		if err == nil && isAdmin {
			addChat(candidate.ChatId, candidate.ChatName)
		}
		if checkCtx.Err() != nil {
			break
		}
	}
	return entries
}

// settingsPickerLoadPanicked logs an admin-list load panic the picker received
// instead of re-raising it, so it is still reported once.
func settingsPickerLoadPanicked(err error) bool {
	var loadPanic *cache.AdminLoadPanicError
	if !errors.As(err, &loadPanic) {
		return false
	}
	log.Errorf("[Settings] %v", loadPanic)
	return true
}

func settingsSummaryView(ctx *ext.Context, tr *i18n.Translator, chat *gotgbot.Chat) (string, gotgbot.InlineKeyboardMarkup) {
	text := renderSettingsSummary(tr, chat.Title, collectChatSettings(tracing.UpdateContext(ctx), chat.Id))
	refresh, _ := tr.GetString("settings_btn_refresh")
	other, _ := tr.GetString("settings_btn_other_group")
	kb := gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
		{Text: refresh, CallbackData: encodeCallbackData("settings", map[string]string{"a": "r", "c": strconv.FormatInt(chat.Id, 10)})},
		{Text: other, CallbackData: encodeCallbackData("settings", map[string]string{"a": "l"})},
	}}}
	return text, kb
}

// connectForSettings re-checks that the user may manage chatID, connects them
// and returns the chat. On refusal it returns the i18n key to show instead.
func connectForSettings(b *gotgbot.Bot, chatID, userID int64) (*gotgbot.Chat, string) {
	if allowed, _ := canUserConnectToChat(b, chatID, userID); !allowed || !chat_status.IsUserAdmin(b, chatID, userID) {
		return nil, "connections_is_user_connected_user_not_admin"
	}
	full, err := b.GetChat(chatID, nil)
	if err != nil || full == nil {
		return nil, "helpers_chat_not_found"
	}
	if err := connections.ConnectId(userID, chatID); err != nil {
		return nil, "common_settings_save_failed"
	}
	chat := full.ToChat()
	return &chat, ""
}

func (m moduleStruct) settingsCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	msg := query.Message
	decoded, ok := decodeCallbackData(query.Data, "settings")
	// Every action edits the message the button sits on, so a forged button on
	// a bot message in a group must not overwrite it (or connect the presser).
	if msg == nil || !ok || ctx.EffectiveChat == nil || ctx.EffectiveChat.Type != "private" {
		return answerInvalidCallback(b, ctx, query)
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	action, _ := decoded.Field("a")

	var (
		text string
		kb   gotgbot.InlineKeyboardMarkup
	)
	switch action {
	case "p":
		raw, _ := decoded.Field("c")
		chatID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return answerInvalidCallback(b, ctx, query)
		}
		chat, denyKey := connectForSettings(b, chatID, query.From.Id)
		if chat == nil {
			deny, _ := tr.GetString(denyKey)
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: deny, ShowAlert: true})
			return ext.EndGroups
		}
		text, kb = settingsSummaryView(ctx, tr, chat)
	case "r":
		chat := callbackAdminChat(b, ctx, query, decoded, false)
		if chat == nil {
			return ext.EndGroups
		}
		text, kb = settingsSummaryView(ctx, tr, chat)
	case "l":
		if notice, wait := settingsPickerWait(tr, query.From.Id); wait {
			// The button sits on a summary, so the user picked a group since the
			// last build: show that build again instead of making them wait.
			var stored bool
			if text, kb, stored = storedSettingsPicker(tr, query.From.Id); !stored {
				_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: notice})
				return ext.EndGroups
			}
			break
		}
		text, kb = m.settingsPicker(b, ctx, tr, query.From.Id)
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	if _, _, err := msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: text, ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
		// Refresh with nothing changed re-sends identical content.
		if strings.Contains(err.Error(), "message is not modified") {
			log.Debug(err)
		} else {
			log.Error(err)
		}
	}
	_, _ = query.Answer(b, nil)
	return ext.EndGroups
}

func settingsDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chatID, err := strconv.ParseInt(strings.TrimPrefix(arg, "settings_"), 10, 64)
	if err != nil {
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}
	chat, denyKey := connectForSettings(b, chatID, user.Id)
	if chat == nil {
		text, _ := tr.GetString(denyKey)
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}
	text, kb := settingsSummaryView(ctx, tr, chat)
	if _, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML, ReplyMarkup: kb}); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
