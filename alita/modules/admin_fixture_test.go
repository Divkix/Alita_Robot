package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

// The fixture's admin must be authorised by the admin-cache load, not by the
// service-account short-circuit that 777000 and 1087968824 take.
func TestAdminChatFixtureAuthorisesThroughAdminCache(t *testing.T) {
	f := newAdminChatFixture(t)

	if _, known := chat_status.IsUserAdminCached(f.chat.Id, 777000); !known {
		t.Fatal("777000 is not answered without a lookup; the guard below proves nothing")
	}
	if _, known := chat_status.IsUserAdminCached(f.chat.Id, f.admin.Id); known {
		t.Fatalf("admin %d answered before any admin-cache load; it is a hardcoded admin", f.admin.Id)
	}

	if !chat_status.IsUserAdmin(f.bot, f.chat.Id, f.admin.Id) {
		t.Fatalf("IsUserAdmin(%d) = false, want true from the admin list", f.admin.Id)
	}
	f.client.onlyCall(t, "getChatAdministrators")
	if chat_status.IsUserAdmin(f.bot, f.chat.Id, f.member.Id) {
		t.Fatalf("IsUserAdmin(%d) = true, want false for a member", f.member.Id)
	}
}

func TestAdminChatFixtureMemberCannotModerate(t *testing.T) {
	f := newAdminChatFixture(t)
	other := f.member
	other.Id = uniquePositiveUserID()

	// The permission checks live in banDesc, run before the handler as
	// WrapCommand does.
	ban := anonPipelineHandler(banDesc, bansModule.ban)
	ctx := newBanReplyContext(f.bot, f.chat, f.member, other, "/ban")
	if err := ban(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban(by member) error = %v, want EndGroups", err)
	}
	if calls := f.client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("member's /ban sent %d banChatMember calls, want none", len(calls))
	}
	f.client.onlyCall(t, "getChatAdministrators")

	ctx = newBanReplyContext(f.bot, f.chat, f.admin, other, "/ban")
	if err := ban(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban(by admin) error = %v, want EndGroups", err)
	}
	assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, other.Id))
}
