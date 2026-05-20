package formatting

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
)

type formattingBotClient struct{}

func (formattingBotClient) RequestWithContext(_ context.Context, _ string, method string, _ map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatMemberCount" {
		return json.RawMessage(`42`), nil
	}
	return json.RawMessage(`true`), nil
}

func (formattingBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL
}

func (formattingBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL + "/file/bot" + token + "/" + path
}

func TestFormattingReplacerWithoutRulesDoesNotRequireDatabase(t *testing.T) {
	t.Parallel()

	originalDB := db.DB
	db.DB = nil
	t.Cleanup(func() { db.DB = originalDB })

	chat := &gotgbot.Chat{Id: -1001234567890, Title: `<Group & Co>`}
	user := &gotgbot.User{
		Id:        42,
		FirstName: `<Ada>`,
		LastName:  `Lovelace & Byron`,
		Username:  `ada<&>`,
	}
	input := `{first}|{last}|{fullname}|{username}|{mention}|{chatname}|{id}`

	got, buttons := FormattingReplacerWithLanguage(nil, chat, user, input, nil, "en")
	want := `&lt;Ada&gt;|Lovelace &amp; Byron|&lt;Ada&gt; Lovelace &amp; Byron|@ada&lt;&amp;&gt;|@ada&lt;&amp;&gt;|&lt;Group &amp; Co&gt;|42`
	if got != want {
		t.Fatalf("FormattingReplacerWithLanguage() = %q, want %q", got, want)
	}
	if len(buttons) != 0 {
		t.Fatalf("FormattingReplacerWithLanguage() buttons = %#v, want none", buttons)
	}
}

func TestFormattingReplacerHandlesNilUserAndMemberCount(t *testing.T) {
	originalDB := db.DB
	db.DB = nil
	t.Cleanup(func() { db.DB = originalDB })

	bot := &gotgbot.Bot{
		Token:     "123:test",
		BotClient: formattingBotClient{},
		User:      gotgbot.User{Id: 123, IsBot: true, Username: "FormatBot"},
	}
	chat := &gotgbot.Chat{Id: -100123, Type: "supergroup", Title: "Format Chat"}

	got, buttons := FormattingReplacerWithLanguage(
		bot,
		chat,
		nil,
		"{first}|{fullname}|{username}|{mention}|{count}|{id}",
		nil,
		"en",
	)
	want := "PersonWithNoName|PersonWithNoName|PersonWithNoName|PersonWithNoName|42|0"
	if got != want {
		t.Fatalf("FormattingReplacerWithLanguage(nil user) = %q, want %q", got, want)
	}
	if len(buttons) != 0 {
		t.Fatalf("buttons = %#v, want none", buttons)
	}
}

