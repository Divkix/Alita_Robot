//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// setPMGroupCandidates makes groups the user's recently active groups, turns
// the picker pause off and returns a counter of candidate scans.
func setPMGroupCandidates(t *testing.T, groups ...models.Chat) *atomic.Int32 {
	t.Helper()
	original, originalCooldown := settingsCandidateChats, settingsPickerCooldown
	var scans atomic.Int32
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) {
		scans.Add(1)
		return groups, nil
	}
	settingsPickerCooldown = 0
	t.Cleanup(func() { settingsCandidateChats, settingsPickerCooldown = original, originalCooldown })
	return &scans
}

// titleFixture is adminChatFixture's group titled title, listed among the
// admin's groups, with every locale loaded so help examples resolve.
func titleFixture(t *testing.T, title string) adminChatFixture {
	t.Helper()
	useRealLocales(t)
	f := newAdminChatFixture(t)
	f.chat.Title = title
	f.addGroup(t, f.chat.Id, title, true)
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: title})
	return f
}

// addGroup makes chatID a supergroup titled title, administered by the
// fixture's admin when admin is true.
func (f adminChatFixture) addGroup(t *testing.T, chatID int64, title string, admin bool) {
	t.Helper()
	if err := chats.EnsureChatInDb(chatID, title); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	f.client.chatsByID[fmt.Sprint(chatID)] = json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":%q}`, chatID, title))
	if chatID == f.chat.Id {
		return
	}
	if admin {
		f.client.adminsByChat[fmt.Sprint(chatID)] = adminListJSON(f.admin.Id)
	} else {
		f.client.adminsByChat[fmt.Sprint(chatID)] = adminListJSON()
	}
}

func addTestNote(t *testing.T, chatID int64, name string) {
	t.Helper()
	if err := notes.AddNote(chatID, name, "note body", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
}

func notesDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadNotes(dispatcher)
	return dispatcher
}

func filtersDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadFilters(dispatcher)
	return dispatcher
}

func connectionsDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadConnections(dispatcher)
	return dispatcher
}

// A quoted title names the group among those the user administers,
// whatever its case.
func TestNotesInPMByQuotedTitle(t *testing.T) {
	f := titleFixture(t, "My Group")
	addTestNote(t, f.chat.Id, "welcome-pack")

	sendPMCommand(t, notesDispatcher(), f.bot, f.client, f.admin, `/notes "my group"`, "welcome-pack")
}

// A one-word title needs no quotes.
func TestFiltersInPMByBareTitle(t *testing.T) {
	f := titleFixture(t, "MyGroup")
	if err := filters.AddFilter(f.chat.Id, "hello", "Hi there", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}

	sendPMCommand(t, filtersDispatcher(), f.bot, f.client, f.admin, "/filters mygroup", "<code>hello</code>")
}

// A title names a group only where the user is an admin: a group they merely
// posted in is not found by its title.
func TestPMGroupTitleIgnoresGroupsWhereUserIsNotAdmin(t *testing.T) {
	f := titleFixture(t, "My Group")
	other := uniqueModuleChatID()
	f.addGroup(t, other, "Other Group", false)
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: "My Group"}, models.Chat{ChatId: other, ChatName: "Other Group"})
	addTestNote(t, other, "secret-note")

	notFound := fmt.Sprintf(translated(t, "connections_pm_group_not_found"), "other group")
	sendPMCommand(t, notesDispatcher(), f.bot, f.client, f.admin, `/notes "other group"`, strings.TrimSpace(notFound))

	if strings.Contains(sentText(f.client), "secret-note") {
		t.Fatalf("listed notes of a group the user does not administer: %s", sentText(f.client))
	}
}

// Two of the user's groups with one title: the bot lists both as text and
// asks for the @username or ID instead of picking one.
func TestPMGroupDuplicateTitlesListsChoices(t *testing.T) {
	f := titleFixture(t, "Chat")
	twin := uniqueModuleChatID()
	f.addGroup(t, twin, "Chat", true)
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: "Chat"}, models.Chat{ChatId: twin, ChatName: "Chat"})
	addTestNote(t, f.chat.Id, "first-note")
	addTestNote(t, twin, "twin-note")

	call := sendPMCommand(t, notesDispatcher(), f.bot, f.client, f.admin, "/notes chat", fmt.Sprint(twin))

	text := fmt.Sprint(call.Params["text"])
	if !strings.Contains(text, fmt.Sprint(f.chat.Id)) {
		t.Fatalf("duplicate list %q lacks group %d", text, f.chat.Id)
	}
	if got := sentText(f.client); strings.Contains(got, "first-note") || strings.Contains(got, "twin-note") {
		t.Fatalf("listed notes for an ambiguous title: %s", got)
	}
	if markup := markupOf(t, call); len(markup.InlineKeyboard) != 0 {
		t.Fatalf("duplicate list has buttons %+v, want text only", markup.InlineKeyboard)
	}
}

