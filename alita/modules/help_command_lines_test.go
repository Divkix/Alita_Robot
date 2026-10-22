//go:build testtools

package modules

import (
	"sort"
	"strings"
	"testing"
)

// Every command listed in en <module>_extend_msg must have its own line in
// <module>_help_msg in every locale. Commands match by whole name, so /ban is
// not satisfied by /banme.
func TestEveryExtendCommandHasALineInEveryLocaleHelp(t *testing.T) {
	locales := make(map[string]map[string]string, len(localeCodes))
	for _, code := range localeCodes {
		locales[code] = readLocaleFile(t, code)
	}

	for module, commands := range extendCommands(t) {
		for _, code := range localeCodes {
			defined := map[string]bool{}
			for _, line := range helpCommandLine.FindAllStringSubmatch(locales[code][module+"_help_msg"], -1) {
				for _, m := range helpCommandName.FindAllStringSubmatch(line[1], -1) {
					defined[m[1]] = true
				}
			}
			var missing []string
			for _, cmd := range commands {
				if !defined[cmd] {
					missing = append(missing, "/"+cmd)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf("%s.yml: %s_help_msg has no line for %s", code, module, strings.Join(missing, " "))
			}
		}
	}
}
