package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// pmGroupTitleCheckLimit bounds the admin checks of a title lookup. Each check
// of a group whose admin list is not cached is up to four Telegram calls, and
// they run one after another; past this many groups with the title the user
// names the group by its @username or ID instead.
const pmGroupTitleCheckLimit = 3

// pmGroupToken is the first argument of a PM command, which may name a group.
// start and end are byte offsets in the message text: the cut runs from the
// end of the command to the end of the token, so the whitespace that follows
// the token stays.
type pmGroupToken struct {
	Raw        string
	Quoted     bool
	start, end int
}

// pmGroupQuotes pairs each opening quote with its closing one. Curly quotes
// are what iOS Smart Punctuation sends for typed straight ones.
var pmGroupQuotes = [][2]string{{`"`, `"`}, {"“", "”"}}

// parsePMGroupToken returns the first argument after the command. A quoted
// argument runs to the closing quote and may contain spaces.
func parsePMGroupToken(text string) (pmGroupToken, bool) {
	cmdEnd := strings.IndexFunc(text, unicode.IsSpace)
	if cmdEnd < 0 {
		return pmGroupToken{}, false
	}
	rest := strings.TrimLeftFunc(text[cmdEnd:], unicode.IsSpace)
	if rest == "" {
		return pmGroupToken{}, false
	}
	tokStart := len(text) - len(rest)

	for _, quotes := range pmGroupQuotes {
		open, end := quotes[0], quotes[1]
		if !strings.HasPrefix(rest, open) {
			continue
		}
		if closing := closingQuote(rest, len(open), end); closing > len(open) {
			return pmGroupToken{Raw: rest[len(open):closing], Quoted: true, start: cmdEnd, end: tokStart + closing + len(end)}, true
		}
	}
	tokLen := strings.IndexFunc(rest, unicode.IsSpace)
	if tokLen < 0 {
		tokLen = len(rest)
	}
	return pmGroupToken{Raw: rest[:tokLen], start: cmdEnd, end: tokStart + tokLen}, true
}

// closingQuote returns the index in s of the quote that closes the one ending
// at s[from]: the first quote followed by whitespace or the end of s, or -1.
func closingQuote(s string, from int, quote string) int {
	for i := from; i < len(s); i++ {
		if !strings.HasPrefix(s[i:], quote) {
			continue
		}
		after := i + len(quote)
		if next, _ := utf8.DecodeRuneInString(s[after:]); after == len(s) || unicode.IsSpace(next) {
			return i
		}
	}
	return -1
}

// stripPMGroupToken returns a copy of msg without tok, cut from the caption
// when the message has one (as GetText reads it) and from the text otherwise.
// Entities after the cut move back by its UTF-16 length; entities inside it
// are dropped, and entities that cross it keep their part outside it. msg is
// not changed.
func stripPMGroupToken(msg *gotgbot.Message, tok pmGroupToken) *gotgbot.Message {
	stripped := *msg
	if msg.Caption != "" {
		stripped.Caption, stripped.CaptionEntities = cutPMGroupToken(msg.Caption, msg.CaptionEntities, tok)
	} else {
		stripped.Text, stripped.Entities = cutPMGroupToken(msg.Text, msg.Entities, tok)
	}
	return &stripped
}

func cutPMGroupToken(text string, entities []gotgbot.MessageEntity, tok pmGroupToken) (string, []gotgbot.MessageEntity) {
	cutStart := utf16Len(text[:tok.start])
	cutLen := utf16Len(text[tok.start:tok.end])
	cutEnd := cutStart + cutLen

	var kept []gotgbot.MessageEntity
	for _, entity := range entities {
		start, end := entity.Offset, entity.Offset+entity.Length
		overlap := max(0, min(end, cutEnd)-max(start, cutStart))
		if entity.Length -= overlap; entity.Length <= 0 {
			continue
		}
		switch {
		case start >= cutEnd:
			entity.Offset -= cutLen
		case start > cutStart:
			entity.Offset = cutStart
		}
		kept = append(kept, entity)
	}
	return text[:tok.start] + text[tok.end:], kept
}

