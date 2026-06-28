package modules

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

func TestAddGetListAndRemoveTextNote(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	addCtx := newModuleMessageContext(bot, chat, admin, "/save rules Be kind to each other")
	if err := notesModule.addNote(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addNote() error = %v, want EndGroups", err)
	}
	if !notes.DoesNoteExists(chat.Id, "rules") {
		t.Fatal("note was not stored")
	}
	if note := notes.GetNote(chat.Id, "rules"); !strings.Contains(note.NoteContent, "Be kind") {
		t.Fatalf("note content = %q, want saved text", note.NoteContent)
	}

	getCtx := newModuleMessageContext(bot, chat, admin, "/get rules")
	if err := notesModule.getNotes(bot, getCtx); err != ext.EndGroups {
		t.Fatalf("getNotes() error = %v, want EndGroups", err)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/notes")
	if err := notesModule.notesList(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("notesList() error = %v, want EndGroups", err)
	}

	rmCtx := newModuleMessageContext(bot, chat, admin, "/clear rules")
	if err := notesModule.rmNote(bot, rmCtx); err != ext.EndGroups {
		t.Fatalf("rmNote() error = %v, want EndGroups", err)
	}
	if notes.DoesNoteExists(chat.Id, "rules") {
		t.Fatal("note still exists after /clear")
	}
}

func TestPrivNoteTogglesNewChatSetting(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	onCtx := newModuleMessageContext(bot, chat, admin, "/privnote on")
	if err := notesModule.privNote(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("privNote on error = %v, want EndGroups", err)
	}
	if !notes.GetNotes(chat.Id).PrivateNotesEnabled() {
		t.Fatal("private notes were not enabled for new chat")
	}

	statusCtx := newModuleMessageContext(bot, chat, admin, "/privnote")
	if err := notesModule.privNote(bot, statusCtx); err != ext.EndGroups {
		t.Fatalf("privNote status error = %v, want EndGroups", err)
	}

	offCtx := newModuleMessageContext(bot, chat, admin, "/privnote off")
	if err := notesModule.privNote(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("privNote off error = %v, want EndGroups", err)
	}
	if notes.GetNotes(chat.Id).PrivateNotesEnabled() {
		t.Fatal("private notes stayed enabled")
	}

	invalidCtx := newModuleMessageContext(bot, chat, admin, "/privnote maybe")
	if err := notesModule.privNote(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("privNote invalid option error = %v, want EndGroups", err)
	}
}

func TestAddExistingNoteUsesOverwriteConfirmation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "rules", "old", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, admin, "/save rules new text")
	if err := notesModule.addNote(bot, ctx); err != ext.EndGroups {
		t.Fatalf("addNote existing error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want overwrite confirmation", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("overwrite confirmation did not include reply_markup")
	}
	if got := notes.GetNote(chat.Id, "rules").NoteContent; got != "old" {
		t.Fatalf("existing note was overwritten before confirmation: %q", got)
	}
}

func TestAddNoteValidationAndPrivateFlagConflict(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	missingKeywordCtx := newModuleMessageContext(bot, chat, admin, "/save")
	if err := notesModule.addNote(bot, missingKeywordCtx); err != ext.EndGroups {
		t.Fatalf("addNote(missing content) error = %v, want EndGroups", err)
	}

	replyMissingKeywordCtx := newModuleMessageContext(bot, chat, admin, "/save")
	replyMissingKeywordCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{Text: "reply content"}
	if err := notesModule.addNote(bot, replyMissingKeywordCtx); err != ext.EndGroups {
		t.Fatalf("addNote(reply missing keyword) error = %v, want EndGroups", err)
	}

	conflictCtx := newModuleMessageContext(bot, chat, admin, "/save conflict visible {private}{noprivate}")
	if err := notesModule.addNote(bot, conflictCtx); err != ext.EndGroups {
		t.Fatalf("addNote(conflict flags) error = %v, want EndGroups", err)
	}
	note := notes.GetNote(chat.Id, "conflict")
	if note.PrivateOnly || note.GroupOnly {
		t.Fatalf("conflicting privacy flags were not normalized: private=%v group=%v", note.PrivateOnly, note.GroupOnly)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want validation replies plus save reply", len(calls))
	}
}

