package modules

import (
	"context"
	"errors"
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

	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// pmGroupToken is the first argument of a PM command, which may name a group.
// start and end are byte offsets in the message text: the cut runs from the
// end of the command to the end of the token, so the whitespace that follows
// the token stays.
type pmGroupToken struct {
	Raw        string
	Quoted     bool
	start, end int
}

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

	if strings.HasPrefix(rest, `"`) {
		if closing := closingQuote(rest); closing > 1 {
			return pmGroupToken{Raw: rest[1:closing], Quoted: true, start: cmdEnd, end: tokStart + closing + 1}, true
		}
	}
	tokLen := strings.IndexFunc(rest, unicode.IsSpace)
	if tokLen < 0 {
		tokLen = len(rest)
	}
	return pmGroupToken{Raw: rest[:tokLen], start: cmdEnd, end: tokStart + tokLen}, true
}

// closingQuote returns the index in s of the quote that closes the one at
// s[0]: the first quote followed by whitespace or the end of s, or -1.
func closingQuote(s string) int {
	for i := 1; i < len(s); i++ {
		if s[i] != '"' {
			continue
		}
		if next, _ := utf8.DecodeRuneInString(s[i+1:]); i+1 == len(s) || unicode.IsSpace(next) {
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

// pmArgShape says whether a command takes arguments of its own after the group.
type pmArgShape = helpers.PMGroupArg

const (
	pmArgsNone = helpers.PMGroupArgNone
	pmArgsOwn  = helpers.PMGroupArgOwn
)

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
	text := commandHelpText(tr, cmd, errText)
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
func withPMGroupArg(cmd string, shape pmArgShape, h handlers.Response) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		if !resolvePMGroupArg(b, ctx, cmd, shape) {
			return ext.EndGroups
		}
		if chat_status.OneOffGroup(ctx) == nil {
			return h(b, ctx)
		}
		// Later handler groups must not see the cut message.
		if err := h(b, ctx); err != nil && !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups) {
			helpers.LogHandlerError(ctx, err)
		}
		return ext.EndGroups
	}
}