func utf16Len(s string) int64 {
	return int64(len(utf16.Encode([]rune(s))))
}

func init() {
	helpers.SetPMGroupResolver(resolvePMGroupArg)
}

// isPMGroupRef reports whether tok names a chat directly, by a negative chat
// ID or an @username, rather than by a group's title.
func isPMGroupRef(tok pmGroupToken) bool {
	if tok.Quoted {
		return false
	}
	if id, err := strconv.ParseInt(tok.Raw, 10, 64); err == nil && id < 0 {
		return true
	}
	return strings.HasPrefix(tok.Raw, "@") && len(tok.Raw) > 1
}

// lookupPMGroup returns the group ref (a chat ID or @username) names. A user
// or channel is not a group.
func lookupPMGroup(b *gotgbot.Bot, ref string) *gotgbot.Chat {
	chat, err := chat_status.GetChat(b, ref)
	if err != nil {
		log.WithFields(log.Fields{"token": ref, "error": err}).Debug("[PMGroupArg] group lookup failed")
		return nil
	}
	if chat.Type != "group" && chat.Type != "supergroup" {
		return nil
	}
	return chat
}

// pmGroupsTitled returns the entries titled title, ignoring case.
func pmGroupsTitled(entries []settingsPickerEntry, title string) []settingsPickerEntry {
	var matches []settingsPickerEntry
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry.Title), strings.TrimSpace(title)) {
			matches = append(matches, entry)
		}
	}
	return matches
}

// pmGroupList renders entries as text lines, title and ID, so a user can name
// any of them by either.
func pmGroupList(entries []settingsPickerEntry) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		id := fmt.Sprintf("<code>%d</code>", entry.ChatID)
		if title := strings.TrimSpace(entry.Title); title != "" {
			lines = append(lines, "• "+html.EscapeString(title)+" — "+id)
			continue
		}
		lines = append(lines, "• "+id)
	}
	return strings.Join(lines, "\n")
}

// pmGroupHelpText puts errText (HTML) above cmd's help and the groups the user
// administers, or the picker's wait notice in place of the list.
func pmGroupHelpText(tr *i18n.Translator, cmd, errText string, entries []settingsPickerEntry, wait string) string {
	text := commandHelpTextWith(tr, cmd, errText, commandPMExample(cmd))
	switch {
	case wait != "":
		return text + "\n\n" + wait
	case len(entries) == 0:
		empty, _ := tr.GetString("settings_picker_empty")
		return text + "\n\n" + empty
	default:
		header, _ := tr.GetString("connections_pm_group_your_groups")
		return text + "\n\n" + header + "\n" + pmGroupList(entries)
	}
}

// replyPMGroupHelp answers a PM command that names no usable group with
// errText (HTML), the command's help and the groups the user administers.
func replyPMGroupHelp(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, cmd, errText string) error {
	entries, wait := adminGroupsForUser(b, ctx, tr, ctx.EffectiveUser.Id)
	_, err := ctx.EffectiveMessage.Reply(b, pmGroupHelpText(tr, cmd, errText, entries, wait), formatting.Shtml())
	return err
}

// withPMGroupArg lets a PM command name its group as the first argument:
// "/cmd -1001234567890 args", "/cmd @group args", or the title of a group the
// user administers ("/cmd mygroup args", "/cmd \"My Group\" args"). The group
// passes the same checks as /connect, then the command runs with it in place
// of the saved connection and with the group cut from its message. A call
// with no usable group gets the command's help and the user's groups. Outside
// PM the command runs unchanged.
func withPMGroupArg(cmd string, arg helpers.PMGroupArg, h handlers.Response) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		if !resolvePMGroupArg(b, ctx, cmd, arg) {
			return ext.EndGroups
		}
		if chat_status.OneOffGroup(ctx) == nil {
			return h(b, ctx)
		}
		// Later handler groups must not see the cut message.
		return helpers.EndGroupsLogging(ctx, h(b, ctx))
	}
}

