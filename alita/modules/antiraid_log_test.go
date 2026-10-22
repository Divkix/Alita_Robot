package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
)

const raidLogChannel = int64(-1009876543213)

// newRaidLogFixture is an admin chat with a log channel; settingsOn keeps the
// settings category on, as a new log channel has it.
func newRaidLogFixture(t *testing.T, settingsOn bool) adminChatFixture {
	t.Helper()
	f := newAdminChatFixture(t)
	if err := logchannels.Set(f.chat.Id, f.chat.Title, raidLogChannel); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	if !settingsOn {
		if err := logchannels.SetCategory(f.chat.Id, logchannels.CategorySettings, false); err != nil {
			t.Fatalf("SetCategory(settings) error = %v", err)
		}
	}
	t.Cleanup(func() { _, _ = antiRaidModule.disableRaid(f.chat.Id) })
	return f
}

func requireRaidLogEntries(t *testing.T, f adminChatFixture, want ...[]string) {
	t.Helper()
	entries := spamDBMessagesTo(f.client, raidLogChannel)
	if len(entries) != len(want) {
		t.Fatalf("log channel entries = %d, want %d: %v", len(entries), len(want), entries)
	}
	for i, parts := range want {
		for _, part := range append([]string{"Admin Fixture Chat", "#ANTIRAID\n", "Admin: ", fmt.Sprint(f.admin.Id)}, parts...) {
			if !strings.Contains(entries[i], part) {
				t.Fatalf("log entry %d %q does not contain %q", i, entries[i], part)
			}
		}
	}
}

// An admin switching raid mode on or off logs under the settings category.
func TestAntiRaidCommandLogsRaidModeChanges(t *testing.T) {
	t.Run("logged", func(t *testing.T) {
		f := newRaidLogFixture(t, true)
		raidTime := formatDuration(antiraid.GetAntiRaidSettings(f.chat.Id).RaidTime)
		for _, text := range []string{"/antiraid on", "/antiraid 45m", "/antiraid off"} {
			_ = antiRaidModule.antiraid(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, text))
		}
		requireRaidLogEntries(t, f,
			[]string{"Raid mode: on", "Duration: " + raidTime},
			[]string{"Raid mode: on", "Duration: 45m"},
			[]string{"Raid mode: off"},
		)
	})

	t.Run("settings category off", func(t *testing.T) {
		f := newRaidLogFixture(t, false)
		for _, text := range []string{"/antiraid on", "/antiraid off"} {
			_ = antiRaidModule.antiraid(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, text))
		}
		requireRaidLogEntries(t, f)
	})

	// Switching off a raid that is not on, or on one that is, changes nothing.
	t.Run("nothing changed", func(t *testing.T) {
		f := newRaidLogFixture(t, true)
		_ = antiRaidModule.antiraid(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/antiraid off"))
		requireRaidLogEntries(t, f)
		_ = antiRaidModule.antiraid(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/antiraid on"))
		_ = antiRaidModule.antiraid(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, "/antiraid on"))
		requireRaidLogEntries(t, f, []string{"Raid mode: on"})
	})
}

func TestAntiRaidButtonsLogRaidModeChanges(t *testing.T) {
	press := func(f adminChatFixture, action string) {
		data := encodeCallbackData("antiraid", map[string]string{"a": action})
		_ = antiRaidModule.callbackHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data))
	}

	t.Run("logged", func(t *testing.T) {
		f := newRaidLogFixture(t, true)
		press(f, "on")
		press(f, "off")
		requireRaidLogEntries(t, f, []string{"Raid mode: on", "Duration: "}, []string{"Raid mode: off"})
	})

	t.Run("settings category off", func(t *testing.T) {
		f := newRaidLogFixture(t, false)
		press(f, "on")
		press(f, "off")
		requireRaidLogEntries(t, f)
	})

	t.Run("nothing changed", func(t *testing.T) {
		f := newRaidLogFixture(t, true)
		press(f, "off")
		requireRaidLogEntries(t, f)
	})
}
