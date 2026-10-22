package config

import (
	"strings"
	"testing"
)

// Not parallel: t.Setenv.
func TestLoadConfigAISpamDailyLimit(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{
		{value: "", want: 0},
		{value: "5000", want: 5000},
		{value: " 1 ", want: 1},
	} {
		t.Run(tc.value, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("AISPAM_DAILY_LIMIT", tc.value)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if cfg.AISpamDailyLimit != tc.want {
				t.Fatalf("AISpamDailyLimit = %d, want %d", cfg.AISpamDailyLimit, tc.want)
			}
		})
	}
}

// Unlike the lenient integer settings, a bad cap fails startup: falling back
// to no cap would lift the limit the operator set.
func TestLoadConfigAISpamDailyLimitRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"lots", "1e3", "0", "-5"} {
		t.Run(value, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("AISPAM_DAILY_LIMIT", value)

			_, err := LoadConfig()
			if err == nil {
				t.Fatal("LoadConfig() error = nil, want startup to fail")
			}
			if !strings.Contains(err.Error(), "AISPAM_DAILY_LIMIT") {
				t.Fatalf("LoadConfig() error = %q, want it to name AISPAM_DAILY_LIMIT", err)
			}
		})
	}
}
