package backup

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

type storedWarnEntry struct {
	Reason    string
	WarnedBy  int64
	Anonymous bool
	WarnedAt  sql.NullTime
}

func storedWarnEntries(t *testing.T, chatID, userID int64) []storedWarnEntry {
	t.Helper()
	rows, err := db.DB.Raw(
		"SELECT reason, warned_by, anonymous, warned_at FROM warn_entries WHERE chat_id = ? AND user_id = ? ORDER BY id",
		chatID, userID,
	).Rows()
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []storedWarnEntry
	for rows.Next() {
		var e storedWarnEntry
		require.NoError(t, rows.Scan(&e.Reason, &e.WarnedBy, &e.Anonymous, &e.WarnedAt))
		out = append(out, e)
	}
	require.NoError(t, rows.Err())
	return out
}

func importWarnsBackup(t *testing.T, chatID int64, warns, entries []interface{}) error {
	t.Helper()
	bkp := NewBackupFormat(chatID, "warn_entries", 1, []string{BackupModuleWarns})
	data := map[string]interface{}{
		"warn_settings": map[string]interface{}{"warn_limit": 3, "warn_mode": "mute"},
		"warns":         warns,
	}
	if entries != nil {
		data["entries"] = entries
	}
	bkp.Data[BackupModuleWarns] = data
	return ImportChatData(chatID, bkp, nil)
}

func cleanupWarnEntriesChat(t *testing.T, chatID int64, userIDs ...int64) {
	t.Helper()
	t.Cleanup(func() {
		cleanupBackupChat(t, chatID)
		_ = db.DB.Exec("DELETE FROM warn_entries WHERE chat_id = ?", chatID).Error
		require.NoError(t, db.DB.Where("user_id IN ?", userIDs).Delete(&models.User{}).Error)
	})
}

func TestImportLegacyWarnsBuildsEntries(t *testing.T) {
	skipIfNoDb(t)
	chatID := time.Now().UnixNano()
	userID := chatID + 1
	cleanupWarnEntriesChat(t, chatID, userID)

	require.NoError(t, importWarnsBackup(t, chatID, []interface{}{
		map[string]interface{}{"user_id": userID, "num_warns": 2, "warns": []interface{}{"first", "second"}},
	}, nil))

	got := storedWarnEntries(t, chatID, userID)
	require.Len(t, got, 2)
	assert.Equal(t, "first", got[0].Reason)
	assert.Equal(t, "second", got[1].Reason)
	for _, e := range got {
		assert.Zero(t, e.WarnedBy, "a legacy reason has no known issuer")
		assert.False(t, e.WarnedAt.Valid, "a legacy reason has no known time")
	}
}

func TestWarnEntriesRoundTripThroughBackup(t *testing.T) {
	skipIfNoDb(t)
	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	userID := srcChat + 2
	cleanupWarnEntriesChat(t, srcChat, userID)
	cleanupWarnEntriesChat(t, dstChat, userID)
	warnedAt := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

	require.NoError(t, importWarnsBackup(t, srcChat, []interface{}{
		map[string]interface{}{"user_id": userID, "num_warns": 1, "warns": []interface{}{"flood"}},
	}, []interface{}{
		map[string]interface{}{
			"user_id": userID, "reason": "flood", "warned_by": 77, "anonymous": true,
			"warned_at": warnedAt.Format(time.RFC3339),
		},
	}))

	exported, err := ExportChatData(srcChat, "src", 1, []string{BackupModuleWarns})
	require.NoError(t, err)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	require.NoError(t, ImportChatData(dstChat, decoded, nil))

	got := storedWarnEntries(t, dstChat, userID)
	require.Len(t, got, 1)
	assert.Equal(t, "flood", got[0].Reason)
	assert.Zero(t, got[0].WarnedBy, "the backup must not carry the anonymous admin's account")
	assert.True(t, got[0].Anonymous)
	require.True(t, got[0].WarnedAt.Valid)
	assert.True(t, got[0].WarnedAt.Time.Equal(warnedAt), "warned_at = %v, want %v", got[0].WarnedAt.Time, warnedAt)

	// The legacy reasons array is still exported for bots that predate entries.
	var generic map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &generic))
	rows := generic["data"].(map[string]interface{})[BackupModuleWarns].(map[string]interface{})["warns"].([]interface{})
	assert.Equal(t, []interface{}{"flood"}, rows[0].(map[string]interface{})["warns"])
}

func exportedWarnEntries(t *testing.T, raw []byte) []map[string]interface{} {
	t.Helper()
	var generic map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &generic))
	module := generic["data"].(map[string]interface{})[BackupModuleWarns].(map[string]interface{})
	list, _ := module["entries"].([]interface{})
	out := make([]map[string]interface{}, len(list))
	for i, e := range list {
		out[i] = e.(map[string]interface{})
	}
	return out
}

