package db

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// Message type constants - maintain compatibility with existing code
const (
	// TEXT types of senders
	TEXT      int = 1
	STICKER   int = 2
	DOCUMENT  int = 3
	PHOTO     int = 4
	AUDIO     int = 5
	VOICE     int = 6
	VIDEO     int = 7
	VideoNote int = 8
)

// Default greeting messages - deprecated constants, use GetDefaultWelcome/GetDefaultGoodbye instead
const (
	DefaultWelcome = "Hey {first}, how are you?"
	DefaultGoodbye = "Sad to see you leaving {first}"
)

// Button represents a button structure used in filters, greetings, etc.
type Button struct {
	Name     string `gorm:"column:name" json:"name,omitempty"`
	Url      string `gorm:"column:url" json:"url,omitempty"`
	SameLine bool   `gorm:"column:btn_sameline;default:false" json:"btn_sameline" default:"false"`
}

// ButtonArray is a custom type for handling arrays of buttons as JSONB
type ButtonArray []Button

// Scan implements the Scanner interface for database deserialization of ButtonArray.
// It converts JSONB data from the database into a ButtonArray slice.
func (ba *ButtonArray) Scan(value any) error {
	if value == nil {
		*ba = ButtonArray{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, ba)
}

// Value implements the driver Valuer interface for database serialization of ButtonArray.
// It converts a ButtonArray slice to JSON for storage in the database.
func (ba ButtonArray) Value() (driver.Value, error) {
	if len(ba) == 0 {
		return "[]", nil
	}
	return json.Marshal(ba)
}

// StringArray is a custom type for handling arrays of strings as JSONB
type StringArray []string

// Scan implements the Scanner interface for database deserialization of StringArray.
// It converts JSONB data from the database into a StringArray slice.
func (sa *StringArray) Scan(value any) error {
	if value == nil {
		*sa = StringArray{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, sa)
}

// Value implements the driver Valuer interface for database serialization of StringArray.
// It converts a StringArray slice to JSON for storage in the database.
func (sa StringArray) Value() (driver.Value, error) {
	if len(sa) == 0 {
		return "[]", nil
	}
	return json.Marshal(sa)
}

// Int64Array is a custom type for handling arrays of int64 as JSONB
type Int64Array []int64

// Scan implements the Scanner interface for database deserialization of Int64Array.
// It converts JSONB data from the database into an Int64Array slice.
func (ia *Int64Array) Scan(value any) error {
	if value == nil {
		*ia = Int64Array{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, ia)
}

// Value implements the driver Valuer interface for database serialization of Int64Array.
// It converts an Int64Array slice to JSON for storage in the database.
func (ia Int64Array) Value() (driver.Value, error) {
	if len(ia) == 0 {
		return "[]", nil
	}
	return json.Marshal(ia)
}

// User represents a user in the system
type User struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId       int64     `gorm:"column:user_id;uniqueIndex;not null" json:"_id,omitempty"`
	UserName     string    `gorm:"column:username;index" json:"username" default:"nil"`
	Name         string    `gorm:"column:name" json:"name" default:"nil"`
	Language     string    `gorm:"column:language;default:'en'" json:"language" default:"en"`
	LastActivity time.Time `gorm:"column:last_activity" json:"last_activity,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`

	// Note: Chat membership is managed via JSONB users field in chats table
}

// TableName returns the database table name for the User model.
// This method overrides GORM's default table naming convention.
func (User) TableName() string {
	return "users"
}

// Chat represents a chat/group in the system
type Chat struct {
	ID           uint       `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId       int64      `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	ChatName     string     `gorm:"column:chat_name" json:"chat_name" default:"nil"`
	Language     string     `gorm:"column:language" json:"language" default:"nil"`
	Users        Int64Array `gorm:"column:users;type:jsonb" json:"users" default:"nil"`
	IsInactive   bool       `gorm:"column:is_inactive;default:false" json:"is_inactive" default:"false"`
	LastActivity time.Time  `gorm:"column:last_activity" json:"last_activity,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty"`

	// Note: User membership is managed via JSONB users field
}

// TableName returns the database table name for the Chat model.
// This method overrides GORM's default table naming convention.
func (Chat) TableName() string {
	return "chats"
}

// ChatUser represents the many-to-many relationship between chats and users
type ChatUser struct {
	ChatID int64 `gorm:"column:chat_id;primaryKey" json:"chat_id"`
	UserID int64 `gorm:"column:user_id;primaryKey" json:"user_id"`
}

// WarnSettings represents warning settings for a chat
type WarnSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	WarnLimit int       `gorm:"column:warn_limit;default:3" json:"warn_limit" default:"3"`
	WarnMode  string    `gorm:"column:warn_mode" json:"warn_mode,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the WarnSettings model.
// This method overrides GORM's default table naming convention.
func (WarnSettings) TableName() string {
	return "warns_settings"
}

// Warns represents user warnings in a chat
type Warns struct {
	ID        uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64       `gorm:"column:user_id;not null;index:idx_warns_user_chat" json:"user_id,omitempty"`
	ChatId    int64       `gorm:"column:chat_id;not null;index:idx_warns_user_chat" json:"chat_id,omitempty"`
	NumWarns  int         `gorm:"column:num_warns;default:0" json:"num_warns,omitempty"`
	Reasons   StringArray `gorm:"column:warns;type:jsonb" json:"warns" default:"[]"`
	CreatedAt time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the Warns model.
// This method overrides GORM's default table naming convention.
func (Warns) TableName() string {
	return "warns_users"
}

// WelcomeSettings represents welcome message settings
type WelcomeSettings struct {
	CleanWelcome  bool        `gorm:"column:clean_old;default:false" json:"clean_old" default:"false"`
	LastMsgId     int64       `gorm:"column:last_msg_id" json:"last_msg_id,omitempty"`
	ShouldWelcome bool        `gorm:"column:enabled;default:true" json:"welcome_enabled" default:"true"`
	WelcomeText   string      `gorm:"column:text" json:"welcome_text,omitempty"`
	FileID        string      `gorm:"column:file_id" json:"file_id,omitempty"`
	WelcomeType   int         `gorm:"column:type;default:1" json:"welcome_type,omitempty"`
	Button        ButtonArray `gorm:"column:btns;type:jsonb" json:"btns,omitempty"`
}

// GoodbyeSettings represents goodbye message settings
type GoodbyeSettings struct {
	CleanGoodbye  bool        `gorm:"column:clean_old;default:false" json:"clean_old" default:"false"`
	LastMsgId     int64       `gorm:"column:last_msg_id" json:"last_msg_id,omitempty"`
	ShouldGoodbye bool        `gorm:"column:enabled;default:true" json:"enabled" default:"true"`
	GoodbyeText   string      `gorm:"column:text" json:"text,omitempty"`
	FileID        string      `gorm:"column:file_id" json:"file_id,omitempty"`
	GoodbyeType   int         `gorm:"column:type;default:1" json:"type,omitempty"`
	Button        ButtonArray `gorm:"column:btns;type:jsonb" json:"btns,omitempty"`
}

// GreetingSettings represents greeting settings for a chat
type GreetingSettings struct {
	ID                 uint             `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID             int64            `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	ShouldCleanService bool             `gorm:"column:clean_service_settings;default:false" json:"clean_service_settings" default:"false"`
	WelcomeSettings    *WelcomeSettings `gorm:"embedded;embeddedPrefix:welcome_" json:"welcome_settings" default:"false"`
	GoodbyeSettings    *GoodbyeSettings `gorm:"embedded;embeddedPrefix:goodbye_" json:"goodbye_settings" default:"false"`
	ShouldAutoApprove  bool             `gorm:"column:auto_approve;default:false" json:"auto_approve" default:"false"`
	CreatedAt          time.Time        `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt          time.Time        `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the GreetingSettings model.
// This method overrides GORM's default table naming convention.
func (GreetingSettings) TableName() string {
	return "greetings"
}

// ChatFilters represents chat filters
type ChatFilters struct {
	ID          uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId      int64       `gorm:"column:chat_id;not null;index:idx_filters_chat_keyword" json:"chat_id,omitempty"`
	KeyWord     string      `gorm:"column:keyword;not null;index:idx_filters_chat_keyword" json:"keyword,omitempty"`
	FilterReply string      `gorm:"column:filter_reply" json:"filter_reply,omitempty"`
	MsgType     int         `gorm:"column:msgtype" json:"msgtype,omitempty"`
	FileID      string      `gorm:"column:fileid" json:"fileid,omitempty"`
	NoNotif     bool        `gorm:"column:nonotif;default:false" json:"nonotif,omitempty"`
	Buttons     ButtonArray `gorm:"column:filter_buttons;type:jsonb" json:"filter_buttons,omitempty"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the ChatFilters model.
// This method overrides GORM's default table naming convention.
func (ChatFilters) TableName() string {
	return "filters"
}

// AdminSettings represents admin settings for a chat
type AdminSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"_id,omitempty"`
	AnonAdmin bool      `gorm:"column:anon_admin;default:false" json:"anon_admin"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the AdminSettings model.
// This method overrides GORM's default table naming convention.
func (AdminSettings) TableName() string {
	return "admin"
}

// BlacklistSettings represents blacklist settings for a chat
type BlacklistSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;not null;index:idx_blacklist_chat_word" json:"chat_id,omitempty"`
	Word      string    `gorm:"column:word;not null;index:idx_blacklist_chat_word" json:"word,omitempty"`
	Action    string    `gorm:"column:action;default:'warn'" json:"action,omitempty"`
	Reason    string    `gorm:"column:reason" json:"reason,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// BlacklistSettingsSlice is a custom type for []*BlacklistSettings with additional methods
type BlacklistSettingsSlice []*BlacklistSettings

// Triggers returns all blacklisted words as a slice of strings for compatibility.
// This method extracts the Word field from each BlacklistSettings in the slice.
func (bss BlacklistSettingsSlice) Triggers() []string {
	var triggers []string
	for _, bs := range bss {
		triggers = append(triggers, bs.Word)
	}
	return triggers
}

// Action returns the action for the first blacklist setting in the slice.
// All blacklist settings for a chat should have the same action, so we return the first one.
func (bss BlacklistSettingsSlice) Action() string {
	if len(bss) > 0 {
		return bss[0].Action
	}
	return "warn" // default
}

// Reason returns the reason for the first blacklist setting in the slice.
// If no settings exist or reason is empty, returns a default format string with placeholder for trigger word.
func (bss BlacklistSettingsSlice) Reason() string {
	if len(bss) > 0 && bss[0].Reason != "" {
		return bss[0].Reason
	}
	return "Blacklisted word: '%s'" // default format string with placeholder for trigger word
}

// TableName returns the database table name for the BlacklistSettings model.
// This method overrides GORM's default table naming convention.
func (BlacklistSettings) TableName() string {
	return "blacklists"
}

// PinSettings represents pin settings for a chat
type PinSettings struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId         int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	MsgId          int64     `gorm:"column:msg_id" json:"msg_id,omitempty"`
	CleanLinked    bool      `gorm:"column:clean_linked;default:false" json:"clean_linked,omitempty"`
	AntiChannelPin bool      `gorm:"column:anti_channel_pin;default:false" json:"anti_channel_pin,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the PinSettings model.
// This method overrides GORM's default table naming convention.
func (PinSettings) TableName() string {
	return "pins"
}

// ReportChatSettings represents report settings for a chat
type ReportChatSettings struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId      int64      `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Enabled     bool       `gorm:"column:enabled;default:true" json:"enabled,omitempty"`
	Status      bool       `gorm:"column:status;default:true" json:"status,omitempty"`           // Alias for Enabled for compatibility
	BlockedList Int64Array `gorm:"column:blocked_list;type:jsonb" json:"blocked_list,omitempty"` // List of blocked user IDs
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the ReportChatSettings model.
// This method overrides GORM's default table naming convention.
func (ReportChatSettings) TableName() string {
	return "report_chat_settings"
}

// ReportUserSettings represents report settings for a user
type ReportUserSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64     `gorm:"column:user_id;uniqueIndex;not null" json:"user_id,omitempty"`
	Enabled   bool      `gorm:"column:enabled;default:true" json:"enabled,omitempty"`
	Status    bool      `gorm:"column:status;default:true" json:"status,omitempty"` // Alias for Enabled for compatibility
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the ReportUserSettings model.
// This method overrides GORM's default table naming convention.
func (ReportUserSettings) TableName() string {
	return "report_user_settings"
}

// DevSettings represents developer settings
type DevSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64     `gorm:"column:user_id;uniqueIndex;not null" json:"user_id,omitempty"`
	IsDev     bool      `gorm:"column:is_dev;default:false" json:"is_dev,omitempty"`
	Dev       bool      `gorm:"column:dev;default:false" json:"dev,omitempty"`   // Alias for IsDev for compatibility
	Sudo      bool      `gorm:"column:sudo;default:false" json:"sudo,omitempty"` // Sudo privileges
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the DevSettings model.
// This method overrides GORM's default table naming convention.
func (DevSettings) TableName() string {
	return "devs"
}

// ChannelSettings represents channel settings including channel metadata
type ChannelSettings struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId      int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	ChannelId   int64     `gorm:"column:channel_id" json:"channel_id,omitempty"`
	ChannelName string    `gorm:"column:channel_name" json:"channel_name,omitempty"`
	Username    string    `gorm:"column:username" json:"username,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the ChannelSettings model.
// This method overrides GORM's default table naming convention.
func (ChannelSettings) TableName() string {
	return "channels"
}

// AntifloodSettings represents antiflood settings for a chat
type AntifloodSettings struct {
	ID                     uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId                 int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Limit                  int       `gorm:"column:flood_limit;default:5" json:"limit,omitempty"`
	Action                 string    `gorm:"column:action;default:'mute'" json:"action,omitempty"`
	Mode                   string    `gorm:"column:mode;default:'mute'" json:"mode,omitempty"` // Alias for Action for compatibility
	DeleteAntifloodMessage bool      `gorm:"column:delete_antiflood_message;default:false" json:"delete_antiflood_message,omitempty"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt              time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the AntifloodSettings model.
// This method overrides GORM's default table naming convention.
func (AntifloodSettings) TableName() string {
	return "antiflood_settings"
}

// ConnectionSettings represents connection settings
type ConnectionSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64     `gorm:"column:user_id;not null;index:idx_connection_user_chat" json:"user_id,omitempty"`
	ChatId    int64     `gorm:"column:chat_id;not null;index:idx_connection_user_chat" json:"chat_id,omitempty"`
	Connected bool      `gorm:"column:connected;default:false" json:"connected,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the ConnectionSettings model.
// This method overrides GORM's default table naming convention.
func (ConnectionSettings) TableName() string {
	return "connection"
}

// ConnectionChatSettings represents connection chat settings for a chat.
// AllowConnect determines whether users can connect to this chat remotely.
type ConnectionChatSettings struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId       int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	AllowConnect bool      `gorm:"column:allow_connect;default:true" json:"allow_connect,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the ConnectionChatSettings model.
// This method overrides GORM's default table naming convention.
func (ConnectionChatSettings) TableName() string {
	return "connection_settings"
}

// DisableSettings represents disable settings for commands
type DisableSettings struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId         int64     `gorm:"column:chat_id;not null;index:idx_disable_chat_command" json:"chat_id,omitempty"`
	Command        string    `gorm:"column:command;not null;index:idx_disable_chat_command" json:"command,omitempty"`
	Disabled       bool      `gorm:"column:disabled;default:true" json:"disabled,omitempty"`
	DeleteCommands bool      `gorm:"column:delete_commands;default:false" json:"delete_commands,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the DisableSettings model.
// This method overrides GORM's default table naming convention.
func (DisableSettings) TableName() string {
	return "disable"
}

// DisableChatSettings represents chat-level disable settings
type DisableChatSettings struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId         int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	DeleteCommands bool      `gorm:"column:delete_commands;default:false" json:"delete_commands,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the DisableChatSettings model.
// This method overrides GORM's default table naming convention.
func (DisableChatSettings) TableName() string {
	return "disable_chat_settings"
}

// RulesSettings represents rules settings for a chat
type RulesSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Rules     string    `gorm:"column:rules;type:text" json:"rules,omitempty"`
	RulesBtn  string    `gorm:"column:rules_btn" json:"rules_btn,omitempty"`
	Private   bool      `gorm:"column:private;default:false" json:"private,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the RulesSettings model.
// This method overrides GORM's default table naming convention.
func (RulesSettings) TableName() string {
	return "rules"
}

// LockSettings represents lock settings for a chat
type LockSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;not null;uniqueIndex:idx_lock_chat_type" json:"chat_id,omitempty"`
	LockType  string    `gorm:"column:lock_type;not null;uniqueIndex:idx_lock_chat_type" json:"lock_type,omitempty"`
	Locked    bool      `gorm:"column:locked;default:false" json:"locked,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the LockSettings model.
// This method overrides GORM's default table naming convention.
func (LockSettings) TableName() string {
	return "locks"
}

// NotesSettings represents notes settings for a chat
type NotesSettings struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId    int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Private   bool      `gorm:"column:private;default:false" json:"private,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// PrivateNotesEnabled returns whether private notes are enabled for the chat.
// This method provides compatibility with existing code that expects this method name.
func (ns *NotesSettings) PrivateNotesEnabled() bool {
	return ns.Private
}

// TableName returns the database table name for the NotesSettings model.
// This method overrides GORM's default table naming convention.
func (NotesSettings) TableName() string {
	return "notes_settings"
}

// Notes represents notes in a chat
type Notes struct {
	ID          uint        `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatId      int64       `gorm:"column:chat_id;not null;index:idx_notes_chat_name" json:"chat_id,omitempty"`
	NoteName    string      `gorm:"column:note_name;not null;index:idx_notes_chat_name" json:"note_name,omitempty"`
	NoteContent string      `gorm:"column:note_content;type:text" json:"note_content,omitempty"`
	FileID      string      `gorm:"column:file_id" json:"file_id,omitempty"`
	MsgType     int         `gorm:"column:msg_type" json:"msg_type,omitempty"`
	Buttons     ButtonArray `gorm:"column:buttons;type:jsonb" json:"buttons,omitempty"`
	AdminOnly   bool        `gorm:"column:admin_only;default:false" json:"admin_only,omitempty"`
	PrivateOnly bool        `gorm:"column:private_only;default:false" json:"private_only,omitempty"`
	GroupOnly   bool        `gorm:"column:group_only;default:false" json:"group_only,omitempty"`
	WebPreview  bool        `gorm:"column:web_preview;default:true" json:"web_preview,omitempty"`
	IsProtected bool        `gorm:"column:is_protected;default:false" json:"is_protected,omitempty"`
	NoNotif     bool        `gorm:"column:no_notif;default:false" json:"no_notif,omitempty"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt   time.Time   `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the Notes model.
// This method overrides GORM's default table naming convention.
func (Notes) TableName() string {
	return "notes"
}

// CaptchaSettings represents captcha settings for a chat
type CaptchaSettings struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	ChatID        int64     `gorm:"column:chat_id;uniqueIndex;not null" json:"chat_id,omitempty"`
	Enabled       bool      `gorm:"column:enabled;default:false" json:"enabled,omitempty"`
	CaptchaMode   string    `gorm:"column:captcha_mode;default:'math'" json:"captcha_mode,omitempty"`     // math or text
	Timeout       int       `gorm:"column:timeout;default:2" json:"timeout,omitempty"`                    // minutes
	FailureAction string    `gorm:"column:failure_action;default:'kick'" json:"failure_action,omitempty"` // kick, ban, or mute
	MaxAttempts   int       `gorm:"column:max_attempts;default:3" json:"max_attempts,omitempty"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the CaptchaSettings model.
// This method overrides GORM's default table naming convention.
func (CaptchaSettings) TableName() string {
	return "captcha_settings"
}

// CaptchaAttempts represents active captcha attempts for users
type CaptchaAttempts struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID       int64     `gorm:"column:user_id;not null;index:idx_captcha_user_chat" json:"user_id,omitempty"`
	ChatID       int64     `gorm:"column:chat_id;not null;index:idx_captcha_user_chat" json:"chat_id,omitempty"`
	Answer       string    `gorm:"column:answer;not null" json:"answer,omitempty"`
	Attempts     int       `gorm:"column:attempts;default:0" json:"attempts,omitempty"`
	MessageID    int64     `gorm:"column:message_id" json:"message_id,omitempty"`
	RefreshCount int       `gorm:"column:refresh_count;default:0" json:"refresh_count,omitempty"`
	ExpiresAt    time.Time `gorm:"column:expires_at;not null" json:"expires_at,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the CaptchaAttempts model.
// This method overrides GORM's default table naming convention.
func (CaptchaAttempts) TableName() string {
	return "captcha_attempts"
}

// StoredMessages represents messages sent by users before completing captcha verification
type StoredMessages struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID      int64     `gorm:"column:user_id;not null;index:idx_stored_user_chat" json:"user_id,omitempty"`
	ChatID      int64     `gorm:"column:chat_id;not null;index:idx_stored_user_chat" json:"chat_id,omitempty"`
	MessageType int       `gorm:"column:message_type;not null;default:1" json:"message_type,omitempty"` // TEXT, STICKER, etc.
	Content     string    `gorm:"column:content;type:text" json:"content,omitempty"`
	FileID      string    `gorm:"column:file_id" json:"file_id,omitempty"`                // For media messages
	Caption     string    `gorm:"column:caption;type:text" json:"caption,omitempty"`      // For media captions
	AttemptID   uint      `gorm:"column:attempt_id;not null" json:"attempt_id,omitempty"` // Foreign key to CaptchaAttempts
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
}

// TableName returns the database table name for the StoredMessages model.
// This method overrides GORM's default table naming convention.
func (StoredMessages) TableName() string {
	return "stored_messages"
}

// CaptchaMutedUsers tracks users who failed captcha with mute action
// They will be automatically unmuted after UnmuteAt time
type CaptchaMutedUsers struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_captcha_muted_user_chat" json:"user_id,omitempty"`
	ChatID    int64     `gorm:"column:chat_id;not null;index:idx_captcha_muted_user_chat" json:"chat_id,omitempty"`
	UnmuteAt  time.Time `gorm:"column:unmute_at;not null;index:idx_captcha_unmute_at" json:"unmute_at,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at,omitempty"`
}

// TableName returns the database table name for the CaptchaMutedUsers model.
// This method overrides GORM's default table naming convention.
func (CaptchaMutedUsers) TableName() string {
	return "captcha_muted_users"
}

// Database instance
var DB *gorm.DB

// Initialize database connection and auto-migrate
func init() {
	var err error

	// Configure GORM logger
	gormLogger := logger.New(
		log.StandardLogger(),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// Open PostgreSQL connection using DATABASE_URL with retry logic
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		DB, err = gorm.Open(postgres.Open(config.AppConfig.DatabaseURL), &gorm.Config{
			Logger:      gormLogger,
			PrepareStmt: true, // Enable prepared statement caching for better performance
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
		})
		if err == nil {
			break
		}

		log.WithFields(log.Fields{
			"attempt": attempt + 1,
			"error":   err,
		}).Warning("[Database][Connection] Failed to connect, retrying...")

		if attempt < maxRetries-1 {
			// Exponential backoff: 1s, 2s, 4s, 8s
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}
	if err != nil {
		log.Fatalf("[Database][Connection] Failed after %d attempts: %v", maxRetries, err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("[Database][SQL DB]: %v", err)
	}

	// Configure connection pool with configurable values
	sqlDB.SetMaxIdleConns(config.AppConfig.DBMaxIdleConns)
	sqlDB.SetMaxOpenConns(config.AppConfig.DBMaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.AppConfig.DBConnMaxLifetimeMin) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(config.AppConfig.DBConnMaxIdleTimeMin) * time.Minute)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("[Database][Ping]: %v", err)
	}

	log.Info("Connected to PostgreSQL database successfully!")

	// Check if auto-migration is enabled
	if config.AppConfig.AutoMigrate {
		log.Info("[Database] AUTO_MIGRATE is enabled, running database migrations...")
		runner := NewMigrationRunner(DB)
		if err := runner.RunMigrations(); err != nil {
			if config.AppConfig.AutoMigrateSilentFail {
				log.Errorf("[Database][AutoMigrate] Migration failed but continuing (AUTO_MIGRATE_SILENT_FAIL=true): %v", err)
			} else {
				log.Fatalf("[Database][AutoMigrate] Migration failed: %v", err)
			}
		} else {
			log.Info("[Database][AutoMigrate] All migrations applied successfully")
		}
	} else {
		// Note: GORM AutoMigrate is disabled because we use SQL migrations in migrations/
		// This prevents constraint naming conflicts between GORM's naming convention (uni_*)
		// and our SQL migrations (uk_*). Database schema is managed via SQL migration files.
		log.Info("Database schema managed via SQL migrations - skipping auto-migration (set AUTO_MIGRATE=true to enable)")
	}
}

// Helper functions for GORM-specific operations

// getSpanAttributes returns common span attributes for database operations
func getSpanAttributes(model any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{}
	if model != nil {
		attrs = append(attrs, attribute.String("db.model", fmt.Sprintf("%T", model)))
	}
	return attrs
}

// CreateRecord creates a new database record using the provided model.
// It logs any errors that occur during the creation process.
func CreateRecord(model any) error {
	return CreateRecordWithContext(context.Background(), model)
}

// CreateRecordWithContext creates a new database record with context support for trace propagation.
// The provided context is used for both span parenting and GORM query-level context.
func CreateRecordWithContext(ctx context.Context, model any) error {
	ctx, span := tracing.StartSpan(ctx, "db.create",
		trace.WithAttributes(getSpanAttributes(model)...))
	defer span.End()

	result := DB.WithContext(ctx).Create(model)
	if result.Error != nil {
		log.Errorf("[Database][CreateRecord]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}

// UpdateRecord updates an existing database record with the provided updates.
// It uses the where clause to find the record and applies the updates map.
// NOTE: This function skips zero values when updating with structs. Use UpdateRecordWithZeroValues
// if you need to update boolean fields to false or other zero values.
func UpdateRecord(model any, where any, updates any) error {
	return UpdateRecordWithContext(context.Background(), model, where, updates)
}

// UpdateRecordWithContext updates a database record with context support for trace propagation.
// The provided context is used for both span parenting and GORM query-level context.
func UpdateRecordWithContext(ctx context.Context, model any, where any, updates any) error {
	ctx, span := tracing.StartSpan(ctx, "db.update",
		trace.WithAttributes(getSpanAttributes(model)...))
	defer span.End()

	result := DB.WithContext(ctx).Model(model).Where(where).Updates(updates)
	if result.Error != nil {
		log.Errorf("[Database][UpdateRecord]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		span.SetStatus(codes.Error, "record not found")
		return gorm.ErrRecordNotFound
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}

// UpdateRecordWithZeroValues updates a database record including zero values (false, 0, "").
// Updates must be a map[string]any to ensure zero values are persisted correctly.
// Maps bypass GORM's zero-value skip logic, unlike structs.
// Returns gorm.ErrRecordNotFound if no matching record exists.
func UpdateRecordWithZeroValues(model any, where any, updates map[string]any) error {
	return UpdateRecordWithZeroValuesWithContext(context.Background(), model, where, updates)
}

// UpdateRecordWithZeroValuesWithContext updates a database record including zero values with context support.
// The provided context is used for both span parenting and GORM query-level context.
func UpdateRecordWithZeroValuesWithContext(ctx context.Context, model any, where any, updates map[string]any) error {
	ctx, span := tracing.StartSpan(ctx, "db.update",
		trace.WithAttributes(getSpanAttributes(model)...))
	defer span.End()

	result := DB.WithContext(ctx).Model(model).Where(where).Updates(updates)
	if result.Error != nil {
		log.Errorf("[Database][UpdateRecordWithZeroValues]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		span.SetStatus(codes.Error, "record not found")
		return gorm.ErrRecordNotFound
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}

// Close closes the database connection gracefully.
// This should be called during application shutdown to properly close all database connections.
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get underlying SQL DB: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}

// GetRecord retrieves a single database record matching the where clause.
// Returns gorm.ErrRecordNotFound if no matching record is found.
func GetRecord(model any, where any) error {
	return GetRecordWithContext(context.Background(), model, where)
}

// GetRecordWithContext retrieves a single database record with context support for trace propagation.
// The provided context is used for both span parenting and GORM query-level context.
func GetRecordWithContext(ctx context.Context, model any, where any) error {
	ctx, span := tracing.StartSpan(ctx, "db.get",
		trace.WithAttributes(getSpanAttributes(model)...))
	defer span.End()

	result := DB.WithContext(ctx).Where(where).First(model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			span.SetAttributes(attribute.Bool("db.record_found", false))
			return result.Error
		}
		log.Errorf("[Database][GetRecord]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	span.SetAttributes(attribute.Bool("db.record_found", true))
	return nil
}

// ChatExists checks if a chat with the given ID exists in the database.
// Returns true if the chat exists, false otherwise.
func ChatExists(chatID int64) bool {
	chatExists := &Chat{}
	err := GetRecord(chatExists, Chat{ChatId: chatID})
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

// GetRecords retrieves multiple database records matching the where clause.
// The results are stored in the provided models slice.
func GetRecords(models any, where any) error {
	return GetRecordsWithContext(context.Background(), models, where)
}

// GetRecordsWithContext retrieves multiple database records with context support for trace propagation.
// The provided context is used for both span parenting and GORM query-level context.
func GetRecordsWithContext(ctx context.Context, models any, where any) error {
	ctx, span := tracing.StartSpan(ctx, "db.find",
		trace.WithAttributes(getSpanAttributes(models)...))
	defer span.End()

	result := DB.WithContext(ctx).Where(where).Find(models)
	if result.Error != nil {
		log.Errorf("[Database][GetRecords]: %v", result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
		return result.Error
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", result.RowsAffected))
	return nil
}
