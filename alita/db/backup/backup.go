package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExportModuleData exports data for a specific module from a chat.
func ExportModuleData(chatID int64, module string) (interface{}, error) {
	switch module {
	case BackupModuleAdmin:
		return exportAdminData(chatID)
	case BackupModuleAntiflood:
		return exportAntifloodData(chatID)
	case BackupModuleAntiraid:
		return exportAntiraidData(chatID)
	case BackupModuleApprovals:
		return exportApprovalsData(chatID)
	case BackupModuleBlacklists:
		return exportBlacklistsData(chatID)
	case BackupModuleCaptcha:
		return exportCaptchaData(chatID)
	case BackupModuleConnections:
		return exportConnectionsData(chatID)
	case BackupModuleDisabling:
		return exportDisablingData(chatID)
	case BackupModuleFilters:
		return exportFiltersData(chatID)
	case BackupModuleGreetings:
		return exportGreetingsData(chatID)
	case BackupModuleLocks:
		return exportLocksData(chatID)
	case BackupModuleNotes:
		return exportNotesData(chatID)
	case BackupModulePins:
		return exportPinsData(chatID)
	case BackupModuleReactions:
		return exportReactionsData(chatID)
	case BackupModuleReports:
		return exportReportsData(chatID)
	case BackupModuleRules:
		return exportRulesData(chatID)
	case BackupModuleWarns:
		return exportWarnsData(chatID)
	default:
		return nil, fmt.Errorf("unknown module: %s", module)
	}
}

// ImportModuleData imports one module atomically into a chat.
func ImportModuleData(chatID int64, module string, data interface{}) error {
	database, err := backupDB()
	if err != nil {
		return err
	}

	var keys []string
	err = database.Transaction(func(tx *gorm.DB) error {
		var importErr error
		keys, importErr = importModuleData(tx, chatID, module, data, false)
		return importErr
	})
	if err != nil {
		return err
	}
	invalidate(keys...)
	return nil
}

// ClearModuleData clears one module atomically from a chat.
func ClearModuleData(chatID int64, module string) error {
	database, err := backupDB()
	if err != nil {
		return err
	}

	var keys []string
	err = database.Transaction(func(tx *gorm.DB) error {
		var clearErr error
		keys, clearErr = clearModuleData(tx, chatID, module)
		return clearErr
	})
	if err != nil {
		return err
	}
	invalidate(keys...)
	return nil
}

// ExportChatData exports the selected modules. A failed module aborts the
// export so callers never receive a backup that only looks complete.
func ExportChatData(chatID int64, chatName string, exportedBy int64, modules []string) (*BackupFormat, error) {
	modules, err := checkedModules(modules)
	if err != nil {
		return nil, err
	}

	backup := NewBackupFormat(chatID, chatName, exportedBy, modules)
	for _, module := range modules {
		data, err := ExportModuleData(chatID, module)
		if err != nil {
			return nil, fmt.Errorf("failed to export module %s: %w", module, err)
		}
		backup.Data[module] = data
	}
	return backup, nil
}

// ImportChatData imports every selected module in one transaction.
func ImportChatData(chatID int64, backup *BackupFormat, modules []string) error {
	if backup == nil {
		return fmt.Errorf("invalid backup: backup cannot be nil")
	}
	if err := backup.Validate(); err != nil {
		return fmt.Errorf("invalid backup: %w", err)
	}
	if !backup.IsCompatibleVersion() {
		return fmt.Errorf("unsupported backup version %q", backup.Version)
	}
	if len(modules) == 0 {
		modules = backup.Modules
	}
	for _, module := range modules {
		if !IsValidModule(module) {
			return fmt.Errorf("unknown module: %s", module)
		}
		if _, ok := backup.Data[module]; !ok {
			return fmt.Errorf("missing data for module: %s", module)
		}
	}

	database, err := backupDB()
	if err != nil {
		return err
	}

	var keys []string
	err = database.Transaction(func(tx *gorm.DB) error {
		for _, module := range modules {
			data := backup.Data[module]
			moduleKeys, err := importModuleData(tx, chatID, module, data, backup.Version == legacyFormatVersion)
			if err != nil {
				return fmt.Errorf("failed to import module %s: %w", module, err)
			}
			keys = append(keys, moduleKeys...)
		}
		return nil
	})
	if err != nil {
		return err
	}
	invalidate(keys...)
	return nil
}

