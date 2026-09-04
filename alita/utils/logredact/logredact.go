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
	"regexp"
	"slices"
	"strings"
	"sync"

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
	// including the password-only form redis://:secret@host.
	{
		re:   regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://[^:@/\s]*:)[^@/\s]+(@)`),
		repl: `${1}` + Placeholder + `${2}`,
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
		switch v := value.(type) {
		case string:
			entry.Data[key] = Scrub(v)
		case error:
			if v != nil {
				entry.Data[key] = Scrub(v.Error())
			}
		}
	}

	return nil
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
