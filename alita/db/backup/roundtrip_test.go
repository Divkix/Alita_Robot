package backup

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

func TestAllModulesRoundTripEveryMeaningfulField(t *testing.T) {
	skipIfNoDb(t)

	srcChat := time.Now().UnixNano()
	dstChat := srcChat + 1
	warnUserID := srcChat + 2
	staleWarnUserID := srcChat + 3
	require.NoError(t, chats.EnsureChatInDb(srcChat, "backup_source"))
	require.NoError(t, chats.EnsureChatInDb(dstChat, "backup_destination"))
	require.NoError(t, user.EnsureUserInDb(warnUserID, "", ""))
	require.NoError(t, user.EnsureUserInDb(staleWarnUserID, "", ""))
	t.Cleanup(func() {
		cleanupBackupChat(t, srcChat)
		cleanupBackupChat(t, dstChat)
		require.NoError(t, db.DB.Where("user_id IN ?", []int64{warnUserID, staleWarnUserID}).Delete(&models.User{}).Error)
	})

	buttons := models.ButtonArray{{Name: "docs", Url: "https://example.com", SameLine: true}}
	require.NoError(t, db.DB.Create(&models.AdminSettings{ChatId: srcChat, AnonAdmin: true}).Error)
	require.NoError(t, db.DB.Create(&models.AntifloodSettings{
		ChatId:                 srcChat,
		Limit:                  9,
		Action:                 "tmute",
		DeleteAntifloodMessage: true,
	}).Error)
	require.NoError(t, db.DB.Create(&models.AntiRaidSettings{
		ChatID:                srcChat,
		RaidTime:              7777,
		RaidActionTime:        8888,
		AutoAntiRaidThreshold: 12,
	}).Error)
	require.NoError(t, db.DB.Create(&models.ApprovedUsers{
		ChatID: srcChat, UserID: 101, ApprovedBy: 202, Reason: "trusted",
	}).Error)
	require.NoError(t, db.DB.Create(&models.BlacklistSettings{
		ChatId: srcChat, Word: "scam", Action: "tban", Reason: "custom reason",
	}).Error)
	require.NoError(t, db.DB.Create(&models.CaptchaSettings{
		ChatID: srcChat, Enabled: true, CaptchaMode: "text", Timeout: 9,
		FailureAction: "mute", MaxAttempts: 8,
	}).Error)
	require.NoError(t, db.DB.Create(&models.ConnectionChatSettings{
		ChatId: srcChat, AllowConnect: true,
	}).Error)
	require.NoError(t, db.DB.Model(&models.ConnectionChatSettings{}).
		Where("chat_id = ?", srcChat).
		Update("allow_connect", false).Error)
	require.NoError(t, db.DB.Create(&models.DisableChatSettings{
		ChatId: srcChat, DeleteCommands: true,
	}).Error)
	require.NoError(t, db.DB.Create(&models.DisableSettings{
		ChatId: srcChat, Command: "ban", Disabled: true,
	}).Error)
	require.NoError(t, db.DB.Model(&models.DisableSettings{}).
		Where("chat_id = ?", srcChat).
		Update("disabled", false).Error)
	require.NoError(t, db.DB.Create(&models.ChatFilters{
		ChatId: srcChat, KeyWord: "hello", FilterReply: "world", MsgType: 2,
		FileID: "filter-file", NoNotif: true, Buttons: buttons,
	}).Error)
	require.NoError(t, db.DB.Create(&models.GreetingSettings{
		ChatID:             srcChat,
		ShouldCleanService: true,
		ShouldAutoApprove:  true,
		WelcomeSettings: &models.WelcomeSettings{
			CleanWelcome: true, LastMsgId: 111, ShouldWelcome: true,
			WelcomeText: "welcome", FileID: "welcome-file", WelcomeType: 2, Button: buttons,
		},
		GoodbyeSettings: &models.GoodbyeSettings{
			CleanGoodbye: true, LastMsgId: 222, ShouldGoodbye: true,
			GoodbyeText: "goodbye", FileID: "goodbye-file", GoodbyeType: 3, Button: buttons,
		},
	}).Error)
	require.NoError(t, db.DB.Model(&models.GreetingSettings{}).
		Where("chat_id = ?", srcChat).
		Update("welcome_enabled", false).Error)
	require.NoError(t, db.DB.Create(&models.LockSettings{
		ChatId: srcChat, LockType: "stickers", Locked: false,
	}).Error)
	require.NoError(t, db.DB.Create(&models.NotesSettings{ChatId: srcChat, Private: true}).Error)
	require.NoError(t, db.DB.Create(&models.Notes{
		ChatId: srcChat, NoteName: "policy", NoteContent: "read it", FileID: "note-file",
		MsgType: 4, Buttons: buttons, AdminOnly: true, PrivateOnly: true,
		GroupOnly: false, WebPreview: true, IsProtected: true, NoNotif: true,
	}).Error)
	require.NoError(t, db.DB.Model(&models.Notes{}).
		Where("chat_id = ?", srcChat).
		Update("web_preview", false).Error)
	require.NoError(t, db.DB.Create(&models.PinSettings{
		ChatId: srcChat, MsgId: 4242, CleanLinked: true, AntiChannelPin: true,
	}).Error)
	require.NoError(t, db.DB.Create(&models.Reactions{
		ChatID: srcChat, Keyword: "nice", Emoji: "🔥",
	}).Error)
	require.NoError(t, db.DB.Create(&models.ReportChatSettings{
		ChatId: srcChat, Enabled: true, Status: true, BlockedList: models.Int64Array{303, 404},
	}).Error)
	require.NoError(t, db.DB.Model(&models.ReportChatSettings{}).
		Where("chat_id = ?", srcChat).
		Updates(map[string]interface{}{"enabled": false, "status": false}).Error)
	require.NoError(t, db.DB.Create(&models.RulesSettings{
		ChatId: srcChat, Rules: "be kind", RulesBtn: "Read", Private: true,
	}).Error)
	require.NoError(t, db.DB.Create(&models.WarnSettings{
		ChatId: srcChat, WarnLimit: 7, WarnMode: "tmute",
	}).Error)
	require.NoError(t, db.DB.Create(&models.Warns{
		UserId: warnUserID, ChatId: srcChat, NumWarns: 2, Reasons: models.StringArray{"one", "two"},
	}).Error)

	// Existing destination data must be replaced, not merged.
	require.NoError(t, db.DB.Create(&models.Reactions{
		ChatID: dstChat, Keyword: "stale", Emoji: "❌",
	}).Error)
	require.NoError(t, db.DB.Create(&models.ChatFilters{
		ChatId: dstChat, KeyWord: "stale", FilterReply: "stale",
	}).Error)
	require.NoError(t, db.DB.Create(&models.Warns{
		UserId: staleWarnUserID, ChatId: dstChat, NumWarns: 1, Reasons: models.StringArray{"stale"},
	}).Error)

	exported, err := ExportChatData(srcChat, "source", 42, nil)
	require.NoError(t, err)
	require.Len(t, exported.Data, len(AllExportableModules()))

	raw, err := exported.ToJSON()
	require.NoError(t, err)
	decoded, err := BackupFormatFromJSON(raw)
	require.NoError(t, err)
	require.NoError(t, ImportChatData(dstChat, decoded, nil))

	adminData, err := exportAdminData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, adminData.AdminSettings)
	assert.True(t, adminData.AdminSettings.AnonAdmin)

	antifloodData, err := exportAntifloodData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, antifloodData.Settings)
	assert.Equal(t, 9, antifloodData.Settings.Limit)
	assert.Equal(t, "tmute", antifloodData.Settings.Action)
	assert.True(t, antifloodData.Settings.DeleteAntifloodMessage)

	antiraidData, err := exportAntiraidData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, antiraidData.Settings)
	assert.Equal(t, 7777, antiraidData.Settings.RaidTime)
	assert.Equal(t, 8888, antiraidData.Settings.RaidActionTime)
	assert.Equal(t, 12, antiraidData.Settings.AutoAntiRaidThreshold)

	approvalsData, err := exportApprovalsData(dstChat)
	require.NoError(t, err)
	require.Len(t, approvalsData.ApprovedUsers, 1)
	assert.Equal(t, int64(101), approvalsData.ApprovedUsers[0].UserID)
	assert.Equal(t, int64(202), approvalsData.ApprovedUsers[0].ApprovedBy)
	assert.Equal(t, "trusted", approvalsData.ApprovedUsers[0].Reason)

	blacklistsData, err := exportBlacklistsData(dstChat)
	require.NoError(t, err)
	require.Len(t, blacklistsData.Entries, 1)
	assert.Equal(t, "scam", blacklistsData.Entries[0].Word)
	assert.Equal(t, "tban", blacklistsData.Entries[0].Action)
	assert.Equal(t, "custom reason", blacklistsData.Entries[0].Reason)

	captchaData, err := exportCaptchaData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, captchaData.Settings)
	assert.True(t, captchaData.Settings.Enabled)
	assert.Equal(t, "text", captchaData.Settings.CaptchaMode)
	assert.Equal(t, 9, captchaData.Settings.Timeout)
	assert.Equal(t, "mute", captchaData.Settings.FailureAction)
	assert.Equal(t, 8, captchaData.Settings.MaxAttempts)

	connectionsData, err := exportConnectionsData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, connectionsData.Settings)
	assert.False(t, connectionsData.Settings.AllowConnect)

	disablingData, err := exportDisablingData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, disablingData.ChatSettings)
	assert.True(t, disablingData.ChatSettings.DeleteCommands)
	require.Len(t, disablingData.Commands, 1)
	assert.Equal(t, "ban", disablingData.Commands[0].Command)
	assert.False(t, disablingData.Commands[0].Disabled)

	filtersData, err := exportFiltersData(dstChat)
	require.NoError(t, err)
	require.Len(t, filtersData.Filters, 1)
	assert.Equal(t, "hello", filtersData.Filters[0].KeyWord)
	assert.Equal(t, "world", filtersData.Filters[0].FilterReply)
	assert.Equal(t, 2, filtersData.Filters[0].MsgType)
	assert.Equal(t, "filter-file", filtersData.Filters[0].FileID)
	assert.True(t, filtersData.Filters[0].NoNotif)
	assert.Equal(t, buttons, filtersData.Filters[0].Buttons)

	greetingsData, err := exportGreetingsData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, greetingsData.Settings)
	assert.True(t, greetingsData.Settings.ShouldCleanService)
	assert.True(t, greetingsData.Settings.ShouldAutoApprove)
	require.NotNil(t, greetingsData.Settings.WelcomeSettings)
	assert.True(t, greetingsData.Settings.WelcomeSettings.CleanWelcome)
	assert.Equal(t, int64(111), greetingsData.Settings.WelcomeSettings.LastMsgId)
	assert.False(t, greetingsData.Settings.WelcomeSettings.ShouldWelcome)
	assert.Equal(t, "welcome", greetingsData.Settings.WelcomeSettings.WelcomeText)
	assert.Equal(t, "welcome-file", greetingsData.Settings.WelcomeSettings.FileID)
	assert.Equal(t, 2, greetingsData.Settings.WelcomeSettings.WelcomeType)
	assert.Equal(t, buttons, greetingsData.Settings.WelcomeSettings.Button)
	require.NotNil(t, greetingsData.Settings.GoodbyeSettings)
	assert.True(t, greetingsData.Settings.GoodbyeSettings.CleanGoodbye)
	assert.Equal(t, int64(222), greetingsData.Settings.GoodbyeSettings.LastMsgId)
	assert.True(t, greetingsData.Settings.GoodbyeSettings.ShouldGoodbye)
	assert.Equal(t, "goodbye", greetingsData.Settings.GoodbyeSettings.GoodbyeText)
	assert.Equal(t, "goodbye-file", greetingsData.Settings.GoodbyeSettings.FileID)
	assert.Equal(t, 3, greetingsData.Settings.GoodbyeSettings.GoodbyeType)
	assert.Equal(t, buttons, greetingsData.Settings.GoodbyeSettings.Button)

	locksData, err := exportLocksData(dstChat)
	require.NoError(t, err)
	require.Len(t, locksData.Locks, 1)
	assert.Equal(t, "stickers", locksData.Locks[0].LockType)
	assert.False(t, locksData.Locks[0].Locked)

	notesData, err := exportNotesData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, notesData.Settings)
	assert.True(t, notesData.Settings.Private)
	require.Len(t, notesData.Notes, 1)
	note := notesData.Notes[0]
	assert.Equal(t, "policy", note.NoteName)
	assert.Equal(t, "read it", note.NoteContent)
	assert.Equal(t, "note-file", note.FileID)
	assert.Equal(t, 4, note.MsgType)
	assert.Equal(t, buttons, note.Buttons)
	assert.True(t, note.AdminOnly)
	assert.True(t, note.PrivateOnly)
	assert.False(t, note.GroupOnly)
	assert.False(t, note.WebPreview)
	assert.True(t, note.IsProtected)
	assert.True(t, note.NoNotif)

	pinsData, err := exportPinsData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, pinsData.Settings)
	assert.Equal(t, int64(4242), pinsData.Settings.MsgId)
	assert.True(t, pinsData.Settings.CleanLinked)
	assert.True(t, pinsData.Settings.AntiChannelPin)

	reactionsData, err := exportReactionsData(dstChat)
	require.NoError(t, err)
	require.Len(t, reactionsData.Reactions, 1)
	assert.Equal(t, "nice", reactionsData.Reactions[0].Keyword)
	assert.Equal(t, "🔥", reactionsData.Reactions[0].Emoji)

	reportsData, err := exportReportsData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, reportsData.Settings)
	assert.False(t, reportsData.Settings.Enabled)
	assert.False(t, reportsData.Settings.Status)
	assert.Equal(t, models.Int64Array{303, 404}, reportsData.Settings.BlockedList)

	rulesData, err := exportRulesData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, rulesData.Settings)
	assert.Equal(t, "be kind", rulesData.Settings.Rules)
	assert.Equal(t, "Read", rulesData.Settings.RulesBtn)
	assert.True(t, rulesData.Settings.Private)

	warnsData, err := exportWarnsData(dstChat)
	require.NoError(t, err)
	require.NotNil(t, warnsData.WarnSettings)
	assert.Equal(t, 7, warnsData.WarnSettings.WarnLimit)
	assert.Equal(t, "tmute", warnsData.WarnSettings.WarnMode)
	require.Len(t, warnsData.Warns, 1)
	assert.Equal(t, warnUserID, warnsData.Warns[0].UserId)
	assert.Equal(t, 2, warnsData.Warns[0].NumWarns)
	assert.Equal(t, models.StringArray{"one", "two"}, warnsData.Warns[0].Reasons)
}