// ClearChatData clears every selected module in one transaction.
func ClearChatData(chatID int64, modules []string) error {
	modules, err := checkedModules(modules)
	if err != nil {
		return err
	}
	database, err := backupDB()
	if err != nil {
		return err
	}

	var keys []string
	err = database.Transaction(func(tx *gorm.DB) error {
		for _, module := range modules {
			moduleKeys, err := clearModuleData(tx, chatID, module)
			if err != nil {
				return fmt.Errorf("failed to clear module %s: %w", module, err)
			}
			keys = append(keys, moduleKeys...)
		}
		return nil
	})
	if err != nil {
		return err
	}
	invalidate(keys...)
	return nil
}

func checkedModules(modules []string) ([]string, error) {
	if len(modules) == 0 {
		return AllExportableModules(), nil
	}
	for _, module := range modules {
		if !IsValidModule(module) {
			return nil, fmt.Errorf("unknown module: %s", module)
		}
	}
	return modules, nil
}

func backupDB() (*gorm.DB, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return db.DB, nil
}

func findChatSetting[T any](chatID int64) (*T, error) {
	database, err := backupDB()
	if err != nil {
		return nil, err
	}
	var setting T
	err = database.Where("chat_id = ?", chatID).Take(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func findChatRows[T any](chatID int64) ([]T, error) {
	database, err := backupDB()
	if err != nil {
		return nil, err
	}
	var rows []T
	if err := database.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func replaceChatSetting[T any](tx *gorm.DB, chatID int64, setting *T) error {
	if err := tx.Where("chat_id = ?", chatID).Delete(new(T)).Error; err != nil {
		return err
	}
	if setting == nil {
		return nil
	}
	var desired T
	raw, err := json.Marshal(setting)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &desired); err != nil {
		return err
	}
	// Use GORM schema to build map preserving zero values (json omitempty and
	// struct default handling would clobber false/0). Single bulk insert.
	stmt := &gorm.Statement{DB: tx}
	if err := stmt.Parse(&desired); err != nil {
		return err
	}
	m := make(map[string]interface{}, len(stmt.Schema.Fields))
	rv := reflect.ValueOf(&desired).Elem()
	for _, field := range stmt.Schema.Fields {
		if field.Name == "ID" || field.DBName == "created_at" {
			continue
		}
		val, _ := field.ValueOf(context.Background(), rv)
		m[field.DBName] = val
	}
	return tx.Model(new(T)).Create(m).Error
}

func replaceChatRows[T any](tx *gorm.DB, chatID int64, rows []T) error {
	if err := tx.Where("chat_id = ?", chatID).Delete(new(T)).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	var desired []T
	raw, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &desired); err != nil {
		return err
	}
	// Build slice of maps via schema to preserve zero values and avoid N Updates.
	maps := make([]map[string]interface{}, len(desired))
	for i := range desired {
		stmt := &gorm.Statement{DB: tx}
		if err := stmt.Parse(&desired[i]); err != nil {
			return err
		}
		m := make(map[string]interface{}, len(stmt.Schema.Fields))
		rv := reflect.ValueOf(&desired[i]).Elem()
		for _, field := range stmt.Schema.Fields {
			if field.Name == "ID" || field.DBName == "created_at" {
				continue
			}
			val, _ := field.ValueOf(context.Background(), rv)
			m[field.DBName] = val
		}
		maps[i] = m
	}
	return tx.Model(new(T)).Create(&maps).Error
}

func decodeModuleData(data interface{}, module string, target interface{}) error {
	if _, ok := data.(map[string]interface{}); !ok {
		return fmt.Errorf("invalid %s data format", module)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("invalid %s data format: %w", module, err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("failed to parse %s data: %w", module, err)
	}
	return nil
}

func invalidate(keys ...string) {
	for _, key := range keys {
		dbcache.DeleteCache(key)
	}
}

func cacheKey(module string, chatID int64) string {
	return dbcache.CacheKey(module, chatID)
}

// Exporters query complete rows instead of lossy summary getters.

func exportAdminData(chatID int64) (*AdminBackup, error) {
	adminSettings, err := findChatSetting[models.AdminSettings](chatID)
	if err != nil {
		return nil, fmt.Errorf("get admin settings: %w", err)
	}
	antifloodSettings, err := findChatSetting[models.AntifloodSettings](chatID)
	if err != nil {
		return nil, fmt.Errorf("get antiflood settings: %w", err)
	}
	captchaSettings, err := findChatSetting[models.CaptchaSettings](chatID)
	if err != nil {
		return nil, fmt.Errorf("get captcha settings: %w", err)
	}
	connectionSettings, err := findChatSetting[models.ConnectionChatSettings](chatID)
	if err != nil {
		return nil, fmt.Errorf("get connection settings: %w", err)
	}
	blacklistEntries, err := findChatRows[models.BlacklistSettings](chatID)
	if err != nil {
		return nil, fmt.Errorf("get blacklist settings: %w", err)
	}

	result := &AdminBackup{
		AdminSettings:      adminSettings,
		AntifloodSettings:  antifloodSettings,
		CaptchaSettings:    captchaSettings,
		ConnectionSettings: connectionSettings,
	}
	if len(blacklistEntries) > 0 {
		result.BlacklistMode = blacklistEntries[0].Action
	}
	return result, nil
}

func exportAntifloodData(chatID int64) (*AntifloodBackup, error) {
	settings, err := findChatSetting[models.AntifloodSettings](chatID)
	return &AntifloodBackup{Settings: settings}, err
}

func exportAntiraidData(chatID int64) (*AntiraidBackup, error) {
	settings, err := findChatSetting[models.AntiRaidSettings](chatID)
	return &AntiraidBackup{Settings: settings}, err
}

func exportApprovalsData(chatID int64) (*ApprovalsBackup, error) {
	users, err := findChatRows[models.ApprovedUsers](chatID)
	return &ApprovalsBackup{ApprovedUsers: users}, err
}

func exportBlacklistsData(chatID int64) (*BlacklistsBackup, error) {
	entries, err := findChatRows[models.BlacklistSettings](chatID)
	if err != nil {
		return nil, err
	}
	result := &BlacklistsBackup{Entries: entries}
	if len(entries) > 0 {
		result.BlacklistMode = entries[0].Action
	}
	return result, nil
}

func exportCaptchaData(chatID int64) (*CaptchaBackup, error) {
	settings, err := findChatSetting[models.CaptchaSettings](chatID)
	return &CaptchaBackup{Settings: settings}, err
}

func exportConnectionsData(chatID int64) (*ConnectionsBackup, error) {
	settings, err := findChatSetting[models.ConnectionChatSettings](chatID)
	return &ConnectionsBackup{Settings: settings}, err
}

func exportDisablingData(chatID int64) (*DisablingBackup, error) {
	settings, err := findChatSetting[models.DisableChatSettings](chatID)
	if err != nil {
		return nil, err
	}
	commands, err := findChatRows[models.DisableSettings](chatID)
	if err != nil {
		return nil, err
	}
	return &DisablingBackup{ChatSettings: settings, Commands: commands}, nil
}

func exportFiltersData(chatID int64) (*FiltersBackup, error) {
	rows, err := findChatRows[models.ChatFilters](chatID)
	return &FiltersBackup{Filters: rows}, err
}

func exportGreetingsData(chatID int64) (*GreetingsBackup, error) {
	settings, err := findChatSetting[models.GreetingSettings](chatID)
	return &GreetingsBackup{Settings: settings}, err
}

func exportLocksData(chatID int64) (*LocksBackup, error) {
	rows, err := findChatRows[models.LockSettings](chatID)
	return &LocksBackup{Locks: rows}, err
}

func exportNotesData(chatID int64) (*NotesBackup, error) {
	settings, err := findChatSetting[models.NotesSettings](chatID)
	if err != nil {
		return nil, err
	}
	rows, err := findChatRows[models.Notes](chatID)
	if err != nil {
		return nil, err
	}
	return &NotesBackup{Settings: settings, Notes: rows}, nil
}

func exportPinsData(chatID int64) (*PinsBackup, error) {
	settings, err := findChatSetting[models.PinSettings](chatID)
	return &PinsBackup{Settings: settings}, err
}

func exportReactionsData(chatID int64) (*ReactionsBackup, error) {
	rows, err := findChatRows[models.Reactions](chatID)
	return &ReactionsBackup{Reactions: rows}, err
}

func exportReportsData(chatID int64) (*ReportsBackup, error) {
	settings, err := findChatSetting[models.ReportChatSettings](chatID)
	return &ReportsBackup{Settings: settings}, err
}

func exportRulesData(chatID int64) (*RulesBackup, error) {
	settings, err := findChatSetting[models.RulesSettings](chatID)
	return &RulesBackup{Settings: settings}, err
}

func exportWarnsData(chatID int64) (*WarnsBackup, error) {
	settings, err := findChatSetting[models.WarnSettings](chatID)
	if err != nil {
		return nil, err
	}
	rows, err := findChatRows[models.Warns](chatID)
	if err != nil {
		return nil, err
	}
	return &WarnsBackup{WarnSettings: settings, Warns: rows}, nil
}

func importModuleData(tx *gorm.DB, chatID int64, module string, data interface{}, preserveLegacyOmissions bool) ([]string, error) {
	if err := ensureBackupChat(tx, chatID); err != nil {
		return nil, err
	}
	switch module {
	case BackupModuleAdmin:
		return importAdmin(tx, chatID, data)
	case BackupModuleAntiflood:
		return importAntiflood(tx, chatID, data)
	case BackupModuleAntiraid:
		return importAntiraid(tx, chatID, data)
	case BackupModuleApprovals:
		return importApprovals(tx, chatID, data)
	case BackupModuleBlacklists:
		return importBlacklists(tx, chatID, data)
	case BackupModuleCaptcha:
		return importCaptcha(tx, chatID, data)
	case BackupModuleConnections:
		return importConnections(tx, chatID, data)
	case BackupModuleDisabling:
		return importDisabling(tx, chatID, data)
	case BackupModuleFilters:
		return importFilters(tx, chatID, data)
	case BackupModuleGreetings:
		return importGreetings(tx, chatID, data)
	case BackupModuleLocks:
		return importLocks(tx, chatID, data)
	case BackupModuleNotes:
		return importNotes(tx, chatID, data, preserveLegacyOmissions)
	case BackupModulePins:
		return importPins(tx, chatID, data)
	case BackupModuleReactions:
		return importReactions(tx, chatID, data)
	case BackupModuleReports:
		return importReports(tx, chatID, data)
	case BackupModuleRules:
		return importRules(tx, chatID, data)
	case BackupModuleWarns:
		return importWarns(tx, chatID, data, preserveLegacyOmissions)
	default:
		return nil, fmt.Errorf("unknown module: %s", module)
	}
}

func importAdmin(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data AdminBackup
	if err := decodeModuleData(payload, BackupModuleAdmin, &data); err != nil {
		return nil, err
	}
	if data.AdminSettings != nil {
		data.AdminSettings.ChatId = chatID
	}
	if data.AntifloodSettings != nil {
		data.AntifloodSettings.ChatId = chatID
	}
	if data.CaptchaSettings != nil {
		data.CaptchaSettings.ChatID = chatID
	}
	if data.ConnectionSettings != nil {
		data.ConnectionSettings.ChatId = chatID
	}

	if err := replaceChatSetting(tx, chatID, data.AdminSettings); err != nil {
		return nil, fmt.Errorf("restore admin settings: %w", err)
	}
	if err := replaceChatSetting(tx, chatID, data.AntifloodSettings); err != nil {
		return nil, fmt.Errorf("restore antiflood settings: %w", err)
	}
	if err := replaceChatSetting(tx, chatID, data.CaptchaSettings); err != nil {
		return nil, fmt.Errorf("restore captcha settings: %w", err)
	}
	if err := replaceChatSetting(tx, chatID, data.ConnectionSettings); err != nil {
		return nil, fmt.Errorf("restore connection settings: %w", err)
	}
	if data.BlacklistMode != "" {
		if err := tx.Model(&models.BlacklistSettings{}).
			Where("chat_id = ?", chatID).
			Update("action", data.BlacklistMode).Error; err != nil {
			return nil, fmt.Errorf("restore blacklist mode: %w", err)
		}
	}
	return []string{
		cacheKey("antiflood", chatID),
		cacheKey("captcha_settings", chatID),
		cacheKey("blacklist", chatID),
	}, nil
}

func importAntiflood(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data AntifloodBackup
	if err := decodeModuleData(payload, BackupModuleAntiflood, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatId = chatID
		if data.Settings.Limit < 0 {
			return nil, fmt.Errorf("invalid antiflood limit %d", data.Settings.Limit)
		}
	}
	if err := replaceChatSetting(tx, chatID, data.Settings); err != nil {
		return nil, err
	}
	return []string{cacheKey("antiflood", chatID)}, nil
}

func importAntiraid(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data AntiraidBackup
	if err := decodeModuleData(payload, BackupModuleAntiraid, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatID = chatID
		if data.Settings.RaidTime < 0 || data.Settings.RaidActionTime < 0 || data.Settings.AutoAntiRaidThreshold < 0 {
			return nil, fmt.Errorf("invalid antiraid settings")
		}
	}
	if err := replaceChatSetting(tx, chatID, data.Settings); err != nil {
		return nil, err
	}
	return []string{cacheKey("antiraid", chatID)}, nil
}

func importApprovals(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data ApprovalsBackup
	if err := decodeModuleData(payload, BackupModuleApprovals, &data); err != nil {
		return nil, err
	}
	for i := range data.ApprovedUsers {
		if data.ApprovedUsers[i].UserID == 0 {
			return nil, fmt.Errorf("invalid approved user ID")
		}
		data.ApprovedUsers[i].ChatID = chatID
	}
	if err := replaceChatRows(tx, chatID, data.ApprovedUsers); err != nil {
		return nil, err
	}
	return []string{cacheKey("approvals", chatID)}, nil
}

func importBlacklists(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data BlacklistsBackup
	if err := decodeModuleData(payload, BackupModuleBlacklists, &data); err != nil {
		return nil, err
	}
	if len(data.Entries) == 0 && data.Settings != nil {
		data.Entries = []models.BlacklistSettings{*data.Settings}
	}
	for i := range data.Entries {
		if data.Entries[i].Word == "" {
			return nil, fmt.Errorf("invalid empty blacklist word")
		}
		data.Entries[i].ChatId = chatID
		if data.Entries[i].Action == "" {
			data.Entries[i].Action = data.BlacklistMode
		}
		if data.Entries[i].Action == "" {
			data.Entries[i].Action = "warn"
		}
	}
	if err := replaceChatRows(tx, chatID, data.Entries); err != nil {
		return nil, err
	}
	return []string{cacheKey("blacklist", chatID)}, nil
}

func importCaptcha(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data CaptchaBackup
	if err := decodeModuleData(payload, BackupModuleCaptcha, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatID = chatID
	}
	if err := replaceChatSetting(tx, chatID, data.Settings); err != nil {
		return nil, err
	}
	return []string{cacheKey("captcha_settings", chatID)}, nil
}

func importConnections(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data ConnectionsBackup
	if err := decodeModuleData(payload, BackupModuleConnections, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatId = chatID
	}
	return nil, replaceChatSetting(tx, chatID, data.Settings)
}

func importDisabling(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) { //nolint:dupl // module-specific schema
	var data DisablingBackup
	if err := decodeModuleData(payload, BackupModuleDisabling, &data); err != nil {
		return nil, err
	}
	if data.ChatSettings != nil {
		data.ChatSettings.ChatId = chatID
	}
	for i := range data.Commands {
		if data.Commands[i].Command == "" {
			return nil, fmt.Errorf("invalid empty disabled command")
		}
		data.Commands[i].ChatId = chatID
	}
	if err := replaceChatSetting(tx, chatID, data.ChatSettings); err != nil {
		return nil, err
	}
	if err := replaceChatRows(tx, chatID, data.Commands); err != nil {
		return nil, err
	}
	return []string{cacheKey("disabled_cmds", chatID)}, nil
}

func importFilters(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) { //nolint:dupl // module-specific schema
	var data FiltersBackup
	if err := decodeModuleData(payload, BackupModuleFilters, &data); err != nil {
		return nil, err
	}
	for i := range data.Filters {
		if data.Filters[i].KeyWord == "" {
			return nil, fmt.Errorf("invalid empty filter keyword")
		}
		data.Filters[i].ChatId = chatID
	}
	if err := replaceChatRows(tx, chatID, data.Filters); err != nil {
		return nil, err
	}
	return []string{
		cacheKey("filter_list", chatID),
		cacheKey("filters_optimized", chatID),
	}, nil
}

func importGreetings(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data GreetingsBackup
	if err := decodeModuleData(payload, BackupModuleGreetings, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatID = chatID
	}
	if err := replaceChatSetting(tx, chatID, data.Settings); err != nil {
		return nil, err
	}
	return []string{cacheKey("greetings", chatID)}, nil
}

func importLocks(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) { //nolint:dupl // module-specific schema
	var data LocksBackup
	if err := decodeModuleData(payload, BackupModuleLocks, &data); err != nil {
		return nil, err
	}
	for i := range data.Locks {
		if data.Locks[i].LockType == "" {
			return nil, fmt.Errorf("invalid empty lock type")
		}
		data.Locks[i].ChatId = chatID
	}
	if err := replaceChatRows(tx, chatID, data.Locks); err != nil {
		return nil, err
	}
	return []string{cacheKey("lock", chatID), cacheKey("locks_map", chatID)}, nil
}

func importNotes(tx *gorm.DB, chatID int64, payload interface{}, preserveLegacyOmissions bool) ([]string, error) { //nolint:dupl // module-specific schema
	restoreSettings := !preserveLegacyOmissions || moduleFieldPresent(payload, "settings")
	var data NotesBackup
	if err := decodeModuleData(payload, BackupModuleNotes, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatId = chatID
	}
	for i := range data.Notes {
		if data.Notes[i].NoteName == "" {
			return nil, fmt.Errorf("invalid empty note name")
		}
		data.Notes[i].ChatId = chatID
	}
	if restoreSettings {
		if err := replaceChatSetting(tx, chatID, data.Settings); err != nil {
			return nil, err
		}
	}
	if err := replaceChatRows(tx, chatID, data.Notes); err != nil {
		return nil, err
	}
	return []string{cacheKey("notes_settings", chatID)}, nil
}

func importPins(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data PinsBackup
	if err := decodeModuleData(payload, BackupModulePins, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatId = chatID
	}
	return nil, replaceChatSetting(tx, chatID, data.Settings)
}

func importReactions(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data ReactionsBackup
	if err := decodeModuleData(payload, BackupModuleReactions, &data); err != nil {
		return nil, err
	}
	for i := range data.Reactions {
		if data.Reactions[i].Keyword == "" || data.Reactions[i].Emoji == "" {
			return nil, fmt.Errorf("invalid reaction")
		}
		data.Reactions[i].ChatID = chatID
	}
	if err := replaceChatRows(tx, chatID, data.Reactions); err != nil {
		return nil, err
	}
	return []string{cacheKey("reactions", chatID)}, nil
}

func importReports(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data ReportsBackup
	if err := decodeModuleData(payload, BackupModuleReports, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatId = chatID
		data.Settings.Status = data.Settings.Enabled
	}
	return nil, replaceChatSetting(tx, chatID, data.Settings)
}

func importRules(tx *gorm.DB, chatID int64, payload interface{}) ([]string, error) {
	var data RulesBackup
	if err := decodeModuleData(payload, BackupModuleRules, &data); err != nil {
		return nil, err
	}
	if data.Settings != nil {
		data.Settings.ChatId = chatID
	}
	return nil, replaceChatSetting(tx, chatID, data.Settings)
}

func importWarns(tx *gorm.DB, chatID int64, payload interface{}, preserveLegacyOmissions bool) ([]string, error) {
	restoreWarns := !preserveLegacyOmissions || moduleFieldPresent(payload, "warns")
	var data WarnsBackup
	if err := decodeModuleData(payload, BackupModuleWarns, &data); err != nil {
		return nil, err
	}
	if data.WarnSettings != nil {
		data.WarnSettings.ChatId = chatID
		if data.WarnSettings.WarnLimit <= 0 {
			return nil, fmt.Errorf("invalid warn limit %d", data.WarnSettings.WarnLimit)
		}
	}
	for i := range data.Warns {
		if data.Warns[i].UserId == 0 || data.Warns[i].NumWarns < 0 {
			return nil, fmt.Errorf("invalid warn record")
		}
		data.Warns[i].ChatId = chatID
	}
	if len(data.Warns) > 0 {
		users := make([]models.User, len(data.Warns))
		for i := range data.Warns {
			users[i].UserId = data.Warns[i].UserId
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoNothing: true,
		}).Create(&users).Error; err != nil {
			return nil, fmt.Errorf("ensure warned users: %w", err)
		}
	}

	var oldUserIDs []int64
	if restoreWarns {
		if err := tx.Model(&models.Warns{}).Where("chat_id = ?", chatID).Pluck("user_id", &oldUserIDs).Error; err != nil {
			return nil, err
		}
	}
	if err := replaceChatSetting(tx, chatID, data.WarnSettings); err != nil {
		return nil, err
	}
	if restoreWarns {
		if err := replaceChatRows(tx, chatID, data.Warns); err != nil {
			return nil, err
		}
	}

	keys := []string{cacheKey("warn_settings", chatID)}
	for _, userID := range oldUserIDs {
		keys = append(keys, dbcache.CacheKey("warns", userID, chatID))
	}
	if restoreWarns {
		for _, row := range data.Warns {
			keys = append(keys, dbcache.CacheKey("warns", row.UserId, chatID))
		}
	}
	return keys, nil
}

func moduleFieldPresent(payload interface{}, field string) bool {
	raw, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	_, ok := fields[field]
	return ok
}

func clearModuleData(tx *gorm.DB, chatID int64, module string) ([]string, error) {
	if err := ensureBackupChat(tx, chatID); err != nil {
		return nil, err
	}
	switch module {
	case BackupModuleAdmin:
		return clearAdmin(tx, chatID)
	case BackupModuleAntiflood:
		return clearAntiflood(tx, chatID)
	case BackupModuleAntiraid:
		return clearAntiraid(tx, chatID)
	case BackupModuleApprovals:
		return clearApprovals(tx, chatID)
	case BackupModuleBlacklists:
		return clearBlacklists(tx, chatID)
	case BackupModuleCaptcha:
		return clearCaptcha(tx, chatID)
	case BackupModuleConnections:
		return clearConnections(tx, chatID)
	case BackupModuleDisabling:
		return clearDisabling(tx, chatID)
	case BackupModuleFilters:
		return clearFilters(tx, chatID)
	case BackupModuleGreetings:
		return clearGreetings(tx, chatID)
	case BackupModuleLocks:
		return clearLocks(tx, chatID)
	case BackupModuleNotes:
		return clearNotes(tx, chatID)
	case BackupModulePins:
		return clearPins(tx, chatID)
	case BackupModuleReactions:
		return clearReactions(tx, chatID)
	case BackupModuleReports:
		return clearReports(tx, chatID)
	case BackupModuleRules:
		return clearRules(tx, chatID)
	case BackupModuleWarns:
		return clearWarns(tx, chatID)
	default:
		return nil, fmt.Errorf("unknown module: %s", module)
	}
}

func ensureBackupChat(tx *gorm.DB, chatID int64) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoNothing: true,
	}).Create(&models.Chat{ChatId: chatID}).Error
}

