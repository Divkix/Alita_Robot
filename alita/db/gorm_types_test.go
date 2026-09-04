package db

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/migrations"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

//nolint:dupl // Scan test patterns are intentionally similar across types
func TestButtonArray_Scan(t *testing.T) {

	tests := []struct {
		name        string
		input       any
		wantEmpty   bool
		wantErr     bool
		errContains string
		wantLen     int
		validate    func(t *testing.T, ba models.ButtonArray)
	}{
		{
			name:      "nil value returns empty no error",
			input:     nil,
			wantEmpty: true,
			wantErr:   false,
		},
		{
			name:    "valid JSON bytes parses correctly",
			input:   []byte(`[{"name":"btn1","url":"https://example.com","btn_sameline":true}]`),
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, ba models.ButtonArray) {
				t.Helper()
				if ba[0].Name != "btn1" {
					t.Fatalf("expected Name=btn1, got %q", ba[0].Name)
				}
				if ba[0].Url != "https://example.com" {
					t.Fatalf("expected Url=https://example.com, got %q", ba[0].Url)
				}
				if !ba[0].SameLine {
					t.Fatalf("expected SameLine=true, got false")
				}
			},
		},
		{
			name:        "invalid JSON string returns unmarshal error",
			input:       "not valid json",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:    "invalid JSON returns unmarshal error",
			input:   []byte(`not valid json`),
			wantErr: true,
		},
		{
			name:    "empty JSON array parses to empty slice",
			input:   []byte(`[]`),
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "multiple buttons parsed correctly",
			input:   []byte(`[{"name":"a","url":"http://a.com"},{"name":"b","url":"http://b.com","btn_sameline":true}]`),
			wantErr: false,
			wantLen: 2,
			validate: func(t *testing.T, ba models.ButtonArray) {
				t.Helper()
				if ba[0].Name != "a" {
					t.Fatalf("expected Name=a, got %q", ba[0].Name)
				}
				if ba[1].SameLine != true {
					t.Fatalf("expected SameLine=true for second button")
				}
			},
		},
		{
			name:    "special chars in fields parsed correctly",
			input:   []byte(`[{"name":"btn <&> special","url":"https://example.com/?q=a&b=c"}]`),
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, ba models.ButtonArray) {
				t.Helper()
				if ba[0].Name != "btn <&> special" {
					t.Fatalf("expected special name, got %q", ba[0].Name)
				}
			},
		},
		{
			name:        "integer type returns type assertion error",
			input:       42,
			wantErr:     true,
			errContains: "type assertion",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			var ba models.ButtonArray
			err := ba.Scan(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error containing %q, got %q", tc.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantEmpty && len(ba) != 0 {
				t.Fatalf("expected empty models.ButtonArray, got len=%d", len(ba))
			}

			if tc.wantLen > 0 && len(ba) != tc.wantLen {
				t.Fatalf("expected len=%d, got len=%d", tc.wantLen, len(ba))
			}

			if tc.validate != nil {
				tc.validate(t, ba)
			}
		})
	}
}

//nolint:dupl // Value test patterns are intentionally similar across types
func TestButtonArray_Value(t *testing.T) {

	tests := []struct {
		name    string
		input   models.ButtonArray
		wantStr string
		wantErr bool
	}{
		{
			name:    "empty array returns empty JSON array string",
			input:   models.ButtonArray{},
			wantStr: "[]",
		},
		{
			name:    "nil array returns empty JSON array string",
			input:   nil,
			wantStr: "[]",
		},
		{
			name:    "single element produces valid JSON",
			input:   models.ButtonArray{{Name: "btn1", Url: "https://example.com", SameLine: false}},
			wantErr: false,
		},
		{
			name:    "empty string fields produce valid JSON",
			input:   models.ButtonArray{{Name: "", Url: "", SameLine: false}},
			wantErr: false,
		},
		{
			name:  "multiple elements produce valid JSON",
			input: models.ButtonArray{{Name: "a", Url: "http://a.com"}, {Name: "b", Url: "http://b.com", SameLine: true}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			val, err := tc.input.Value()

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantStr != "" {
				if val != tc.wantStr {
					t.Fatalf("expected %q, got %q", tc.wantStr, val)
				}
				return
			}

			b, ok := val.([]byte)
			if !ok {
				t.Fatalf("expected []byte value for non-empty array, got %T", val)
			}
			var result models.ButtonArray
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatalf("Value() produced invalid JSON: %v", err)
			}
			if len(result) != len(tc.input) {
				t.Fatalf("round-trip length mismatch: expected %d, got %d", len(tc.input), len(result))
			}
		})
	}
}

