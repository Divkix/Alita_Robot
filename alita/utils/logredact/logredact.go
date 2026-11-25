// Package logredact provides sensitive-data scrubbing for application logs.
//
// Telegram bots routinely handle credentials that must never reach logs,
// crash dumps, or shipped log aggregators: the bot token, the PostgreSQL DSN,
// the Redis password, the webhook secret, and the metrics bearer token.
// Logrus by default writes log messages and structured fields verbatim, so a
// stray Errorf that includes a request URL or a wrapped error from the
// database driver can leak a live secret.
//
// This package centralizes two complementary defenses:
//
//   - Pattern-based redaction (Scrub) rewrites credential-bearing structures
//     that are recognizable by shape even when the exact value is unknown:
//     Telegram bot tokens, connection-string passwords (scheme://user:pass@host),
//     and HTTP Authorization bearer tokens.
//   - Exact-value redaction removes known secrets registered at startup from
//     the running configuration via RegisterSecret.
//
// Install returns a logrus.Hook that applies both layers to every log entry
// (the message and all string-valued fields) regardless of log level, so the
// protection holds even for fire-and-forget Warnf/Errorf calls scattered
// across the codebase.
package logredact

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const Placeholder = "[REDACTED]"

const minSecretLen = 6

// structuralPatterns redacts credentials that are identifiable by their shape,
// independent of any registered value. Order matters: each replacement keeps
// the surrounding context (host, scheme, header name) intact so logs remain
// useful for debugging while the secret itself is removed.
var structuralPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	// Telegram bot token: "<bot_id>:<auth_hash>" (e.g. 123456789:AA...). No
	// leading word boundary because the token is commonly embedded in an API
	// URL path as "/bot123456789:AA..." (letter->digit is not a \b boundary).
	// Bounds are deliberately open-ended (no upper limit) because for a
	// redaction tool over-matching the secret is safer than leaving a tail.
	{
		re:   regexp.MustCompile(`\d{6,}:[A-Za-z0-9_-]{30,}`),
		repl: Placeholder,
	},
	// Credentials in connection strings: scheme://user:password@host -> redact
	// only the password segment. Covers postgres://, redis://, amqp://, etc.,
	// including the password-only form redis://:secret@host. The password runs
	// to the last @ in the whitespace-delimited token, so a password containing
	// @ or / cannot leave a suffix behind.
	{
		re:   regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://[^:@/\s]*:)[^\s]*@`),
		repl: `${1}` + Placeholder + `@`,
	},
	// HTTP Authorization header tokens. Anchored to the "Authorization:" header
	// name so that the common English words "bearer"/"basic" in ordinary log
	// prose are not mistaken for credentials. The token must be reasonably long
	// (>= 8 chars) to look like an actual credential.
	{
		re:   regexp.MustCompile(`(?i)(authorization:\s*(?:bearer|basic)\s+)[A-Za-z0-9+/._\-=]{8,}`),
		repl: `${1}` + Placeholder,
	},
}

var registry = struct {
	mu      sync.RWMutex
	secrets []string
}{}

func RegisterSecret(values ...string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	existing := make(map[string]struct{}, len(registry.secrets))
	for _, s := range registry.secrets {
		existing[s] = struct{}{}
	}

	for _, v := range values {
		if len(v) < minSecretLen {
			continue
		}
		if _, ok := existing[v]; ok {
			continue
		}
		existing[v] = struct{}{}
		registry.secrets = append(registry.secrets, v)
	}

	slices.SortFunc(registry.secrets, func(a, b string) int { return len(b) - len(a) })
}

func reset() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.secrets = nil
}

func Scrub(s string) string {
	if s == "" {
		return s
	}

	registry.mu.RLock()
	for _, secret := range registry.secrets {
		if strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, Placeholder)
		}
	}
	registry.mu.RUnlock()

	for _, p := range structuralPatterns {
		if p.re.MatchString(s) {
			s = p.re.ReplaceAllString(s, p.repl)
		}
	}

	return s
}

type hook struct{}

func (hook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook) Fire(entry *logrus.Entry) error {
	entry.Message = Scrub(entry.Message)

	for key, value := range entry.Data {
		entry.Data[key] = scrubField(value)
	}

	return nil
}

// scrubField scrubs a log field value. Strings and errors are scrubbed
// directly. Structs, maps, slices, arrays and pointers are JSON-marshaled and
// scrubbed as text; when that changes the text, they are decoded back so the
// log entry keeps its JSON shape instead of collapsing to a single opaque
// string. Values that cannot be marshaled fall back to scrubbing their text
// form.
func scrubField(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return Scrub(v)
	case error:
		if v == nil {
			return nil
		}
		return Scrub(v.Error())
	case time.Time, *time.Time:
		return v
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if rv.IsNil() {
			return value
		}
	case reflect.Struct, reflect.Array:
	default:
		return value
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return Scrub(fmt.Sprint(value))
	}

	scrubbed := Scrub(string(encoded))
	if scrubbed == string(encoded) {
		return value
	}

	dec := json.NewDecoder(strings.NewReader(scrubbed))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return Scrub(fmt.Sprint(value))
	}
	return decoded
}

var stdInstallOnce sync.Once

func Install(logger *logrus.Logger) {
	if logger == nil {
		stdInstallOnce.Do(func() {
			logrus.AddHook(hook{})
		})
		return
	}
	logger.AddHook(hook{})
}
