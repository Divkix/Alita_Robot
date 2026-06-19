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
	"sort"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// Placeholder is the token substituted in place of redacted secrets.
const Placeholder = "[REDACTED]"

// minSecretLen is the shortest exact secret we will register for redaction.
// Very short values (e.g. a 2-character password) are too likely to collide
// with ordinary log text, so we skip them to avoid corrupting messages. The
// structural patterns below still cover credentials embedded in URLs.
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

// registry holds exact secret strings registered from configuration. It is
// guarded by a RWMutex because RegisterSecret may run during startup while log
// entries are concurrently scrubbed by the hook.
var registry = struct {
	mu      sync.RWMutex
	secrets []string
}{}

// RegisterSecret records one or more exact secret values that must be redacted
// from all subsequent log output. Empty values, and values shorter than
// minSecretLen, are ignored. Duplicate values are deduplicated. Secrets are
// matched longest-first so that a secret which is a substring of another (for
// example a password that also appears inside a DSN) does not leave a partial
// leak behind.
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

	// Longest-first ensures we redact the most specific (largest) secret before
	// any of its substrings.
	sort.Slice(registry.secrets, func(i, j int) bool {
		return len(registry.secrets[i]) > len(registry.secrets[j])
	})
}

// reset clears all registered secrets. It exists for tests.
func reset() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.secrets = nil
}

// Scrub returns s with all known secrets and credential-shaped substrings
// replaced by Placeholder. It is safe for concurrent use and is the single
// entry point used by both the logrus hook and any caller that wants to
// pre-sanitize a string (such as a URL) before logging it.
func Scrub(s string) string {
	if s == "" {
		return s
	}

	// Exact registered secrets first: these are guaranteed leaks.
	registry.mu.RLock()
	for _, secret := range registry.secrets {
		if strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, Placeholder)
		}
	}
	registry.mu.RUnlock()

	// Then structural patterns for secrets we cannot enumerate. Guard each
	// replacement with MatchString: unlike strings.ReplaceAll, regexp's
	// ReplaceAllString always copies the input even when nothing matches, so on
	// the common (no-secret) path the guard avoids three full-length string
	// allocations per log field at the cost of one extra (allocation-free) scan.
	for _, p := range structuralPatterns {
		if p.re.MatchString(s) {
			s = p.re.ReplaceAllString(s, p.repl)
		}
	}

	return s
}

// hook is a logrus.Hook that scrubs the message and string fields of every log
// entry before it is written to any output.
type hook struct{}

// Levels reports that the hook fires for all log levels.
func (hook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire scrubs the entry message and every string-valued field in place. Errors
// stored in fields are re-wrapped as scrubbed strings because their underlying
// type cannot be mutated.
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

// stdInstallOnce ensures the hook is registered on the logrus standard logger
// at most once, since logrus.AddHook unconditionally appends and the empty
// hook struct has no identity to deduplicate on.
var stdInstallOnce sync.Once

// Install registers the redaction hook on the supplied logger. Passing nil
// installs the hook on the logrus standard logger; that path is guarded by a
// sync.Once so repeated calls do not stack duplicate hooks (which would scrub
// every entry multiple times). For an explicit logger the caller owns
// deduplication, so the hook is added directly.
func Install(logger *logrus.Logger) {
	if logger == nil {
		stdInstallOnce.Do(func() {
			logrus.AddHook(hook{})
		})
		return
	}
	logger.AddHook(hook{})
}