//nolint:dupl // Scan test patterns are intentionally similar across types
func TestStringArray_Scan(t *testing.T) {

	tests := []struct {
		name        string
		input       any
		wantEmpty   bool
		wantErr     bool
		errContains string
		wantLen     int
		validate    func(t *testing.T, sa models.StringArray)
	}{
		{
			name:      "nil value returns empty no error",
			input:     nil,
			wantEmpty: true,
			wantErr:   false,
		},
		{
			name:    "valid JSON string array parses correctly",
			input:   []byte(`["hello","world"]`),
			wantErr: false,
			wantLen: 2,
			validate: func(t *testing.T, sa models.StringArray) {
				t.Helper()
				if sa[0] != "hello" {
					t.Fatalf("expected sa[0]=hello, got %q", sa[0])
				}
				if sa[1] != "world" {
					t.Fatalf("expected sa[1]=world, got %q", sa[1])
				}
			},
		},
		{
			name:        "invalid JSON string returns unmarshal error",
			input:       "not valid json",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:    "invalid JSON returns error",
			input:   []byte(`not valid json`),
			wantErr: true,
		},
		{
			name:    "empty JSON array parses to empty slice",
			input:   []byte(`[]`),
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "unicode strings parsed correctly",
			input:   []byte(`["日本語","한국어","العربية"]`),
			wantErr: false,
			wantLen: 3,
			validate: func(t *testing.T, sa models.StringArray) {
				t.Helper()
				if sa[0] != "日本語" {
					t.Fatalf("expected unicode string, got %q", sa[0])
				}
			},
		},
		{
			name:        "integer type returns type assertion error",
			input:       100,
			wantErr:     true,
			errContains: "type assertion",
		},
		{
			name:    "single element parsed correctly",
			input:   []byte(`["only"]`),
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, sa models.StringArray) {
				t.Helper()
				if sa[0] != "only" {
					t.Fatalf("expected sa[0]=only, got %q", sa[0])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			var sa models.StringArray
			err := sa.Scan(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error containing %q, got %q", tc.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantEmpty && len(sa) != 0 {
				t.Fatalf("expected empty StringArray, got len=%d", len(sa))
			}

			if tc.wantLen > 0 && len(sa) != tc.wantLen {
				t.Fatalf("expected len=%d, got len=%d", tc.wantLen, len(sa))
			}

			if tc.validate != nil {
				tc.validate(t, sa)
			}
		})
	}
}

//nolint:dupl // Value test patterns are intentionally similar across types
func TestStringArray_Value(t *testing.T) {

	tests := []struct {
		name    string
		input   models.StringArray
		wantStr string
		wantErr bool
	}{
		{
			name:    "empty array returns empty JSON array string",
			input:   models.StringArray{},
			wantStr: "[]",
		},
		{
			name:    "nil array returns empty JSON array string",
			input:   nil,
			wantStr: "[]",
		},
		{
			name:    "multiple elements produce valid JSON",
			input:   models.StringArray{"hello", "world", "foo"},
			wantErr: false,
		},
		{
			name:    "empty string element produces valid JSON",
			input:   models.StringArray{""},
			wantErr: false,
		},
		{
			name:  "unicode elements produce valid JSON",
			input: models.StringArray{"日本語", "한국어"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			val, err := tc.input.Value()

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantStr != "" {
				if val != tc.wantStr {
					t.Fatalf("expected %q, got %q", tc.wantStr, val)
				}
				return
			}

			b, ok := val.([]byte)
			if !ok {
				t.Fatalf("expected []byte value for non-empty array, got %T", val)
			}
			var result models.StringArray
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatalf("Value() produced invalid JSON: %v", err)
			}
			if len(result) != len(tc.input) {
				t.Fatalf("round-trip length mismatch: expected %d, got %d", len(tc.input), len(result))
			}
		})
	}
}

