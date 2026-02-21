package helpers

import (
	"fmt"
	"strings"
	"testing"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
)

// ---------------------------------------------------------------------------
// IsChannelID
// ---------------------------------------------------------------------------

func TestIsChannelIDTrue(t *testing.T) {
	t.Parallel()

	ids := []int64{-1000000000001, -1000000123456, -9999999999999}
	for _, id := range ids {

		t.Run(fmt.Sprintf("id=%d", id), func(t *testing.T) {
			t.Parallel()
			if !IsChannelID(id) {
				t.Fatalf("IsChannelID(%d) expected true", id)
			}
		})
	}
}

func TestIsChannelIDFalse(t *testing.T) {
	t.Parallel()

	ids := []int64{0, 123456, -1000000000000, -999999999999, -100}
	for _, id := range ids {

		t.Run(fmt.Sprintf("id=%d", id), func(t *testing.T) {
			t.Parallel()
			if IsChannelID(id) {
				t.Fatalf("IsChannelID(%d) expected false", id)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SplitMessage
// ---------------------------------------------------------------------------

func TestSplitMessageShort(t *testing.T) {
	t.Parallel()

	msg := "hello world"
	parts := SplitMessage(msg)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part for short message, got %d", len(parts))
	}
	if parts[0] != msg {
		t.Fatalf("expected %q, got %q", msg, parts[0])
	}
}

func TestSplitMessageExactLimit(t *testing.T) {
	t.Parallel()

	msg := strings.Repeat("a", MaxMessageLength)
	parts := SplitMessage(msg)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part at exact limit, got %d", len(parts))
	}
}

func TestSplitMessageLong(t *testing.T) {
	t.Parallel()

	// Build a message with many short lines that together exceed limit.
	line := strings.Repeat("x", 100) + "\n"
	repeat := (MaxMessageLength / 100) + 5
	msg := strings.Repeat(line, repeat)
	parts := SplitMessage(msg)
	if len(parts) < 2 {
		t.Fatalf("expected multiple parts for long message, got %d", len(parts))
	}
	// Reconstruct and verify no data lost.
	joined := strings.Join(parts, "")
	if joined != msg {
		t.Fatalf("joined parts do not match original message")
	}
}

func TestSplitMessageUnicode(t *testing.T) {
	t.Parallel()

	// Each Chinese character is 3 bytes but 1 rune. A message of exactly
	// MaxMessageLength runes should still be 1 part.
	msg := strings.Repeat("中", MaxMessageLength)
	parts := SplitMessage(msg)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part for unicode message at rune limit, got %d", len(parts))
	}
}

func TestSplitMessageVeryLongLine(t *testing.T) {
	t.Parallel()

	// A single line longer than MaxMessageLength must produce multiple parts.
	msg := strings.Repeat("y", MaxMessageLength*2+1)
	parts := SplitMessage(msg)
	if len(parts) < 2 {
		t.Fatalf("expected multiple parts for very long single line, got %d", len(parts))
	}
}

// ---------------------------------------------------------------------------
// MentionHtml and MentionUrl
// ---------------------------------------------------------------------------

func TestMentionHtml(t *testing.T) {
	t.Parallel()

	result := MentionHtml(123456, "John")
	expected := `<a href="tg://user?id=123456">John</a>`
	if result != expected {
		t.Fatalf("MentionHtml expected %q, got %q", expected, result)
	}
}

func TestMentionUrlEscapesName(t *testing.T) {
	t.Parallel()

	result := MentionUrl("https://example.com", "<b>Evil</b>")
	if strings.Contains(result, "<b>") {
		t.Fatalf("MentionUrl should HTML-escape the name, got %q", result)
	}
	if !strings.Contains(result, "&lt;b&gt;") {
		t.Fatalf("MentionUrl expected escaped name, got %q", result)
	}
}

func TestMentionUrlFormat(t *testing.T) {
	t.Parallel()

	result := MentionUrl("https://t.me/test", "Alice")
	expected := `<a href="https://t.me/test">Alice</a>`
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

// ---------------------------------------------------------------------------
// HtmlEscape
// ---------------------------------------------------------------------------

func TestHtmlEscapeAmpersand(t *testing.T) {
	t.Parallel()

	if got := HtmlEscape("a & b"); got != "a &amp; b" {
		t.Fatalf("expected 'a &amp; b', got %q", got)
	}
}

func TestHtmlEscapeAngles(t *testing.T) {
	t.Parallel()

	if got := HtmlEscape("<script>"); got != "&lt;script&gt;" {
		t.Fatalf("expected '&lt;script&gt;', got %q", got)
	}
}

func TestHtmlEscapeNoChange(t *testing.T) {
	t.Parallel()

	plain := "hello world 123"
	if got := HtmlEscape(plain); got != plain {
		t.Fatalf("expected no change for %q, got %q", plain, got)
	}
}

func TestHtmlEscapeMultiple(t *testing.T) {
	t.Parallel()

	got := HtmlEscape("<a> & <b>")
	expected := "&lt;a&gt; &amp; &lt;b&gt;"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// ---------------------------------------------------------------------------
// GetFullName
// ---------------------------------------------------------------------------

func TestGetFullNameWithLastName(t *testing.T) {
	t.Parallel()

	name := GetFullName("John", "Doe")
	if name != "John Doe" {
		t.Fatalf("expected 'John Doe', got %q", name)
	}
}

func TestGetFullNameNoLastName(t *testing.T) {
	t.Parallel()

	name := GetFullName("Alice", "")
	if name != "Alice" {
		t.Fatalf("expected 'Alice', got %q", name)
	}
}

// ---------------------------------------------------------------------------
// BuildKeyboard
// ---------------------------------------------------------------------------

func TestBuildKeyboardEmpty(t *testing.T) {
	t.Parallel()

	keyb := BuildKeyboard([]db.Button{})
	if len(keyb) != 0 {
		t.Fatalf("expected empty keyboard, got %d rows", len(keyb))
	}
}

func TestBuildKeyboardSingleButton(t *testing.T) {
	t.Parallel()

	btns := []db.Button{{Name: "Click me", Url: "https://example.com", SameLine: false}}
	keyb := BuildKeyboard(btns)
	if len(keyb) != 1 {
		t.Fatalf("expected 1 row, got %d", len(keyb))
	}
	if len(keyb[0]) != 1 {
		t.Fatalf("expected 1 button in row, got %d", len(keyb[0]))
	}
	if keyb[0][0].Text != "Click me" {
		t.Fatalf("expected button text 'Click me', got %q", keyb[0][0].Text)
	}
}

func TestBuildKeyboardSameLine(t *testing.T) {
	t.Parallel()

	btns := []db.Button{
		{Name: "A", Url: "https://a.com", SameLine: false},
		{Name: "B", Url: "https://b.com", SameLine: true},
	}
	keyb := BuildKeyboard(btns)
	if len(keyb) != 1 {
		t.Fatalf("expected 1 row (B is same-line as A), got %d", len(keyb))
	}
	if len(keyb[0]) != 2 {
		t.Fatalf("expected 2 buttons in same row, got %d", len(keyb[0]))
	}
}

func TestBuildKeyboardNewLines(t *testing.T) {
	t.Parallel()

	btns := []db.Button{
		{Name: "A", Url: "https://a.com", SameLine: false},
		{Name: "B", Url: "https://b.com", SameLine: false},
		{Name: "C", Url: "https://c.com", SameLine: false},
	}
	keyb := BuildKeyboard(btns)
	if len(keyb) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(keyb))
	}
}

func TestBuildKeyboardSameLineFirst(t *testing.T) {
	t.Parallel()

	// SameLine=true on first button when keyb is empty should start new row.
	btns := []db.Button{{Name: "Solo", Url: "https://solo.com", SameLine: true}}
	keyb := BuildKeyboard(btns)
	if len(keyb) != 1 {
		t.Fatalf("expected 1 row, got %d", len(keyb))
	}
}

// ---------------------------------------------------------------------------
// ConvertButtonV2ToDbButton
// ---------------------------------------------------------------------------

func TestConvertButtonV2ToDbButton(t *testing.T) {
	t.Parallel()

	v2Btns := []tgmd2html.ButtonV2{
		{Name: "Btn1", Content: "https://one.com", SameLine: false},
		{Name: "Btn2", Content: "https://two.com", SameLine: true},
	}
	dbBtns := ConvertButtonV2ToDbButton(v2Btns)
	if len(dbBtns) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(dbBtns))
	}
	if dbBtns[0].Name != "Btn1" || dbBtns[0].Url != "https://one.com" || dbBtns[0].SameLine {
		t.Fatalf("unexpected first button: %+v", dbBtns[0])
	}
	if dbBtns[1].Name != "Btn2" || dbBtns[1].Url != "https://two.com" || !dbBtns[1].SameLine {
		t.Fatalf("unexpected second button: %+v", dbBtns[1])
	}
}