func TestExportChatDataReturnsDatabaseErrors(t *testing.T) {
	original := db.DB
	db.DB = nil
	t.Cleanup(func() { db.DB = original })

	_, err := ExportChatData(1, "chat", 2, []string{BackupModuleFilters})
	require.ErrorContains(t, err, "database not initialized")
}

func TestImportChatDataRollsBackEarlierModules(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	require.NoError(t, chats.EnsureChatInDb(chatID, "backup_atomicity"))
	t.Cleanup(func() { cleanupBackupChat(t, chatID) })
	require.NoError(t, db.DB.Create(&models.RulesSettings{
		ChatId: chatID, Rules: "original",
	}).Error)

	backup := NewBackupFormat(chatID, "chat", 1, []string{BackupModuleRules, BackupModuleCaptcha})
	backup.Data[BackupModuleRules] = map[string]interface{}{
		"settings": map[string]interface{}{"rules": "replacement"},
	}
	backup.Data[BackupModuleCaptcha] = map[string]interface{}{
		"settings": map[string]interface{}{
			"enabled": true, "captcha_mode": "invalid", "timeout": 2,
			"failure_action": "kick", "max_attempts": 3,
		},
	}

	require.Error(t, ImportChatData(chatID, backup, nil))
	settings, err := findChatSetting[models.RulesSettings](chatID)
	require.NoError(t, err)
	require.NotNil(t, settings)
	assert.Equal(t, "original", settings.Rules)
}