//nolint:dupl // Scan test patterns are intentionally similar across types
func TestInt64Array_Scan(t *testing.T) {

	tests := []struct {
		name        string
		input       any
		wantEmpty   bool
		wantErr     bool
		errContains string
		wantLen     int
		validate    func(t *testing.T, ia models.Int64Array)
	}{
		{
			name:      "nil value returns empty no error",
			input:     nil,
			wantEmpty: true,
			wantErr:   false,
		},
		{
			name:    "valid JSON int64 array parses correctly",
			input:   []byte(`[1,2,3]`),
			wantErr: false,
			wantLen: 3,
			validate: func(t *testing.T, ia models.Int64Array) {
				t.Helper()
				if ia[0] != 1 || ia[1] != 2 || ia[2] != 3 {
					t.Fatalf("expected [1,2,3], got %v", ia)
				}
			},
		},
		{
			name:        "invalid JSON string returns unmarshal error",
			input:       "not valid json",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:    "invalid JSON returns error",
			input:   []byte(`not valid json`),
			wantErr: true,
		},
		{
			name:    "empty JSON array parses to empty slice",
			input:   []byte(`[]`),
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "MaxInt64 value parsed correctly",
			input:   []byte(`[9223372036854775807]`),
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, ia models.Int64Array) {
				t.Helper()
				if ia[0] != math.MaxInt64 {
					t.Fatalf("expected MaxInt64=%d, got %d", int64(math.MaxInt64), ia[0])
				}
			},
		},
		{
			name:    "MinInt64 value parsed correctly",
			input:   []byte(`[-9223372036854775808]`),
			wantErr: false,
			wantLen: 1,
			validate: func(t *testing.T, ia models.Int64Array) {
				t.Helper()
				if ia[0] != math.MinInt64 {
					t.Fatalf("expected MinInt64=%d, got %d", int64(math.MinInt64), ia[0])
				}
			},
		},
		{
			name:    "mixed signs parsed correctly",
			input:   []byte(`[-100, 0, 100]`),
			wantErr: false,
			wantLen: 3,
			validate: func(t *testing.T, ia models.Int64Array) {
				t.Helper()
				if ia[0] != -100 {
					t.Fatalf("expected ia[0]=-100, got %d", ia[0])
				}
				if ia[1] != 0 {
					t.Fatalf("expected ia[1]=0, got %d", ia[1])
				}
				if ia[2] != 100 {
					t.Fatalf("expected ia[2]=100, got %d", ia[2])
				}
			},
		},
		{
			name:        "integer type returns type assertion error",
			input:       int64(42),
			wantErr:     true,
			errContains: "type assertion",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			var ia models.Int64Array
			err := ia.Scan(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error containing %q, got %q", tc.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantEmpty && len(ia) != 0 {
				t.Fatalf("expected empty models.Int64Array, got len=%d", len(ia))
			}

			if tc.wantLen > 0 && len(ia) != tc.wantLen {
				t.Fatalf("expected len=%d, got len=%d", tc.wantLen, len(ia))
			}

			if tc.validate != nil {
				tc.validate(t, ia)
			}
		})
	}
}

//nolint:dupl // Value test patterns are intentionally similar across types
func TestInt64Array_Value(t *testing.T) {

	tests := []struct {
		name    string
		input   models.Int64Array
		wantStr string
		wantErr bool
	}{
		{
			name:    "empty array returns empty JSON array string",
			input:   models.Int64Array{},
			wantStr: "[]",
		},
		{
			name:    "nil array returns empty JSON array string",
			input:   nil,
			wantStr: "[]",
		},
		{
			name:    "MaxInt64 produces valid JSON",
			input:   models.Int64Array{math.MaxInt64},
			wantErr: false,
		},
		{
			name:    "MinInt64 produces valid JSON",
			input:   models.Int64Array{math.MinInt64},
			wantErr: false,
		},
		{
			name:  "multiple elements produce valid JSON",
			input: models.Int64Array{-100, 0, 100, math.MaxInt64},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			val, err := tc.input.Value()

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantStr != "" {
				if val != tc.wantStr {
					t.Fatalf("expected %q, got %q", tc.wantStr, val)
				}
				return
			}

			b, ok := val.([]byte)
			if !ok {
				t.Fatalf("expected []byte value for non-empty array, got %T", val)
			}
			var result models.Int64Array
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatalf("Value() produced invalid JSON: %v", err)
			}
			if len(result) != len(tc.input) {
				t.Fatalf("round-trip length mismatch: expected %d, got %d", len(tc.input), len(result))
			}
			for i, v := range tc.input {
				if result[i] != v {
					t.Fatalf("round-trip value mismatch at index %d: expected %d, got %d", i, v, result[i])
				}
			}
		})
	}
}

