package modules

import (
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

const (
	// banListPageSize is how many bans one /banlist page shows.
	banListPageSize = 10
	// banListNamespace is the callback namespace of the /banlist page buttons.
	banListNamespace = "banlist"
	// banListSearchTTL is how long the page buttons of a filtered list work:
	// the search behind their token is kept that long.
	banListSearchTTL = time.Hour
)

// From PM, /banlist takes the group as its first argument; any other words
// are its search.
var banListDesc = helpers.CommandDescriptor{Name: "banlist", PMGroupArg: helpers.PMGroupArgOwn}

// banListSearch is the search of a filtered /banlist, kept in Redis behind
// the token its page buttons carry: callback data has no room for user text.
type banListSearch struct {
	ChatID int64
	Search string
}

func banListSearchKey(token string) string {
	return "alita:banlist:" + token
}

// storeBanListSearch keeps search for chatID's page buttons and returns its
// token, or "" when it could not be kept (no Redis): the list is then shown
// without page buttons.
func storeBanListSearch(chatID int64, search string) string {
	m := cache.GetMarshal()
	if m == nil {
		return ""
	}
	token, err := newOverwriteToken()
	if err != nil {
		log.WithError(err).Error("[Bans] Failed to make a ban list token")
		return ""
	}
	if err := m.Set(cache.Context, banListSearchKey(token), banListSearch{ChatID: chatID, Search: search}, store.WithExpiration(banListSearchTTL)); err != nil {
		log.WithError(err).Error("[Bans] Failed to keep the ban list search")
		return ""
	}
	return token
}

// loadBanListSearch returns the search behind token, if it is still kept and
// was made for chatID.
func loadBanListSearch(chatID int64, token string) (string, bool) {
	m := cache.GetMarshal()
	if m == nil {
		return "", false
	}
	var kept banListSearch
	if _, err := m.Get(cache.Context, banListSearchKey(token), &kept); err != nil || kept.ChatID != chatID {
		return "", false
	}
	return kept.Search, true
}

// banListEntry renders one open ban of the list as line n.
func banListEntry(tr *i18n.Translator, n int, r models.BanRecord) string {
	params := i18n.TranslationParams{
		"n":      n,
		"user":   banRecordUser(r.UserId),
		"id":     r.UserId,
		"source": banRecordSource(&r),
		"date":   r.CreatedAt.UTC().Format(banRecordTimeFormat),
	}
	key := "bans_banlist_entry"
	if r.Until != nil {
		key = "bans_banlist_entry_until"
		params["until"] = r.Until.UTC().Format(banRecordTimeFormat)
	}
	line, _ := tr.GetString(key, params)
	return line
}

// banListPage renders page (from 0) of chat's open bans matching search, as
// of now, for a message in msg's chat. Its buttons turn the page; for a
// search they carry token, or a new one when token is "", and are left out
// when the search could not be kept. A page past the end shows the last one.
func banListPage(tr *i18n.Translator, msg *gotgbot.Message, chat *gotgbot.Chat, search, token string, page int, now time.Time) (string, gotgbot.InlineKeyboardMarkup, error) {
	var none gotgbot.InlineKeyboardMarkup
	rows, total, page, err := bans.ListOpen(chat.Id, search, now, page, banListPageSize)
	if err != nil {
		return "", none, err
	}
	if total == 0 {
		var text string
		if search == "" {
			text, _ = tr.GetString("bans_banlist_empty")
		} else {
			text, _ = tr.GetString("bans_banlist_no_match", i18n.TranslationParams{"query": html.EscapeString(search)})
		}
		return withGroupLine(tr, msg, chat, text), none, nil
	}
	pages := int((total + banListPageSize - 1) / banListPageSize)

	params := i18n.TranslationParams{"count": total, "page": page + 1, "pages": pages}
	headerKey := "bans_banlist_header"
	if search != "" {
		headerKey = "bans_banlist_header_search"
		params["query"] = html.EscapeString(search)
	}
	header, _ := tr.GetString(headerKey, params)
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, header)
	for i := range rows {
		lines = append(lines, banListEntry(tr, page*banListPageSize+i+1, rows[i]))
	}
	text := withGroupLine(tr, msg, chat, strings.Join(lines, "\n"))

	if pages == 1 {
		return text, none, nil
	}
	if search != "" && token == "" {
		// The search is kept only once there is a page to turn to.
		if token = storeBanListSearch(chat.Id, search); token == "" {
			return text, none, nil
		}
	}
	button := func(key string, to int) gotgbot.InlineKeyboardButton {
		label, _ := tr.GetString(key)
		fields := map[string]string{"p": strconv.Itoa(to)}
		if token != "" {
			fields["t"] = token
		}
		return gotgbot.InlineKeyboardButton{Text: label, CallbackData: moderationButtonData(banListNamespace, msg, chat, fields)}
	}
	var nav []gotgbot.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, button("bans_banlist_prev", page-1))
	}
	if page < pages-1 {
		nav = append(nav, button("bans_banlist_next", page+1))
	}
	if len(nav) == 0 {
		return text, none, nil
	}
	return text, gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{nav}}, nil
}

// banList is /banlist [search]: the open bans the bot recorded in the group,
// newest first, a page at a time. A search narrows them to the users whose
// ID, username or name matches.
func (moduleStruct) banList(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	search := ""
	if args := ctx.Args(); len(args) > 1 {
		search = bans.NormalizeSearch(strings.Join(args[1:], " "))
	}
	text, kb, err := banListPage(tr, msg, chat, search, "", 0, time.Now())
	if err != nil {
		log.WithError(err).WithField("chat_id", chat.Id).Error("[Bans] Failed to list the open bans")
		text, _ = tr.GetString("bans_record_read_failed")
	}
	opts := formatting.Shtml()
	opts.ReplyMarkup = kb
	if _, err := msg.Reply(b, text, opts); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// banListCallback turns a /banlist page. The presser must be able to restrict
// members in the group the list shows (buttonChat), wherever it was sent.
func (moduleStruct) banListCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query.Message == nil {
		return answerInvalidCallback(b, ctx, query)
	}
	decoded, ok := decodeCallbackData(query.Data, banListNamespace)
	if !ok {
		return answerInvalidCallback(b, ctx, query)
	}
	chat, ok := buttonChat(ctx, decoded)
	if !ok || chat == nil {
		return answerInvalidCallback(b, ctx, query)
	}
	rawPage, _ := decoded.Field("p")
	page, err := strconv.Atoi(rawPage)
	if err != nil || page < 0 {
		return answerInvalidCallback(b, ctx, query)
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, query.From.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	answer := func(key string) error {
		text, _ := tr.GetString(key)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text, ShowAlert: true})
		return ext.EndGroups
	}

	token, _ := decoded.Field("t")
	search := ""
	if token != "" {
		if search, ok = loadBanListSearch(chat.Id, token); !ok {
			return answer("bans_banlist_expired")
		}
	}
	shown := &gotgbot.Message{Chat: query.Message.GetChat()}
	if inPM(shown) {
		// The group line names the group by title, which "c" does not carry.
		if full, err := b.GetChat(chat.Id, nil); err == nil && full != nil {
			named := full.ToChat()
			chat = &named
		}
	}
	text, kb, err := banListPage(tr, shown, chat, search, token, page, time.Now())
	if err != nil {
		log.WithError(err).WithField("chat_id", chat.Id).Error("[Bans] Failed to list the open bans")
		return answer("bans_record_read_failed")
	}
	editSettingsMessage(b, query.Message, text, kb)
	_, _ = query.Answer(b, nil)
	return ext.EndGroups
}