func TestNoteCommandsPropagateGotgbotReplyErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name  string
		text  string
		setup func(t *testing.T)
		run   func(*gotgbot.Bot, *ext.Context) error
		user  gotgbot.User
	}{
		{
			name: "add note validation reply",
			text: "/save",
			run:  notesModule.addNote,
			user: admin,
		},
		{
			name: "add note success reply",
			text: "/save rules Be kind",
			run:  notesModule.addNote,
			user: admin,
		},
		{
			name: "remove note missing keyword reply",
			text: "/clear",
			run:  notesModule.rmNote,
			user: admin,
		},
		{
			name: "remove note missing note reply",
			text: "/clear missing",
			run:  notesModule.rmNote,
			user: admin,
		},
		{
			name: "remove note success reply",
			text: "/clear cleanup",
			setup: func(t *testing.T) {
				t.Helper()
				if err := notes.AddNote(chat.Id, "cleanup", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
					t.Fatalf("AddNote setup error = %v", err)
				}
			},
			run:  notesModule.rmNote,
			user: admin,
		},
		{
			name: "private note reply",
			text: "/privnote maybe",
			run:  notesModule.privNote,
			user: admin,
		},
		{
			name: "notes empty list reply",
			text: "/notes",
			run:  notesModule.notesList,
			user: member,
		},
		{
			name: "notes populated list reply",
			text: "/notes",
			setup: func(t *testing.T) {
				t.Helper()
				if err := notes.AddNote(chat.Id, "listed", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
					t.Fatalf("AddNote setup error = %v", err)
				}
			},
			run:  notesModule.notesList,
			user: member,
		},
		{
			name: "clear all empty reply",
			text: "/clearall",
			run:  notesModule.rmAllNotes,
			user: admin,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors["sendMessage"] = requestErr
			if tt.setup != nil {
				tt.setup(t)
			}
			ctx := newModuleMessageContext(bot, chat, tt.user, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestRmAllNotesConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}
	data := encodeCallbackData("rmAllNotes", map[string]string{"a": "yes"})

	confirmCtx := newModuleMessageContext(bot, chat, admin, "/clearall")
	if err := notesModule.rmAllNotes(bot, confirmCtx); err != ext.EndGroups {
		t.Fatalf("rmAllNotes() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want confirmation reply", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("clear-all confirmation did not include reply_markup")
	}

	callbackCtx := newModuleCallbackContext(bot, chat, admin, data)
	if err := notesModule.notesButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("notesButtonHandler() error = %v, want EndGroups", err)
	}
	if notes := notes.GetNotesList(chat.Id, true); len(notes) != 0 {
		t.Fatalf("notes after clear all = %v, want none", notes)
	}
}

func TestRemoveNoteAndClearAllValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, text := range []string{"/clear", "/clear missing"} {
		ctx := newModuleMessageContext(bot, chat, admin, text)
		if err := notesModule.rmNote(bot, ctx); err != ext.EndGroups {
			t.Fatalf("rmNote(%q) error = %v, want EndGroups", text, err)
		}
	}

	emptyClearAllCtx := newModuleMessageContext(bot, chat, admin, "/clearall")
	if err := notesModule.rmAllNotes(bot, emptyClearAllCtx); err != ext.EndGroups {
		t.Fatalf("rmAllNotes empty error = %v, want EndGroups", err)
	}

	if err := notes.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}
	memberClearAllCtx := newModuleMessageContext(bot, chat, member, "/clearall")
	if err := notesModule.rmAllNotes(bot, memberClearAllCtx); err != ext.EndGroups {
		t.Fatalf("rmAllNotes non-owner error = %v, want EndGroups", err)
	}
	if !notes.DoesNoteExists(chat.Id, "rules") {
		t.Fatal("non-owner clearall removed note")
	}
}

func TestNotesButtonHandlerCancelAndInvalidCallbacks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}

	cancelCtx := newModuleCallbackContext(
		bot,
		chat,
		admin,
		encodeCallbackData("rmAllNotes", map[string]string{"a": "no"}),
	)
	if err := notesModule.notesButtonHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("notesButtonHandler(cancel) error = %v, want EndGroups", err)
	}
	if !notes.DoesNoteExists(chat.Id, "rules") {
		t.Fatal("note was removed by cancel callback")
	}

	invalidCtx := newModuleCallbackContext(bot, chat, admin, "rmAllNotes")
	if err := notesModule.notesButtonHandler(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("notesButtonHandler(invalid) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want cancel and invalid callback answers", len(calls))
	}
}

