package helpers

import (
	"strings"
	"testing"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
)

// --- SplitMessage ---

func TestSplitMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantParts int
	}{
		{
			name:      "short message stays as single part",
			input:     "hello world",
			wantParts: 1,
		},
		{
			name:      "empty message stays as single part",
			input:     "",
			wantParts: 1,
		},
		{
			name:      "exact max length is single part",
			input:     strings.Repeat("a", MaxMessageLength),
			wantParts: 1,
		},
		{
			name:      "one rune over max splits into two",
			input:     strings.Repeat("a", MaxMessageLength) + "\n" + "b",
			wantParts: 2,
		},
		{
			name:      "multibyte emoji counted as runes not bytes",
			input:     strings.Repeat("🔥", MaxMessageLength),
			wantParts: 1,
		},
		{
			name:      "very long single line splits into chunks",
			input:     strings.Repeat("x", MaxMessageLength*2),
			wantParts: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parts := SplitMessage(tc.input)
			if len(parts) != tc.wantParts {
				t.Errorf("SplitMessage() got %d parts, want %d", len(parts), tc.wantParts)
			}
		})
	}
}

func TestSplitMessage_ContentPreserved(t *testing.T) {
	t.Parallel()
	msg := strings.Repeat("line\n", MaxMessageLength/5)
	parts := SplitMessage(msg)
	joined := strings.Join(parts, "")
	// All original content should still be present (order preserved)
	if joined != msg {
		t.Errorf("SplitMessage() lost content: len(joined)=%d, len(original)=%d", len(joined), len(msg))
	}
}

// --- MentionHtml ---

func TestMentionHtml(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		userId int64
		name_  string
		want   string
	}{
		{
			name:   "normal user",
			userId: 123456,
			name_:  "Alice",
			want:   `<a href="tg://user?id=123456">Alice</a>`,
		},
		{
			name:   "name with HTML special chars is escaped",
			userId: 1,
			name_:  "<Bob>",
			want:   `<a href="tg://user?id=1">&lt;Bob&gt;</a>`,
		},
		{
			name:   "large user ID",
			userId: 9999999999,
			name_:  "Carol",
			want:   `<a href="tg://user?id=9999999999">Carol</a>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := MentionHtml(tc.userId, tc.name_)
			if got != tc.want {
				t.Errorf("MentionHtml() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- MentionUrl ---

func TestMentionUrl(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		url     string
		display string
		want    string
	}{
		{
			name:    "normal url and name",
			url:     "https://example.com",
			display: "Click",
			want:    `<a href="https://example.com">Click</a>`,
		},
		{
			name:    "name with ampersand is html escaped",
			url:     "https://example.com",
			display: "A&B",
			want:    `<a href="https://example.com">A&amp;B</a>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := MentionUrl(tc.url, tc.display)
			if got != tc.want {
				t.Errorf("MentionUrl() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- HtmlEscape ---

func TestHtmlEscape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ampersand", "&", "&amp;"},
		{"less than", "<", "&lt;"},
		{"greater than", ">", "&gt;"},
		{"normal string unchanged", "hello world", "hello world"},
		{"combined chars", "<script>&", "&lt;script&gt;&amp;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HtmlEscape(tc.input)
			if got != tc.want {
				t.Errorf("HtmlEscape(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// --- GetFullName ---

func TestGetFullName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		firstName string
		lastName  string
		want      string
	}{
		{"both names", "John", "Doe", "John Doe"},
		{"first name only", "Alice", "", "Alice"},
		{"both empty", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := GetFullName(tc.firstName, tc.lastName)
			if got != tc.want {
				t.Errorf("GetFullName(%q, %q) = %q, want %q", tc.firstName, tc.lastName, got, tc.want)
			}
		})
	}
}

// --- BuildKeyboard ---

func TestBuildKeyboard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		buttons  []db.Button
		wantRows int
	}{
		{
			name:     "empty buttons returns empty keyboard",
			buttons:  []db.Button{},
			wantRows: 0,
		},
		{
			name: "single button creates one row",
			buttons: []db.Button{
				{Name: "Btn1", Url: "https://a.com", SameLine: false},
			},
			wantRows: 1,
		},
		{
			name: "same-line button joins existing row",
			buttons: []db.Button{
				{Name: "Btn1", Url: "https://a.com", SameLine: false},
				{Name: "Btn2", Url: "https://b.com", SameLine: true},
			},
			wantRows: 1,
		},
		{
			name: "non-same-line button creates new row",
			buttons: []db.Button{
				{Name: "Btn1", Url: "https://a.com", SameLine: false},
				{Name: "Btn2", Url: "https://b.com", SameLine: false},
			},
			wantRows: 2,
		},
		{
			name: "first same-line button without prior row starts new row",
			buttons: []db.Button{
				{Name: "Btn1", Url: "https://a.com", SameLine: true},
			},
			wantRows: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kb := BuildKeyboard(tc.buttons)
			if len(kb) != tc.wantRows {
				t.Errorf("BuildKeyboard() rows = %d, want %d", len(kb), tc.wantRows)
			}
		})
	}
}

func TestBuildKeyboard_SameLineContent(t *testing.T) {
	t.Parallel()
	buttons := []db.Button{
		{Name: "A", Url: "https://a.com", SameLine: false},
		{Name: "B", Url: "https://b.com", SameLine: true},
		{Name: "C", Url: "https://c.com", SameLine: false},
	}
	kb := BuildKeyboard(buttons)
	if len(kb) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(kb))
	}
	if len(kb[0]) != 2 {
		t.Errorf("row 0 should have 2 buttons, got %d", len(kb[0]))
	}
	if len(kb[1]) != 1 {
		t.Errorf("row 1 should have 1 button, got %d", len(kb[1]))
	}
}

