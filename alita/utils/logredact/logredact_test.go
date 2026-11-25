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

func TestScrubConnectionStringExact(t *testing.T) {
	reset()
	t.Cleanup(reset)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "password containing at",
			input: "dial postgres://app:p@ss@db.internal:5432/alita failed",
			want:  "dial postgres://app:" + Placeholder + "@db.internal:5432/alita failed",
		},
		{
			name:  "password containing slash",
			input: "redis://:pa/ss@cache:6379",
			want:  "redis://:" + Placeholder + "@cache:6379",
		},
		{
			name:  "password containing at and slash",
			input: "postgres://u:p@ss/w@rd@host:5432/db",
			want:  "postgres://u:" + Placeholder + "@host:5432/db",
		},
		{
			name:  "plain password",
			input: "amqp://guest:guest@broker:5672/",
			want:  "amqp://guest:" + Placeholder + "@broker:5672/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Scrub(tc.input); got != tc.want {
				t.Fatalf("Scrub(%q) = %q, want %q", tc.input, got, tc.want)
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

	RegisterSecret("", "abc")
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

func TestHookScrubsStructuredFields(t *testing.T) {
	reset()
	t.Cleanup(reset)

	const secret = "structuredsecret42"
	RegisterSecret(secret)

	type conn struct {
		ID  int    `json:"id"`
		DSN string `json:"dsn"`
	}

	logger := logrus.New()
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.JSONFormatter{})
	Install(logger)

	logger.WithFields(logrus.Fields{
		"conn":    conn{ID: 7, DSN: "postgres://app:" + secret + "@host/db"},
		"ptr":     &conn{ID: 8, DSN: "postgres://app:" + secret + "@host/db"},
		"nilconn": (*conn)(nil),
		"meta":    map[string]any{"token": secret, "count": 3},
		"list":    []any{secret},
	}).Error("structured fields")

	out := buf.String()
	if strings.Contains(out, secret) {
		t.Fatalf("secret leaked in log output: %s", out)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output not valid JSON: %v\n%s", err, out)
	}

	for _, key := range []string{"conn", "ptr"} {
		object, ok := entry[key].(map[string]any)
		if !ok {
			t.Fatalf("%s field = %T, want a JSON object", key, entry[key])
		}
		if object["dsn"] == nil || strings.Contains(object["dsn"].(string), secret) {
			t.Fatalf("%s.dsn = %v, want the secret redacted in place", key, object["dsn"])
		}
	}
	if entry["conn"].(map[string]any)["id"] != float64(7) {
		t.Fatalf("conn.id = %v, want 7", entry["conn"].(map[string]any)["id"])
	}
	if entry["nilconn"] != nil {
		t.Fatalf("nil pointer field = %v, want null", entry["nilconn"])
	}

	meta, ok := entry["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta field = %T, want a JSON object", entry["meta"])
	}
	if meta["token"] != Placeholder || meta["count"] != float64(3) {
		t.Fatalf("meta = %v, want token redacted and count preserved", meta)
	}

	list, ok := entry["list"].([]any)
	if !ok || len(list) != 1 || list[0] != Placeholder {
		t.Fatalf("list = %v, want a one-element redacted array", entry["list"])
	}
}

func TestScrubEmptyString(t *testing.T) {
	reset()
	t.Cleanup(reset)
	if got := Scrub(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