// Group titles are user text: the help list and the duplicate list show them
// escaped, and the help list leaves out groups the user does not administer.
func TestPMGroupListsEscapeTitles(t *testing.T) {
	const title = "A & <b>"
	escaped := html.EscapeString(title)
	f := titleFixture(t, title)
	twin, notMine := uniqueModuleChatID(), uniqueModuleChatID()
	f.addGroup(t, twin, title, true)
	f.addGroup(t, notMine, "Not Mine", false)
	setPMGroupCandidates(t,
		models.Chat{ChatId: f.chat.Id, ChatName: title},
		models.Chat{ChatId: twin, ChatName: title},
		models.Chat{ChatId: notMine, ChatName: "Not Mine"},
	)
	dispatcher := notesDispatcher()

	help := fmt.Sprint(sendPMCommand(t, dispatcher, f.bot, f.client, f.admin, "/notes", translated(t, "connections_pm_group_pick")).Params["text"])
	if !strings.Contains(help, escaped) || strings.Contains(help, title) {
		t.Fatalf("help list %q, want the title escaped as %q", help, escaped)
	}
	if strings.Contains(help, "Not Mine") || strings.Contains(help, fmt.Sprint(notMine)) {
		t.Fatalf("help list %q includes a group the user does not administer", help)
	}

	resetCalls(f.client)
	duplicates := fmt.Sprint(sendPMCommand(t, dispatcher, f.bot, f.client, f.admin, `/notes "a & <b>"`, fmt.Sprint(twin)).Params["text"])
	if !strings.Contains(duplicates, escaped) || strings.Contains(duplicates, title) || strings.Contains(duplicates, "a & <b>") {
		t.Fatalf("duplicate list %q, want the title escaped as %q", duplicates, escaped)
	}
}

// assertPMGroupHelp checks a reply to cmd sent from PM without a group: the
// command's help entry, its example and the user's groups.
func assertPMGroupHelp(t *testing.T, text, cmd string, groups ...gotgbot.Chat) {
	t.Helper()
	block := commandHelpBlock(translated(t, commandHelpModules[cmd]+"_help_msg"), cmd)
	if block == "" {
		t.Fatalf("en.yml has no help entry for /%s", cmd)
	}
	if !strings.Contains(text, formatting.ToTelegramHTML(block)) {
		t.Fatalf("reply %q lacks the /%s help entry %q", text, cmd, block)
	}
	if example := commandExample(cmd); example == "" || !strings.Contains(text, html.EscapeString(example)) {
		t.Fatalf("reply %q lacks the /%s example %q", text, cmd, example)
	}
	for _, group := range groups {
		if !strings.Contains(text, html.EscapeString(group.Title)) || !strings.Contains(text, fmt.Sprint(group.Id)) {
			t.Fatalf("reply %q does not list group %q (%d)", text, group.Title, group.Id)
		}
	}
}

// Without a group or a connection the bot answers with the command's help,
// an example with a group and the groups the user administers.
func TestPMGroupMissingReplyListsGroups(t *testing.T) {
	f := titleFixture(t, "My Group")

	call := sendPMCommand(t, notesDispatcher(), f.bot, f.client, f.admin, "/notes", translated(t, "connections_pm_group_pick"))

	assertPMGroupHelp(t, fmt.Sprint(call.Params["text"]), "notes", f.chat)
	if markup := markupOf(t, call); len(markup.InlineKeyboard) != 0 {
		t.Fatalf("help has buttons %+v, want text only", markup.InlineKeyboard)
	}
}

func TestPMGroupMissingReplyWithoutGroupsSaysPickerEmpty(t *testing.T) {
	f := titleFixture(t, "My Group")
	setPMGroupCandidates(t)

	call := sendPMCommand(t, filtersDispatcher(), f.bot, f.client, f.admin, "/filters", translated(t, "settings_picker_empty"))

	assertPMGroupHelp(t, fmt.Sprint(call.Params["text"]), "filters")
}