// --- ConvertButtonV2ToDbButton ---

func TestConvertButtonV2ToDbButton(t *testing.T) {
	t.Parallel()
	t.Run("empty input returns empty slice", func(t *testing.T) {
		t.Parallel()
		got := ConvertButtonV2ToDbButton([]tgmd2html.ButtonV2{})
		if len(got) != 0 {
			t.Errorf("expected 0, got %d", len(got))
		}
	})
	t.Run("populated input maps fields correctly", func(t *testing.T) {
		t.Parallel()
		input := []tgmd2html.ButtonV2{
			{Name: "Google", Content: "https://google.com", SameLine: false},
			{Name: "GitHub", Content: "https://github.com", SameLine: true},
		}
		got := ConvertButtonV2ToDbButton(input)
		if len(got) != 2 {
			t.Fatalf("expected 2, got %d", len(got))
		}
		if got[0].Name != "Google" || got[0].Url != "https://google.com" || got[0].SameLine != false {
			t.Errorf("first button mismatch: %+v", got[0])
		}
		if got[1].Name != "GitHub" || got[1].Url != "https://github.com" || got[1].SameLine != true {
			t.Errorf("second button mismatch: %+v", got[1])
		}
	})
}

// --- RevertButtons ---

func TestRevertButtons(t *testing.T) {
	t.Parallel()
	t.Run("empty buttons returns empty string", func(t *testing.T) {
		t.Parallel()
		got := RevertButtons([]db.Button{})
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
	t.Run("same-line button uses :same suffix", func(t *testing.T) {
		t.Parallel()
		buttons := []db.Button{{Name: "Click", Url: "https://example.com", SameLine: true}}
		got := RevertButtons(buttons)
		want := "\n[Click](buttonurl://https://example.com:same)"
		if got != want {
			t.Errorf("RevertButtons() = %q, want %q", got, want)
		}
	})
	t.Run("non-same-line button has no :same suffix", func(t *testing.T) {
		t.Parallel()
		buttons := []db.Button{{Name: "Visit", Url: "https://example.com", SameLine: false}}
		got := RevertButtons(buttons)
		want := "\n[Visit](buttonurl://https://example.com)"
		if got != want {
			t.Errorf("RevertButtons() = %q, want %q", got, want)
		}
	})
}

// --- ChunkKeyboardSlices ---

func TestChunkKeyboardSlices(t *testing.T) {
	t.Parallel()
	makeButtons := func(n int) []gotgbot.InlineKeyboardButton {
		btns := make([]gotgbot.InlineKeyboardButton, n)
		for i := range btns {
			btns[i] = gotgbot.InlineKeyboardButton{Text: "btn"}
		}
		return btns
	}

	tests := []struct {
		name      string
		input     []gotgbot.InlineKeyboardButton
		chunkSize int
		wantRows  int
	}{
		{
			name:      "empty slice returns nil",
			input:     []gotgbot.InlineKeyboardButton{},
			chunkSize: 2,
			wantRows:  0,
		},
		{
			name:      "exact chunk size produces one row",
			input:     makeButtons(2),
			chunkSize: 2,
			wantRows:  1,
		},
		{
			name:      "remainder creates extra row",
			input:     makeButtons(3),
			chunkSize: 2,
			wantRows:  2,
		},
		{
			name:      "single chunk fits all",
			input:     makeButtons(5),
			chunkSize: 10,
			wantRows:  1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			chunks := ChunkKeyboardSlices(tc.input, tc.chunkSize)
			if len(chunks) != tc.wantRows {
				t.Errorf("ChunkKeyboardSlices() = %d rows, want %d", len(chunks), tc.wantRows)
			}
		})
	}
}

// --- notesParser ---

func TestNotesParser(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		input            string
		wantPvt          bool
		wantGrp          bool
		wantAdmin        bool
		wantPreview      bool
		wantProtect      bool
		wantNoNotif      bool
		containsTagAfter bool // whether the tag should remain in sentBack
	}{
		{
			name:  "no tags — all false",
			input: "just a normal note",
		},
		{
			name:      "private tag",
			input:     "hello {private}",
			wantPvt:   true,
			wantGrp:   false,
			wantAdmin: false,
		},
		{
			name:    "noprivate tag sets grpOnly",
			input:   "hello {noprivate}",
			wantGrp: true,
		},
		{
			name:      "admin tag",
			input:     "admin only {admin}",
			wantAdmin: true,
		},
		{
			name:        "preview tag",
			input:       "{preview} note text",
			wantPreview: true,
		},
		{
			name:        "protect tag",
			input:       "protect {protect}",
			wantProtect: true,
		},
		{
			name:        "nonotif tag",
			input:       "silent {nonotif}",
			wantNoNotif: true,
		},
		{
			name:        "multiple tags in same note",
			input:       "{private}{admin}{preview}",
			wantPvt:     true,
			wantAdmin:   true,
			wantPreview: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pvt, grp, admin, preview, protect, noNotif, sentBack := notesParser(tc.input)
			if pvt != tc.wantPvt {
				t.Errorf("pvtOnly = %v, want %v", pvt, tc.wantPvt)
			}
			if grp != tc.wantGrp {
				t.Errorf("grpOnly = %v, want %v", grp, tc.wantGrp)
			}
			if admin != tc.wantAdmin {
				t.Errorf("adminOnly = %v, want %v", admin, tc.wantAdmin)
			}
			if preview != tc.wantPreview {
				t.Errorf("webPrev = %v, want %v", preview, tc.wantPreview)
			}
			if protect != tc.wantProtect {
				t.Errorf("protect = %v, want %v", protect, tc.wantProtect)
			}
			if noNotif != tc.wantNoNotif {
				t.Errorf("noNotif = %v, want %v", noNotif, tc.wantNoNotif)
			}
			// tags should be removed from sentBack
			for _, tag := range []string{"{private}", "{noprivate}", "{admin}", "{preview}", "{protect}", "{nonotif}"} {
				if strings.Contains(sentBack, tag) {
					t.Errorf("sentBack still contains tag %q: %q", tag, sentBack)
				}
			}
		})
	}
}

