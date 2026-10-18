//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// Handlers that build their translator after pointing ctx.EffectiveChat at the
// connected group still reply to the admin's PM message, so they must answer
// in the admin's language. So must the refusals PermissionResponder sends
// without a translator of its own, and the replies of helpers such as
// extraction.ExtractDuration that read the language from ctx.
func TestConnectedCommandsFromPMBuildRepliesInUserLanguage(t *testing.T) {
	withMiniredis(t)
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	adminNoInfo := fmt.Sprintf(`{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"},"can_change_info":false}`, admin.Id)
	client.adminsByChat[fmt.Sprint(groupA)] = json.RawMessage(`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_change_info":true,"can_restrict_members":true,"can_delete_messages":true,"can_pin_messages":true},` + adminNoInfo + `]`)
	client.membersByUser = map[string]json.RawMessage{fmt.Sprint(admin.Id): json.RawMessage(adminNoInfo)}

	cases := []struct{ text, key string }{
		{"/setflood", "antiflood_errors_expected_args"},
		{"/disabled", "disabling_no_disabled_commands"},
		{"/filters", "filters_none_in_chat"},
		{"/locktypes", "locks_locktypes_header"},
		{"/locks", "locks_current_locks_header"},
		{"/notes", "notes_none_in_chat"},
		{"/antichannelpin maybe", "pins_input_not_recognized"},
		{"/reports block", "reports_group_only"},
		{"/setwarnlimit", "warns_limit_set_help"},
		{"/resetwelcome", "chat_status_change_info_cmd_error"},
		{"/setwarnmode tban 1h 2h", "extraction_invalid_time_format"},
	}
	locales := map[string]string{}
	for _, code := range []string{"en", "ru"} {
		var yaml strings.Builder
		for _, tc := range cases {
			fmt.Fprintf(&yaml, "%s: \"%s:%s %%s\"\n", tc.key, code, tc.key)
		}
		locales[code] = yaml.String()
	}
	restoreI18n, err := i18n.OverrideLocalesForTest(locales)
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restoreI18n)
	if err := lang.ChangeUserLanguage(admin.Id, "ru"); err != nil {
		t.Fatalf("ChangeUserLanguage() error = %v", err)
	}
	t.Cleanup(func() {
		if err := lang.ChangeUserLanguage(admin.Id, "en"); err != nil {
			t.Errorf("ChangeUserLanguage() cleanup error = %v", err)
		}
	})
	if got := groupLanguageOf(t, groupA); got == "ru" {
		t.Fatalf("group language = %q, want it to differ from the admin's", got)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAntiflood(dispatcher)
	LoadDisabling(dispatcher)
	LoadFilters(dispatcher)
	LoadLocks(dispatcher)
	LoadNotes(dispatcher)
	LoadPin(dispatcher)
	LoadReports(dispatcher)
	LoadWarns(dispatcher)
	LoadGreetings(dispatcher)

	for _, tc := range cases {
		resetCalls(client)
		update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{MessageId: 101, Date: 1, Chat: privateChatOf(admin), From: &admin, Text: tc.text}}
		if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
			t.Fatalf("ProcessUpdate(%q) error = %v", tc.text, err)
		}
		sent := client.callsFor("sendMessage")
		if len(sent) == 0 {
			t.Errorf("%s from PM sent no reply", tc.text)
			continue
		}
		last := sent[len(sent)-1]
		if got := fmt.Sprint(last.Params["chat_id"]); got != fmt.Sprint(admin.Id) {
			t.Errorf("%s from PM replied in chat %s, want the PM", tc.text, got)
		}
		if got, want := fmt.Sprint(last.Params["text"]), "ru:"+tc.key; !strings.HasPrefix(got, want) {
			t.Errorf("%s from PM replied %q, want the admin's language (%s...)", tc.text, got, want)
		}
	}
}

// /lang group in PM replies in the admin's language but names the language of
// the connected group, not the admin's own.
func TestLangGroupFromPMNamesTheGroupLanguage(t *testing.T) {
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	restoreI18n, err := i18n.OverrideLocalesForTest(map[string]string{
		"en": "language_name: EnName\nlanguage_current_group: \"en:language_current_group %s\"\n",
		"ru": "language_name: RuName\nlanguage_current_group: \"ru:language_current_group %s\"\n",
	})
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restoreI18n)
	if err := lang.ChangeUserLanguage(admin.Id, "ru"); err != nil {
		t.Fatalf("ChangeUserLanguage() error = %v", err)
	}
	t.Cleanup(func() {
		if err := lang.ChangeUserLanguage(admin.Id, "en"); err != nil {
			t.Errorf("ChangeUserLanguage() cleanup error = %v", err)
		}
	})
	if err := lang.ChangeGroupLanguage(groupA, "en"); err != nil {
		t.Fatalf("ChangeGroupLanguage() error = %v", err)
	}

	runInPM(t, bot, admin, "/lang group", languagesModule.changeLanguage)
	sent := client.callsFor("sendMessage")
	if len(sent) == 0 {
		t.Fatal("/lang group from PM sent no reply")
	}
	if got, want := fmt.Sprint(sent[len(sent)-1].Params["text"]), "ru:language_current_group EnName"; !strings.HasPrefix(got, want) {
		t.Errorf("/lang group from PM replied %q, want %q...", got, want)
	}
}

// /locks in the group itself answers in the group's language, whatever
// language the admin who sent it has set for their PM.
func TestLocksInGroupRepliesInGroupLanguage(t *testing.T) {
	withMiniredis(t)
	client, bot, groupA, admin, _ := connectedPMFixture(t)
	restoreI18n, err := i18n.OverrideLocalesForTest(map[string]string{
		"en": "locks_current_locks_header: \"en:locks_current_locks_header\"\n",
		"ru": "locks_current_locks_header: \"ru:locks_current_locks_header\"\n",
	})
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restoreI18n)
	if err := lang.ChangeGroupLanguage(groupA, "ru"); err != nil {
		t.Fatalf("ChangeGroupLanguage() error = %v", err)
	}
	if err := lang.ChangeUserLanguage(admin.Id, "en"); err != nil {
		t.Fatalf("ChangeUserLanguage() error = %v", err)
	}

	group := gotgbot.Chat{Id: groupA, Type: "supergroup", Title: "Group A"}
	ctx := newModuleMessageContext(bot, group, admin, "/locks")
	if err := locksModule.locks(bot, ctx); err != nil && err != ext.EndGroups {
		t.Fatalf("/locks in group error = %v", err)
	}
	sent := client.callsFor("sendMessage")
	if len(sent) == 0 {
		t.Fatal("/locks in group sent no reply")
	}
	last := sent[len(sent)-1]
	if got := fmt.Sprint(last.Params["chat_id"]); got != fmt.Sprint(groupA) {
		t.Errorf("/locks in group replied in chat %s, want the group", got)
	}
	if got, want := fmt.Sprint(last.Params["text"]), "ru:locks_current_locks_header"; !strings.HasPrefix(got, want) {
		t.Errorf("/locks in group replied %q, want the group's language (%s...)", got, want)
	}
}