// /rules without a group answers like the other commands, not with the bare
// request for a group.
func TestRulesInPMWithoutGroupRepliesHelp(t *testing.T) {
	f := titleFixture(t, "My Group")

	call := sendPMCommand(t, rulesDispatcher(), f.bot, f.client, f.admin, "/rules", translated(t, "connections_pm_group_pick"))

	assertPMGroupHelp(t, fmt.Sprint(call.Params["text"]), "rules", f.chat)
}

// /connect without an argument in PM shows its help and the user's groups
// with their IDs, which /connect takes.
func TestConnectWithoutArgInPMRepliesHelp(t *testing.T) {
	f := titleFixture(t, "My Group")

	call := sendPMCommand(t, connectionsDispatcher(), f.bot, f.client, f.admin, "/connect", translated(t, "extraction_need_chat_id"))

	assertPMGroupHelp(t, fmt.Sprint(call.Params["text"]), "connect", f.chat)
}

// A second lookup inside the picker pause reuses the list the first one
// built instead of scanning the user's groups again.
func TestPMGroupTitleResolutionRespectsPickerPause(t *testing.T) {
	f := titleFixture(t, "My Group")
	scans := setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: "My Group"})
	settingsPickerCooldown = 5 * time.Second
	addTestNote(t, f.chat.Id, "pause-note")
	dispatcher := notesDispatcher()

	sendPMCommand(t, dispatcher, f.bot, f.client, f.admin, `/notes "my group"`, "pause-note")
	resetCalls(f.client)
	sendPMCommand(t, dispatcher, f.bot, f.client, f.admin, `/notes "my group"`, "pause-note")

	if n := scans.Load(); n != 1 {
		t.Fatalf("candidate scans = %d, want 1: the second lookup ignored the pause", n)
	}
}

// Inside a pause with no list kept, the bot asks the user to wait instead of
// scanning again.
func TestPMGroupTitleDuringPauseWithoutListWaits(t *testing.T) {
	f := titleFixture(t, "My Group")
	scans := setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: "My Group"})
	settingsPickerCooldown = 5 * time.Second
	if _, wait := settingsPickerWait(i18n.MustNewTranslator("en"), f.admin.Id); wait {
		t.Fatal("settingsPickerWait() paused a fresh user")
	}

	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 101, Date: 1, Chat: privateChatOf(f.admin), From: &f.admin, Text: `/notes "my group"`,
	}}
	if err := notesDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}

	assertWaitNotice(t, sentText(f.client))
	if n := scans.Load(); n != 0 {
		t.Fatalf("candidate scans during the pause = %d, want 0", n)
	}
}

// Without a connection a command with arguments of its own still takes the
// group first, by title too.
func TestPMGroupArgOwnArgsTakesTitleWithoutConnection(t *testing.T) {
	f := titleFixture(t, "My Group")
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, `/cmd "My Group" on`)
	var args []string
	var group *gotgbot.Chat
	handler := withPMGroupArg("cmd", pmArgsOwn, func(_ *gotgbot.Bot, ctx *ext.Context) error {
		args, group = ctx.Args(), chat_status.OneOffGroup(ctx)
		return ext.EndGroups
	})

	if err := handler(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("handler error = %v, want ext.EndGroups", err)
	}

	if fmt.Sprint(args) != "[/cmd on]" || group == nil || group.Id != f.chat.Id {
		t.Fatalf("handler saw args %q group %+v, want [/cmd on] and group %d", args, group, f.chat.Id)
	}
}

// With a connection a command with arguments of its own keeps a word that is
// not an @username or ID and no group's title: it is the command's argument.
func TestPMGroupArgOwnArgsKeepsTitleWithConnection(t *testing.T) {
	f := titleFixture(t, "My Group")
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: "My Group"})
	f.connectAdmin(t)
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, "/cmd mygroup on")
	var args []string
	handler := withPMGroupArg("cmd", pmArgsOwn, func(_ *gotgbot.Bot, ctx *ext.Context) error {
		args = ctx.Args()
		return ext.EndGroups
	})

	if err := handler(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("handler error = %v, want ext.EndGroups", err)
	}

	if fmt.Sprint(args) != "[/cmd mygroup on]" {
		t.Fatalf("handler saw args %q, want [/cmd mygroup on]", args)
	}
}
