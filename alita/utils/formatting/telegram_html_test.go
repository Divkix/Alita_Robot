package formatting

import (
	"strings"
	"testing"
)

func TestToTelegramHTMLMarkdownBold(t *testing.T) {
	t.Parallel()

	got := ToTelegramHTML("*Admin Commands:*")
	if !strings.Contains(got, "<b>") || !strings.Contains(got, "Admin Commands") {
		t.Fatalf("ToTelegramHTML(markdown) = %q, want HTML bold", got)
	}
	if strings.Contains(got, "&lt;b&gt;") {
		t.Fatalf("ToTelegramHTML(markdown) escaped its own tags: %q", got)
	}
}

func TestToTelegramHTMLPreservesHTMLBold(t *testing.T) {
	t.Parallel()

	input := "<b>🎭 Auto-Reactions</b>\n\n<b>Admin Commands:</b>"
	got := ToTelegramHTML(input)
	if !strings.Contains(got, "<b>🎭 Auto-Reactions</b>") {
		t.Fatalf("ToTelegramHTML(html) = %q, want preserved <b> tags", got)
	}
	if strings.Contains(got, "&lt;b&gt;") {
		t.Fatalf("ToTelegramHTML(html) escaped <b> tags: %q", got)
	}
}

func TestToTelegramHTMLEscapesCommandPlaceholdersInHTML(t *testing.T) {
	t.Parallel()

	input := "<b>Admin Commands:</b>\n• /addreaction <keyword> <emoji> - Add"
	got := ToTelegramHTML(input)
	if !strings.Contains(got, "<b>Admin Commands:</b>") {
		t.Fatalf("ToTelegramHTML() lost heading tags: %q", got)
	}
	if !strings.Contains(got, "&lt;keyword&gt;") || !strings.Contains(got, "&lt;emoji&gt;") {
		t.Fatalf("ToTelegramHTML() did not escape placeholders: %q", got)
	}
	if strings.Contains(got, "<keyword>") || strings.Contains(got, "<emoji>") {
		t.Fatalf("ToTelegramHTML() left raw placeholder tags: %q", got)
	}
}

func TestToTelegramHTMLEscapesPlaceholdersInsideCodeTags(t *testing.T) {
	t.Parallel()

	input := `/approve <code><reply/username/mention/userid></code>`
	got := ToTelegramHTML(input)
	want := `/approve <code>&lt;reply/username/mention/userid&gt;</code>`
	if got != want {
		t.Fatalf("ToTelegramHTML() = %q, want %q", got, want)
	}
}

func TestToTelegramHTMLEscapesAmpersandButKeepsEntities(t *testing.T) {
	t.Parallel()

	got := ToTelegramHTML("<b>Backup & Restore</b>")
	if got != "<b>Backup &amp; Restore</b>" {
		t.Fatalf("ToTelegramHTML(raw amp) = %q, want escaped ampersand", got)
	}

	got = ToTelegramHTML("<b>Texte &amp; Médias</b>")
	if got != "<b>Texte &amp; Médias</b>" {
		t.Fatalf("ToTelegramHTML(entity) = %q, want entity preserved", got)
	}
}

func TestToTelegramHTMLMarkdownPlaceholdersStayEscaped(t *testing.T) {
	t.Parallel()

	got := ToTelegramHTML("× /promote `<reply/username>`: Promote a user.")
	if strings.Contains(got, "<reply/username>") {
		t.Fatalf("ToTelegramHTML(markdown) left raw placeholder: %q", got)
	}
	if !strings.Contains(got, "&lt;reply/username&gt;") {
		t.Fatalf("ToTelegramHTML(markdown) = %q, want escaped placeholder", got)
	}
}

func TestToTelegramHTMLPreservesTrailingNewlines(t *testing.T) {
	t.Parallel()

	got := ToTelegramHTML("Here is the help for the *Reactions* module:\n\n")
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("ToTelegramHTML(header) = %q, want trailing blank line", got)
	}
	if !strings.Contains(got, "<b>Reactions</b>") {
		t.Fatalf("ToTelegramHTML(header) = %q, want converted module name", got)
	}
}

func TestToTelegramHTMLEmpty(t *testing.T) {
	t.Parallel()

	if got := ToTelegramHTML(""); got != "" {
		t.Fatalf("ToTelegramHTML(empty) = %q, want empty", got)
	}
	if got := ToTelegramHTML("\n\n"); got != "\n\n" {
		t.Fatalf("ToTelegramHTML(newlines) = %q, want unchanged newlines", got)
	}
}

func TestToTelegramHTMLMarkdownCloseTagInCodeSpanStaysMarkdown(t *testing.T) {
	t.Parallel()

	got := ToTelegramHTML("Use `</b>` to end *bold* text.")
	if strings.Contains(got, "`") {
		t.Fatalf("ToTelegramHTML() treated markdown as HTML: %q", got)
	}
	if !strings.Contains(got, "<code>") || !strings.Contains(got, "&lt;/b&gt;") {
		t.Fatalf("ToTelegramHTML() = %q, want markdown code span around escaped </b>", got)
	}
	if !strings.Contains(got, "<b>bold</b>") {
		t.Fatalf("ToTelegramHTML() = %q, want converted markdown bold", got)
	}
}

func TestToTelegramHTMLOpenTagWithoutCloserStaysMarkdown(t *testing.T) {
	t.Parallel()

	got := ToTelegramHTML("Mention <b> in *help* text")
	if !strings.Contains(got, "&lt;b&gt;") {
		t.Fatalf("ToTelegramHTML() = %q, want escaped standalone <b>", got)
	}
	if !strings.Contains(got, "<b>help</b>") {
		t.Fatalf("ToTelegramHTML() = %q, want converted markdown bold", got)
	}
}