func clearAdmin(tx *gorm.DB, chatID int64) ([]string, error) {
	if err := replaceChatSetting(tx, chatID, &models.AdminSettings{ChatId: chatID}); err != nil {
		return nil, err
	}
	if _, err := clearAntiflood(tx, chatID); err != nil {
		return nil, err
	}
	if _, err := clearCaptcha(tx, chatID); err != nil {
		return nil, err
	}
	if _, err := clearConnections(tx, chatID); err != nil {
		return nil, err
	}
	if err := tx.Model(&models.BlacklistSettings{}).
		Where("chat_id = ?", chatID).
		Update("action", "warn").Error; err != nil {
		return nil, err
	}
	return []string{
		cacheKey("antiflood", chatID),
		cacheKey("captcha_settings", chatID),
		cacheKey("blacklist", chatID),
	}, nil
}

func clearAntiflood(tx *gorm.DB, chatID int64) ([]string, error) {
	settings := &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: "mute"}
	return []string{cacheKey("antiflood", chatID)}, replaceChatSetting(tx, chatID, settings)
}

func clearAntiraid(tx *gorm.DB, chatID int64) ([]string, error) {
	settings := &models.AntiRaidSettings{
		ChatID:                chatID,
		RaidTime:              21600,
		RaidActionTime:        3600,
		AutoAntiRaidThreshold: 0,
	}
	return []string{cacheKey("antiraid", chatID)}, replaceChatSetting(tx, chatID, settings)
}

