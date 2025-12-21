package modules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"

	"github.com/divkix/Alita_Robot/alita/utils/decorators/cmdDecorator"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/media"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

var notesModule = moduleStruct{
	moduleName: "Notes",
	// overwriteNotesMap is a sync.Map, initialized to zero value (no make needed)
}

// addNote handles the /save command to create new notes
// with support for various media types and formatting options.
func (m moduleStruct) addNote(b *gotgbot.Bot, ctx *ext.Context) error {
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User
	args := ctx.Args()

	// check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	noteString, _ := tr.GetString("notes_save_success")

	if msg.ReplyToMessage != nil && len(args) <= 1 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_keyword_required")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(args) <= 2 && msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_invalid")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	noteWord, fileid, text, dataType, buttons, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif, errorMsg := helpers.GetNoteAndFilterType(msg, false, db.GetLanguage(ctx))
	if dataType == -1 && errorMsg != "" {
		_, err := msg.Reply(b, errorMsg, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// if user specifies both noprivate and private, the note will be sent to default.
	// If privatenotes is enabled, the private else group
	if grpOnly && pvtOnly {
		grpOnly, pvtOnly = false, false
		noteConflictText, _ := tr.GetString("notes_private_conflict_warning")
		noteString += noteConflictText
	}

	noteWord = strings.ToLower(noteWord)

	// check if note already exists or not
	if db.DoesNoteExists(chat.Id, noteWord) {
		noteWordMapKey := fmt.Sprintf("%d_%s", chat.Id, noteWord)
		notesOverwriteMap.Store(noteWordMapKey, overwriteNote{
			noteWord:    noteWord,
			text:        text,
			fileId:      fileid,
			buttons:     buttons,
			dataType:    dataType,
			pvtOnly:     pvtOnly,
			grpOnly:     grpOnly,
			adminOnly:   adminOnly,
			webPrev:     webPrev,
			isProtected: isProtected,
			noNotif:     noNotif,
		})
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		overwriteText, _ := tr.GetString("notes_overwrite_confirm")
		yesText, _ := tr.GetString("button_yes")
		noText, _ := tr.GetString("button_no")
		_, err := msg.Reply(b,
			overwriteText,
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         yesText,
								CallbackData: fmt.Sprintf("notes.overwrite.yes.%s", noteWordMapKey),
							},
							{
								Text:         noText,
								CallbackData: fmt.Sprintf("notes.overwrite.no.%s", noteWordMapKey),
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
		return ext.EndGroups
	}

	go db.AddNote(chat.Id, noteWord, text, fileid, buttons, dataType, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif)

	_, err := msg.Reply(b, fmt.Sprintf(noteString, noteWord, noteWord, noteWord), helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// rmNote handles the /clear command to remove existing notes
// from the chat, requiring admin permissions.
func (moduleStruct) rmNote(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()

	if len(args) == 1 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_remove_keyword_required")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Extract note word safely to prevent panic
	parts := strings.SplitN(msg.Text, " ", 2)
	if len(parts) < 2 {
		return ext.EndGroups // should not happen due to len(args) check above
	}
	noteWord := strings.TrimLeft(parts[1], "#")

	// check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	// check if note exists in admin notes as well
	if !string_handling.FindInStringSlice(db.GetNotesList(chat.Id, true), strings.ToLower(noteWord)) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_not_exists")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	noteWord, _ = extraction.ExtractQuotes(noteWord, false, true)

	db.RemoveNote(chat.Id, strings.ToLower(noteWord))

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("notes_removed_success")
	_, err := msg.Reply(b, fmt.Sprintf(text, noteWord), helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// privNote handles the /privnote command to toggle private notes
// setting, controlling whether notes are sent privately or in group.
func (moduleStruct) privNote(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	var txt string

	if len(args) == 1 {
		option := args[0]
		switch option {
		case "on", "yes", "true":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_enabled")
			go db.TooglePrivateNote(chat.Id, true)
		case "off", "no", "false":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_disabled")
			go db.TooglePrivateNote(chat.Id, false)
		default:
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			txt, _ = tr.GetString("notes_private_invalid_option")
		}
	} else {
		tmp := db.GetNotes(chat.Id).PrivateNotesEnabled()
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if tmp {
			txt, _ = tr.GetString("notes_private_status_on")
		} else {
			txt, _ = tr.GetString("notes_private_status_off")
		}
	}
	_, err := msg.Reply(b, txt, helpers.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// notesList handles the /notes command to display all available
// notes in the chat with appropriate access controls.
func (moduleStruct) notesList(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "notes") {
		return ext.EndGroups
	}
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	noteKeys := db.GetNotesList(chat.Id, chat_status.RequireUserAdmin(b, ctx, nil, user.Id, true))
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	info, _ := tr.GetString("notes_none_in_chat")

	if len(noteKeys) == 0 {
		_, err := msg.Reply(b, info, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// if user uses the /note command in private chat
	// No matter if privRules are set or not
	if ctx.Message.Chat.Type == "private" {
		// check if want admin notes or not
		admin := chat_status.IsUserAdmin(b, chat.Id, user.Id)
		noteKeys := db.GetNotesList(chat.Id, admin)
		listText, _ := tr.GetString("notes_list_for_chat")
		info = fmt.Sprintf(listText, chat.Title)
		var sb strings.Builder
		for _, note := range noteKeys {
			sb.WriteString(fmt.Sprintf("\n - <a href='https://t.me/%s?start=note_%d_%s'>%s</a>",
				b.Username, chat.Id, note, note))
		}
		info += sb.String()
		_, err := msg.Reply(b, info, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	privNote := db.GetNotes(chat.Id).PrivateNotesEnabled()
	if privNote {
		checkBtnText, _ := tr.GetString("notes_check_button")
		_, err := msg.Reply(b, checkBtnText,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: func() string {
									tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
			sb.WriteString(fmt.Sprintf(" - <code>#%s</code>\n", note))
		}
		info += sb.String()
		instructionText, _ := tr.GetString("notes_get_instruction")
		info += instructionText
		_, err := msg.Reply(b, info, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

// rmAllNotes handles the /clearall command to remove all notes
// from the chat, restricted to chat owners only.
func (moduleStruct) rmAllNotes(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}

	// check notes in adminkeys as well
	noteKeys := db.GetNotesList(chat.Id, true)
	if len(noteKeys) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_none_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
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
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		clearAllText, _ := tr.GetString("notes_clear_all_confirm")
		yesText, _ := tr.GetString("button_yes")
		noText, _ := tr.GetString("button_no")
		_, err := msg.Reply(b, clearAllText,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{Text: yesText, CallbackData: "rmAllNotes.yes"},
							{Text: noText, CallbackData: "rmAllNotes.no"},
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
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_creator_only")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

// noteOverWriteHandler processes callback queries for note overwrite
// confirmations when adding notes that already exist.
// Callback format: notes.overwrite.{action}.{chatId}_{noteWord}
func (m moduleStruct) noteOverWriteHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	user := query.From

	// permission checks
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	var helpText string
	args := strings.Split(query.Data, ".")

	// Validate callback data format: notes.overwrite.{action}.{key}
	if len(args) < 4 {
		log.WithField("data", query.Data).Warn("Invalid note overwrite callback data format")
		return ext.EndGroups
	}

	action := args[2]         // "yes" or "no"
	noteWordMapKey := args[3] // "{chatId}_{noteWord}"

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	switch action {
	case "no":
		// Clean up the pending overwrite entry when user cancels
		notesOverwriteMap.Delete(noteWordMapKey)
		helpText, _ = tr.GetString("notes_overwrite_cancelled")
	case "yes":
		dataSplit := strings.Split(noteWordMapKey, "_")
		if len(dataSplit) < 2 {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
			break
		}
		strChatId, noteWord := dataSplit[0], dataSplit[1]
		chatId, _ := strconv.ParseInt(strChatId, 10, 64)
		noteDataRaw, ok := notesOverwriteMap.Load(noteWordMapKey)
		if !ok {
			helpText, _ = tr.GetString("notes_overwrite_cancelled")
			break
		}
		noteData := noteDataRaw.(overwriteNote)
		if db.DoesNoteExists(chatId, noteWord) {
			db.RemoveNote(chatId, noteWord)
			db.AddNote(chatId, noteData.noteWord, noteData.text, noteData.fileId, noteData.buttons, noteData.dataType, noteData.pvtOnly, noteData.grpOnly, noteData.adminOnly, noteData.webPrev, noteData.isProtected, noteData.noNotif)
			notesOverwriteMap.Delete(noteWordMapKey)
			helpText, _ = tr.GetString("notes_overwrite_success")
		}
	default:
		log.WithField("action", action).Warn("Unknown note overwrite action")
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(
		b,
		helpText,
		nil,
	)
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

// notesButtonHandler processes callback queries for the remove all notes
// confirmation dialog, restricted to chat owners.
func (moduleStruct) notesButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	user := query.From

	// permission checks
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	response := args[1]
	var helpText string

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	switch response {
	case "yes":
		db.RemoveAllNotes(query.Message.GetChat().Id)
		helpText, _ = tr.GetString("notes_clear_all_success")
	case "no":
		helpText, _ = tr.GetString("notes_clear_all_cancelled")
	}

	_, _, err := query.Message.EditText(
		b,
		helpText,
		nil,
	)
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

// notesWatcher monitors messages starting with '#' and automatically
// sends the corresponding note if it exists in the chat.
func (m moduleStruct) notesWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	var replyMsgId int64
	var err error

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	parseText := strings.ToLower(msg.Text)[1:] // remove '#' from note name
	noteNameArgs := strings.Split(parseText, " ")
	noteName := noteNameArgs[0]
	noformatNote := len(noteNameArgs) == 2 && noteNameArgs[1] == "noformat"

	// if note does not exist, continue groups
	if !string_handling.FindInStringSlice(db.GetNotesList(chat.Id, true), strings.ToLower(noteName)) {
		return ext.EndGroups
	}

	noteData := db.GetNote(chat.Id, noteName)

	// check if notedata is correct or not
	if noteData.NoteContent == "" && noteData.FileID == "" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_parsing_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// check for admin only notes
	// admin notes follow the group note policy
	if noteData.AdminOnly {
		if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("notes_admin_only")
			_, err := msg.Reply(b, text, helpers.Shtml())
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

		// chat has private notes enabled or note is private and not group note
		privateNoteOnly := (db.GetNotes(chat.Id).PrivateNotesEnabled() || noteData.PrivateOnly) && !noteData.GroupOnly

		// send private note if private notes is enabled or note is private, and it is not group note
		if privateNoteOnly {
			if ctx.Message.Chat.Type == "private" {
				_, err = helpers.SendNote(b, chat, ctx, noteData, replyMsgId)
			} else {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
										Text: func() string { t, _ := tr.GetString("button_click_me"); return t }(),
										Url:  fmt.Sprintf("https://t.me/%s?start=note_%d_%s", b.Username, chat.Id, noteName),
									},
								},
							},
						},
						ParseMode: helpers.Markdown,
					},
				)
			}
		} else {
			_, err = helpers.SendNote(b, chat, ctx, noteData, replyMsgId)
		}
	}

	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// getNotes handles the /get command to retrieve and send
// specific notes by name with format options.
func (m moduleStruct) getNotes(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "get") {
		return ext.EndGroups
	}
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, false, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	var err error

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_get_insufficient_args")
		_, err := msg.Reply(b, text, helpers.Shtml())
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

	user := ctx.EffectiveSender.User
	noteName := args[0]

	// check if note exists or not
	if !string_handling.FindInStringSlice(db.GetNotesList(chat.Id, true), strings.ToLower(noteName)) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_does_not_exist")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	noteData := db.GetNote(chat.Id, noteName)

	// check if notedata is correct or not
	if noteData.NoteContent == "" && noteData.FileID == "" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("notes_parsing_error_support")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// check for admin only notes
	// admin notes follow the group note policy
	if noteData.AdminOnly {
		if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("notes_admin_only_access")
			_, err = msg.Reply(b, text, helpers.Shtml())
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
		// send private note if private notes is enabled or note is private, and it is not group note
		if (db.GetNotes(chat.Id).PrivateNotesEnabled() || noteData.PrivateOnly) && !noteData.GroupOnly {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			clickForPrivateText, _ := tr.GetString("notes_click_for_private")
			_, err = msg.Reply(b,
				fmt.Sprintf(clickForPrivateText, noteName),
				&gotgbot.SendMessageOpts{
					ReplyMarkup: gotgbot.InlineKeyboardMarkup{
						InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
							{
								{
									Text: func() string { t, _ := tr.GetString("button_click_me"); return t }(),
									Url:  fmt.Sprintf("https://t.me/%s?start=note_%d_%s", b.Username, chat.Id, noteName),
								},
							},
						},
					},
					ParseMode: helpers.Markdown,
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
				},
			)
		} else {
			_, err = helpers.SendNote(b, chat, ctx, noteData, replyMsgId)
		}
	}

	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// sendNoFormatNote sends a note in raw format without markdown processing,
// showing the original formatting codes, restricted to admins.
func (moduleStruct) sendNoFormatNote(b *gotgbot.Bot, ctx *ext.Context, replyMsgId int64, noteData *db.Notes) error {
	user := ctx.EffectiveSender.User

	// check if user is admin or not
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	// Reverse notedata
	noteData.NoteContent = helpers.ReverseHTML2MD(noteData.NoteContent)

	// show the buttons back as text
	noteData.NoteContent += helpers.RevertButtons(noteData.Buttons)

	// Send note using the new media package
	// raw note does not need webpreview
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
		NoFormat:          true, // noformat mode
		NoNotif:           noteData.NoNotif,
		WebPreview:        false,
		IsProtected:       noteData.IsProtected,
		AllowWithoutReply: true,
	})
	// if strings.Contains(err.Error(), "replied message not found") {
	// 	return ext.EndGroups
	// }
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// LoadNotes registers all notes module handlers with the dispatcher,
// including note management commands and the notes watcher.
func LoadNotes(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(notesModule.moduleName, true)

	HelpModule.helpableKb[notesModule.moduleName] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         func() string { tr := i18n.MustNewTranslator("en"); t, _ := tr.GetString("button_formatting"); return t }(),
				CallbackData: fmt.Sprintf("helpq.%s", "Formatting"),
			},
		},
	} // Adds Formatting kb button to Notes Menu
	dispatcher.AddHandler(handlers.NewCommand("save", notesModule.addNote))
	dispatcher.AddHandler(handlers.NewCommand("addnote", notesModule.addNote))
	dispatcher.AddHandler(handlers.NewCommand("clear", notesModule.rmNote))
	dispatcher.AddHandler(handlers.NewCommand("rmnote", notesModule.rmNote))
	dispatcher.AddHandler(handlers.NewCommand("notes", notesModule.notesList))
	// Alias: /saved should behave like /notes per documentation
	dispatcher.AddHandler(handlers.NewCommand("saved", notesModule.notesList))
	misc.AddCmdToDisableable("notes")
	dispatcher.AddHandler(handlers.NewCommand("clearall", notesModule.rmAllNotes))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllNotes"), notesModule.notesButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("notes.overwrite."), notesModule.noteOverWriteHandler))
	dispatcher.AddHandler(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return strings.HasPrefix(msg.Text, "#")
			},
			notesModule.notesWatcher,
		),
	)
	cmdDecorator.MultiCommand(dispatcher, []string{"privnote", "privatenotes"}, notesModule.privNote)
	dispatcher.AddHandler(handlers.NewCommand("get", notesModule.getNotes))
	misc.AddCmdToDisableable("get")
}
