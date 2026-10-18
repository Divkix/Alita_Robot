package config

import (
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/logredact"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BOT_TOKEN", "test-token")
	t.Setenv("OWNER_ID", "12345")
	t.Setenv("MESSAGE_DUMP", "67890")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("REDIS_ADDRESS", "redis:6379")
	t.Setenv("REDIS_URL", "")
}

// Not parallel: t.Setenv.
func TestLoadConfigReadsCASAPIKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CAS_API_KEY", "cas-key-from-env")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.CASAPIKey != "cas-key-from-env" {
		t.Fatalf("CASAPIKey = %q, want %q", cfg.CASAPIKey, "cas-key-from-env")
	}
}

// The operator switch is on unless set; chats still opt in one by one.
func TestLoadConfigEnableSpamDB(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", true},
		{"true", true},
		{"false", false},
	}
	for _, tc := range cases {
		t.Run("ENABLE_SPAMDB="+tc.env, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("ENABLE_SPAMDB", tc.env)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if cfg.EnableSpamDB != tc.want {
				t.Fatalf("EnableSpamDB = %v, want %v", cfg.EnableSpamDB, tc.want)
			}
		})
	}
}

func TestRegisterConfigSecretsRedactsCASAPIKey(t *testing.T) {
	const key = "cas-secret-key-5f3a9c"
	registerConfigSecrets(&Config{CASAPIKey: key})

	if got := logredact.Scrub("GET /check?api_key=" + key); strings.Contains(got, key) {
		t.Fatalf("Scrub() = %q, still contains the CAS key", got)
	}
}
