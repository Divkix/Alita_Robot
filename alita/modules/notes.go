package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/media"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var notesModule = moduleStruct{
	moduleName: "Notes",
}

// noteStartSafeName matches note names Telegram accepts verbatim in a /start parameter.
var noteStartSafeName = regexp.MustCompile(`^[a-z0-9_-]+$`)

// noteNameHash is the short stand-in for a note name that cannot appear in a /start parameter.
func noteNameHash(name string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(name)))
	return "h" + hex.EncodeToString(sum[:6])
}

// noteStartParam returns the /start payload that opens a note in PM.
func noteStartParam(chatID int64, name string) string {
	name = strings.ToLower(name)
	p := fmt.Sprintf("note_%d_%s", chatID, name)
	if len(p) <= 64 && noteStartSafeName.MatchString(name) {
		return p
	}
	return fmt.Sprintf("note_%d_%s", chatID, noteNameHash(name))
}

func noteOverwriteCacheKey(token string) string {
	return overwriteCacheKey("note", token)
}

func setNoteOverwriteCache(token string, data overwriteNote) error {
	return setOverwriteCache(noteOverwriteCacheKey(token), data)
}

func consumeNoteOverwriteCache(token string) (*overwriteNote, error) {
	return consumeOverwriteCache[overwriteNote](noteOverwriteCacheKey(token))
}

func deleteNoteOverwriteCache(token string) {
	deleteOverwriteCache(noteOverwriteCacheKey(token))
}

