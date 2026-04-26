package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// BackupFormatVersion is the current backup format version
const BackupFormatVersion = "1.0"

// BackupFormat represents the structure of an exported backup file
type BackupFormat struct {
	Version    string                 `json:"version"`     // Backup format version (e.g., "1.0")
	ExportedAt time.Time              `json:"exported_at"` // Timestamp of export
	BotName    string                 `json:"bot_name"`    // Bot identifier (e.g., "AlitaRobot")
	ChatID     int64                  `json:"chat_id"`     // Source chat ID
	ChatName   string                 `json:"chat_name"`   // Source chat name
	ExportedBy int64                  `json:"exported_by"` // User ID who exported
	Modules    []string               `json:"modules"`     // List of exported module names
	Data       map[string]interface{} `json:"data"`        // Module-specific data
}

// NewBackupFormat creates a new backup format instance
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

// Validate checks if the backup format is valid
func (b *BackupFormat) Validate() error {
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
	return nil
}

// IsCompatibleVersion checks if the backup version is compatible
func (b *BackupFormat) IsCompatibleVersion() bool {
	// For now, only support exact version match
	// Future: support migration from older versions
	return b.Version == BackupFormatVersion
}

// ToJSON marshals the backup format to JSON bytes
func (b *BackupFormat) ToJSON() ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}

// BackupFormatFromJSON unmarshals JSON bytes to BackupFormat
func BackupFormatFromJSON(data []byte) (*BackupFormat, error) {
	var backup BackupFormat
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("failed to parse backup file: %w", err)
	}
	return &backup, nil
}

// Module names for export/import
const (
	BackupModuleAdmin       = "admin"
	BackupModuleAntiflood   = "antiflood"
	BackupModuleBlacklists  = "blacklists"
	BackupModuleCaptcha     = "captcha"
	BackupModuleConnections = "connections"
	BackupModuleDisabling   = "disabling"
	BackupModuleFilters     = "filters"
	BackupModuleGreetings   = "greetings"
	BackupModuleLocks       = "locks"
	BackupModuleNotes       = "notes"
	BackupModulePins        = "pins"
	BackupModuleReports     = "reports"
	BackupModuleRules       = "rules"
	BackupModuleWarns       = "warns"
)

// AllExportableModules returns a list of all module names that support export
func AllExportableModules() []string {
	return []string{
		BackupModuleAdmin,
		BackupModuleAntiflood,
		BackupModuleBlacklists,
		BackupModuleCaptcha,
		BackupModuleConnections,
		BackupModuleDisabling,
		BackupModuleFilters,
		BackupModuleGreetings,
		BackupModuleLocks,
		BackupModuleNotes,
		BackupModulePins,
		BackupModuleReports,
		BackupModuleRules,
		BackupModuleWarns,
	}
}

// IsValidModule checks if a module name is valid for export
func IsValidModule(module string) bool {
	for _, m := range AllExportableModules() {
		if m == module {
			return true
		}
	}
	return false
}

// FilterValidModules returns only valid module names from a list
func FilterValidModules(modules []string) []string {
	var valid []string
	for _, m := range modules {
		if IsValidModule(m) {
			valid = append(valid, m)
		}
	}
	return valid
}

// Per-module backup data structures - using existing db types

// AdminBackup represents admin settings backup data
type AdminBackup struct {
	AdminSettings       *AdminSettings        `json:"admin_settings,omitempty"`
	AntifloodSettings   *AntifloodSettings    `json:"antiflood_settings,omitempty"`
	BlacklistMode       string                `json:"blacklist_mode,omitempty"`
	CaptchaSettings     *CaptchaSettings      `json:"captcha_settings,omitempty"`
	ConnectionSettings  *ConnectionChatSettings `json:"connection_settings,omitempty"`
}

// AntifloodBackup represents antiflood settings backup data
type AntifloodBackup struct {
	Settings *AntifloodSettings `json:"settings,omitempty"`
}

// BlacklistsBackup represents blacklist settings and entries backup data
type BlacklistsBackup struct {
	Settings      *BlacklistSettings   `json:"settings,omitempty"`
	BlacklistMode string               `json:"blacklist_mode,omitempty"`
	Entries       []BlacklistSettings  `json:"entries,omitempty"`
}

// CaptchaBackup represents captcha settings backup data
type CaptchaBackup struct {
	Settings *CaptchaSettings `json:"settings,omitempty"`
}

// ConnectionsBackup represents connection settings backup data
type ConnectionsBackup struct {
	Settings *ConnectionChatSettings `json:"settings,omitempty"`
}

// DisablingBackup represents disabled commands backup data
type DisablingBackup struct {
	ChatSettings *DisableChatSettings `json:"chat_settings,omitempty"`
	Commands     []DisableSettings    `json:"commands,omitempty"`
}

// FiltersBackup represents filters backup data
type FiltersBackup struct {
	Filters []ChatFilters `json:"filters,omitempty"`
}

// GreetingsBackup represents greetings/welcome settings backup data
type GreetingsBackup struct {
	Settings *GreetingSettings `json:"settings,omitempty"`
}

// LocksBackup represents lock settings backup data
type LocksBackup struct {
	Locks []LockSettings `json:"locks,omitempty"`
}

// NotesBackup represents notes backup data
type NotesBackup struct {
	Notes []Notes `json:"notes,omitempty"`
}

// PinsBackup represents pin settings backup data
type PinsBackup struct {
	Settings *PinSettings `json:"settings,omitempty"`
}

// ReportsBackup represents report settings backup data
type ReportsBackup struct {
	Settings *ReportChatSettings `json:"settings,omitempty"`
}

// RulesBackup represents rules backup data
type RulesBackup struct {
	Settings *RulesSettings `json:"settings,omitempty"`
}

// WarnsBackup represents warning settings backup data
type WarnsBackup struct {
	WarnSettings *WarnSettings `json:"warn_settings,omitempty"`
}