package modules

import (
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

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
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
type pmArgShape int

const (
	// pmArgsNone: the command takes no arguments, so with a connection any
	// argument that names no group is a mistake.
	pmArgsNone pmArgShape = iota
	// pmArgsOwn: the command has arguments of its own, so with a connection an
	// argument that names no group belongs to the command.
	pmArgsOwn
)

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
		msg, user := ctx.EffectiveMessage, ctx.EffectiveUser
		if msg == nil || user == nil || msg.Chat.Type != "private" {
			return h(b, ctx)
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		reply := func(text string) error {
			if _, err := msg.Reply(b, text, formatting.Shtml()); err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		}
		// A rich message keeps its text in blocks, so a group in it cannot be
		// cut out; ask for a plain message rather than ignore the group.
		if msg.RichMessage != nil {
			if len(strings.Fields(msg.RichMessage.PlainText())) > 1 {
				text, _ := tr.GetString("connections_pm_group_need")
				return reply(fmt.Sprintf(text, cmd))
			}
			return h(b, ctx)
		}
		conn := connections.Connection(user.Id)
		connected := conn.Connected && conn.ChatId != 0

		replyHelp := func(errText string) error {
			if err := replyPMGroupHelp(b, ctx, tr, cmd, errText); err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		}

		tok, ok := parsePMGroupToken(msg.GetText())
		if !ok {
			if connected {
				return h(b, ctx)
			}
			text, _ := tr.GetString("connections_pm_group_pick")
			return replyHelp(text)
		}

		var group *gotgbot.Chat
		if isPMGroupRef(tok) {
			group = lookupPMGroup(b, tok.Raw)
		}
		if group == nil && connected && shape == pmArgsOwn {
			// The argument is the command's own: with a connection only an
			// @username or ID picks another group.
			return h(b, ctx)
		}
		notFoundText, _ := tr.GetString("connections_pm_group_not_found")
		notFound := fmt.Sprintf(notFoundText, html.EscapeString(tok.Raw))
		if group == nil && isPMGroupRef(tok) {
			return replyHelp(notFound)
		}
		if group == nil {
			entries, wait := adminGroupsForUser(b, ctx, tr, user.Id)
			if wait != "" {
				return reply(wait)
			}
			switch matches := pmGroupsTitled(entries, tok.Raw); len(matches) {
			case 0:
				return reply(pmGroupHelpText(tr, cmd, notFound, entries, ""))
			case 1:
				if group = lookupPMGroup(b, strconv.FormatInt(matches[0].ChatID, 10)); group == nil {
					return reply(pmGroupHelpText(tr, cmd, notFound, entries, ""))
				}
			default:
				text, _ := tr.GetString("connections_pm_group_duplicate")
				return reply(fmt.Sprintf(text, html.EscapeString(tok.Raw)) + "\n" + pmGroupList(matches))
			}
		}
		if allowed, key := canUserConnectToChat(b, group.Id, user.Id); !allowed {
			text, _ := tr.GetString(key)
			return reply(text)
		}

		chat_status.SetOneOffGroup(ctx, group)
		ctx.EffectiveMessage = stripPMGroupToken(msg, tok)
		// Later handler groups must not see the cut message.
		if err := h(b, ctx); err != nil && !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups) {
			helpers.LogHandlerError(ctx, err)
		}
		return ext.EndGroups
	}
}