//nolint:dupl // addNote shares validation logic with filters module by design
func (m moduleStruct) addNote(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	noteString, _ := tr.GetString("notes_save_success")

	if msg.ReplyToMessage != nil && len(args) <= 1 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_keyword_required")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(args) <= 2 && msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_invalid")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	result := content.ExtractNoteAndFilter(msg, false, lang.GetLanguage(ctx))
	noteWord, fileid, text, dataType, buttons, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif, errorMsg := result.KeyWord, result.FileID, result.Text, result.DataType, result.Buttons, result.PvtOnly, result.GrpOnly, result.AdminOnly, result.WebPreview, result.IsProtected, result.NoNotif, result.ErrorMsg
	if dataType == -1 && errorMsg != "" {
		_, err := msg.Reply(b, errorMsg, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if grpOnly && pvtOnly {
		grpOnly, pvtOnly = false, false
		noteConflictText, _ := tr.GetString("notes_private_conflict_warning")
		noteString += noteConflictText
	}

	noteWord = strings.ToLower(noteWord)

	if notes.DoesNoteExists(chat.Id, noteWord) {
		token, tokenErr := newOverwriteToken()
		if tokenErr != nil {
			log.Errorf("[Notes] Failed to generate overwrite token: %v", tokenErr)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			errorText, _ := tr.GetString("notes_overwrite_token_failed")
			_, _ = msg.Reply(b, errorText, formatting.Shtml())
			return ext.EndGroups
		}
		if err := setNoteOverwriteCache(token, overwriteNote{
			overwriteBase: overwriteBase{
				ChatID:   chat.Id,
				UserID:   user.Id,
				ItemName: noteWord,
				Text:     text,
				FileID:   fileid,
				Buttons:  buttons,
				DataType: dataType,
			},
			PvtOnly:     pvtOnly,
			GrpOnly:     grpOnly,
			AdminOnly:   adminOnly,
			WebPrev:     webPrev,
			IsProtected: isProtected,
			NoNotif:     noNotif,
		}); err != nil {
			log.Errorf("[Notes] Failed to cache overwrite data: %v", err)
			errorText, _ := tr.GetString("notes_overwrite_token_failed")
			_, _ = msg.Reply(b, errorText, formatting.Shtml())
			return ext.EndGroups
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		overwriteText, _ := tr.GetString("notes_overwrite_confirm")
		yesText, _ := tr.GetString("button_yes")
		noText, _ := tr.GetString("button_no")
		_, err := msg.Reply(b,
			overwriteText,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: yesText,
								CallbackData: encodeCallbackData("notes.overwrite", map[string]string{
									"a": "yes",
									"t": token,
								}),
							},
							{
								Text: noText,
								CallbackData: encodeCallbackData("notes.overwrite", map[string]string{
									"a": "no",
									"t": token,
								}),
							},
						},
					},
				},
			},
		)
		if err != nil {
			deleteNoteOverwriteCache(token)
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if err := notes.AddNote(chat.Id, noteWord, text, fileid, buttons, dataType, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif); err != nil {
		log.Errorf("[Notes] Failed to add note %s in chat %d: %v", noteWord, chat.Id, err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		errorText, _ := tr.GetString("notes_save_failed")
		_, err := msg.Reply(b, errorText, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	esc := html.EscapeString(noteWord)
	_, err := msg.Reply(b, fmt.Sprintf(noteString, esc, esc, esc), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) rmNote(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()

	if len(args) == 1 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_remove_keyword_required")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	parts := strings.SplitN(msg.Text, " ", 2)
	if len(parts) < 2 {
		return ext.EndGroups
	}
	noteWord := strings.TrimLeft(parts[1], "#")

	if !slices.Contains(notes.GetNotesListContext(tracing.UpdateContext(ctx), chat.Id, true), strings.ToLower(noteWord)) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_not_exists")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	noteWord, _ = extraction.ExtractQuotes(noteWord, false, true)

	if err := notes.RemoveNote(chat.Id, strings.ToLower(noteWord)); err != nil {
		log.Errorf("[Notes] Failed to remove note %s in chat %d: %v", noteWord, chat.Id, err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		errorText, _ := tr.GetString("error_generic")
		_, err := msg.Reply(b, errorText, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("notes_removed_success")
	_, err := msg.Reply(b, fmt.Sprintf(text, html.EscapeString(noteWord)), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (moduleStruct) privNote(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	var txt string

	if len(args) == 1 {
		option := args[0]
		switch option {
		case "on", "yes", "true":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_enabled")
			if err := chatsettings.PrivateNotes.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, true); err != nil {
				log.Errorf("[Notes] Failed to enable private notes for chat %d: %v", chat.Id, err)
				txt, _ = tr.GetString("error_generic")
			}
		case "off", "no", "false":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_disabled")
			if err := chatsettings.PrivateNotes.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, false); err != nil {
				log.Errorf("[Notes] Failed to disable private notes for chat %d: %v", chat.Id, err)
				txt, _ = tr.GetString("error_generic")
			}
		default:
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_invalid_option")
		}
	} else {
		tmp := chatsettings.PrivateNotes.ReadContext(tracing.UpdateContext(ctx), chat.Id).Value.(bool)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		if tmp {
			txt, _ = tr.GetString("notes_private_status_on")
		} else {
			txt, _ = tr.GetString("notes_private_status_off")
		}
	}
	_, err := msg.Reply(b, txt, formatting.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (moduleStruct) notesList(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	noteKeys := notes.GetNotesListContext(tracing.UpdateContext(ctx), chat.Id, chat_status.RequireUserAdmin(b, ctx, nil, user.Id))
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	info, _ := tr.GetString("notes_none_in_chat")

	if len(noteKeys) == 0 {
		_, err := msg.Reply(b, info, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if ctx.Message.Chat.Type == "private" {
		admin := chat_status.IsUserAdmin(b, chat.Id, user.Id)
		noteKeys := notes.GetNotesListContext(tracing.UpdateContext(ctx), chat.Id, admin)
		listText, _ := tr.GetString("notes_list_for_chat")
		info = fmt.Sprintf(listText, html.EscapeString(chat.Title))
		var sb strings.Builder
		for _, note := range noteKeys {
			fmt.Fprintf(&sb, "\n - <a href='https://t.me/%s?start=%s'>%s</a>",
				b.Username, noteStartParam(chat.Id, note), html.EscapeString(note))
		}
		info += sb.String()
		_, err := msg.Reply(b, info, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	privNote := notes.GetNotes(chat.Id).PrivateNotesEnabled()
	if privNote {
		// With /ephemeral on, the asker sees the PM list in the group.
		if sendEphemeral(b, ctx, notesLinkList(b, tr, chat.Id, noteKeys), *formatting.Shtml()) {
			return ext.EndGroups
		}
		checkBtnText, _ := tr.GetString("notes_check_button")
		_, err := msg.Reply(b, checkBtnText,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: trS(ctxTr(ctx), "button_click_me"),
								Url:  fmt.Sprintf("https://t.me/%s?start=notes_%d", b.Username, chat.Id),
							},
						},
					},
				},
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		currentNotesText, _ := tr.GetString("notes_current_in_chat")
		info = currentNotesText
		var sb strings.Builder
		for _, note := range noteKeys {
			fmt.Fprintf(&sb, " - <code>#%s</code>\n", html.EscapeString(note))
		}
		info += sb.String()
		instructionText, _ := tr.GetString("notes_get_instruction")
		info += instructionText
		_, err := msg.Reply(b, info, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

//nolint:dupl // rmAllNotes shares confirmation pattern with filters module by design
func (moduleStruct) rmAllNotes(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	noteKeys := notes.GetNotesListContext(tracing.UpdateContext(ctx), chat.Id, true)
	if len(noteKeys) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_none_in_chat")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	clearAllText, _ := tr.GetString("notes_clear_all_confirm")
	clearAllText = confirmText(ctx, tr, clearAllText, chat)
	_, err := msg.Reply(b, clearAllText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: confirmKeyboard(tr, "rmAllNotes", chat),
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) noteOverWriteHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From

	var helpText string
	action, token, ok := parseOverwriteCallbackData(query.Data, "notes.overwrite")
	if !ok {
		log.WithField("data", query.Data).Warn("Invalid note overwrite callback data format")
		return ext.EndGroups
	}
	chatID, ok := overwriteGroup(b, ctx, noteOverwriteCacheKey(token), user.Id)
	if !ok {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	switch action {
	case "no":
		if token != "" {
			if data, err := consumeNoteOverwriteCache(token); err == nil {
				if data.UserID != 0 && data.UserID != user.Id {
					helpText, _ = tr.GetString("notes_overwrite_cancelled")
					break
				}
			}
		}
		helpText, _ = tr.GetString("notes_overwrite_cancelled")
	case "yes":
		if token == "" {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
			break
		}
		noteData, err := consumeNoteOverwriteCache(token)
		if err != nil || (noteData.UserID != 0 && noteData.UserID != user.Id) {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
			break
		}

		updated, err := notes.UpdateNote(
			noteData.ChatID,
			noteData.ItemName,
			noteData.Text,
			noteData.FileID,
			noteData.Buttons,
			noteData.DataType,
			noteData.PvtOnly,
			noteData.GrpOnly,
			noteData.AdminOnly,
			noteData.WebPrev,
			noteData.IsProtected,
			noteData.NoNotif,
		)
		if err != nil {
			log.Errorf("[Notes] Failed to update note during overwrite: %v", err)
			helpText, _ = tr.GetString("notes_save_failed")
		} else if updated {
			helpText, _ = tr.GetString("notes_overwrite_success")
		} else {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
		}
	default:
		log.WithField("action", action).Warn("Unknown note overwrite action")
		return answerInvalidCallback(b, ctx, query)
	}

	if query.Message == nil {
		if _, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText}); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	editText := helpText
	if chatID != 0 {
		editText = withButtonGroupLine(b, ctx, tr, chatID, helpText)
	}
	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: editText, ParseMode: formatting.HTML})
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

func (moduleStruct) notesButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	decoded, _ := decodeCallbackData(query.Data, "rmAllNotes")
	chat := ownerConfirmChat(b, ctx, query, decoded)
	if chat == nil {
		return ext.EndGroups
	}

	response, _ := decoded.Field("a")
	if response == "" {
		log.Warnf("[Notes] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	switch response {
	case "yes":
		if err := notes.RemoveAllNotes(chat.Id); err != nil {
			log.Errorf("[Notes] Failed to remove all notes: %v", err)
			helpText, _ = tr.GetString("error_generic")
		} else {
			helpText, _ = tr.GetString("notes_clear_all_success")
		}
	case "no":
		helpText, _ = tr.GetString("notes_clear_all_cancelled")
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	if query.Message == nil {
		if _, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText}); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
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

func (m moduleStruct) notesWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.ContinueGroups
	}

	var replyMsgId int64
	var err error

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	parseText := strings.ToLower(msg.Text)[1:]
	noteNameArgs := strings.Split(parseText, " ")
	noteName := noteNameArgs[0]
	noformatNote := len(noteNameArgs) == 2 && noteNameArgs[1] == "noformat"

	if !slices.Contains(notes.GetNotesListContext(tracing.UpdateContext(ctx), chat.Id, true), strings.ToLower(noteName)) {
		return ext.ContinueGroups
	}

	// The list is cached, so it can still name a note whose row is gone.
	noteData := notes.GetNote(chat.Id, strings.ToLower(noteName))
	if noteData == nil {
		return ext.ContinueGroups
	}

	if noteData.NoteContent == "" && noteData.FileID == "" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_parsing_error")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if noteData.AdminOnly {
		if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("notes_admin_only")
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
	}

	if noformatNote {
		c, buildErr := helpers.BuildCommandContext(b, ctx)
		if buildErr != nil {
			return ext.EndGroups
		}
		err = m.sendNoFormatNote(c, replyMsgId, noteData)
		if err != nil {
			if !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups) {
				log.Error(err)
			}
			return err
		}
	} else {

		privateNoteOnly := (notes.GetNotes(chat.Id).PrivateNotesEnabled() || noteData.PrivateOnly) && !noteData.GroupOnly

		if privateNoteOnly {
			if ctx.Message.Chat.Type == "private" {
				err = sendNote(b, ctx, chat, noteData, replyMsgId, ctx.Message.MessageThreadId, media.Rendered)
			} else if !sendNoteEphemeral(b, ctx, chat, noteData) {
				tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
				clickForPrivateText, _ := tr.GetString("notes_click_for_private")
				_, err = msg.Reply(b,
					fmt.Sprintf(clickForPrivateText, noteName),
					&gotgbot.SendMessageOpts{
						ReplyParameters: &gotgbot.ReplyParameters{
							MessageId:                replyMsgId,
							AllowSendingWithoutReply: true,
						},
						ReplyMarkup: gotgbot.InlineKeyboardMarkup{
							InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
								{
									{
										Text: trS(tr, "button_click_me"),
										Url:  "https://t.me/" + b.Username + "?start=" + noteStartParam(chat.Id, noteName),
									},
								},
							},
						},
						ParseMode: formatting.Markdown,
					},
				)
			}
		} else {
			err = sendNote(b, ctx, chat, noteData, replyMsgId, ctx.Message.MessageThreadId, media.Rendered)
		}
	}

	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) getNotes(c *helpers.CommandContext) error {
	b, ctx, msg, chat := c.Bot, c.Ctx, c.Msg, c.Chat
	args := ctx.Args()[1:]
	var err error

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_get_insufficient_args")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	var replyMsgId int64

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	user := c.User
	noteName := args[0]

	// The list is cached, so it can still name a note whose row is gone.
	var noteData *db.Notes
	if slices.Contains(notes.GetNotesListContext(tracing.UpdateContext(ctx), chat.Id, true), strings.ToLower(noteName)) {
		noteData = notes.GetNote(chat.Id, strings.ToLower(noteName))
	}
	if noteData == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_does_not_exist")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if noteData.NoteContent == "" && noteData.FileID == "" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_parsing_error_support")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if noteData.AdminOnly {
		if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("notes_admin_only_access")
			_, err = msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
	}

	if len(args) == 2 && strings.ToLower(args[1]) == "noformat" {
		err = m.sendNoFormatNote(c, replyMsgId, noteData)
	} else {
		err = sendGetNote(c, noteName, replyMsgId, noteData)
	}

	if err != nil {
		if !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups) {
			log.Error(err)
		}
		return err
	}

	return ext.EndGroups
}

// sendGetNote answers /get with the note itself, or with a button to read it
// in PM when the note or the chat keeps notes private.
func sendGetNote(c *helpers.CommandContext, noteName string, replyMsgId int64, noteData *db.Notes) error {
	b, ctx, msg, chat := c.Bot, c.Ctx, c.Msg, c.Chat
	if (notes.GetNotes(chat.Id).PrivateNotesEnabled() || noteData.PrivateOnly) && !noteData.GroupOnly {
		if sendNoteEphemeral(b, ctx, chat, noteData) {
			return nil
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		clickForPrivateText, _ := tr.GetString("notes_click_for_private")
		_, err := msg.Reply(b,
			fmt.Sprintf(clickForPrivateText, noteName),
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: trS(tr, "button_click_me"),
								Url:  "https://t.me/" + b.Username + "?start=" + noteStartParam(chat.Id, noteName),
							},
						},
					},
				},
				ParseMode: formatting.Markdown,
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
			},
		)
		return err
	}
	return sendNote(b, ctx, chat, noteData, replyMsgId, ctx.Message.MessageThreadId, media.Rendered)
}

// sendNote sends a note to the chat the message was sent in: a PM command
// acting on a connected group points ctx.EffectiveChat at that group, which
// still fills the note's fillings. It answers replyMsgID in threadID.
func sendNote(b *gotgbot.Bot, ctx *ext.Context, group *gotgbot.Chat, note *db.Notes, replyMsgID, threadID int64, view media.View) error {
	chatID := ctx.EffectiveChat.Id
	if msg := ctx.EffectiveMessage; msg != nil {
		chatID = msg.Chat.Id
	}
	_, err := media.SendSaved(tracing.UpdateContext(ctx), b, media.NoteMessage(note), media.Target{
		Chat:       group,
		ChatID:     chatID,
		ThreadID:   threadID,
		ReplyMsgID: replyMsgID,
		Member:     ctx.EffectiveUser,
	}, view)
	return err
}

// sendNoteEphemeral shows a note that would open in PM only to its asker, in
// the group, when ephemeralReceiver allows it. On false the caller sends its
// PM button as before: not eligible, or Telegram refused the send (the bot is
// not an admin, or the note has a web_app or login_url button).
func sendNoteEphemeral(b *gotgbot.Bot, ctx *ext.Context, group *gotgbot.Chat, note *db.Notes) bool {
	receiver, ok := ephemeralReceiver(ctx)
	if !ok {
		return false
	}
	msg := ctx.EffectiveMessage
	var threadID int64
	if msg.IsTopicMessage {
		threadID = msg.MessageThreadId
	}
	_, err := media.SendSaved(tracing.UpdateContext(ctx), b, media.NoteMessage(note), media.Target{
		Chat:        group,
		ChatID:      msg.Chat.Id,
		ThreadID:    threadID,
		Member:      ctx.EffectiveUser,
		EphemeralTo: receiver,
	}, media.Rendered)
	if err != nil {
		log.WithContext(tracing.UpdateContext(ctx)).WithFields(log.Fields{"chat_id": msg.Chat.Id, "error": err}).
			Debug("[Ephemeral] could not show the note only to its asker; answering publicly")
		return false
	}
	return true
}

func (moduleStruct) sendNoFormatNote(c *helpers.CommandContext, replyMsgId int64, noteData *db.Notes) error {
	if !helpers.RunChecks(c, []helpers.CheckFunc{helpers.RequireUserAdmin()}) {
		return ext.EndGroups
	}

	if err := sendNote(c.Bot, c.Ctx, c.Chat, noteData, replyMsgId, c.Ctx.Message.MessageThreadId, media.Raw); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// notesClearAllOwnerCheck preserves the empty-list response, live creator
// lookup, custom refusal and silent rejection of system accounts.
func notesClearAllOwnerCheck(c *helpers.CommandContext) bool {
	if len(notes.GetNotesListContext(tracing.UpdateContext(c.Ctx), c.Chat.Id, true)) == 0 {
		return true
	}
	if !chat_status.IsValidUserId(c.User.Id) || chat_status.IsSystemUser(c.User.Id) {
		return false
	}
	mem, err := c.Bot.GetChatMemberWithContext(tracing.UpdateContext(c.Ctx), c.Chat.Id, c.User.Id, nil)
	if err != nil && !error_handling.IsMissingChatMemberError(err) {
		_ = helpers.EndGroupsLogging(c.Ctx, err)
		return false
	}
	if mem != nil && mem.GetStatus() == "creator" {
		return true
	}
	text, _ := c.Tr.GetString("notes_creator_only")
	if _, err := c.Msg.Reply(c.Bot, text, formatting.Shtml()); err != nil {
		_ = helpers.EndGroupsLogging(c.Ctx, err)
	}
	return false
}

func LoadNotes(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(notesModule.moduleName, true)

	SetModuleHelpBuilder(notesModule.moduleName, func(l string) [][]gotgbot.InlineKeyboardButton {
		return [][]gotgbot.InlineKeyboardButton{{{
			Text:         trS(i18n.MustNewTranslator(l), "button_formatting"),
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Formatting"}),
		}}}
	})
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "save",
		Aliases:        []string{"addnote"},
		RequiredChecks: []helpers.CheckFunc{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserChangeInfo()},
	}, pipelineHandler(notesModule.addNote))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:    "clear",
		Aliases: []string{"rmnote"},
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireConnectedAdmin(helpers.BotAdminRequired),
			func(c *helpers.CommandContext) bool {
				// Missing keywords and non-text commands return before this right.
				return len(c.Ctx.Args()) == 1 || !strings.Contains(c.Msg.Text, " ") || helpers.CanUserChangeInfo()(c)
			},
		},
	}, pipelineHandler(notesModule.rmNote))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "notes",
		Aliases:        []string{"saved"},
		PMGroupArg:     helpers.PMGroupArgNone,
		Disableable:    true,
		RequiredChecks: []helpers.CheckFunc{helpers.RequireConnectedMember(helpers.BotAdminRequired)},
	}, pipelineHandler(notesModule.notesList))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "clearall",
		PMGroupArg:     helpers.PMGroupArgNone,
		RequiredChecks: []helpers.CheckFunc{groupOnlyChatCheck, helpers.RequireGroup(), notesClearAllOwnerCheck},
	}, pipelineHandler(notesModule.rmAllNotes))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllNotes"), notesModule.notesButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("notes.overwrite"), notesModule.noteOverWriteHandler))
	dispatcher.AddHandler(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return strings.HasPrefix(msg.Text, "#")
			},
			notesModule.notesWatcher,
		),
	)
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "privnote",
		Aliases:        []string{"privatenotes"},
		PMGroupArg:     helpers.PMGroupArgOwn,
		RequiredChecks: append([]helpers.CheckFunc{groupOnlyChatCheck}, declaredGroupSettingChecks(chatsettings.PrivateNotes)...),
	}, pipelineHandler(notesModule.privNote))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "get",
		Disableable:    true,
		RequiredChecks: []helpers.CheckFunc{helpers.RequireConnectedMember(helpers.BotAdminNotRequired)},
	}, notesModule.getNotes)
}