func TestConvertButtonV2ToDbButtonEmpty(t *testing.T) {
	t.Parallel()

	dbBtns := ConvertButtonV2ToDbButton([]tgmd2html.ButtonV2{})
	if len(dbBtns) != 0 {
		t.Fatalf("expected 0 buttons, got %d", len(dbBtns))
	}
}

// ---------------------------------------------------------------------------
// RevertButtons
// ---------------------------------------------------------------------------

func TestRevertButtonsEmpty(t *testing.T) {
	t.Parallel()

	result := RevertButtons([]db.Button{})
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestRevertButtonsRegular(t *testing.T) {
	t.Parallel()

	btns := []db.Button{{Name: "Click", Url: "https://example.com", SameLine: false}}
	result := RevertButtons(btns)
	expected := "\n[Click](buttonurl://https://example.com)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRevertButtonsSameLine(t *testing.T) {
	t.Parallel()

	btns := []db.Button{{Name: "Click", Url: "https://example.com", SameLine: true}}
	result := RevertButtons(btns)
	expected := "\n[Click](buttonurl://https://example.com:same)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRevertButtonsMultiple(t *testing.T) {
	t.Parallel()

	btns := []db.Button{
		{Name: "A", Url: "https://a.com", SameLine: false},
		{Name: "B", Url: "https://b.com", SameLine: true},
	}
	result := RevertButtons(btns)
	if !strings.Contains(result, "[A](buttonurl://https://a.com)") {
		t.Fatalf("expected A button in result, got %q", result)
	}
	if !strings.Contains(result, "[B](buttonurl://https://b.com:same)") {
		t.Fatalf("expected B sameline button in result, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// ChunkKeyboardSlices
// ---------------------------------------------------------------------------

func TestChunkKeyboardSlicesEmpty(t *testing.T) {
	t.Parallel()

	chunks := ChunkKeyboardSlices([]gotgbot.InlineKeyboardButton{}, 2)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestChunkKeyboardSlicesEven(t *testing.T) {
	t.Parallel()

	btns := []gotgbot.InlineKeyboardButton{
		{Text: "A"}, {Text: "B"}, {Text: "C"}, {Text: "D"},
	}
	chunks := ChunkKeyboardSlices(btns, 2)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len(chunk) != 2 {
			t.Fatalf("expected chunk size 2, got %d", len(chunk))
		}
	}
}

func TestChunkKeyboardSlicesOdd(t *testing.T) {
	t.Parallel()

	btns := []gotgbot.InlineKeyboardButton{
		{Text: "A"}, {Text: "B"}, {Text: "C"},
	}
	chunks := ChunkKeyboardSlices(btns, 2)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 3 buttons with size 2, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 {
		t.Fatalf("expected first chunk size 2, got %d", len(chunks[0]))
	}
	if len(chunks[1]) != 1 {
		t.Fatalf("expected last chunk size 1, got %d", len(chunks[1]))
	}
}

func TestChunkKeyboardSlicesLargerThanSlice(t *testing.T) {
	t.Parallel()

	btns := []gotgbot.InlineKeyboardButton{{Text: "A"}}
	chunks := ChunkKeyboardSlices(btns, 10)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if len(chunks[0]) != 1 {
		t.Fatalf("expected 1 button in chunk, got %d", len(chunks[0]))
	}
}

// ---------------------------------------------------------------------------
// ReverseHTML2MD
// ---------------------------------------------------------------------------

func TestReverseHTML2MDBold(t *testing.T) {
	t.Parallel()

	input := "<b>hello</b>"
	result := ReverseHTML2MD(input)
	if !strings.Contains(result, "*hello*") {
		t.Fatalf("expected bold markdown in %q", result)
	}
}

func TestReverseHTML2MDItalic(t *testing.T) {
	t.Parallel()

	input := "<i>hello</i>"
	result := ReverseHTML2MD(input)
	if !strings.Contains(result, "_hello_") {
		t.Fatalf("expected italic markdown in %q", result)
	}
}

func TestReverseHTML2MDLink(t *testing.T) {
	t.Parallel()

	input := `<a href="https://example.com">click</a>`
	result := ReverseHTML2MD(input)
	if !strings.Contains(result, "click") || !strings.Contains(result, "https://example.com") {
		t.Fatalf("expected link markdown in %q", result)
	}
}

func TestReverseHTML2MDPlainText(t *testing.T) {
	t.Parallel()

	input := "no html here"
	result := ReverseHTML2MD(input)
	if result != input {
		t.Fatalf("plain text should not be modified, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// IsExpectedTelegramError
// ---------------------------------------------------------------------------

func TestIsExpectedTelegramErrorNil(t *testing.T) {
	t.Parallel()

	if IsExpectedTelegramError(nil) {
		t.Fatalf("IsExpectedTelegramError(nil) expected false")
	}
}

func TestIsExpectedTelegramErrorKnown(t *testing.T) {
	t.Parallel()

	knownErrors := []string{
		"bot was kicked from the group",
		"bot was blocked by the user",
		"chat not found",
		"message can't be deleted",
		"message to delete not found",
		"group chat was deactivated",
		"not enough rights to restrict/unrestrict chat member",
		"context deadline exceeded",
		"message thread not found",
	}
	for _, msg := range knownErrors {
		t.Run(msg, func(t *testing.T) {
			t.Parallel()
			err := fmt.Errorf("%s", msg)
			if !IsExpectedTelegramError(err) {
				t.Fatalf("IsExpectedTelegramError(%q) expected true", msg)
			}
		})
	}
}

func TestIsExpectedTelegramErrorUnknown(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("some unknown telegram error xyz")
	if IsExpectedTelegramError(err) {
		t.Fatalf("IsExpectedTelegramError for unknown error expected false")
	}
}

// ---------------------------------------------------------------------------
// notesParser (unexported — accessible via package helpers internal test)
// ---------------------------------------------------------------------------

func TestNotesParserPrivate(t *testing.T) {
	t.Parallel()

	pvt, _, _, _, _, _, _ := notesParser("{private} hello")
	if !pvt {
		t.Fatalf("expected pvtOnly=true")
	}
}

func TestNotesParserNoPrivate(t *testing.T) {
	t.Parallel()

	_, grp, _, _, _, _, _ := notesParser("{noprivate} hello")
	if !grp {
		t.Fatalf("expected grpOnly=true")
	}
}

func TestNotesParserAdmin(t *testing.T) {
	t.Parallel()

	_, _, admin, _, _, _, _ := notesParser("{admin} only admins")
	if !admin {
		t.Fatalf("expected adminOnly=true")
	}
}

func TestNotesParserPreview(t *testing.T) {
	t.Parallel()

	_, _, _, web, _, _, _ := notesParser("{preview} with link")
	if !web {
		t.Fatalf("expected webPrev=true")
	}
}

func TestNotesParserProtect(t *testing.T) {
	t.Parallel()

	_, _, _, _, protect, _, _ := notesParser("{protect} content")
	if !protect {
		t.Fatalf("expected protectedContent=true")
	}
}

func TestNotesParserNoNotif(t *testing.T) {
	t.Parallel()

	_, _, _, _, _, noNotif, _ := notesParser("{nonotif} quiet")
	if !noNotif {
		t.Fatalf("expected noNotif=true")
	}
}

func TestNotesParserTagsRemovedFromOutput(t *testing.T) {
	t.Parallel()

	_, _, _, _, _, _, sentBack := notesParser("{private}{admin} some text")
	if strings.Contains(sentBack, "{private}") || strings.Contains(sentBack, "{admin}") {
		t.Fatalf("tags should be removed from output, got %q", sentBack)
	}
	if !strings.Contains(sentBack, "some text") {
		t.Fatalf("content should remain in output, got %q", sentBack)
	}
}

func TestNotesParserNoFlags(t *testing.T) {
	t.Parallel()

	pvt, grp, admin, web, protect, noNotif, sentBack := notesParser("normal text")
	if pvt || grp || admin || web || protect || noNotif {
		t.Fatalf("expected all flags false for plain text")
	}
	if !strings.Contains(sentBack, "normal text") {
		t.Fatalf("expected text preserved, got %q", sentBack)
	}
}