func TestTableNames(t *testing.T) {

	tests := []struct {
		name      string
		model     interface{ TableName() string }
		wantTable string
	}{
		{"User", User{}, "users"},
		{"Chat", Chat{}, "chats"},
		{"WarnSettings", models.WarnSettings{}, "warns_settings"},
		{"Warns", models.Warns{}, "warns_users"},
		{"GreetingSettings", models.GreetingSettings{}, "greetings"},
		{"ChatFilters", ChatFilters{}, "filters"},
		{"AdminSettings", models.AdminSettings{}, "admin"},
		{"BlacklistSettings", models.BlacklistSettings{}, "blacklists"},
		{"PinSettings", models.PinSettings{}, "pins"},
		{"ReportChatSettings", models.ReportChatSettings{}, "report_chat_settings"},
		{"ReportUserSettings", models.ReportUserSettings{}, "report_user_settings"},
		{"DevSettings", DevSettings{}, "devs"},
		{"ChannelSettings", models.ChannelSettings{}, "channels"},
		{"AntifloodSettings", AntifloodSettings{}, "antiflood_settings"},
		{"ConnectionSettings", models.ConnectionSettings{}, "connection"},
		{"ConnectionChatSettings", models.ConnectionChatSettings{}, "connection_settings"},
		{"DisableSettings", models.DisableSettings{}, "disable"},
		{"DisableChatSettings", models.DisableChatSettings{}, "disable_chat_settings"},
		{"RulesSettings", models.RulesSettings{}, "rules"},
		{"LockSettings", LockSettings{}, "locks"},
		{"NotesSettings", NotesSettings{}, "notes_settings"},
		{"Notes", Notes{}, "notes"},
		{"CaptchaSettings", CaptchaSettings{}, "captcha_settings"},
		{"CaptchaAttempts", CaptchaAttempts{}, "captcha_attempts"},
		{"StoredMessages", models.StoredMessages{}, "stored_messages"},
		{"CaptchaMutedUsers", models.CaptchaMutedUsers{}, "captcha_muted_users"},
		{"SchemaMigration", migrations.SchemaMigration{}, "schema_migrations"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.model.TableName(); got != tc.wantTable {
				t.Fatalf("%s.TableName() = %q, want %q", tc.name, got, tc.wantTable)
			}
		})
	}
}

func TestBlacklistSettingsSlice_Triggers(t *testing.T) {

	tests := []struct {
		name     string
		slice    models.BlacklistSettingsSlice
		wantLen  int
		contains []string
	}{
		{
			name:    "nil slice returns nil triggers",
			slice:   nil,
			wantLen: 0,
		},
		{
			name:    "empty slice returns empty triggers",
			slice:   models.BlacklistSettingsSlice{},
			wantLen: 0,
		},
		{
			name: "single entry returns word",
			slice: models.BlacklistSettingsSlice{
				{Word: "spam", Action: "warn"},
			},
			wantLen:  1,
			contains: []string{"spam"},
		},
		{
			name: "multiple entries returns all words",
			slice: models.BlacklistSettingsSlice{
				{Word: "badword1", Action: "ban"},
				{Word: "badword2", Action: "kick"},
				{Word: "badword3", Action: "warn"},
			},
			wantLen:  3,
			contains: []string{"badword1", "badword2", "badword3"},
		},
		{
			name: "entries with empty word included",
			slice: models.BlacklistSettingsSlice{
				{Word: "", Action: "warn"},
				{Word: "foo", Action: "ban"},
			},
			wantLen:  2,
			contains: []string{"", "foo"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := tc.slice.Triggers()

			if len(got) != tc.wantLen {
				t.Fatalf("Triggers() len=%d, want %d; got %v", len(got), tc.wantLen, got)
			}

			gotSet := make(map[string]bool, len(got))
			for _, g := range got {
				gotSet[g] = true
			}
			for _, w := range tc.contains {
				if !gotSet[w] {
					t.Fatalf("Triggers() missing %q; got %v", w, got)
				}
			}
		})
	}
}