func TestExportHidesAnonymousWarnIssuer(t *testing.T) {
	skipIfNoDb(t)
	chatID := time.Now().UnixNano()
	userID := chatID + 1
	cleanupWarnEntriesChat(t, chatID, userID)

	require.NoError(t, importWarnsBackup(t, chatID, []interface{}{
		map[string]interface{}{"user_id": userID, "num_warns": 2, "warns": []interface{}{"hidden", "public"}},
	}, []interface{}{
		map[string]interface{}{"user_id": userID, "reason": "hidden", "warned_by": 555123, "anonymous": true},
		map[string]interface{}{"user_id": userID, "reason": "public", "warned_by": 66},
	}))

	exported, err := ExportChatData(chatID, "src", 1, []string{BackupModuleWarns})
	require.NoError(t, err)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)

	assert.NotContains(t, string(raw), "555123", "the backup is posted to the group and must not name the anonymous admin")
	entries := exportedWarnEntries(t, raw)
	require.Len(t, entries, 2)
	assert.Equal(t, true, entries[0]["anonymous"])
	assert.Equal(t, float64(66), entries[1]["warned_by"], "a public issuer is still exported")
}

func TestOwnBackupImportsWithOrphanedEntries(t *testing.T) {
	skipIfNoDb(t)
	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	kept, orphan, cleared := srcChat+2, srcChat+3, srcChat+4
	cleanupWarnEntriesChat(t, srcChat, kept, orphan, cleared)
	cleanupWarnEntriesChat(t, dstChat, kept, orphan, cleared)

	require.NoError(t, importWarnsBackup(t, srcChat, []interface{}{
		map[string]interface{}{"user_id": kept, "num_warns": 1, "warns": []interface{}{"kept"}},
		map[string]interface{}{"user_id": orphan, "num_warns": 1, "warns": []interface{}{"orphan"}},
		map[string]interface{}{"user_id": cleared, "num_warns": 1, "warns": []interface{}{"cleared"}},
	}, nil))
	// An older release resets warns_users alone and leaves the entries behind.
	require.NoError(t, db.DB.Exec("DELETE FROM warns_users WHERE chat_id = ? AND user_id = ?", srcChat, orphan).Error)
	require.NoError(t, db.DB.Exec("UPDATE warns_users SET num_warns = 0, warns = '[]' WHERE chat_id = ? AND user_id = ?", srcChat, cleared).Error)

	exported, err := ExportChatData(srcChat, "src", 1, []string{BackupModuleWarns})
	require.NoError(t, err)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	require.NoError(t, ImportChatData(dstChat, decoded, nil))

	got := storedWarnEntries(t, dstChat, kept)
	require.Len(t, got, 1)
	assert.Equal(t, "kept", got[0].Reason)
	assert.Empty(t, storedWarnEntries(t, dstChat, orphan))
	assert.Empty(t, storedWarnEntries(t, dstChat, cleared))
}

func TestImportWarnsReplacesEntries(t *testing.T) {
	skipIfNoDb(t)
	chatID := time.Now().UnixNano()
	userID := chatID + 1
	cleanupWarnEntriesChat(t, chatID, userID)

	require.NoError(t, importWarnsBackup(t, chatID, []interface{}{
		map[string]interface{}{"user_id": userID, "num_warns": 1, "warns": []interface{}{"stale"}},
	}, nil))
	require.NoError(t, importWarnsBackup(t, chatID, []interface{}{
		map[string]interface{}{"user_id": userID, "num_warns": 1, "warns": []interface{}{"fresh"}},
	}, nil))

	got := storedWarnEntries(t, chatID, userID)
	require.Len(t, got, 1)
	assert.Equal(t, "fresh", got[0].Reason)
}

func TestImportWarnsRejectsEntryForUnknownUser(t *testing.T) {
	skipIfNoDb(t)
	chatID := time.Now().UnixNano()
	userID := chatID + 1
	cleanupWarnEntriesChat(t, chatID, userID, userID+1)

	err := importWarnsBackup(t, chatID, []interface{}{
		map[string]interface{}{"user_id": userID, "num_warns": 1, "warns": []interface{}{"ok"}},
	}, []interface{}{
		map[string]interface{}{"user_id": userID + 1, "reason": "orphan"},
	})
	require.Error(t, err)
}

func TestClearWarnsModuleDeletesEntries(t *testing.T) {
	skipIfNoDb(t)
	chatID := time.Now().UnixNano()
	userID := chatID + 1
	cleanupWarnEntriesChat(t, chatID, userID)
	require.NoError(t, importWarnsBackup(t, chatID, []interface{}{
		map[string]interface{}{"user_id": userID, "num_warns": 1, "warns": []interface{}{"gone"}},
	}, nil))
	require.Len(t, storedWarnEntries(t, chatID, userID), 1)

	require.NoError(t, ClearModuleData(chatID, BackupModuleWarns))

	assert.Empty(t, storedWarnEntries(t, chatID, userID))
}
