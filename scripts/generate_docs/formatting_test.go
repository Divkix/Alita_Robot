package main

import (
	"testing"
)

func TestConvertTelegramMarkdown_CrossBulletWithCommand(t *testing.T) {
	input := "× /flood: Get the current antiflood settings."
	want := "- `/flood`: Get the current antiflood settings."
	got := convertTelegramMarkdown(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertTelegramMarkdown_CrossBulletWithoutCommand(t *testing.T) {
	input := "× Some plain text item"
	want := "- Some plain text item"
	got := convertTelegramMarkdown(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertTelegramMarkdown_MultipleCrossBullets(t *testing.T) {
	input := "× /flood: First\n× /setflood: Second\n× /setfloodmode: Third"
	want := "- `/flood`: First\n- `/setflood`: Second\n- `/setfloodmode`: Third"
	got := convertTelegramMarkdown(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertTelegramMarkdown_DotBullets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bullet dot", "• Feature item", "- Feature item"},
		{"middle dot", "· Another item", "- Another item"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertTelegramMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertTelegramMarkdown_HTMLTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bold", "<b>Important</b>", "**Important**"},
		{"italic", "<i>note</i>", "*note*"},
		{"code", "<code>/command</code>", "`/command`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertTelegramMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertTelegramMarkdown_HTMLEntities(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ampersand", "Text &amp; Media", "Text & Media"},
		{"less-than", "Set to &lt;number&gt;", "Set to <number>"},
		{"combined with tags", "<b>Text &amp; Media</b>", "**Text & Media**"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertTelegramMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertTelegramMarkdown_ArrowSubExample(t *testing.T) {
	input := "-> /filter hello Hello there!"
	want := "  - `/filter hello Hello there!`"
	got := convertTelegramMarkdown(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertTelegramMarkdown_MultipleArrows(t *testing.T) {
	input := "-> /filter hello Hi\n-> /filter bye Goodbye"
	want := "  - `/filter hello Hi`\n  - `/filter bye Goodbye`"
	got := convertTelegramMarkdown(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatHelpText_SectionHeader(t *testing.T) {
	input := "*Admin commands*:"
	want := "### Admin commands"
	got := formatHelpText(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatHelpText_SectionHeaderWithoutColon(t *testing.T) {
	input := "*User Commands*"
	want := "### User Commands"
	got := formatHelpText(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatHelpText_InlineBold(t *testing.T) {
	input := "use *bold* formatting in text"
	want := "use **bold** formatting in text"
	got := formatHelpText(input)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatHelpText_AntifloodHelp(t *testing.T) {
	input := `You know how sometimes, people join, send 100 messages, and ruin your chat? With antiflood, that happens no more!

*Admin commands*:

× /flood: Get the current antiflood settings.

× /setflood: Set the number of messages after which to take action on a user.

× /setfloodmode: Choose which action to take on a user who has been flooding.`

	got := formatHelpText(input)

	// Should have ### heading
	if !contains(got, "### Admin commands") {
		t.Error("expected ### Admin commands heading")
	}
	// Should have backtick-wrapped commands
	if !contains(got, "- `/flood`:") {
		t.Errorf("expected backtick-wrapped /flood command, got:\n%s", got)
	}
	if !contains(got, "- `/setflood`:") {
		t.Errorf("expected backtick-wrapped /setflood command, got:\n%s", got)
	}
}

func TestFormatHelpText_FiltersHelpWithArrows(t *testing.T) {
	input := `Filters are case insensitive.

Commands:

- /filter <trigger> <reply>: Set a filter.

Examples:

- Set a filter:

-> /filter hello Hello there!

- Set a multiword filter:

-> /filter hello friend Hello back!`

	got := formatHelpText(input)

	if !contains(got, "  - `/filter hello Hello there!`") {
		t.Errorf("expected indented sub-example, got:\n%s", got)
	}
	if !contains(got, "  - `/filter hello friend Hello back!`") {
		t.Errorf("expected indented multiword sub-example, got:\n%s", got)
	}
}

func TestConvertTelegramMarkdown_EmptyInput(t *testing.T) {
	got := convertTelegramMarkdown("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatHelpText_EmptyInput(t *testing.T) {
	got := formatHelpText("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestConvertTelegramMarkdown_NoSpecialPatterns(t *testing.T) {
	input := "This is plain text with no special patterns."
	got := convertTelegramMarkdown(input)
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

func TestFormatHelpText_NestedFormatting(t *testing.T) {
	input := "*Section*:\n× /cmd: Does <b>bold</b> &amp; things"
	got := formatHelpText(input)
	if !contains(got, "### Section") {
		t.Errorf("expected ### Section, got:\n%s", got)
	}
	if !contains(got, "**bold**") {
		t.Errorf("expected **bold**, got:\n%s", got)
	}
	if !contains(got, "& things") {
		t.Errorf("expected decoded ampersand, got:\n%s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
