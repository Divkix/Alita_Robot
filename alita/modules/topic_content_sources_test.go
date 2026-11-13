//go:build testtools

package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	db_filters "github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
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

func TestSetGreetingInTopic(t *testing.T) {
	for _, tt := range []struct {
		command  string
		run      func(*gotgbot.Bot, *ext.Context) error
		stored   func(chatID int64) string
		needText string
	}{
		{
			command:  "/setwelcome",
			run:      greetingsModule.setWelcome,
			stored:   func(id int64) string { return greetings.GetGreetingSettings(id).WelcomeSettings.WelcomeText },
			needText: "content_need_welcome_content",
		},
		{
			command:  "/setgoodbye",
			run:      greetingsModule.setGoodbye,
			stored:   func(id int64) string { return greetings.GetGreetingSettings(id).GoodbyeSettings.GoodbyeText },
			needText: "content_need_goodbye_content",
		},
	} {
		t.Run(tt.command+" takes typed text", func(t *testing.T) {
			useRealLocales(t)
			f := newAdminChatFixture(t)
			f.chat.IsForum = true
			ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, tt.command+" Hello {first}"), nil)
			if err := tt.run(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.command, err)
			}
			if got := tt.stored(f.chat.Id); got != "Hello {first}" {
				t.Fatalf("stored text = %q, want the typed text", got)
			}
		})
		t.Run(tt.command+" without content asks for it", func(t *testing.T) {
			useRealLocales(t)
			f := newAdminChatFixture(t)
			f.chat.IsForum = true
			before := tt.stored(f.chat.Id)
			ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, tt.command), nil)
			if err := tt.run(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.command, err)
			}
			if got := tt.stored(f.chat.Id); got != before {
				t.Fatalf("stored text = %q, want it unchanged (%q)", got, before)
			}
			if got, want := lastMessageTo(t, f.client, f.chat.Id), translated(t, tt.needText); got != want {
				t.Fatalf("reply = %q, want %q", got, want)
			}
		})
	}
}