func TestNotesButtonHandlerSkipsMissingCallbackAndPropagatesRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noCallbackClient := newModuleBotClient()
	noCallbackBot := newModuleTestBot(noCallbackClient)
	noCallbackCtx := newModuleMessageContext(noCallbackBot, chat, admin, "/clearall")
	if err := notesModule.notesButtonHandler(noCallbackBot, noCallbackCtx); err != ext.EndGroups {
		t.Fatalf("notesButtonHandler(no callback) error = %v, want EndGroups", err)
	}
	if calls := noCallbackClient.callsFor("answerCallbackQuery"); len(calls) != 0 {
		t.Fatalf("answerCallbackQuery calls = %d, want none without callback query", len(calls))
	}

	for _, tt := range []struct {
		name   string
		method string
		data   string
	}{
		{
			name:   "edit failure",
			method: "editMessageText",
			data:   encodeCallbackData("rmAllNotes", map[string]string{"a": "no"}),
		},
		{
			name:   "answer failure",
			method: "answerCallbackQuery",
			data:   encodeCallbackData("rmAllNotes", map[string]string{"a": "no"}),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			ctx := newModuleCallbackContext(bot, chat, admin, tt.data)

			err := notesModule.notesButtonHandler(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("notesButtonHandler() error = %v, want request error", err)
			}
		})
	}
}

func TestNoteOverwriteHandlerCancelAndSuccess(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "rules", "old", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}

	cancelToken := "cancel-note-token"
	notesOverwriteMap.Store(cancelToken, overwriteNote{
		overwriteBase: overwriteBase{ChatID: chat.Id, ItemName: "rules", Text: "cancelled", DataType: db.TEXT},
	})
	cancelCtx := newModuleCallbackContext(
		bot,
		chat,
		admin,
		encodeCallbackData("notes.overwrite", map[string]string{"a": "no", "t": cancelToken}),
	)
	if err := notesModule.noteOverWriteHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("noteOverWriteHandler(cancel) error = %v, want EndGroups", err)
	}
	if _, ok := notesOverwriteMap.Load(cancelToken); ok {
		t.Fatal("cancelled overwrite token remained in map")
	}
	if got := notes.GetNote(chat.Id, "rules").NoteContent; got != "old" {
		t.Fatalf("cancel changed note content to %q", got)
	}

	yesToken := "yes-note-token"
	notesOverwriteMap.Store(yesToken, overwriteNote{
		overwriteBase: overwriteBase{ChatID: chat.Id, ItemName: "rules", Text: "new", DataType: db.TEXT},
	})
	yesCtx := newModuleCallbackContext(
		bot,
		chat,
		admin,
		encodeCallbackData("notes.overwrite", map[string]string{"a": "yes", "t": yesToken}),
	)
	if err := notesModule.noteOverWriteHandler(bot, yesCtx); err != ext.EndGroups {
		t.Fatalf("noteOverWriteHandler(yes) error = %v, want EndGroups", err)
	}
	if got := notes.GetNote(chat.Id, "rules").NoteContent; got != "new" {
		t.Fatalf("overwrite content = %q, want new", got)
	}
}

func TestNoteOverwriteHandlerMissingMalformedAndRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "rules", "old", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote setup error = %v", err)
	}

	noCallbackClient := newModuleBotClient()
	noCallbackBot := newModuleTestBot(noCallbackClient)
	noCallbackCtx := newModuleMessageContext(noCallbackBot, chat, admin, "/save rules new")
	if err := notesModule.noteOverWriteHandler(noCallbackBot, noCallbackCtx); err != ext.EndGroups {
		t.Fatalf("noteOverWriteHandler(no callback) error = %v, want EndGroups", err)
	}

	for _, data := range []string{
		"notes.overwrite",
		"notes.overwrite.maybe." + strconv.FormatInt(chat.Id, 10) + "_rules",
		encodeCallbackData("notes.overwrite", map[string]string{"a": "yes", "t": "missing-token"}),
	} {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		ctx := newModuleCallbackContext(bot, chat, admin, data)
		if err := notesModule.noteOverWriteHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("noteOverWriteHandler(%q) error = %v, want EndGroups", data, err)
		}
	}

	t.Run("edit failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.errors["editMessageText"] = requestErr
		token := "edit-failure-note-token"
		notesOverwriteMap.Store(token, overwriteNote{
			overwriteBase: overwriteBase{ChatID: chat.Id, ItemName: "rules", Text: "new", DataType: db.TEXT},
		})
		ctx := newModuleCallbackContext(
			bot,
			chat,
			admin,
			encodeCallbackData("notes.overwrite", map[string]string{"a": "no", "t": token}),
		)

		err := notesModule.noteOverWriteHandler(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("noteOverWriteHandler edit failure error = %v, want request error", err)
		}
	})

	t.Run("answer failure", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		client.errors["answerCallbackQuery"] = requestErr
		token := "answer-failure-note-token"
		notesOverwriteMap.Store(token, overwriteNote{
			overwriteBase: overwriteBase{ChatID: chat.Id, ItemName: "rules", Text: "new", DataType: db.TEXT},
		})
		ctx := newModuleCallbackContext(
			bot,
			chat,
			admin,
			encodeCallbackData("notes.overwrite", map[string]string{"a": "no", "t": token}),
		)

		err := notesModule.noteOverWriteHandler(bot, ctx)
		if !errors.Is(err, requestErr) {
			t.Fatalf("noteOverWriteHandler answer failure error = %v, want request error", err)
		}
	})
}

