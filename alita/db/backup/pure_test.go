package backup

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// BackupFormat
// ---------------------------------------------------------------------------

func TestNewBackupFormat(t *testing.T) {

	chatID := int64(123456)
	chatName := "Test Chat"
	exportedBy := int64(789)
	modules := []string{"admin", "filters"}

	bf := NewBackupFormat(chatID, chatName, exportedBy, modules)

	if bf.Version != BackupFormatVersion {
		t.Fatalf("Version = %q, want %q", bf.Version, BackupFormatVersion)
	}
	if bf.BotName != "AlitaRobot" {
		t.Fatalf("BotName = %q, want %q", bf.BotName, "AlitaRobot")
	}
	if bf.ChatID != chatID {
		t.Fatalf("ChatID = %d, want %d", bf.ChatID, chatID)
	}
	if bf.ChatName != chatName {
		t.Fatalf("ChatName = %q, want %q", bf.ChatName, chatName)
	}
	if bf.ExportedBy != exportedBy {
		t.Fatalf("ExportedBy = %d, want %d", bf.ExportedBy, exportedBy)
	}
	if len(bf.Modules) != len(modules) {
		t.Fatalf("Modules len = %d, want %d", len(bf.Modules), len(modules))
	}
	if bf.Data == nil {
		t.Fatal("Data should be initialized to non-nil map")
	}
	if bf.ExportedAt.IsZero() {
		t.Fatal("ExportedAt should be set to current time")
	}
}

