//go:build testtools

package modules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var leftoverMarkdownBold = regexp.MustCompile(`\*[^*\s][^*]*\*`)

func TestAllLocaleHelpMessagesKeepHTMLTags(t *testing.T) {
	t.Parallel()

	localeDir := findLocalesDir(t)
	entries, err := os.ReadDir(localeDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", localeDir, err)
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "config.yml" || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		path := filepath.Join(localeDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		var data map[string]any
		if err := yaml.Unmarshal(raw, &data); err != nil {
			t.Fatalf("yaml.Unmarshal(%s): %v", path, err)
		}
		header, _ := data["helpers_module_help_header"].(string)
		if header == "" {
			t.Fatalf("%s missing helpers_module_help_header", path)
		}
		for key, value := range data {
			if !strings.HasSuffix(key, "_help_msg") {
				continue
			}
			helpMsg, ok := value.(string)
			if !ok {
				t.Fatalf("%s %s is %T, want string", path, key, value)
			}
			checked++
			got := renderModuleHelp(header, "Reactions", helpMsg)
			for _, escaped := range []string{"&lt;b&gt;", "&lt;/b&gt;", "&lt;i&gt;", "&lt;code&gt;", "&lt;/code&gt;"} {
				if strings.Contains(got, escaped) {
					t.Errorf("%s %s rendered escaped tag %s:\n%s", path, key, escaped, got)
					break
				}
			}
			if isHTMLAuthoredHelp(helpMsg) {
				if strings.Contains(helpMsg, "`") {
					t.Errorf("%s %s HTML help contains markdown backticks", path, key)
				}
				if leftoverMarkdownBold.MatchString(helpMsg) {
					t.Errorf("%s %s HTML help contains leftover *bold* markdown", path, key)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no *_help_msg keys found in locales")
	}
}

func isHTMLAuthoredHelp(s string) bool {
	hasOpen := strings.Contains(s, "<b>") || strings.Contains(s, "<code>") ||
		strings.Contains(s, "<i>") || strings.Contains(s, "<u>")
	hasClose := strings.Contains(s, "</b>") || strings.Contains(s, "</code>") ||
		strings.Contains(s, "</i>") || strings.Contains(s, "</u>")
	return hasOpen && hasClose
}

func findLocalesDir(t *testing.T) string {
	t.Helper()
	candidates := []string{"locales", "../locales", "../../locales"}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	t.Fatal("cannot find locales directory")
	return ""
}