// resolvePMGroupArg is withPMGroupArg's part that finds the group, also run by
// helpers.WrapCommand for commands with a PMGroupArg. When the call names a
// group that passes the /connect checks, it stores the group with
// chat_status.SetOneOffGroup and cuts it from ctx.EffectiveMessage. It
// returns false when it has answered the call itself.
func resolvePMGroupArg(b *gotgbot.Bot, ctx *ext.Context, cmd string, shape pmArgShape) bool {
	msg, user := ctx.EffectiveMessage, ctx.EffectiveUser
	if msg == nil || user == nil || msg.Chat.Type != "private" {
		return true
	}
	c := pmGroupCall{b: b, ctx: ctx, tr: i18n.MustNewTranslator(lang.GetLanguage(ctx)), cmd: cmd}
	// A rich message keeps its text in blocks, so a group in it cannot be
	// cut out; ask for a plain message rather than ignore the group.
	if msg.RichMessage != nil {
		if len(strings.Fields(msg.RichMessage.PlainText())) > 1 {
			text, _ := c.tr.GetString("connections_pm_group_need")
			_ = c.reply(fmt.Sprintf(text, cmd))
			return false
		}
		return true
	}

	group, tok, runAsIs := c.group(shape)
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
func (c pmGroupCall) group(shape pmArgShape) (group *gotgbot.Chat, tok pmGroupToken, runAsIs bool) {
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
	if connected && shape == pmArgsOwn {
		// The argument is the command's own: with a connection only an
		// @username or ID picks another group. The title of one of the user's
		// groups is refused rather than passed on, or the command would act on
		// the connected group with the title as its argument.
		if !isPMGroupRef(tok) {
			if matches := c.ownGroupsTitled(tok.Raw, conn.ChatId); len(matches) > 0 {
				text, _ := c.tr.GetString("connections_pm_group_title_on_connection")
				_ = c.reply(fmt.Sprintf(text, html.EscapeString(tok.Raw), conn.ChatId) + "\n" + pmGroupList(matches))
				return nil, tok, false
			}
		}
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
// nil after answering why there is none.
func (c pmGroupCall) groupByTitle(tok pmGroupToken, notFound string) *gotgbot.Chat {
	entries, wait := adminGroupsForUser(c.b, c.ctx, c.tr, c.ctx.EffectiveUser.Id)
	if wait != "" {
		_ = c.reply(wait)
		return nil
	}
	switch matches := pmGroupsTitled(entries, tok.Raw); len(matches) {
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
	_ = c.reply(pmGroupHelpText(c.tr, c.cmd, notFound, entries, ""))
	return nil
}

// ownGroupsTitled returns the groups titled title, ignoring case, that the
// user administers among those the /settings picker lists: the connected
// group and the groups the user has posted in. Only groups with that title
// are checked, so an argument that is no title costs at most one query and
// leaves the picker's pause alone.
func (c pmGroupCall) ownGroupsTitled(title string, connected int64) []settingsPickerEntry {
	userID := c.ctx.EffectiveUser.Id
	candidates, err := pmGroupCandidates(tracing.UpdateContext(c.ctx), userID)
	if err != nil {
		log.WithFields(log.Fields{"userId": userID, "error": err}).Debug("[PMGroupArg] candidate chats failed")
	}
	entries := make([]settingsPickerEntry, 0, len(candidates)+1)
	if info, err := chats.GetChatBasicInfoCached(connected); err == nil && info != nil {
		entries = append(entries, settingsPickerEntry{ChatID: connected, Title: info.ChatName})
	}
	entries = append(entries, candidates...)

	var matches []settingsPickerEntry
	seen := map[int64]bool{}
	for _, entry := range pmGroupsTitled(entries, title) {
		if !seen[entry.ChatID] && chat_status.IsUserAdmin(c.b, entry.ChatID, userID) {
			matches = append(matches, entry)
		}
		seen[entry.ChatID] = true
	}
	return matches
}

// pmGroupCandidates lists the groups userID has posted in, as IDs and titles,
// for the title check of a command with arguments of its own. Every such
// command on a connection asks for them, and the query scans the chats table,
// so the list is kept in the cache for the picker's pause: one scan per user
// per pause, shared by concurrent commands. Admin rights are not kept; a
// failed query is not kept either.
func pmGroupCandidates(ctx context.Context, userID int64) ([]settingsPickerEntry, error) {
	load := func(ctx context.Context) ([]settingsPickerEntry, error) {
		candidates, err := settingsCandidateChats(ctx, userID, settingsCandidateLimit)
		if err != nil {
			return nil, err
		}
		entries := make([]settingsPickerEntry, 0, len(candidates))
		for _, candidate := range candidates {
			entries = append(entries, settingsPickerEntry{ChatID: candidate.ChatId, Title: candidate.ChatName})
		}
		return entries, nil
	}
	if settingsPickerCooldown <= 0 {
		return load(ctx)
	}
	return dbcache.GetFromCacheOrLoad(ctx, dbcache.CacheKey("pm_group_candidates", userID), settingsPickerCooldown, load)
}

// useGroup checks the group like /connect, then puts it in place of the saved
// connection for this update and cuts tok from the message. It returns false
// after refusing the group.
func (c pmGroupCall) useGroup(group *gotgbot.Chat, tok pmGroupToken) bool {
	if allowed, key := canUserConnectToChat(c.b, group.Id, c.ctx.EffectiveUser.Id); !allowed {
		text, _ := c.tr.GetString(key)
		_ = c.reply(text)
		return false
	}

	chat_status.SetOneOffGroup(c.ctx, group)
	c.ctx.EffectiveMessage = stripPMGroupToken(c.ctx.EffectiveMessage, tok)
	return true
}