func TestBackupFormat_Validate(t *testing.T) {

	now := time.Now().UTC()

	tests := []struct {
		name    string
		bf      *BackupFormat
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid backup format returns no error",
			bf: &BackupFormat{
				Version:    BackupFormatVersion,
				BotName:    "AlitaRobot",
				ChatID:     123,
				Modules:    []string{"admin"},
				Data:       make(map[string]interface{}),
				ExportedAt: now,
			},
			wantErr: false,
		},
		{
			name: "empty version returns error",
			bf: &BackupFormat{
				Version:    "",
				BotName:    "AlitaRobot",
				ChatID:     123,
				Modules:    []string{"admin"},
				Data:       make(map[string]interface{}),
				ExportedAt: now,
			},
			wantErr: true,
			errMsg:  "backup version is required",
		},
		{
			name: "empty bot name returns error",
			bf: &BackupFormat{
				Version:    BackupFormatVersion,
				BotName:    "",
				ChatID:     123,
				Modules:    []string{"admin"},
				Data:       make(map[string]interface{}),
				ExportedAt: now,
			},
			wantErr: true,
			errMsg:  "bot name is required",
		},
		{
			name: "zero chat ID returns error",
			bf: &BackupFormat{
				Version:    BackupFormatVersion,
				BotName:    "AlitaRobot",
				ChatID:     0,
				Modules:    []string{"admin"},
				Data:       make(map[string]interface{}),
				ExportedAt: now,
			},
			wantErr: true,
			errMsg:  "chat ID is required",
		},
		{
			name: "empty modules returns error",
			bf: &BackupFormat{
				Version:    BackupFormatVersion,
				BotName:    "AlitaRobot",
				ChatID:     123,
				Modules:    []string{},
				Data:       make(map[string]interface{}),
				ExportedAt: now,
			},
			wantErr: true,
			errMsg:  "at least one module must be specified",
		},
		{
			name: "nil data returns error",
			bf: &BackupFormat{
				Version:    BackupFormatVersion,
				BotName:    "AlitaRobot",
				ChatID:     123,
				Modules:    []string{"admin"},
				Data:       nil,
				ExportedAt: now,
			},
			wantErr: true,
			errMsg:  "data field cannot be nil",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.bf.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Error() != tc.errMsg {
					t.Fatalf("error = %q, want %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBackupFormat_IsCompatibleVersion(t *testing.T) {

	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "exact version match returns true",
			version: BackupFormatVersion,
			want:    true,
		},
		{
			name:    "different version returns false",
			version: "2.0",
			want:    false,
		},
		{
			name:    "empty version returns false",
			version: "",
			want:    false,
		},
		{
			name:    "older version returns false",
			version: "0.9",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			bf := &BackupFormat{
				Version:    tc.version,
				BotName:    "AlitaRobot",
				ChatID:     123,
				Modules:    []string{"admin"},
				Data:       make(map[string]interface{}),
				ExportedAt: time.Now().UTC(),
			}
			got := bf.IsCompatibleVersion()
			if got != tc.want {
				t.Fatalf("IsCompatibleVersion() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBackupFormat_ToJSON(t *testing.T) {

	bf := NewBackupFormat(123, "Test", 456, []string{"admin"})
	bf.Data["admin"] = map[string]interface{}{"anon_admin": true}

	data, err := bf.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	// Verify valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ToJSON() produced invalid JSON: %v", err)
	}

	// Verify key fields present
	if parsed["version"] != BackupFormatVersion {
		t.Fatalf("JSON version = %v, want %v", parsed["version"], BackupFormatVersion)
	}
	if parsed["bot_name"] != "AlitaRobot" {
		t.Fatalf("JSON bot_name = %v, want %v", parsed["bot_name"], "AlitaRobot")
	}
	if parsed["chat_id"] != float64(123) {
		t.Fatalf("JSON chat_id = %v, want 123", parsed["chat_id"])
	}

	// Verify indent formatting (contains newlines for readability)
	if !strings.Contains(string(data), "\n") {
		t.Fatalf("JSON missing indentation/newlines: got %q", string(data))
	}
}

func TestBackupFormatFromJSON(t *testing.T) {

	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantVer string
		wantID  int64
	}{
		{
			name:    "valid JSON parses correctly",
			input:   `{"version":"1.0","bot_name":"AlitaRobot","chat_id":123,"chat_name":"Test","exported_by":456,"modules":["admin"],"data":{"admin":true},"exported_at":"2024-01-01T00:00:00Z"}`,
			wantErr: false,
			wantVer: "1.0",
			wantID:  123,
		},
		{
			name:    "invalid JSON returns error",
			input:   `not json at all`,
			wantErr: true,
		},
		{
			name:    "empty JSON returns error",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "minimal valid JSON parses",
			input:   `{"version":"2.0","bot_name":"TestBot","chat_id":789,"exported_by":0,"modules":["filters"],"data":{},"exported_at":"2024-06-01T12:00:00Z"}`,
			wantErr: false,
			wantVer: "2.0",
			wantID:  789,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			bf, err := BackupFormatFromJSON([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bf.Version != tc.wantVer {
				t.Fatalf("Version = %q, want %q", bf.Version, tc.wantVer)
			}
			if bf.ChatID != tc.wantID {
				t.Fatalf("ChatID = %d, want %d", bf.ChatID, tc.wantID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Backup module helpers
// ---------------------------------------------------------------------------

func TestAllExportableModules(t *testing.T) {

	modules := AllExportableModules()
	if len(modules) == 0 {
		t.Fatal("AllExportableModules() returned empty slice")
	}

	expected := []string{
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
		BackupModuleReports,
		BackupModuleRules,
		BackupModuleWarns,
	}

	if len(modules) != len(expected) {
		t.Fatalf("AllExportableModules() len = %d, want %d", len(modules), len(expected))
	}

	gotSet := make(map[string]bool, len(modules))
	for _, m := range modules {
		gotSet[m] = true
	}
	for _, e := range expected {
		if !gotSet[e] {
			t.Fatalf("AllExportableModules() missing expected module %q", e)
		}
	}
}

func TestIsValidModule(t *testing.T) {

	tests := []struct {
		name   string
		module string
		want   bool
	}{
		{
			name:   "valid module admin returns true",
			module: BackupModuleAdmin,
			want:   true,
		},
		{
			name:   "valid module filters returns true",
			module: BackupModuleFilters,
			want:   true,
		},
		{
			name:   "valid module warns returns true",
			module: BackupModuleWarns,
			want:   true,
		},
		{
			name:   "invalid module returns false",
			module: "nonexistent",
			want:   false,
		},
		{
			name:   "empty string returns false",
			module: "",
			want:   false,
		},
		{
			name:   "case-sensitive mismatch returns false",
			module: "Admin",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsValidModule(tc.module)
			if got != tc.want {
				t.Fatalf("IsValidModule(%q) = %v, want %v", tc.module, got, tc.want)
			}
		})
	}
}

func TestFilterValidModules(t *testing.T) {

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "all valid modules returned as-is",
			input:    []string{"admin", "filters", "warns"},
			expected: []string{"admin", "filters", "warns"},
		},
		{
			name:     "invalid modules filtered out",
			input:    []string{"admin", "invalid", "filters", "bogus"},
			expected: []string{"admin", "filters"},
		},
		{
			name:     "all invalid returns empty",
			input:    []string{"foo", "bar"},
			expected: []string{},
		},
		{
			name:     "empty input returns empty",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "nil input returns empty",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "mixed case filtered correctly",
			input:    []string{"admin", "Admin", "ADMIN"},
			expected: []string{"admin"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterValidModules(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("FilterValidModules() len = %d, want %d; got %v", len(got), len(tc.expected), got)
			}
			for i, v := range tc.expected {
				if got[i] != v {
					t.Fatalf("FilterValidModules()[%d] = %q, want %q", i, got[i], v)
				}
			}
		})
	}
}
