package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var userNotesModule = moduleStruct{
	moduleName:   "UserNotes",
	handlerGroup: 0,
}

// LoadUserNotes loads the user notes module with all command handlers
func LoadUserNotes(dispatcher *ext.Dispatcher) {
	// User note commands - available to all users
	dispatcher.AddHandler(handlers.NewCommand("savenote", userNotesModule.saveNote))
	dispatcher.AddHandler(handlers.NewCommand("getnote", userNotesModule.getNote))
	dispatcher.AddHandler(handlers.NewCommand("mynotes", userNotesModule.myNotes))
	dispatcher.AddHandler(handlers.NewCommand("deletenote", userNotesModule.deleteNote))

	// Add alternative command names
	helpers.MultiCommand(dispatcher, []string{"savenotes", "savenot", "addnote"}, userNotesModule.saveNote)
	helpers.MultiCommand(dispatcher, []string{"getnotes", "viewnote", "shownote"}, userNotesModule.getNote)
	helpers.MultiCommand(dispatcher, []string{"listnotes", "allnotes", "usernotes"}, userNotesModule.myNotes)
	helpers.MultiCommand(dispatcher, []string{"removenote", "delnote", "rmnote"}, userNotesModule.deleteNote)

	// Register module as disableable (user notes are typically always enabled, but following pattern)
	HelpModule.AbleMap.Store(userNotesModule.moduleName, true)

	// Add help text
	HelpModule.AltHelpOptions["UserNotes"] = []string{"usernotes", "personalnotes", "mynotes"}
	HelpModule.helpableKb["UserNotes"] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         "Save Note",
				CallbackData: encodeCallbackData("usernotes_help", map[string]string{"cmd": "savenote"}, "usernotes_help.savenote"),
			},
			{
				Text:         "Get Note",
				CallbackData: encodeCallbackData("usernotes_help", map[string]string{"cmd": "getnote"}, "usernotes_help.getnote"),
			},
		},
		{
			{
				Text:         "List Notes",
				CallbackData: encodeCallbackData("usernotes_help", map[string]string{"cmd": "mynotes"}, "usernotes_help.mynotes"),
			},
			{
				Text:         "Delete Note",
				CallbackData: encodeCallbackData("usernotes_help", map[string]string{"cmd": "deletenote"}, "usernotes_help.deletenote"),
			},
		},
	}

	log.Info("[Modules] UserNotes module loaded")
}

// saveNote handles /savenote <note_name> [content] command
// Can save by replying to a message or by providing content directly
func (m moduleStruct) saveNote(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[UserNotes][saveNote] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 2 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("usernotes_save_usage")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	noteName := strings.ToLower(strings.TrimSpace(args[1]))

	// Get content - either from reply or from args
	var content string
	if msg.ReplyToMessage != nil {
		// Get content from replied message
		if msg.ReplyToMessage.Text != "" {
			content = msg.ReplyToMessage.Text
		} else if msg.ReplyToMessage.Caption != "" {
			content = msg.ReplyToMessage.Caption
		} else {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("usernotes_no_text_in_reply")
			_, _ = msg.Reply(b, text, helpers.Shtml())
			return ext.EndGroups
		}
	} else {
		// Get content from command arguments
		if len(args) < 3 {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("usernotes_save_usage")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		content = strings.Join(args[2:], " ")
	}

	// Validate note name
	if noteName == "" || len(noteName) > 100 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("usernotes_invalid_name")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Validate content length (limit to 4000 chars to avoid database issues)
	if len(content) > 4000 {
		content = content[:4000]
	}

	// Save the note
	err := db.AddUserNote(user.Id, noteName, content)
	if err != nil {
		log.Errorf("[UserNotes] Failed to save note: %v", err)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("usernotes_save_error")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("usernotes_save_success", i18n.TranslationParams{
		"name": noteName,
	})
	_, err = msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// getNote handles /getnote <note_name> command
func (m moduleStruct) getNote(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[UserNotes][getNote] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 2 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("usernotes_get_usage")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	noteName := strings.ToLower(strings.TrimSpace(args[1]))

	// Get the note
	note := db.GetUserNote(user.Id, noteName)
	if note == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("usernotes_not_found", i18n.TranslationParams{
			"name": noteName,
		})
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Format and send the note
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	headerTemplate, _ := tr.GetString("usernotes_get_header")
	text := fmt.Sprintf(headerTemplate, note.NoteName, note.Content)

	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// myNotes handles /mynotes command - lists all user notes
func (m moduleStruct) myNotes(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[UserNotes][myNotes] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	// Get all note names
	noteList := db.GetUserNotesList(user.Id)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if len(noteList) == 0 {
		text, _ := tr.GetString("usernotes_no_notes")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Build the list
	var sb strings.Builder
	for _, name := range noteList {
		sb.WriteString(fmt.Sprintf("• <code>%s</code>\n", name))
	}

	text, _ := tr.GetString("usernotes_list", i18n.TranslationParams{
		"count": fmt.Sprintf("%d", len(noteList)),
		"list":  sb.String(),
	})
	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// deleteNote handles /deletenote <note_name> command
func (m moduleStruct) deleteNote(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[UserNotes][deleteNote] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 2 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("usernotes_delete_usage")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	noteName := strings.ToLower(strings.TrimSpace(args[1]))

	// Delete the note
	err := db.DeleteUserNote(user.Id, noteName)
	if err != nil {
		if err.Error() == "record not found" {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("usernotes_not_found", i18n.TranslationParams{
				"name": noteName,
			})
			_, _ = msg.Reply(b, text, helpers.Shtml())
			return ext.EndGroups
		}
		log.Errorf("[UserNotes] Failed to delete note: %v", err)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("usernotes_delete_error")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("usernotes_delete_success", i18n.TranslationParams{
		"name": noteName,
	})
	_, err = msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}