func TestNotesWatcherSendsMatchingNote(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := notes.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, member, "#rules")
	if err := notesModule.notesWatcher(bot, ctx); err != ext.EndGroups {
		t.Fatalf("notesWatcher() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want note response", len(calls))
	}
	if got := calls[0].Params["text"]; got != "be kind" {
		t.Fatalf("note text = %q, want be kind", got)
	}
}

func TestNotesWatcherPrivateAdminOnlyAndNoFormatBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	if err := notes.AddNote(chat.Id, "secret", "private text", "", nil, db.TEXT, true, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(private) setup error = %v", err)
	}
	privateCtx := newModuleMessageContext(bot, chat, member, "#secret")
	if err := notesModule.notesWatcher(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("notesWatcher private note error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 || calls[0].Params["reply_markup"] == nil {
		t.Fatalf("private note calls = %+v, want click-through button", calls)
	}

	if err := notes.AddNote(chat.Id, "admin", "admin text", "", nil, db.TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote(admin) setup error = %v", err)
	}
	memberAdminCtx := newModuleMessageContext(bot, chat, member, "#admin")
	if err := notesModule.notesWatcher(bot, memberAdminCtx); err != ext.EndGroups {
		t.Fatalf("notesWatcher admin-only member error = %v, want EndGroups", err)
	}

	adminNoFormatCtx := newModuleMessageContext(bot, chat, admin, "#admin noformat")
	if err := notesModule.notesWatcher(bot, adminNoFormatCtx); err != ext.EndGroups {
		t.Fatalf("notesWatcher admin noformat error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want private redirect, admin denial, raw note", len(calls))
	}
	if text := calls[len(calls)-1].Params["text"].(string); !strings.Contains(text, "admin text") {
		t.Fatalf("raw admin note text = %q, want stored note content", text)
	}
}

func TestGetNotesValidationPrivateAndNoFormatBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, text := range []string{"/get", "/get missing"} {
		ctx := newModuleMessageContext(bot, chat, member, text)
		if err := notesModule.getNotes(bot, ctx); err != ext.EndGroups {
			t.Fatalf("getNotes(%q) error = %v, want EndGroups", text, err)
		}
	}

	if err := notes.AddNote(chat.Id, "private", "private text", "", nil, db.TEXT, true, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(private) setup error = %v", err)
	}
	privateCtx := newModuleMessageContext(bot, chat, member, "/get private")
	if err := notesModule.getNotes(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("getNotes private note error = %v, want EndGroups", err)
	}

	if err := notes.AddNote(chat.Id, "raw", "<b>raw</b>", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(raw) setup error = %v", err)
	}
	memberNoFormatCtx := newModuleMessageContext(bot, chat, member, "/get raw noformat")
	if err := notesModule.getNotes(bot, memberNoFormatCtx); err != ext.EndGroups {
		t.Fatalf("getNotes member noformat error = %v, want EndGroups", err)
	}

	adminNoFormatCtx := newModuleMessageContext(bot, chat, admin, "/get raw noformat")
	if err := notesModule.getNotes(bot, adminNoFormatCtx); err != ext.EndGroups {
		t.Fatalf("getNotes admin noformat error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 5 {
		t.Fatalf("sendMessage calls = %d, want validation, private redirect, noformat denial, raw note", len(calls))
	}
	if calls[2].Params["reply_markup"] == nil {
		t.Fatal("private /get did not include click-through button")
	}
}

