package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

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
		if closing := strings.Index(rest[1:], `"`); closing > 0 {
			return pmGroupToken{Raw: rest[1 : closing+1], Quoted: true, start: cmdEnd, end: tokStart + closing + 2}, true
		}
	}
	tokLen := strings.IndexFunc(rest, unicode.IsSpace)
	if tokLen < 0 {
		tokLen = len(rest)
	}
	return pmGroupToken{Raw: rest[:tokLen], start: cmdEnd, end: tokStart + tokLen}, true
}

// stripPMGroupToken returns a copy of msg without tok, cut from the caption
// when the message has one (as GetText reads it) and from the text otherwise.
// Entities after the cut move back by its UTF-16 length; entities inside it
// are dropped. msg is not changed.
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
		switch {
		case entity.Offset+entity.Length <= cutStart:
			kept = append(kept, entity)
		case entity.Offset >= cutEnd:
			entity.Offset -= cutLen
			kept = append(kept, entity)
		}
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

// resolvePMGroup finds the group a token names: a negative chat ID or a
// group's @username. A user or channel @username is not a group.
func resolvePMGroup(b *gotgbot.Bot, tok pmGroupToken) *gotgbot.Chat {
	if tok.Quoted {
		return nil
	}
	if id, err := strconv.ParseInt(tok.Raw, 10, 64); err != nil || id >= 0 {
		if !strings.HasPrefix(tok.Raw, "@") || len(tok.Raw) < 2 {
			return nil
		}
	}
	chat, err := chat_status.GetChat(b, tok.Raw)
	if err != nil {
		log.WithFields(log.Fields{"token": tok.Raw, "error": err}).Debug("[PMGroupArg] group lookup failed")
		return nil
	}
	if chat.Type != "group" && chat.Type != "supergroup" {
		return nil
	}
	return chat
}

// withPMGroupArg lets a PM command name its group as the first argument:
// "/cmd -1001234567890 args" or "/cmd @group args". The group passes the same
// checks as /connect, then the command runs with it in place of the saved
// connection and with the group cut from its message. Outside PM the command
// runs unchanged.
func withPMGroupArg(cmd string, shape pmArgShape, h handlers.Response) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		msg, user := ctx.EffectiveMessage, ctx.EffectiveUser
		// A rich message keeps its text in blocks, so there is nothing to cut.
		if msg == nil || user == nil || msg.Chat.Type != "private" || msg.RichMessage != nil {
			return h(b, ctx)
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		reply := func(text string) error {
			if _, err := msg.Reply(b, text, formatting.Shtml()); err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		}
		conn := connections.Connection(user.Id)
		connected := conn.Connected && conn.ChatId != 0

		tok, ok := parsePMGroupToken(msg.GetText())
		if !ok {
			if connected {
				return h(b, ctx)
			}
			text, _ := tr.GetString("connections_pm_group_need")
			return reply(fmt.Sprintf(text, cmd))
		}

		group := resolvePMGroup(b, tok)
		if group == nil {
			if connected && shape == pmArgsOwn {
				return h(b, ctx)
			}
			text, _ := tr.GetString("connections_pm_group_not_found")
			return reply(fmt.Sprintf(text, html.EscapeString(tok.Raw)))
		}
		if allowed, key := canUserConnectToChat(b, group.Id, user.Id); !allowed {
			text, _ := tr.GetString(key)
			return reply(text)
		}

		chat_status.SetOneOffGroup(ctx, group)
		ctx.EffectiveMessage = stripPMGroupToken(msg, tok)
		// Later handler groups must not see the cut message.
		if err := h(b, ctx); err != nil && err != ext.EndGroups && err != ext.ContinueGroups {
			helpers.LogHandlerError(ctx, err)
		}
		return ext.EndGroups
	}
}
