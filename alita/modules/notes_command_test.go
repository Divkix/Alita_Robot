package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
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
	if !db.DoesNoteExists(chat.Id, "rules") {
		t.Fatal("note was not stored")
	}
	if note := db.GetNote(chat.Id, "rules"); !strings.Contains(note.NoteContent, "Be kind") {
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
	if db.DoesNoteExists(chat.Id, "rules") {
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
	if !db.GetNotes(chat.Id).PrivateNotesEnabled() {
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
	if db.GetNotes(chat.Id).PrivateNotesEnabled() {
		t.Fatal("private notes stayed enabled")
	}
}

func TestAddExistingNoteUsesOverwriteConfirmation(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddNote(chat.Id, "rules", "old", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
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
	if got := db.GetNote(chat.Id, "rules").NoteContent; got != "old" {
		t.Fatalf("existing note was overwritten before confirmation: %q", got)
	}
}

func TestRmAllNotesConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() setup error = %v", err)
	}
	data := encodeCallbackData("rmAllNotes", map[string]string{"a": "yes"}, "rmAllNotes.yes")

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
	if notes := db.GetNotesList(chat.Id, true); len(notes) != 0 {
		t.Fatalf("notes after clear all = %v, want none", notes)
	}
}

func TestNotesWatcherSendsMatchingNote(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.AddNote(chat.Id, "rules", "be kind", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
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

func TestNotesWatcherPrivateOnlyNoteSendsDeepLinkInGroup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.AddNote(chat.Id, "secret", "private", "", nil, db.TEXT, true, false, false, false, false, false); err != nil {
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

func TestGetNoteNoFormatRequiresAdminAndSendsRawNote(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Notes Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddNote(chat.Id, "raw", "<b>raw</b>", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
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
