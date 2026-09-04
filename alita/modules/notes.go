package modules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/media"
)

var notesModule = moduleStruct{
	moduleName: "Notes",
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
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()

	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

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

	_, err := msg.Reply(b, fmt.Sprintf(noteString, noteWord, noteWord, noteWord), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) rmNote(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
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

	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	if !slices.Contains(notes.GetNotesList(chat.Id, true), strings.ToLower(noteWord)) {
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
	_, err := msg.Reply(b, fmt.Sprintf(text, noteWord), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (moduleStruct) privNote(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	var txt string

	if len(args) == 1 {
		option := args[0]
		switch option {
		case "on", "yes", "true":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_enabled")
			if err := notes.TooglePrivateNote(chat.Id, true); err != nil {
				log.Errorf("[Notes] Failed to enable private notes for chat %d: %v", chat.Id, err)
			}
		case "off", "no", "false":
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_disabled")
			if err := notes.TooglePrivateNote(chat.Id, false); err != nil {
				log.Errorf("[Notes] Failed to disable private notes for chat %d: %v", chat.Id, err)
			}
		default:
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_invalid_option")
		}
	} else {
		tmp := notes.GetNotes(chat.Id).PrivateNotesEnabled()
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
	if chat_status.CheckDisabledCmd(b, msg, "notes") {
		return ext.EndGroups
	}
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	noteKeys := notes.GetNotesList(chat.Id, chat_status.RequireUserAdmin(b, ctx, nil, user.Id))
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
		noteKeys := notes.GetNotesList(chat.Id, admin)
		listText, _ := tr.GetString("notes_list_for_chat")
		info = fmt.Sprintf(listText, chat.Title)
		var sb strings.Builder
		for _, note := range noteKeys {
			fmt.Fprintf(&sb, "\n - <a href='https://t.me/%s?start=note_%d_%s'>%s</a>",
				b.Username, chat.Id, note, note)
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
		checkBtnText, _ := tr.GetString("notes_check_button")
		_, err := msg.Reply(b, checkBtnText,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: func() string {
									tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
									t, _ := tr.GetString("button_click_me")
									return t
								}(),
								Url: fmt.Sprintf("https://t.me/%s?start=notes_%d", b.Username, chat.Id),
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
			fmt.Fprintf(&sb, " - <code>#%s</code>\n", note)
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

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	noteKeys := notes.GetNotesList(chat.Id, true)
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

	mem, err := chat.GetMember(b, user.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	if mem.MergeChatMember().Status == "creator" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		clearAllText, _ := tr.GetString("notes_clear_all_confirm")
		yesText, _ := tr.GetString("button_yes")
		noText, _ := tr.GetString("button_no")
		_, err := msg.Reply(b, clearAllText,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         yesText,
								CallbackData: encodeCallbackData("rmAllNotes", map[string]string{"a": "yes"}),
							},
							{
								Text:         noText,
								CallbackData: encodeCallbackData("rmAllNotes", map[string]string{"a": "no"}),
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
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_creator_only")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

func (m moduleStruct) noteOverWriteHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var helpText string
	action, token, ok := parseOverwriteCallbackData(query.Data, "notes.overwrite")
	if !ok {
		log.WithField("data", query.Data).Warn("Invalid note overwrite callback data format")
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
		var chatId int64

		if token == "" {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
			break
		}
		noteData, err := consumeNoteOverwriteCache(token)
		if err != nil || (noteData.UserID != 0 && noteData.UserID != user.Id) {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
			break
		}
		chatId = noteData.ChatID
		if chatId == 0 {
			if query.Message != nil {
				chatId = query.Message.GetChat().Id
			} else if ctx.EffectiveChat != nil {
				chatId = ctx.EffectiveChat.Id
			}
		}

		callbackChatID := int64(0)
		if query.Message != nil {
			callbackChatID = query.Message.GetChat().Id
		} else if ctx.EffectiveChat != nil {
			callbackChatID = ctx.EffectiveChat.Id
		}
		if noteData.ChatID != 0 && callbackChatID != 0 && noteData.ChatID != callbackChatID {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
			break
		}

		updated, err := notes.UpdateNote(
			chatId,
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

func (moduleStruct) notesButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	response := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmAllNotes"); ok {
		response, _ = decoded.Field("a")
	}
	if response == "" {
		log.Warnf("[Notes] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chat := ctx.EffectiveChat
	switch response {
	case "yes":
		if chat == nil {
			helpText, _ = tr.GetString("error_generic")
			break
		}
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

	if !slices.Contains(notes.GetNotesList(chat.Id, true), strings.ToLower(noteName)) {
		return ext.ContinueGroups
	}

	noteData := notes.GetNote(chat.Id, strings.ToLower(noteName))

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
		err = m.sendNoFormatNote(b, ctx, replyMsgId, noteData)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {

		privateNoteOnly := (notes.GetNotes(chat.Id).PrivateNotesEnabled() || noteData.PrivateOnly) && !noteData.GroupOnly

		if privateNoteOnly {
			if ctx.Message.Chat.Type == "private" {
				_, err = media.SendNote(b, ctx, chat, noteData, replyMsgId, ctx.Message.MessageThreadId)
			} else {
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
										Url:  fmt.Sprintf("https://t.me/%s?start=note_%d_%s", b.Username, chat.Id, noteName),
									},
								},
							},
						},
						ParseMode: formatting.Markdown,
					},
				)
			}
		} else {
			_, err = media.SendNote(b, ctx, chat, noteData, replyMsgId, ctx.Message.MessageThreadId)
		}
	}

	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) getNotes(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.CheckDisabledCmd(b, msg, "get") {
		return ext.EndGroups
	}
	connectedChat := chat_status.IsUserConnected(b, ctx, false, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
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

	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	noteName := args[0]

	if !slices.Contains(notes.GetNotesList(chat.Id, true), strings.ToLower(noteName)) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("notes_does_not_exist")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	noteData := notes.GetNote(chat.Id, strings.ToLower(noteName))

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
			return ext.ContinueGroups
		}
	}

	if len(args) == 2 && strings.ToLower(args[1]) == "noformat" {
		err = m.sendNoFormatNote(b, ctx, replyMsgId, noteData)
	} else {
		if (notes.GetNotes(chat.Id).PrivateNotesEnabled() || noteData.PrivateOnly) && !noteData.GroupOnly {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			clickForPrivateText, _ := tr.GetString("notes_click_for_private")
			_, err = msg.Reply(b,
				fmt.Sprintf(clickForPrivateText, noteName),
				&gotgbot.SendMessageOpts{
					ReplyMarkup: gotgbot.InlineKeyboardMarkup{
						InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
							{
								{
									Text: trS(tr, "button_click_me"),
									Url:  fmt.Sprintf("https://t.me/%s?start=note_%d_%s", b.Username, chat.Id, noteName),
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
		} else {
			_, err = media.SendNote(b, ctx, chat, noteData, replyMsgId, ctx.Message.MessageThreadId)
		}
	}

	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) sendNoFormatNote(b *gotgbot.Bot, ctx *ext.Context, replyMsgId int64, noteData *db.Notes) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		chat_status.NewPermissionResponder(b).Respond(ctx, "common_cannot_identify_user", "", chat_status.WithReply())
		return ext.EndGroups
	}

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	noteData.NoteContent = formatting.ReverseHTML2MD(noteData.NoteContent)

	noteData.NoteContent += content.RevertButtons(noteData.Buttons)

	_, err := media.Send(b, media.Content{
		Text:    noteData.NoteContent,
		FileID:  noteData.FileID,
		MsgType: noteData.MsgType,
		Name:    noteData.NoteName,
	}, media.Options{
		ChatID:            ctx.Message.Chat.Id,
		ReplyMsgID:        replyMsgId,
		ThreadID:          ctx.Message.MessageThreadId,
		Keyboard:          &gotgbot.InlineKeyboardMarkup{InlineKeyboard: nil},
		NoFormat:          true,
		NoNotif:           noteData.NoNotif,
		WebPreview:        false,
		IsProtected:       noteData.IsProtected,
		AllowWithoutReply: true,
	})
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func LoadNotes(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[notesModule.moduleName] = true

	DefaultHelpRegistry().helpableKb[notesModule.moduleName] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         trS(i18n.MustNewTranslator("en"), "button_formatting"),
				CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Formatting"}),
			},
		},
	}
	dispatcher.AddHandler(handlers.NewCommand("save", notesModule.addNote))
	dispatcher.AddHandler(handlers.NewCommand("addnote", notesModule.addNote))
	dispatcher.AddHandler(handlers.NewCommand("clear", notesModule.rmNote))
	dispatcher.AddHandler(handlers.NewCommand("rmnote", notesModule.rmNote))
	dispatcher.AddHandler(handlers.NewCommand("notes", notesModule.notesList))
	dispatcher.AddHandler(handlers.NewCommand("saved", notesModule.notesList))
	helpers.AddCmdToDisableable("notes")
	dispatcher.AddHandler(handlers.NewCommand("clearall", notesModule.rmAllNotes))
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
	helpers.MultiCommand(dispatcher, []string{"privnote", "privatenotes"}, notesModule.privNote)
	dispatcher.AddHandler(handlers.NewCommand("get", notesModule.getNotes))
	helpers.AddCmdToDisableable("get")
}

func init() {
	RegisterLegacyModule("Notes", 160, LoadNotes)
	RegisterDeepLinkHandler("notes_", notesListDeepLinkHandler)
	RegisterDeepLinkHandler("note_", noteDeepLinkHandler)
	RegisterDeepLinkHandler("note", invalidNoteDeepLinkHandler)
}

func parseChatInfoFromDeepLink(b *gotgbot.Bot, ctx *ext.Context, arg string) (chatinfo *gotgbot.ChatFullInfo, err error) {
	nArgs := strings.SplitN(arg, "_", 3)
	msg := ctx.EffectiveMessage

	if len(nArgs) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return nil, ext.EndGroups
	}

	chatID, parseErr := strconv.ParseInt(nArgs[1], 10, 64)
	if parseErr != nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return nil, ext.EndGroups
	}

	chatinfo, chatErr := b.GetChat(chatID, nil)
	if chatErr != nil || chatinfo == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_chat_not_found")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return nil, ext.EndGroups
	}
	return chatinfo, nil
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
	noteKeys := notes.GetNotesList(chatinfo.Id, admin)
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	info, _ := tr.GetString("notes_none_in_chat")
	if len(noteKeys) > 0 {
		info, _ = tr.GetString("helpers_notes_current_header")
		var sb strings.Builder
		for _, note := range noteKeys {
			fmt.Fprintf(&sb, " - <a href='https://t.me/%s?start=note_%d_%s'>%s</a>\n", b.Username, chatinfo.Id, note, note)
		}
		info += sb.String()
	}

	_, err = msg.Reply(b, info, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
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
			return ext.ContinueGroups
		}
	}
	_, err = media.SendNote(b, ctx, &_chat, noteData, msg.MessageId, msg.MessageThreadId)
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
