//go:build testtools

package backup

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestImportFloodAndBlacklistActionTime(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	for _, module := range []string{BackupModuleAntiflood, BackupModuleAdmin, BackupModuleBlacklists} {
		t.Run(module, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				action     string
				actionTime any
				want       int
			}{
				{name: "valid", action: "tmute", actionTime: 600, want: 600},
				{name: "shortest", action: "tban", actionTime: 60, want: 60},
				{name: "longest", action: "tban", actionTime: 366 * 86400, want: 366 * 86400},
				{name: "too short", action: "tmute", actionTime: 59},
				{name: "too long", action: "tban", actionTime: 366*86400 + 1},
				{name: "beyond int32", action: "tban", actionTime: 1e12},
				{name: "negative", action: "tmute", actionTime: -1},
				{name: "unset", action: "tban", actionTime: 0},
				{name: "missing", action: "tmute"},
				{name: "untimed", action: "ban", actionTime: 600},
			} {
				t.Run(tc.name, func(t *testing.T) {
					chatID := time.Now().UnixNano()
					require.NoError(t, chats.EnsureChatInDb(chatID, "import_action_time"))
					t.Cleanup(func() { cleanupBackupChat(t, chatID) })
					require.NoError(t, antiflood.SetFloodMode(chatID, "tban", 7200))
					require.NoError(t, blacklists.AddBlacklist(chatID, "spam"))
					require.NoError(t, blacklists.SetBlacklistAction(chatID, "tban", 7200))
					_ = antiflood.GetFlood(chatID)
					_ = blacklists.GetBlacklistSettings(chatID)

					settings := map[string]any{"action": tc.action, "limit": 5}
					entry := map[string]any{"word": "spam", "action": tc.action}
					if tc.actionTime != nil {
						settings["flood_action_time"] = tc.actionTime
						entry["action_time"] = tc.actionTime
					}
					var payload map[string]any
					switch module {
					case BackupModuleAntiflood:
						payload = map[string]any{"settings": settings}
					case BackupModuleAdmin:
						payload = map[string]any{"antiflood_settings": settings}
					case BackupModuleBlacklists:
						payload = map[string]any{"entries": []any{entry}}
					}
					backup := NewBackupFormat(chatID, "Test", 1, []string{module})
					backup.Data[module] = payload
					require.NoError(t, ImportChatData(chatID, backup, nil))
					if module == BackupModuleBlacklists {
						word := blacklists.GetBlacklistSettings(chatID).Find("spam")
						require.NotNil(t, word)
						assert.Equal(t, tc.action, word.Action)
						assert.Equal(t, tc.want, word.ActionTime)
					} else {
						flood := antiflood.GetFlood(chatID)
						assert.Equal(t, tc.action, flood.Action)
						assert.Equal(t, tc.want, flood.FloodActionTime)
					}
				})
			}
		})
	}
}

func TestLegacyTimedActionImportsClearDuration(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	for _, module := range []string{BackupModuleAdmin, BackupModuleAntiflood, BackupModuleBlacklists} {
		t.Run(module, func(t *testing.T) {
			chatID := time.Now().UnixNano()
			require.NoError(t, chats.EnsureChatInDb(chatID, "legacy_action_time"))
			t.Cleanup(func() { cleanupBackupChat(t, chatID) })
			require.NoError(t, antiflood.SetFloodMode(chatID, "tmute", 7200))
			require.NoError(t, blacklists.AddBlacklist(chatID, "spam"))
			require.NoError(t, blacklists.SetBlacklistAction(chatID, "tmute", 7200))
			_ = antiflood.GetFlood(chatID)
			_ = blacklists.GetBlacklistSettings(chatID)
			payloads := map[string]string{
				BackupModuleAdmin:      `{"antiflood_settings":{"limit":5,"action":"tban"},"blacklist_mode":"tban"}`,
				BackupModuleAntiflood:  `{"settings":{"limit":5,"action":"tban"}}`,
				BackupModuleBlacklists: `{"settings":{"word":"spam"},"blacklist_mode":"tban"}`,
			}
			raw := fmt.Sprintf(`{"version":"1.0","bot_name":"AlitaRobot","chat_id":%d,"modules":[%q],"data":{%q:%s}}`,
				chatID, module, module, payloads[module])
			legacy, err := BackupFormatFromJSON([]byte(raw))
			require.NoError(t, err)
			require.NoError(t, ImportChatData(chatID, legacy, nil))
			if module != BackupModuleBlacklists {
				flood := antiflood.GetFlood(chatID)
				assert.Equal(t, "tban", flood.Action)
				assert.Zero(t, flood.FloodActionTime)
			}
			if module != BackupModuleAntiflood {
				word := blacklists.GetBlacklistSettings(chatID).Find("spam")
				require.NotNil(t, word)
				assert.Equal(t, "tban", word.Action)
				assert.Zero(t, word.ActionTime)
			}
		})
	}
}

func TestImportAdminPreservesMatchingBlacklistDuration(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	for _, tc := range []struct {
		name    string
		modules []string
	}{
		{name: "admin only", modules: []string{BackupModuleAdmin}},
		{name: "blacklists before admin", modules: []string{BackupModuleBlacklists, BackupModuleAdmin}},
		{name: "admin before blacklists", modules: []string{BackupModuleAdmin, BackupModuleBlacklists}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chatID := time.Now().UnixNano()
			require.NoError(t, chats.EnsureChatInDb(chatID, "preserve_action_time"))
			t.Cleanup(func() { cleanupBackupChat(t, chatID) })
			require.NoError(t, blacklists.AddBlacklistBulk(chatID, []string{"spam", "scam"}))
			require.NoError(t, blacklists.SetBlacklistAction(chatID, "tban", 86400))

			backup := NewBackupFormat(chatID, "Test", 1, tc.modules)
			for _, module := range tc.modules {
				data, err := ExportModuleData(chatID, module, 1)
				require.NoError(t, err)
				backup.Data[module] = data
			}
			raw, err := backup.ToJSON()
			require.NoError(t, err)
			decoded, err := BackupFormatFromJSON(raw)
			require.NoError(t, err)
			if len(tc.modules) > 1 {
				require.NoError(t, blacklists.SetBlacklistAction(chatID, "tmute", 7200))
			}
			_ = blacklists.GetBlacklistSettings(chatID)
			require.NoError(t, ImportChatData(chatID, decoded, nil))

			entries := blacklists.GetBlacklistSettings(chatID)
			require.Len(t, entries, 2)
			for _, entry := range entries {
				assert.Equal(t, "tban", entry.Action)
				assert.Equal(t, 86400, entry.ActionTime)
			}
		})
	}
}

func TestClearAdminResetsBlacklistDuration(t *testing.T) {
	skipIfNoDb(t)
	utilsCache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "clear_action_time"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })
	require.NoError(t, blacklists.AddBlacklist(chatID, "spam"))
	require.NoError(t, blacklists.SetBlacklistAction(chatID, "tban", 7200))
	_ = blacklists.GetBlacklistSettings(chatID)
	require.NoError(t, ClearChatData(chatID, []string{BackupModuleAdmin}))
	word := blacklists.GetBlacklistSettings(chatID).Find("spam")
	require.NotNil(t, word)
	assert.Equal(t, "warn", word.Action)
	assert.Zero(t, word.ActionTime)
}