// resolvePMGroupArg is withPMGroupArg's part that finds the group, also run by
// helpers.WrapCommand for commands with a PMGroupArg. When the call names a
// group that passes the /connect checks, it stores the group with
// chat_status.SetOneOffGroup and cuts it from ctx.EffectiveMessage. It
// returns false when it has answered the call itself.
func resolvePMGroupArg(b *gotgbot.Bot, ctx *ext.Context, cmd string, arg helpers.PMGroupArg) bool {
	msg, user := ctx.EffectiveMessage, ctx.EffectiveUser
	if user == nil || !inPM(msg) {
		return true
	}
	c := pmGroupCall{b: b, ctx: ctx, tr: i18n.MustNewTranslator(lang.GetLanguage(ctx)), cmd: cmd}
	// A rich message keeps its text in blocks, so a group in it cannot be
	// cut out; ask for a plain message rather than ignore the group.
	if msg.RichMessage != nil {
		if len(strings.Fields(msg.RichMessage.PlainText())) > 1 {
			text, _ := c.tr.GetString("connections_pm_group_plain_text")
			_ = c.reply(fmt.Sprintf(text, cmd))
			return false
		}
		return true
	}

	group, tok, runAsIs := c.group(arg)
	if runAsIs {
		return true
	}
	if group == nil {
		return false
	}
	return c.useGroup(group, tok)
}

// pmGroupCall is one PM call of a command wrapped by withPMGroupArg.
type pmGroupCall struct {
	b   *gotgbot.Bot
	ctx *ext.Context
	tr  *i18n.Translator
	cmd string
}