func clearApprovals(tx *gorm.DB, chatID int64) ([]string, error) {
	return []string{cacheKey("approvals", chatID)}, replaceChatRows[models.ApprovedUsers](tx, chatID, nil)
}

func clearBlacklists(tx *gorm.DB, chatID int64) ([]string, error) {
	return []string{cacheKey("blacklist", chatID)}, replaceChatRows[models.BlacklistSettings](tx, chatID, nil)
}

func clearCaptcha(tx *gorm.DB, chatID int64) ([]string, error) {
	settings := &models.CaptchaSettings{
		ChatID:        chatID,
		CaptchaMode:   "math",
		Timeout:       2,
		FailureAction: "kick",
		MaxAttempts:   3,
	}
	return []string{cacheKey("captcha_settings", chatID)}, replaceChatSetting(tx, chatID, settings)
}

func clearConnections(tx *gorm.DB, chatID int64) ([]string, error) {
	return nil, replaceChatSetting(tx, chatID, &models.ConnectionChatSettings{ChatId: chatID})
}

func clearDisabling(tx *gorm.DB, chatID int64) ([]string, error) {
	if err := replaceChatSetting(tx, chatID, &models.DisableChatSettings{ChatId: chatID}); err != nil {
		return nil, err
	}
	return []string{cacheKey("disabled_cmds", chatID)}, replaceChatRows[models.DisableSettings](tx, chatID, nil)
}

