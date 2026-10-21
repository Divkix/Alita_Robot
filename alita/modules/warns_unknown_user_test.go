package modules

import (
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/warns"
)

// warnCaller gives the fixture's plain member one warn with a recognisable reason.
// It returns how many messages were sent so far, to be skipped by sentTexts.
func warnCaller(t *testing.T, f adminChatFixture, reason string) int {
	t.Helper()
	ctx := newWarnReplyContext(f.bot, f.chat, f.admin, f.member, "/warn "+reason)
	if err := warnsModule.warnUser(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("warnUser() error = %v, want EndGroups", err)
	}
	if n, _ := warns.GetWarns(f.member.Id, f.chat.Id); n != 1 {
		t.Fatalf("caller warns = %d, want 1", n)
	}
	return len(f.client.callsFor("sendMessage"))
}

func warnsRepliesAfter(f adminChatFixture, skip int) []string {
	var texts []string
	for _, c := range f.client.callsFor("sendMessage")[skip:] {
		texts = append(texts, c.Params["text"].(string))
	}
	return texts
}

func TestWarnsInGroupWithUnknownUsernameRepliesNotFoundOnly(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	skip := warnCaller(t, f, "callers-own-reason")

	// Telegram answers getChat for an unknown @username with an error.
	f.client.errors["getChat"] = errors.New("Bad Request: chat not found")

	ctx := newModuleMessageContext(f.bot, f.chat, f.member, "/warns @nobody")
	if err := warnsModule.warns(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}

	texts := warnsRepliesAfter(f, skip)
	if len(texts) != 1 {
		t.Fatalf("replies = %q, want exactly the not-found reply", texts)
	}
	if want := translated(t, "extraction_user_not_found"); texts[0] != want {
		t.Fatalf("reply = %q, want %q", texts[0], want)
	}
	for _, text := range texts {
		if strings.Contains(text, "callers-own-reason") {
			t.Fatalf("reply %q shows the caller's own warns", text)
		}
	}
}

func TestWarnsInGroupWithoutArgumentAsksForUser(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	skip := warnCaller(t, f, "callers-own-reason")

	ctx := newModuleMessageContext(f.bot, f.chat, f.member, "/warns")
	if err := warnsModule.warns(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}

	texts := warnsRepliesAfter(f, skip)
	if len(texts) != 1 {
		t.Fatalf("replies = %q, want exactly one", texts)
	}
	if want := translated(t, "common_no_user_specified"); texts[0] != want {
		t.Fatalf("reply = %q, want %q", texts[0], want)
	}
}

func TestWarnsInGroupAsReplyListsTheRepliedUsersWarns(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	skip := warnCaller(t, f, "callers-own-reason")

	ctx := newWarnReplyContext(f.bot, f.chat, f.admin, f.member, "/warns")
	if err := warnsModule.warns(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("warns() error = %v, want EndGroups", err)
	}

	texts := warnsRepliesAfter(f, skip)
	if len(texts) != 1 || !strings.Contains(texts[0], "callers-own-reason") {
		t.Fatalf("replies = %q, want one listing the replied user's warn", texts)
	}
}