func (c pmGroupCall) reply(text string) error {
	if _, err := c.ctx.EffectiveMessage.Reply(c.b, text, formatting.Shtml()); err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

func (c pmGroupCall) replyHelp(errText string) {
	if err := replyPMGroupHelp(c.b, c.ctx, c.tr, c.cmd, errText); err != nil {
		log.Error(err)
	}
}

// group finds the group the call names. runAsIs is true when the command runs
// as it came, on the saved connection; a nil group otherwise means the call
// has been answered.
func (c pmGroupCall) group(arg helpers.PMGroupArg) (group *gotgbot.Chat, tok pmGroupToken, runAsIs bool) {
	conn := connections.Connection(c.ctx.EffectiveUser.Id)
	connected := conn.Connected && conn.ChatId != 0

	tok, ok := parsePMGroupToken(c.ctx.EffectiveMessage.GetText())
	if !ok {
		if connected {
			return nil, tok, true
		}
		text, _ := c.tr.GetString("connections_pm_group_pick")
		c.replyHelp(text)
		return nil, tok, false
	}

	if isPMGroupRef(tok) {
		group = lookupPMGroup(c.b, tok.Raw)
	}
	if group != nil {
		return group, tok, false
	}
	if connected && arg == helpers.PMGroupArgOwn {
		// The argument is the command's own: with a connection only an
		// @username or ID picks another group.
		return nil, tok, true
	}
	notFoundText, _ := c.tr.GetString("connections_pm_group_not_found")
	notFound := fmt.Sprintf(notFoundText, html.EscapeString(tok.Raw))
	if isPMGroupRef(tok) {
		c.replyHelp(notFound)
		return nil, tok, false
	}
	return c.groupByTitle(tok, notFound), tok, false
}

// groupByTitle returns the one group the user administers titled tok.Raw, or
// nil after answering why there is none. It shares the /settings picker's
// pause, so a user repeating the call scans the chats table once per pause:
// taking the pause, it scans the user's groups and, when the title names none,
// keeps the help's list built from that scan; inside the pause it looks the
// title up in the list the last build kept, or asks the user to wait when none
// was kept.
func (c pmGroupCall) groupByTitle(tok pmGroupToken, notFound string) *gotgbot.Chat {
	userID := c.ctx.EffectiveUser.Id
	var entries []settingsPickerEntry
	var candidates []models.Chat
	notice, paused := settingsPickerWait(c.tr, userID)
	if paused {
		stored, ok := storedSettingsPickerEntries(userID)
		if !ok {
			_ = c.reply(notice)
			return nil
		}
		entries = stored
	} else {
		candidates = settingsCandidates(c.ctx, userID)
		entries = c.titleCandidates(candidates)
	}

	matches, tooMany := c.adminGroupsTitled(entries, tok.Raw)
	if tooMany {
		text, _ := c.tr.GetString("connections_pm_group_too_many")
		_ = c.reply(fmt.Sprintf(text, html.EscapeString(tok.Raw)))
		return nil
	}
	switch len(matches) {
	case 0:
	case 1:
		if group := lookupPMGroup(c.b, strconv.FormatInt(matches[0].ChatID, 10)); group != nil {
			return group
		}
	default:
		text, _ := c.tr.GetString("connections_pm_group_duplicate")
		_ = c.reply(fmt.Sprintf(text, html.EscapeString(tok.Raw)) + "\n" + pmGroupList(matches))
		return nil
	}

	list := entries
	if !paused {
		list = settingsPickerEntriesFrom(c.b, c.ctx, userID, candidates)
		storeSettingsPickerEntries(userID, list)
	}
	_ = c.reply(pmGroupHelpText(c.tr, c.cmd, notFound, list, ""))
	return nil
}

// titleCandidates lists the groups a title is looked up among: the connected
// group and candidates, the settingsCandidateLimit groups the user posted in
// most recently. Only groups with that title are checked, so the /settings
// picker's button cap and live budget do not hide a group.
func (c pmGroupCall) titleCandidates(candidates []models.Chat) []settingsPickerEntry {
	var entries []settingsPickerEntry
	if conn := connections.Connection(c.ctx.EffectiveUser.Id); conn.Connected && conn.ChatId != 0 {
		if info, err := chats.GetChatBasicInfoCached(conn.ChatId); err == nil && info != nil {
			entries = append(entries, settingsPickerEntry{ChatID: conn.ChatId, Title: info.ChatName})
		}
	}
	for _, candidate := range candidates {
		entries = append(entries, settingsPickerEntry{ChatID: candidate.ChatId, Title: candidate.ChatName})
	}
	return entries
}

// adminGroupsTitled returns the entries titled title, ignoring case, that the
// user administers. With more than pmGroupTitleCheckLimit groups so titled it
// checks none of them and returns tooMany.
func (c pmGroupCall) adminGroupsTitled(entries []settingsPickerEntry, title string) (matches []settingsPickerEntry, tooMany bool) {
	var titled []settingsPickerEntry
	seen := map[int64]bool{}
	for _, entry := range pmGroupsTitled(entries, title) {
		if !seen[entry.ChatID] {
			titled = append(titled, entry)
		}
		seen[entry.ChatID] = true
	}
	if len(titled) > pmGroupTitleCheckLimit {
		return nil, true
	}
	for _, entry := range titled {
		if chat_status.IsUserAdmin(c.b, entry.ChatID, c.ctx.EffectiveUser.Id) {
			matches = append(matches, entry)
		}
	}
	return matches, false
}

// useGroup checks the group like /connect, then puts it in place of the saved
// connection for this update and cuts tok from the message. It returns false
// after refusing the group.
func (c pmGroupCall) useGroup(group *gotgbot.Chat, tok pmGroupToken) bool {
	userID := c.ctx.EffectiveUser.Id
	allowed, key, memberChecked := checkUserConnectToChat(c.b, group.Id, userID)
	// An admin is let in from the cached admin list, which outlives a lost
	// chat_member update, so look them up in the group once here.
	if allowed && !memberChecked {
		if isMember, err := chat_status.IsUserInChatWithError(c.b, group, userID); err != nil {
			log.WithFields(log.Fields{"userId": userID, "chatId": group.Id, "error": err}).Warn("[PMGroupArg] membership check failed")
			allowed, key = false, "error_generic"
		} else if !isMember {
			allowed, key = false, "connections_connect_not_member"
		}
	}
	if !allowed {
		text, _ := c.tr.GetString(key)
		_ = c.reply(text)
		return false
	}

	chat_status.SetOneOffGroup(c.ctx, group)
	c.ctx.EffectiveMessage = stripPMGroupToken(c.ctx.EffectiveMessage, tok)
	return true
}