func TestFormattingReplacerAddsRulesButtons(t *testing.T) {
	originalDB := db.DB
	sqliteDB, err := gorm.Open(sqlite.Open("file:formatting_rules?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.DB = sqliteDB
	t.Cleanup(func() {
		if sqlDB, sqlErr := db.DB.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
		db.DB = originalDB
	})
	if err := db.DB.AutoMigrate(&db.Chat{}, &db.RulesSettings{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	bot := &gotgbot.Bot{
		Token:     "123:test",
		BotClient: formattingBotClient{},
		User:      gotgbot.User{Id: 123, IsBot: true, Username: "FormatBot"},
	}
	bot.Username = "FormatBot"
	chat := &gotgbot.Chat{Id: -100777, Type: "supergroup", Title: "Rules Chat"}
	db.SetChatRules(chat.Id, "Keep it tidy.")
	db.SetChatRulesButton(chat.Id, "Read Rules")

	got, buttons := FormattingReplacerWithLanguage(
		bot,
		chat,
		&gotgbot.User{Id: 5, FirstName: "Ada"},
		"before {rules:up} after",
		[]db.Button{{Name: "Existing", Url: "https://example.com"}},
		"en",
	)
	if got != "before  after" {
		t.Fatalf("result = %q, want rules placeholder removed", got)
	}
	if len(buttons) != 2 {
		t.Fatalf("buttons = %#v, want rules plus existing", buttons)
	}
	if buttons[0].Name != "Read Rules" || buttons[0].SameLine {
		t.Fatalf("rules button = %#v, want first non-sameline Read Rules button", buttons[0])
	}
	if buttons[0].Url != "https://t.me/FormatBot?start=rules_-100777" {
		t.Fatalf("rules URL = %q", buttons[0].Url)
	}

	got, buttons = FormattingReplacerWithLanguage(
		bot,
		chat,
		&gotgbot.User{Id: 5, FirstName: "Ada"},
		"show {rules:same}",
		nil,
		"en",
	)
	if got != "show " {
		t.Fatalf("same-line result = %q, want placeholder removed", got)
	}
	if len(buttons) != 1 || !buttons[0].SameLine {
		t.Fatalf("same-line buttons = %#v, want one same-line rules button", buttons)
	}
}

func TestSendMessageOptionBuilders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		buildOpts func() string
		wantMode  string
	}{
		{
			name: "html options",
			buildOpts: func() string {
				opts := Shtml()
				if opts.LinkPreviewOptions == nil || !opts.LinkPreviewOptions.IsDisabled {
					t.Fatal("Shtml must disable link previews")
				}
				if opts.ReplyParameters == nil || !opts.ReplyParameters.AllowSendingWithoutReply {
					t.Fatal("Shtml must allow sending without reply")
				}
				return opts.ParseMode
			},
			wantMode: HTML,
		},
		{
			name: "markdown options",
			buildOpts: func() string {
				opts := Smarkdown()
				if opts.LinkPreviewOptions == nil || !opts.LinkPreviewOptions.IsDisabled {
					t.Fatal("Smarkdown must disable link previews")
				}
				if opts.ReplyParameters == nil || !opts.ReplyParameters.AllowSendingWithoutReply {
					t.Fatal("Smarkdown must allow sending without reply")
				}
				return opts.ParseMode
			},
			wantMode: Markdown,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.buildOpts(); got != tc.wantMode {
				t.Fatalf("ParseMode = %q, want %q", got, tc.wantMode)
			}
		})
	}
}

func TestSplitMessage(t *testing.T) {
	t.Parallel()

	longSingleLine := strings.Repeat("x", MaxMessageLength+7)
	firstLine := strings.Repeat("a", MaxMessageLength-2)
	secondLine := "bb"

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "short message is returned unchanged",
			input: "hello",
			want:  []string{"hello"},
		},
		{
			name:  "splits long single line by rune limit",
			input: longSingleLine,
			want:  []string{strings.Repeat("x", MaxMessageLength), strings.Repeat("x", 7)},
		},
		{
			name:  "groups newline separated lines without exceeding limit",
			input: firstLine + "\n" + secondLine,
			want:  []string{firstLine + "\n", secondLine + "\n"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := SplitMessage(tc.input); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitMessage() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHTMLHelpersEscapeUntrustedText(t *testing.T) {
	t.Parallel()

	if got := HtmlEscape(`<tag attr="value">&`); got != `&lt;tag attr=&#34;value&#34;&gt;&amp;` {
		t.Fatalf("HtmlEscape = %q", got)
	}

	gotURL := MentionUrl(`https://example.com/?q=<x>&n="1"`, `A&B <user>`)
	wantURL := `<a href="https://example.com/?q=&lt;x&gt;&amp;n=&#34;1&#34;">A&amp;B &lt;user&gt;</a>`
	if gotURL != wantURL {
		t.Fatalf("MentionUrl = %q, want %q", gotURL, wantURL)
	}

	gotMention := MentionHtml(12345, `A&B`)
	wantMention := `<a href="tg://user?id=12345">A&amp;B</a>`
	if gotMention != wantMention {
		t.Fatalf("MentionHtml = %q, want %q", gotMention, wantMention)
	}
}

func TestReverseHTML2MD(t *testing.T) {
	t.Parallel()

	input := `<b>bold</b> <i>italic</i> <u>under</u> <s>strike</s> <code>code</code> <a href="https://example.com">link</a>`
	want := `*bold* _italic_ __under__ ~strike~ ` + "`code`" + ` [link](https://example.com)`
	if got := ReverseHTML2MD(input); got != want {
		t.Fatalf("ReverseHTML2MD = %q, want %q", got, want)
	}
}