func TestBlacklistSettingsSlice_Action(t *testing.T) {

	tests := []struct {
		name       string
		slice      models.BlacklistSettingsSlice
		wantAction string
	}{
		{
			name:       "nil slice returns default warn",
			slice:      nil,
			wantAction: "warn",
		},
		{
			name:       "empty slice returns default warn",
			slice:      models.BlacklistSettingsSlice{},
			wantAction: "warn",
		},
		{
			name: "single entry returns its action",
			slice: models.BlacklistSettingsSlice{
				{Word: "spam", Action: "ban"},
			},
			wantAction: "ban",
		},
		{
			name: "multiple entries returns first action",
			slice: models.BlacklistSettingsSlice{
				{Word: "a", Action: "kick"},
				{Word: "b", Action: "ban"},
			},
			wantAction: "kick",
		},
		{
			name: "empty action field on first entry returns empty string",
			slice: models.BlacklistSettingsSlice{
				{Word: "spam", Action: ""},
			},
			wantAction: "",
		},
		{
			name: "mute action preserved",
			slice: models.BlacklistSettingsSlice{
				{Word: "x", Action: "mute"},
			},
			wantAction: "mute",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := tc.slice.Action()
			if got != tc.wantAction {
				t.Fatalf("Action()=%q, want %q", got, tc.wantAction)
			}
		})
	}
}

func TestBlacklistSettingsSlice_Reason(t *testing.T) {

	tests := []struct {
		name       string
		slice      models.BlacklistSettingsSlice
		wantReason string
	}{
		{
			name:       "nil slice returns default format string",
			slice:      nil,
			wantReason: "Blacklisted word: '%s'",
		},
		{
			name:       "empty slice returns default format string",
			slice:      models.BlacklistSettingsSlice{},
			wantReason: "Blacklisted word: '%s'",
		},
		{
			name: "entry with empty reason returns default format string",
			slice: models.BlacklistSettingsSlice{
				{Word: "spam", Reason: ""},
			},
			wantReason: "Blacklisted word: '%s'",
		},
		{
			name: "entry with non-empty reason returns it",
			slice: models.BlacklistSettingsSlice{
				{Word: "spam", Reason: "No spamming allowed"},
			},
			wantReason: "No spamming allowed",
		},
		{
			name: "multiple entries returns first entry reason",
			slice: models.BlacklistSettingsSlice{
				{Word: "a", Reason: "first reason"},
				{Word: "b", Reason: "second reason"},
			},
			wantReason: "first reason",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := tc.slice.Reason()
			if got != tc.wantReason {
				t.Fatalf("Reason()=%q, want %q", got, tc.wantReason)
			}
		})
	}
}

func TestBlacklistSettingsSlice_Find(t *testing.T) {
	slice := models.BlacklistSettingsSlice{
		{Word: "oldword", Action: "mute", Reason: "old reason"},
		{Word: "newword", Action: "warn", Reason: "new reason"},
	}
	got := slice.Find("NEWWORD")
	if got == nil || got.Action != "warn" || got.Reason != "new reason" {
		t.Fatalf("Find(newword) = %+v, want warn/new reason", got)
	}
	got = slice.Find("oldword")
	if got == nil || got.Action != "mute" {
		t.Fatalf("Find(oldword) = %+v, want mute", got)
	}
	if slice.Find("absent") != nil {
		t.Fatal("Find(absent) returned a row")
	}
	if (models.BlacklistSettingsSlice{}).Find("x") != nil {
		t.Fatal("empty slice Find returned a row")
	}
}

func TestNotesSettings_PrivateNotesEnabled(t *testing.T) {

	tests := []struct {
		name    string
		private bool
		want    bool
	}{
		{
			name:    "Private=false returns false",
			private: false,
			want:    false,
		},
		{
			name:    "Private=true returns true",
			private: true,
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			ns := &NotesSettings{Private: tc.private}
			got := ns.PrivateNotesEnabled()
			if got != tc.want {
				t.Fatalf("PrivateNotesEnabled()=%v, want %v", got, tc.want)
			}
		})
	}
}
