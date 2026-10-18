package main

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestResolveOutputPath(t *testing.T) {
	t.Parallel()

	projectRoot := filepath.Join(string(filepath.Separator), "repo", "alita")
	absoluteOutput := filepath.Join(string(filepath.Separator), "tmp", "alita-docs")

	tests := []struct {
		name       string
		outputPath string
		want       string
	}{
		{
			name:       "relative output is resolved under project root",
			outputPath: filepath.Join("docs", "src", "content", "docs"),
			want:       filepath.Join(projectRoot, "docs", "src", "content", "docs"),
		},
		{
			name:       "absolute output is preserved",
			outputPath: absoluteOutput,
			want:       absoluteOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveOutputPath(projectRoot, tt.outputPath)
			if got != tt.want {
				t.Fatalf("resolveOutputPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// alt_names keys such as LogChannels are not title case; their aliases and
// spelling must still reach the module built from logchannels_help_msg.
func TestBuildModulesMatchesAliasKeysCaseInsensitively(t *testing.T) {
	modules := buildModules(
		map[string]string{"logchannels_help_msg": "Log help"},
		map[string]ExtendedDocs{"antiraid": {}},
		nil,
		map[string][]string{
			"LogChannels": {"setlog", "logchannel"},
			"AntiRaid":    {"raid"},
		},
	)

	if len(modules) != 2 {
		t.Fatalf("buildModules() returned %d modules, want 2", len(modules))
	}
	for i, want := range []struct {
		name, display string
		aliases       []string
	}{
		{name: "antiraid", display: "AntiRaid", aliases: []string{"raid"}},
		{name: "logchannels", display: "LogChannels", aliases: []string{"setlog", "logchannel"}},
	} {
		got := modules[i]
		if got.Name != want.name || got.DisplayName != want.display {
			t.Errorf("module %d = %q/%q, want %q/%q", i, got.Name, got.DisplayName, want.name, want.display)
		}
		if !slices.Equal(got.Aliases, want.aliases) {
			t.Errorf("%s aliases = %v, want %v", want.name, got.Aliases, want.aliases)
		}
	}
}
