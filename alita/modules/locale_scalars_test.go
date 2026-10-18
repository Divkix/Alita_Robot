//go:build testtools

package modules

import (
	"sort"
	"strings"
	"testing"
)

var localeCodes = []string{"en", "es", "fr", "hi", "id", "pt", "ru"}

// bans_help_msg and mutes_help_msg are rewritten together with the command
// help in #962; this test takes them over once that lands.
var scalarCheckSkipped = map[string]bool{"bans_help_msg": true, "mutes_help_msg": true}

// A YAML scalar written as '…' or inside a "|" block keeps its quotes and
// backslashes verbatim, so users see stray quote marks and a literal "\n".
func TestLocaleStringsHaveNoVisibleYAMLEscapeArtifacts(t *testing.T) {
	for _, code := range localeCodes {
		var bad []string
		for key, value := range readLocaleFile(t, code) {
			if scalarCheckSkipped[key] {
				continue
			}
			trimmed := strings.TrimSpace(value)
			switch {
			case strings.HasPrefix(trimmed, `"`), strings.HasSuffix(trimmed, `"`) && strings.Count(trimmed, `"`)%2 == 1:
				// A trailing quote closing a quoted word ("…\"hello\"") is text; an
				// unpaired one is the wrapper of a "|" block.
				bad = append(bad, key+" (wrapped in quote marks)")
			case strings.Contains(value, `\n`):
				bad = append(bad, key+` (literal \n)`)
			}
		}
		sort.Strings(bad)
		for _, b := range bad {
			t.Errorf("%s.yml: %s", code, b)
		}
	}
}