func clearFilters(tx *gorm.DB, chatID int64) ([]string, error) {
	return []string{
		cacheKey("filter_list", chatID),
		cacheKey("filters_optimized", chatID),
	}, replaceChatRows[models.ChatFilters](tx, chatID, nil)
}

func clearGreetings(tx *gorm.DB, chatID int64) ([]string, error) {
	settings := &models.GreetingSettings{
		ChatID: chatID,
		WelcomeSettings: &models.WelcomeSettings{
			WelcomeText: db.DefaultWelcome,
			WelcomeType: db.TEXT,
			Button:      models.ButtonArray{},
		},
		GoodbyeSettings: &models.GoodbyeSettings{
			GoodbyeText: db.DefaultGoodbye,
			GoodbyeType: db.TEXT,
			Button:      models.ButtonArray{},
		},
	}
	return []string{cacheKey("greetings", chatID)}, replaceChatSetting(tx, chatID, settings)
}

func clearLocks(tx *gorm.DB, chatID int64) ([]string, error) {
	return []string{
		cacheKey("lock", chatID),
		cacheKey("locks_map", chatID),
	}, replaceChatRows[models.LockSettings](tx, chatID, nil)
}

func clearNotes(tx *gorm.DB, chatID int64) ([]string, error) {
	if err := replaceChatSetting(tx, chatID, &models.NotesSettings{ChatId: chatID}); err != nil {
		return nil, err
	}
	return []string{cacheKey("notes_settings", chatID)}, replaceChatRows[models.Notes](tx, chatID, nil)
}

