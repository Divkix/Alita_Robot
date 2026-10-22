package config

import (
	"strings"
	"testing"
)

// Not parallel: t.Setenv.
func TestLoadConfigReputationDefaults(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("ENABLE_REPUTATION", "")
	t.Setenv("REPUTATION_MIN_CHATS", "")
	t.Setenv("REPUTATION_WINDOW_DAYS", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.EnableReputation {
		t.Error("EnableReputation = true, want the feature off by default")
	}
	if cfg.ReputationMinChats != DefaultReputationMinChats {
		t.Errorf("ReputationMinChats = %d, want %d", cfg.ReputationMinChats, DefaultReputationMinChats)
	}
	if cfg.ReputationWindowDays != DefaultReputationWindowDays {
		t.Errorf("ReputationWindowDays = %d, want %d", cfg.ReputationWindowDays, DefaultReputationWindowDays)
	}
}

// Not parallel: t.Setenv.
func TestLoadConfigReputationValues(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("ENABLE_REPUTATION", "true")
	t.Setenv("REPUTATION_MIN_CHATS", "2")
	t.Setenv("REPUTATION_WINDOW_DAYS", "180")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.EnableReputation {
		t.Error("EnableReputation = false, want true")
	}
	if cfg.ReputationMinChats != 2 {
		t.Errorf("ReputationMinChats = %d, want 2", cfg.ReputationMinChats)
	}
	if cfg.ReputationWindowDays != 180 {
		t.Errorf("ReputationWindowDays = %d, want 180", cfg.ReputationWindowDays)
	}
}

// Unlike the lenient integer settings, a bad threshold or window fails startup
// instead of falling back to the default.
func TestLoadConfigReputationRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name       string
		minChats   string
		windowDays string
		wantVar    string
	}{
		{name: "min chats is not an integer", minChats: "three", windowDays: "", wantVar: "REPUTATION_MIN_CHATS"},
		{name: "min chats below two", minChats: "1", windowDays: "", wantVar: "REPUTATION_MIN_CHATS"},
		{name: "window days is not an integer", minChats: "", windowDays: "month", wantVar: "REPUTATION_WINDOW_DAYS"},
		{name: "window days below one", minChats: "", windowDays: "0", wantVar: "REPUTATION_WINDOW_DAYS"},
		{name: "window days beyond retention", minChats: "", windowDays: "181", wantVar: "REPUTATION_WINDOW_DAYS"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("REPUTATION_MIN_CHATS", tc.minChats)
			t.Setenv("REPUTATION_WINDOW_DAYS", tc.windowDays)

			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("LoadConfig() error = nil, want startup to fail")
			}
			if !strings.Contains(err.Error(), tc.wantVar) {
				t.Fatalf("LoadConfig() error = %q, want it to name %s", err, tc.wantVar)
			}
		})
	}
}