// --- ReverseHTML2MD ---

func TestReverseHTML2MD(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bold tag converted to markdown",
			input: "<b>hello</b>",
			want:  "*hello*",
		},
		{
			name:  "plain text unchanged",
			input: "plain text no tags",
			want:  "plain text no tags",
		},
		{
			name:  "code tag converted",
			input: "<code>fmt.Println()</code>",
			want:  "`fmt.Println()`",
		},
		{
			// ReverseHTML2MD splits on spaces, so <a href="url">name</a> tokens get split
			// and the link regex never matches. The input is returned unchanged.
			name:  "link with space in HTML not converted (function limitation)",
			input: `<a href="https://example.com">Click</a>`,
			want:  `<a href="https://example.com">Click</a>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ReverseHTML2MD(tc.input)
			if got != tc.want {
				t.Errorf("ReverseHTML2MD(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// --- ExtractJoinLeftStatusChange ---

func makeUpdated(chatType, oldStatus, newStatus string, oldIsMember, newIsMember bool) *gotgbot.ChatMemberUpdated {
	var oldMember, newMember gotgbot.ChatMember

	switch oldStatus {
	case "member":
		oldMember = gotgbot.ChatMemberMember{}
	case "left":
		oldMember = gotgbot.ChatMemberLeft{}
	case "administrator":
		oldMember = gotgbot.ChatMemberAdministrator{}
	case "creator":
		oldMember = gotgbot.ChatMemberOwner{}
	case "kicked":
		oldMember = gotgbot.ChatMemberBanned{}
	case "restricted":
		oldMember = gotgbot.ChatMemberRestricted{IsMember: oldIsMember}
	default:
		oldMember = gotgbot.ChatMemberLeft{}
	}

	switch newStatus {
	case "member":
		newMember = gotgbot.ChatMemberMember{}
	case "left":
		newMember = gotgbot.ChatMemberLeft{}
	case "administrator":
		newMember = gotgbot.ChatMemberAdministrator{}
	case "creator":
		newMember = gotgbot.ChatMemberOwner{}
	case "kicked":
		newMember = gotgbot.ChatMemberBanned{}
	case "restricted":
		newMember = gotgbot.ChatMemberRestricted{IsMember: newIsMember}
	default:
		newMember = gotgbot.ChatMemberLeft{}
	}

	return &gotgbot.ChatMemberUpdated{
		Chat:          gotgbot.Chat{Type: chatType},
		OldChatMember: oldMember,
		NewChatMember: newMember,
	}
}

func TestExtractJoinLeftStatusChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		u       *gotgbot.ChatMemberUpdated
		wantWas bool
		wantIs  bool
	}{
		{
			name:    "user joins (left → member)",
			u:       makeUpdated("supergroup", "left", "member", false, false),
			wantWas: false,
			wantIs:  true,
		},
		{
			name:    "user leaves (member → left)",
			u:       makeUpdated("supergroup", "member", "left", false, false),
			wantWas: true,
			wantIs:  false,
		},
		{
			name:    "channel type always returns false false",
			u:       makeUpdated("channel", "left", "member", false, false),
			wantWas: false,
			wantIs:  false,
		},
		{
			name:    "same status returns false false",
			u:       makeUpdated("supergroup", "member", "member", false, false),
			wantWas: false,
			wantIs:  false,
		},
		{
			name:    "admin to member (was admin, is member)",
			u:       makeUpdated("supergroup", "administrator", "member", false, false),
			wantWas: true,
			wantIs:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			was, is := ExtractJoinLeftStatusChange(tc.u)
			if was != tc.wantWas || is != tc.wantIs {
				t.Errorf("ExtractJoinLeftStatusChange() = (%v, %v), want (%v, %v)", was, is, tc.wantWas, tc.wantIs)
			}
		})
	}
}

// --- ExtractAdminUpdateStatusChange ---

func TestExtractAdminUpdateStatusChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		u    *gotgbot.ChatMemberUpdated
		want bool
	}{
		{
			name: "member promoted to admin returns true",
			u:    makeUpdated("supergroup", "member", "administrator", false, false),
			want: true,
		},
		{
			name: "admin demoted to member returns true",
			u:    makeUpdated("supergroup", "administrator", "member", false, false),
			want: true,
		},
		{
			name: "channel type returns false",
			u:    makeUpdated("channel", "member", "administrator", false, false),
			want: false,
		},
		{
			name: "no status change returns false",
			u:    makeUpdated("supergroup", "administrator", "administrator", false, false),
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractAdminUpdateStatusChange(tc.u)
			if got != tc.want {
				t.Errorf("ExtractAdminUpdateStatusChange() = %v, want %v", got, tc.want)
			}
		})
	}
}
