package modules

import (
	"fmt"
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// buildExtendMessages renders every help-menu module's one-line command summary
// (`<module>_extend_msg`) in help order and packs the blocks into as few messages
// as fit Telegram's limit. A module never straddles two messages unless its own
// block exceeds the limit, in which case it is split by lines.
func buildExtendMessages(tr *i18n.Translator, modules []string) []string {
	header, _ := tr.GetString("help_extend_header")

	var blocks []string
	for _, module := range modules {
		summary, _ := tr.GetString(strings.ToLower(module) + "_extend_msg")
		summary = strings.TrimSpace(summary)
		if summary == "" {
			continue
		}
		blocks = append(blocks, "<b>"+html.EscapeString(module)+"</b>\n"+summary)
	}

	return packExtendBlocks(strings.TrimSpace(header), blocks, formatting.MaxMessageLength)
}

func packExtendBlocks(header string, blocks []string, limit int) []string {
	var messages []string
	current := header
	flush := func() {
		if strings.TrimSpace(current) != "" {
			messages = append(messages, current)
		}
		current = ""
	}
	appendPart := func(part string) {
		if current == "" {
			current = part
			return
		}
		if formatting.UTF16Len(current)+formatting.UTF16Len("\n\n")+formatting.UTF16Len(part) > limit {
			flush()
			current = part
			return
		}
		current += "\n\n" + part
	}

	for _, block := range blocks {
		if formatting.UTF16Len(block) <= limit {
			appendPart(block)
			continue
		}
		// Oversized module: fall back to line-level packing.
		flush()
		for line := range strings.SplitSeq(block, "\n") {
			if formatting.UTF16Len(line) > limit {
				// A single line over the limit: cut it hard, whole runes only.
				flush()
				pieces := formatting.SplitByUTF16Limit(line, limit)
				for _, piece := range pieces[:len(pieces)-1] {
					current = piece
					flush()
				}
				line = pieces[len(pieces)-1]
			}
			if current != "" && formatting.UTF16Len(current)+1+formatting.UTF16Len(line) > limit {
				flush()
			}
			if current == "" {
				current = line
			} else {
				current += "\n" + line
			}
		}
	}
	flush()
	return messages
}

func sendExtendList(b *gotgbot.Bot, ctx *ext.Context) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	for _, text := range buildExtendMessages(tr, listModulesFrom(DefaultHelpRegistry())) {
		_, err := b.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		})
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// extend sends every command of every help-menu module in one list, one line per
// command. In groups it points to PM instead of flooding the chat.
func (moduleStruct) extend(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if ctx.EffectiveChat.Type == "private" {
		return sendExtendList(b, ctx)
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("help_extend_pm")
	buttonText, _ := tr.GetString("help_extend_button")
	_, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId:                msg.MessageId,
			AllowSendingWithoutReply: true,
		},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
				{Text: buttonText, Url: fmt.Sprintf("https://t.me/%s?start=extend", getBotUsername(b))},
			}},
		},
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func extendDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, _ *gotgbot.User, _ string) error {
	return sendExtendList(b, ctx)
}