func init() {
	RegisterLegacyModule("Notes", 160, LoadNotes)
	RegisterDeepLinkHandler("notes_", notesListDeepLinkHandler)
	RegisterDeepLinkHandler("note_", noteDeepLinkHandler)
	RegisterDeepLinkHandler("note", invalidNoteDeepLinkHandler)
}

func notesListDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage

	chatinfo, err := parseChatInfoFromDeepLink(b, ctx, arg)
	if err != nil {
		return err
	}

	_chat := chatinfo.ToChat()
	if !chat_status.IsUserInChat(b, &_chat, user.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_chat_not_found")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	admin := chat_status.IsUserAdmin(b, chatinfo.Id, user.Id)
	noteKeys := notes.GetNotesListContext(tracing.UpdateContext(ctx), chatinfo.Id, admin)
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	info, _ := tr.GetString("notes_none_in_chat")
	if len(noteKeys) > 0 {
		info = notesLinkList(b, tr, chatinfo.Id, noteKeys)
	}

	_, err = msg.Reply(b, info, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// notesLinkList is the private-notes list: each note links to the /start
// parameter that opens it in PM.
func notesLinkList(b *gotgbot.Bot, tr *i18n.Translator, chatID int64, noteKeys []string) string {
	info, _ := tr.GetString("helpers_notes_current_header")
	var sb strings.Builder
	for _, note := range noteKeys {
		fmt.Fprintf(&sb, " - <a href='https://t.me/%s?start=%s'>%s</a>\n", b.Username, noteStartParam(chatID, note), html.EscapeString(note))
	}
	return info + sb.String()
}

func noteDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage

	chatinfo, err := parseChatInfoFromDeepLink(b, ctx, arg)
	if err != nil {
		return err
	}

	_chat := chatinfo.ToChat()
	if !chat_status.IsUserInChat(b, &_chat, user.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_chat_not_found")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	nArgs := strings.SplitN(arg, "_", 3)
	if len(nArgs) < 3 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	noteName := strings.ToLower(nArgs[2])
	noteData := notes.GetNote(chatinfo.Id, noteName)
	// A name /start cannot carry arrives as its hash; a real note of that name wins.
	if noteData == nil && len(noteName) == 13 && strings.HasPrefix(noteName, "h") {
		for _, n := range notes.GetNotesListContext(tracing.UpdateContext(ctx), chatinfo.Id, true) {
			if noteNameHash(n) == noteName {
				noteData = notes.GetNote(chatinfo.Id, n)
				break
			}
		}
	}
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
		if !chat_status.IsUserAdmin(b, chatinfo.Id, user.Id) {
			text, _ := tr.GetString("helpers_note_admin_only")
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
	}
	err = sendNote(b, ctx, &_chat, noteData, msg.MessageId, msg.MessageThreadId, media.Rendered)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func invalidNoteDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("helpers_invalid_deep_link")
	_, _ = ctx.EffectiveMessage.Reply(b, text, formatting.Shtml())
	return ext.EndGroups
}
