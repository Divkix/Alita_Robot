//go:build testtools

package modules

import (
	"regexp"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

var localeCodes = []string{"en", "es", "fr", "hi", "id", "pt", "ru"}

// telegramKnownTag matches the tag names Telegram HTML parse mode accepts.
// Anything else in angle brackets makes sendMessage fail with "unsupported
// start tag", so the user gets no reply.
var telegramKnownTag = regexp.MustCompile(`^(?i:b|strong|i|em|u|ins|s|strike|del|a|code|pre|tg-spoiler|span|tg-emoji|blockquote)$`)

var angleTagRe = regexp.MustCompile(`<(/?)([^\s>/]*)[^>]*>`)

// unknownTags returns the angle-bracket sequences in s that Telegram would
// reject under HTML parse mode.
func unknownTags(s string) []string {
	var bad []string
	for _, m := range angleTagRe.FindAllStringSubmatch(s, -1) {
		if !telegramKnownTag.MatchString(m[2]) {
			bad = append(bad, m[0])
		}
	}
	return bad
}

// htmlSentLocaleKeys are sent with HTML parse mode as-is (formatting.Shtml,
// replyHTML, or an EditText with formatting.HTML). Placeholders in them must
// be written as &lt;…&gt;.
var htmlSentLocaleKeys = []string{
	"reports_invalid_input",
	"connections_invalid_option",
	"reactions_add_usage",
	"reactions_remove_usage",
	"antiraid_raidtime_usage",
	"antiraid_raidactiontime_usage",
	"logs_usage",
	"misc_github_usage",
	"misc_wiki_usage",
	"misc_ud_usage",
}

// markdownSentLocaleKeys are sent with formatting.Smarkdown, where a literal
// `<…>` is fine and `&lt;` would show up verbatim.
var markdownSentLocaleKeys = []string{
	"notes_private_invalid_option",
	"blacklists_set_bl_action_choose_correct_option",
}

func TestHTMLSentLocaleStringsHaveNoUnknownTags(t *testing.T) {
	for _, code := range localeCodes {
		locale := readLocaleFile(t, code)
		for _, key := range htmlSentLocaleKeys {
			text, ok := locale[key]
			if !ok {
				t.Errorf("%s.yml: %s is missing", code, key)
				continue
			}
			if bad := unknownTags(text); len(bad) > 0 {
				t.Errorf("%s.yml: %s is sent as HTML but contains %q; write &lt;…&gt;", code, key, bad)
			}
		}
	}
}

func TestRenderedModuleHelpHasNoUnknownTags(t *testing.T) {
	for _, code := range localeCodes {
		for key, text := range readLocaleFile(t, code) {
			if !strings.HasSuffix(key, "_help_msg") {
				continue
			}
			if bad := unknownTags(formatting.ToTelegramHTML(text)); len(bad) > 0 {
				t.Errorf("%s.yml: %s renders to HTML with %q", code, key, bad)
			}
		}
	}
}

// A string with a bare <placeholder> must be classified, so a new one cannot
// ship as HTML by accident. Module help is rendered by ToTelegramHTML, which
// escapes it, and is covered by the test above.
func TestLocaleStringsWithPlaceholdersAreClassified(t *testing.T) {
	classified := map[string]bool{}
	for _, key := range htmlSentLocaleKeys {
		classified[key] = true
	}
	for _, key := range markdownSentLocaleKeys {
		classified[key] = true
	}
	for _, code := range localeCodes {
		for key, text := range readLocaleFile(t, code) {
			if strings.HasSuffix(key, "_help_msg") || classified[key] {
				continue
			}
			if bad := unknownTags(text); len(bad) > 0 {
				t.Errorf("%s.yml: %s contains %q: add it to htmlSentLocaleKeys (and write &lt;…&gt;) or markdownSentLocaleKeys", code, key, bad)
			}
		}
	}
}