func clearPins(tx *gorm.DB, chatID int64) ([]string, error) {
	return nil, replaceChatSetting(tx, chatID, &models.PinSettings{ChatId: chatID})
}

func clearReactions(tx *gorm.DB, chatID int64) ([]string, error) {
	return []string{cacheKey("reactions", chatID)}, replaceChatRows[models.Reactions](tx, chatID, nil)
}

func clearReports(tx *gorm.DB, chatID int64) ([]string, error) {
	settings := &models.ReportChatSettings{
		ChatId:      chatID,
		Enabled:     true,
		Status:      true,
		BlockedList: models.Int64Array{},
	}
	return nil, replaceChatSetting(tx, chatID, settings)
}

func clearRules(tx *gorm.DB, chatID int64) ([]string, error) {
	return nil, replaceChatSetting(tx, chatID, &models.RulesSettings{ChatId: chatID})
}

func clearWarns(tx *gorm.DB, chatID int64) ([]string, error) {
	var userIDs []int64
	if err := tx.Model(&models.Warns{}).Where("chat_id = ?", chatID).Pluck("user_id", &userIDs).Error; err != nil {
		return nil, err
	}
	if err := replaceChatSetting(tx, chatID, &models.WarnSettings{ChatId: chatID, WarnLimit: 3}); err != nil {
		return nil, err
	}
	if err := replaceChatRows[models.Warns](tx, chatID, nil); err != nil {
		return nil, err
	}
	keys := []string{cacheKey("warn_settings", chatID)}
	for _, userID := range userIDs {
		keys = append(keys, dbcache.CacheKey("warns", userID, chatID))
	}
	return keys, nil
}
