package logredact

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestScrubStructuralPatterns(t *testing.T) {
	reset()
	t.Cleanup(reset)

	tests := []struct {
		name      string
		input     string
		mustNotIn string
		mustIn    string
	}{
		{
			name:      "telegram bot token",
			input:     "failed to call https://api.telegram.org/bot123456789:AAEhBOweik6ad1b2c3d4e5f6g7h8i9j0k1l2m/getMe",
			mustNotIn: "AAEhBOweik6ad1b2c3d4e5f6g7h8i9j0k1l2m",
			mustIn:    Placeholder,
		},
		{
			name:      "postgres dsn password",
			input:     "dial error: postgres://alita:s3cr3tP@ss@db.internal:5432/alita",
			mustNotIn: "s3cr3tP@ss",
			mustIn:    "postgres://alita:" + Placeholder,
		},
		{
			name:      "redis password-only dsn",
			input:     "redis://:topsecretvalue@cache:6379",
			mustNotIn: "topsecretvalue",
			mustIn:    Placeholder,
		},
		{
			name:      "authorization bearer header",
			input:     "request rejected: Authorization: Bearer abcDEF123456ghiJKL",
			mustNotIn: "abcDEF123456ghiJKL",
			mustIn:    Placeholder,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Scrub(tc.input)
			if strings.Contains(got, tc.mustNotIn) {
				t.Fatalf("secret leaked: %q still contains %q", got, tc.mustNotIn)
			}
			if !strings.Contains(got, tc.mustIn) {
				t.Fatalf("expected %q to contain %q", got, tc.mustIn)
			}
		})
	}
}

func TestScrubDoesNotCorruptOrdinaryProse(t *testing.T) {
	reset()
	t.Cleanup(reset)

	// "bearer" and "basic" are common English words; without anchoring to the
	// Authorization header they would be mangled. These must pass through
	// unchanged.
	cases := []string{
		"basic auth flow completed",
		"using basic configuration for the chat",
		"the bearer of this message is admin",
		"Basic validation passed",
	}
	for _, in := range cases {
		if got := Scrub(in); got != in {
			t.Fatalf("ordinary prose was corrupted: %q -> %q", in, got)
		}
	}
}

func TestScrubRegisteredSecret(t *testing.T) {
	reset()
	t.Cleanup(reset)

	RegisterSecret("hunter2password")
	got := Scrub("connecting with secret hunter2password to host")
	if strings.Contains(got, "hunter2password") {
		t.Fatalf("registered secret leaked: %q", got)
	}
	if !strings.Contains(got, Placeholder) {
		t.Fatalf("expected placeholder in %q", got)
	}
}

func TestRegisterSecretIgnoresShortAndEmpty(t *testing.T) {
	reset()
	t.Cleanup(reset)

	RegisterSecret("", "abc") // both below minSecretLen / empty
	registry.mu.RLock()
	n := len(registry.secrets)
	registry.mu.RUnlock()
	if n != 0 {
		t.Fatalf("expected no secrets registered, got %d", n)
	}

	msg := "abc is a common token"
	if got := Scrub(msg); got != msg {
		t.Fatalf("short value should not be redacted: %q", got)
	}
}

func TestRegisterSecretLongestFirst(t *testing.T) {
	reset()
	t.Cleanup(reset)

	// A password that is also a substring of the DSN. Longest-first ordering
	// must redact the whole DSN, leaving no partial leak.
	RegisterSecret("supersecret", "postgres://u:supersecret@host:5432/db")
	got := Scrub("error dialing postgres://u:supersecret@host:5432/db now")
	if strings.Contains(got, "supersecret") {
		t.Fatalf("secret leaked: %q", got)
	}
}

func TestHookScrubsMessageAndFields(t *testing.T) {
	reset()
	t.Cleanup(reset)
	RegisterSecret("mywebhooksecret")

	logger := logrus.New()
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	Install(logger)

	logger.WithFields(logrus.Fields{
		"dsn": "postgres://app:fieldsecret123@host/db",
		"err": errors.New("token 987654321:AAFhBOweik6ad1b2c3d4e5f6g7h8i9j0k1l2x rejected"),
	}).Errorf("webhook mywebhooksecret failed")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output not valid JSON: %v\n%s", err, buf.String())
	}

	out := buf.String()
	for _, leaked := range []string{"mywebhooksecret", "fieldsecret123", "987654321:AAFhBOweik6ad1b2c3d4e5f6g7h8i9j0k1l2x"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("secret %q leaked in log output: %s", leaked, out)
		}
	}
	if !strings.Contains(out, Placeholder) {
		t.Fatalf("expected redaction placeholder in log output: %s", out)
	}
}

func TestScrubEmptyString(t *testing.T) {
	reset()
	t.Cleanup(reset)
	if got := Scrub(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
