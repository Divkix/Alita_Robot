package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/media"

	log "github.com/sirupsen/logrus"
)

// startHelpPrefixHandler processes /start command arguments for specific help topics.
// Handles deep links for help, connections, rules, notes, and about pages.
func startHelpPrefixHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if strings.HasPrefix(arg, "help_") {
		parts := strings.Split(arg, "_")
		if len(parts) < 2 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_invalid_deep_link")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}
		helpModule := parts[1]
		_, err := sendHelpkb(b, ctx, helpModule, DefaultHelpRegistry())
		if err != nil {
			log.Errorf("[Start]: %v", err)
			return err
		}
	} else if strings.HasPrefix(arg, "connect_") {
		parts := strings.Split(arg, "_")
		if len(parts) < 2 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_invalid_deep_link")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		chatID, err := strconv.Atoi(parts[1])
		if err != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_invalid_deep_link")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		cochat, err := b.GetChat(int64(chatID), nil)
		if err != nil || cochat == nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_chat_not_found")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		if allowed, denyKey := canUserConnectToChat(b, cochat.Id, user.Id); !allowed {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString(denyKey)
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		// Synchronous DB write before user confirmation - fixes issue #694
		connections.ConnectId(user.Id, cochat.Id)

		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		Text, _ := tr.GetString("helpers_connected_to_chat", i18n.TranslationParams{"s": cochat.Title})
		connKeyboard := keyboard.InitButtons(b, cochat.Id, user.Id)

		_, err = ctx.EffectiveMessage.Reply(b, Text,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: connKeyboard,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	} else if strings.HasPrefix(arg, "rules_") {
		parts := strings.Split(arg, "_")
		if len(parts) < 2 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_invalid_deep_link")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		chatID, err := strconv.Atoi(parts[1])
		if err != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_invalid_deep_link")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		chatinfo, err := b.GetChat(int64(chatID), nil)
		if err != nil || chatinfo == nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_chat_not_found")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		rulesrc := rules.GetChatRulesInfo(int64(chatID))
		normalizedRules := normalizeRulesForHTML(rulesrc.Rules)

		if normalizedRules == "" {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("rules_not_set")
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}

		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("rules_for_chat", i18n.TranslationParams{
			"first":  html.EscapeString(chatinfo.Title),
			"second": normalizedRules,
		})
		_, err = msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else if strings.HasPrefix(arg, "note") {
		nArgs := strings.SplitN(arg, "_", 3)

		// Validate deep link has at least chat ID
		if len(nArgs) < 2 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_invalid_deep_link")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		chatID, err := strconv.Atoi(nArgs[1])
		if err != nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_invalid_deep_link")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		chatinfo, err := b.GetChat(int64(chatID), nil)
		if err != nil || chatinfo == nil {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("helpers_chat_not_found")
			_, _ = msg.Reply(b, text, formatting.Shtml())
			return ext.EndGroups
		}

		if strings.HasPrefix(arg, "notes_") {
			// check if feth admin notes or not
			admin := chat_status.IsUserAdmin(b, int64(chatID), user.Id)
			noteKeys := notes.GetNotesList(chatinfo.Id, admin)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			info, _ := tr.GetString("notes_none_in_chat")
			if len(noteKeys) > 0 {
				info, _ = tr.GetString("helpers_notes_current_header")
				var sb strings.Builder
				for _, note := range noteKeys {
					fmt.Fprintf(&sb, " - <a href='https://t.me/%s?start=note_%d_%s'>%s</a>\n", b.Username, int64(chatID), note, note)
				}
				info += sb.String()
			}

			_, err := msg.Reply(b, info, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		} else if strings.HasPrefix(arg, "note_") {
			// Validate deep link has note name
			if len(nArgs) < 3 {
				tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
				text, _ := tr.GetString("helpers_invalid_deep_link")
				_, _ = msg.Reply(b, text, formatting.Shtml())
				return ext.EndGroups
			}

			noteName := strings.ToLower(nArgs[2])
			noteData := notes.GetNote(chatinfo.Id, noteName)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			if noteData == nil {
				text, _ := tr.GetString("helpers_note_not_exist")
				_, err := msg.Reply(b, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return ext.EndGroups
			}
			if noteData.AdminOnly {
				if !chat_status.IsUserAdmin(b, int64(chatID), user.Id) {
					text, _ := tr.GetString("helpers_note_admin_only")
					_, err := msg.Reply(b, text, formatting.Shtml())
					if err != nil {
						log.Error(err)
						return err
					}
					return ext.ContinueGroups
				}
			}
			_chat := chatinfo.ToChat() // need to convert to chat
			_, err := media.SendNote(b, ctx, &_chat, noteData, msg.MessageId, msg.MessageThreadId)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else if arg == "about" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		aboutText := getAboutText(tr)
		aboutKb := getAboutKb(tr)
		_, err := b.SendMessage(chat.Id,
			aboutText,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                msg.MessageId,
					AllowSendingWithoutReply: true,
				},
				ReplyMarkup: &aboutKb,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		// This sends the normal help block
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		startHelpText := getStartHelp(tr)
		startMarkupKb := getStartMarkup(tr, b.Username)
		_, err := b.SendMessage(chat.Id,
			startHelpText,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyMarkup: &startMarkupKb,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}