func TestImportWarnsCreatesMissingParents(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	userID := chatID + 1
	t.Cleanup(func() {
		cleanupBackupChat(t, chatID)
		require.NoError(t, db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error)
	})

	bkp := NewBackupFormat(chatID, "fresh", 1, []string{BackupModuleWarns})
	bkp.Data[BackupModuleWarns] = map[string]interface{}{
		"warn_settings": map[string]interface{}{"warn_limit": 3, "warn_mode": "mute"},
		"warns": []interface{}{
			map[string]interface{}{"user_id": userID, "num_warns": 1, "warns": []interface{}{"reason"}},
		},
	}
	require.NoError(t, ImportChatData(chatID, bkp, nil))

	require.NoError(t, db.DB.Where("chat_id = ?", chatID).Take(&models.Chat{}).Error)
	require.NoError(t, db.DB.Where("user_id = ?", userID).Take(&models.User{}).Error)
	require.NoError(t, db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&models.Warns{}).Error)
}

func TestLegacyBackupPreservesFieldsThatVersionDidNotExport(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	warnUserID := chatID + 1
	require.NoError(t, chats.EnsureChatInDb(chatID, "legacy_backup"))
	require.NoError(t, user.EnsureUserInDb(warnUserID, "", ""))
	t.Cleanup(func() {
		cleanupBackupChat(t, chatID)
		require.NoError(t, db.DB.Where("user_id = ?", warnUserID).Delete(&models.User{}).Error)
	})
	require.NoError(t, db.DB.Create(&models.NotesSettings{ChatId: chatID, Private: true}).Error)
	require.NoError(t, db.DB.Create(&models.Notes{
		ChatId: chatID, NoteName: "old", NoteContent: "old",
	}).Error)
	require.NoError(t, db.DB.Create(&models.WarnSettings{
		ChatId: chatID, WarnLimit: 3, WarnMode: "mute",
	}).Error)
	require.NoError(t, db.DB.Create(&models.Warns{
		ChatId: chatID, UserId: warnUserID, NumWarns: 2, Reasons: models.StringArray{"one", "two"},
	}).Error)

	raw := fmt.Sprintf(`{
		"version":"1.0",
		"bot_name":"AlitaRobot",
		"chat_id":%d,
		"modules":["notes","warns"],
		"data":{
			"notes":{"notes":[{"note_name":"new","note_content":"new"}]},
			"warns":{"warn_settings":{"warn_limit":7,"warn_mode":"kick"}}
		}
	}`, chatID)
	legacy, err := BackupFormatFromJSON([]byte(raw))
	require.NoError(t, err)
	require.NoError(t, ImportChatData(chatID, legacy, nil))

	noteSettings, err := findChatSetting[models.NotesSettings](chatID)
	require.NoError(t, err)
	require.NotNil(t, noteSettings)
	assert.True(t, noteSettings.Private)
	notes, err := findChatRows[models.Notes](chatID)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "new", notes[0].NoteName)

	warnSettings, err := findChatSetting[models.WarnSettings](chatID)
	require.NoError(t, err)
	require.NotNil(t, warnSettings)
	assert.Equal(t, 7, warnSettings.WarnLimit)
	warnRows, err := findChatRows[models.Warns](chatID)
	require.NoError(t, err)
	require.Len(t, warnRows, 1)
	assert.Equal(t, warnUserID, warnRows[0].UserId)
}
