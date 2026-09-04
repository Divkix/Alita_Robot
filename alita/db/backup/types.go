package backup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/divkix/Alita_Robot/alita/db/models"
)

const (
	BackupFormatVersion = "1.1"
	legacyFormatVersion = "1.0"
)

type BackupFormat struct {
	Version    string                 `json:"version"`
	ExportedAt time.Time              `json:"exported_at"`
	BotName    string                 `json:"bot_name"`
	ChatID     int64                  `json:"chat_id"`
	ChatName   string                 `json:"chat_name"`
	ExportedBy int64                  `json:"exported_by"`
	Modules    []string               `json:"modules"`
	Data       map[string]interface{} `json:"data"`
}

func NewBackupFormat(chatID int64, chatName string, exportedBy int64, modules []string) *BackupFormat {
	return &BackupFormat{
		Version:    BackupFormatVersion,
		ExportedAt: time.Now().UTC(),
		BotName:    "AlitaRobot",
		ChatID:     chatID,
		ChatName:   chatName,
		ExportedBy: exportedBy,
		Modules:    modules,
		Data:       make(map[string]interface{}),
	}
}

func (b *BackupFormat) Validate() error {
	if b == nil {
		return fmt.Errorf("backup cannot be nil")
	}
	if b.Version == "" {
		return fmt.Errorf("backup version is required")
	}
	if b.BotName == "" {
		return fmt.Errorf("bot name is required")
	}
	if b.ChatID == 0 {
		return fmt.Errorf("chat ID is required")
	}
	if len(b.Modules) == 0 {
		return fmt.Errorf("at least one module must be specified")
	}
	if b.Data == nil {
		return fmt.Errorf("data field cannot be nil")
	}
	for _, module := range b.Modules {
		if !IsValidModule(module) {
			return fmt.Errorf("unknown module: %s", module)
		}
		if _, ok := b.Data[module]; !ok {
			return fmt.Errorf("missing data for module: %s", module)
		}
	}
	return nil
}

func (b *BackupFormat) IsCompatibleVersion() bool {
	return b.Version == BackupFormatVersion || b.Version == legacyFormatVersion
}

func (b *BackupFormat) ToJSON() ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}

func BackupFormatFromJSON(data []byte) (*BackupFormat, error) {
	var backup BackupFormat
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&backup); err != nil {
		return nil, fmt.Errorf("failed to parse backup file: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("failed to parse backup file: trailing data")
	}
	return &backup, nil
}

const (
	BackupModuleAdmin       = "admin"
	BackupModuleAntiflood   = "antiflood"
	BackupModuleAntiraid    = "antiraid"
	BackupModuleApprovals   = "approvals"
	BackupModuleBlacklists  = "blacklists"
	BackupModuleCaptcha     = "captcha"
	BackupModuleConnections = "connections"
	BackupModuleDisabling   = "disabling"
	BackupModuleFilters     = "filters"
	BackupModuleGreetings   = "greetings"
	BackupModuleLocks       = "locks"
	BackupModuleNotes       = "notes"
	BackupModulePins        = "pins"
	BackupModuleReactions   = "reactions"
	BackupModuleReports     = "reports"
	BackupModuleRules       = "rules"
	BackupModuleWarns       = "warns"
	BackupModuleFederations = "federations"
	BackupModuleLogChannels = "logchannels"
)

func AllExportableModules() []string {
	return []string{
		BackupModuleAdmin,
		BackupModuleAntiflood,
		BackupModuleAntiraid,
		BackupModuleApprovals,
		BackupModuleBlacklists,
		BackupModuleCaptcha,
		BackupModuleConnections,
		BackupModuleDisabling,
		BackupModuleFilters,
		BackupModuleGreetings,
		BackupModuleLocks,
		BackupModuleNotes,
		BackupModulePins,
		BackupModuleReactions,
		BackupModuleReports,
		BackupModuleRules,
		BackupModuleWarns,
		BackupModuleFederations,
		BackupModuleLogChannels,
	}
}

func IsValidModule(module string) bool {
	for _, m := range AllExportableModules() {
		if m == module {
			return true
		}
	}
	return false
}

type AdminBackup struct {
	AdminSettings      *models.AdminSettings          `json:"admin_settings,omitempty"`
	AntifloodSettings  *models.AntifloodSettings      `json:"antiflood_settings,omitempty"`
	BlacklistMode      string                         `json:"blacklist_mode,omitempty"`
	CaptchaSettings    *models.CaptchaSettings        `json:"captcha_settings,omitempty"`
	ConnectionSettings *models.ConnectionChatSettings `json:"connection_settings,omitempty"`
}

type SettingBackup[T any] struct {
	Settings *T `json:"settings,omitempty"`
}

type AntifloodBackup = SettingBackup[models.AntifloodSettings]

type BlacklistsBackup struct {
	Settings      *models.BlacklistSettings  `json:"settings,omitempty"`
	BlacklistMode string                     `json:"blacklist_mode,omitempty"`
	Entries       []models.BlacklistSettings `json:"entries,omitempty"`
}

type CaptchaBackup = SettingBackup[models.CaptchaSettings]

type ConnectionsBackup = SettingBackup[models.ConnectionChatSettings]

type DisablingBackup struct {
	ChatSettings *models.DisableChatSettings `json:"chat_settings,omitempty"`
	Commands     []models.DisableSettings    `json:"commands,omitempty"`
}

type FiltersBackup struct {
	Filters []models.ChatFilters `json:"filters,omitempty"`
}

type GreetingsBackup = SettingBackup[models.GreetingSettings]

type LocksBackup struct {
	Locks []models.LockSettings `json:"locks,omitempty"`
}

type NotesBackup struct {
	Settings *models.NotesSettings `json:"settings,omitempty"`
	Notes    []models.Notes        `json:"notes,omitempty"`
}

type PinsBackup = SettingBackup[models.PinSettings]

type ReportsBackup = SettingBackup[models.ReportChatSettings]

type RulesBackup = SettingBackup[models.RulesSettings]

type WarnsBackup struct {
	WarnSettings *models.WarnSettings `json:"warn_settings,omitempty"`
	Warns        []models.Warns       `json:"warns,omitempty"`
}

type AntiraidBackup = SettingBackup[models.AntiRaidSettings]

type ApprovalsBackup struct {
	ApprovedUsers []models.ApprovedUsers `json:"approved_users,omitempty"`
}

type ReactionsBackup struct {
	Reactions []models.Reactions `json:"reactions,omitempty"`
}

type FederationsBackup struct {
	Membership *models.FederationChat `json:"membership,omitempty"`
}

type LogChannelsBackup = SettingBackup[models.LogChannel]
