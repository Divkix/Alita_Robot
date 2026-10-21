//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/rules"
)

// connectToOtherGroup lists the fixture's group among the admin's groups by
// its title and connects the admin to another group they administer, which
// it returns.
func connectToOtherGroup(t *testing.T, f adminChatFixture) int64 {
	t.Helper()
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: f.chat.Title})
	other := uniqueModuleChatID()
	if err := chats.EnsureChatInDb(other, "Group B"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	f.client.chatsByID[fmt.Sprint(other)] = []byte(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":"Group B"}`, other))
	f.client.adminsByChat[fmt.Sprint(other)] = adminListJSON(f.admin.Id)
	if err := connections.ConnectId(f.admin.Id, other); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.admin.Id) })
	return other
}

// requireTitleRefused fails unless the bot answered that title names one of
// the admin's groups, and the admin is still connected to connected.
func requireTitleRefused(t *testing.T, f adminChatFixture, title string, connected int64) {
	t.Helper()
	want := fmt.Sprintf(translated(t, "connections_pm_group_title_on_connection"), title, connected)
	if sent := sentText(f.client); !strings.Contains(sent, want) || !strings.Contains(sent, fmt.Sprint(f.chat.Id)) {
		t.Fatalf("reply does not refuse the title %q and list group %d; sent:\n%s", title, f.chat.Id, sent)
	}
	if conn := connections.Connection(f.admin.Id); !conn.Connected || conn.ChatId != connected {
		t.Fatalf("connection = %+v, want it kept on %d", conn, connected)
	}
}

// With a connection, a command with arguments of its own does not run when
// its first argument is the title of one of the user's groups: it would act on
// the connected group with the title as its argument.
func TestNologInPMOnConnectionRefusesGroupTitle(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.bindChannel(t)
	other := connectToOtherGroup(t, f.adminChatFixture)
	if err := logchannels.Set(other, "Group B", f.channel.Id); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}

	f.sendPM(t, f.admin, fmt.Sprintf("/nolog %q admin", f.chat.Title))

	for _, id := range []int64{f.chat.Id, other} {
		if got := logchannels.Get(id); got == nil || !got.CatAdmin {
			t.Fatalf("group %d after /nolog by title: %+v, want admin category still on", id, got)
		}
	}
	requireTitleRefused(t, f.adminChatFixture, f.chat.Title, other)
}

func TestSetRulesInPMOnConnectionRefusesGroupTitle(t *testing.T) {
	f := oneOffFixture(t)
	other := connectToOtherGroup(t, f)
	if err := rules.SetChatRules(other, "Rules of the connected group"); err != nil {
		t.Fatalf("SetChatRules() error = %v", err)
	}
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, fmt.Sprintf("/setrules %q Be nice", f.chat.Title))

	_ = withPMGroupArg("setrules", pmArgsOwn, rulesModule.setRules)(f.bot, ctx)

	if got := rules.GetChatRulesInfo(f.chat.Id).Rules; got != "Be kind to each other" {
		t.Fatalf("named group rules = %q, want them unchanged", got)
	}
	if got := rules.GetChatRulesInfo(other).Rules; got != "Rules of the connected group" {
		t.Fatalf("connected group rules = %q, want them unchanged", got)
	}
	requireTitleRefused(t, f, f.chat.Title, other)
}

// A quoted argument that is no group's title stays the command's own.
func TestFilterInPMOnConnectionKeepsQuotedKeyword(t *testing.T) {
	f := oneOffFixture(t)
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: f.chat.Title})
	f.connectAdmin(t)
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, `/filter "hello world" hi there`)

	_ = withPMGroupArg("filter", pmArgsOwn, filtersModule.addFilter)(f.bot, ctx)

	if !filters.DoesFilterExists(f.chat.Id, "hello world") {
		t.Fatalf("filter \"hello world\" not saved in the connected group; sent:\n%s", sentText(f.client))
	}
}
