//go:build testtools

package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	db_filters "github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/notes"
)

// Commands that save content take it from the typed text or the replied-to
// message. In a forum topic Telegram attaches the topic's creation message as
// the reply; it is not a content source.

func TestSaveNoteInTopicTakesTypedText(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	f.chat.IsForum = true

	ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, "/save rules Be kind"), nil)
	if err := notesModule.addNote(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("addNote() error = %v, want EndGroups", err)
	}
	if note := notes.GetNote(f.chat.Id, "rules"); note == nil || note.NoteContent != "Be kind" {
		t.Fatalf("note = %+v, want the typed text", note)
	}
}

func TestSaveNoteInTopicWithoutContentAsksForIt(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	f.chat.IsForum = true

	ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, "/save rules"), nil)
	if err := notesModule.addNote(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("addNote() error = %v, want EndGroups", err)
	}
	if notes.DoesNoteExists(f.chat.Id, "rules") {
		t.Fatal("note stored without content")
	}
	if got, want := lastMessageTo(t, f.client, f.chat.Id), translated(t, "notes_invalid"); got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}
}

func TestSaveNoteFromOrdinaryReplyInTopic(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	f.chat.IsForum = true

	ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, "/save rules"), &gotgbot.Message{
		MessageId: 50, Date: 1, Chat: f.chat, From: &f.member, MessageThreadId: forumTopicID, IsTopicMessage: true,
		Text: "No spam please",
	})
	if err := notesModule.addNote(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("addNote() error = %v, want EndGroups", err)
	}
	if note := notes.GetNote(f.chat.Id, "rules"); note == nil || !strings.Contains(note.NoteContent, "No spam please") {
		t.Fatalf("note = %+v, want the replied message's text", note)
	}
}

func TestAddFilterInTopicTakesTypedText(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	f.chat.IsForum = true

	ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, "/filter hello Hi there"), nil)
	if err := filtersModule.addFilter(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("addFilter() error = %v, want EndGroups", err)
	}
	stored, err := db_filters.GetChatFiltersCached(f.chat.Id)
	if err != nil || len(stored) != 1 || stored[0].KeyWord != "hello" || stored[0].FilterReply != "Hi there" {
		t.Fatalf("filters = %+v (err %v), want hello with the typed text", stored, err)
	}
}

func TestAddFilterInTopicWithoutContentAsksForIt(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	f.chat.IsForum = true

	ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, "/filter hello"), nil)
	if err := filtersModule.addFilter(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("addFilter() error = %v, want EndGroups", err)
	}
	if db_filters.DoesFilterExists(f.chat.Id, "hello") {
		t.Fatal("filter stored without content")
	}
	if got, want := lastMessageTo(t, f.client, f.chat.Id), translated(t, "filters_invalid"); got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}
}