func TestGetNotesAndWatcherPropagateGotgbotSendErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(rules) setup error = %v", err)
	}
	if err := notes.AddNote(chat.Id, "private", "private text", "", nil, db.TEXT, true, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(private) setup error = %v", err)
	}
	if err := notes.AddNote(chat.Id, "admin", "admin text", "", nil, db.TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote(admin) setup error = %v", err)
	}
	if err := notes.AddNote(chat.Id, "broken", "", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(broken) setup error = %v", err)
	}

	for _, tt := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
		user gotgbot.User
	}{
		{name: "get missing args", text: "/get", run: notesModule.getNotes, user: member},
		{name: "get missing note", text: "/get missing", run: notesModule.getNotes, user: member},
		{name: "get broken note", text: "/get broken", run: notesModule.getNotes, user: member},
		{name: "get admin-only denial", text: "/get admin", run: notesModule.getNotes, user: member},
		{name: "get private redirect", text: "/get private", run: notesModule.getNotes, user: member},
		{name: "get normal note", text: "/get rules", run: notesModule.getNotes, user: member},
		{name: "get raw noformat", text: "/get rules noformat", run: notesModule.getNotes, user: admin},
		{name: "watcher broken note", text: "#broken", run: notesModule.notesWatcher, user: member},
		{name: "watcher admin-only denial", text: "#admin", run: notesModule.notesWatcher, user: member},
		{name: "watcher private redirect", text: "#private", run: notesModule.notesWatcher, user: member},
		{name: "watcher normal note", text: "#rules", run: notesModule.notesWatcher, user: member},
		{name: "watcher raw noformat", text: "#rules noformat", run: notesModule.notesWatcher, user: admin},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors["sendMessage"] = requestErr
			ctx := newModuleMessageContext(bot, chat, tt.user, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestNotesWatcherAdminOnlyAndMalformedNotes(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "admin", "admin-only", "", nil, db.TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote(admin) setup error = %v", err)
	}
	if err := notes.AddNote(chat.Id, "broken", "", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(broken) setup error = %v", err)
	}

	memberCtx := newModuleMessageContext(bot, chat, member, "#admin")
	if err := notesModule.notesWatcher(bot, memberCtx); err != ext.EndGroups {
		t.Fatalf("notesWatcher(admin-only/member) error = %v, want EndGroups", err)
	}

	adminCtx := newModuleMessageContext(bot, chat, admin, "#admin")
	if err := notesModule.notesWatcher(bot, adminCtx); err != ext.EndGroups {
		t.Fatalf("notesWatcher(admin-only/admin) error = %v, want EndGroups", err)
	}

	brokenCtx := newModuleMessageContext(bot, chat, member, "#broken")
	if err := notesModule.notesWatcher(bot, brokenCtx); err != ext.EndGroups {
		t.Fatalf("notesWatcher(broken) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want denial, admin note, and parsing error", len(calls))
	}
}

func TestNotesWatcherPrivateOnlyNoteSendsDeepLinkInGroup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := notes.AddNote(chat.Id, "secret", "private", "", nil, db.TEXT, true, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, member, "#secret")
	if err := notesModule.notesWatcher(bot, ctx); err != ext.EndGroups {
		t.Fatalf("notesWatcher() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want private-note deep link", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("private-note response did not include reply markup")
	}
}

func TestNotesListPrivateAndPrivateNotesButton(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}

	if err := notes.TooglePrivateNote(chat.Id, true); err != nil {
		t.Fatalf("TooglePrivateNote() error = %v", err)
	}
	groupCtx := newModuleMessageContext(bot, chat, admin, "/notes")
	if err := notesModule.notesList(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("notesList(group private enabled) error = %v, want EndGroups", err)
	}

	privateChat := gotgbot.Chat{Id: admin.Id, Type: "private", FirstName: "Telegram"}
	privateCtx := newModuleMessageContext(bot, privateChat, admin, "/notes")
	privateCtx.EffectiveChat = &chat
	if err := notesModule.notesList(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("notesList(private connected chat) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want group button and private note list", len(calls))
	}
}

func TestGetNoteNoFormatRequiresAdminAndSendsRawNote(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := notes.AddNote(chat.Id, "raw", "<b>raw</b>", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, admin, "/get raw noformat")
	if err := notesModule.getNotes(bot, ctx); err != ext.EndGroups {
		t.Fatalf("getNotes(noformat) error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want raw note response", len(calls))
	}
	if got := calls[0].Params["text"]; got == "<b>raw</b>" {
		t.Fatalf("raw note text was not reversed from HTML markdown: %q", got)
	}
}
