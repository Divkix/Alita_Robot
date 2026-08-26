package formatting

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/rules"
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

	got, buttons := FormattingReplacer(nil, chat, user, input, nil)
	want := `&lt;Ada&gt;|Lovelace &amp; Byron|&lt;Ada&gt; Lovelace &amp; Byron|@ada&lt;&amp;&gt;|@ada&lt;&amp;&gt;|&lt;Group &amp; Co&gt;|42`
	if got != want {
		t.Fatalf("FormattingReplacer() = %q, want %q", got, want)
	}
	if len(buttons) != 0 {
		t.Fatalf("FormattingReplacer() buttons = %#v, want none", buttons)
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

	got, buttons := FormattingReplacer(
		bot,
		chat,
		nil,
		"{first}|{fullname}|{username}|{mention}|{count}|{id}",
		nil,
	)
	want := "PersonWithNoName|PersonWithNoName|PersonWithNoName|PersonWithNoName|42|0"
	if got != want {
		t.Fatalf("FormattingReplacer(nil user) = %q, want %q", got, want)
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
	if err := db.DB.AutoMigrate(&db.Chat{}, &models.RulesSettings{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	bot := &gotgbot.Bot{
		Token:     "123:test",
		BotClient: formattingBotClient{},
		User:      gotgbot.User{Id: 123, IsBot: true, Username: "FormatBot"},
	}
	bot.Username = "FormatBot"
	chat := &gotgbot.Chat{Id: -100777, Type: "supergroup", Title: "Rules Chat"}

	noRulesText, noRulesButtons := FormattingReplacer(
		bot,
		chat,
		&gotgbot.User{Id: 5, FirstName: "Ada"},
		"before {rules} after",
		nil,
	)
	if noRulesText != "before  after" {
		t.Fatalf("result without rules = %q, want rules token removed", noRulesText)
	}
	if len(noRulesButtons) != 0 {
		t.Fatalf("buttons without rules = %#v, want none", noRulesButtons)
	}

	rules.SetChatRules(chat.Id, "Keep it tidy.")
	rules.SetChatRulesButton(chat.Id, "Read Rules")

	got, buttons := FormattingReplacer(
		bot,
		chat,
		&gotgbot.User{Id: 5, FirstName: "{rules}"},
		"{first}",
		nil,
	)
	if got != "{rules}" || len(buttons) != 0 {
		t.Fatalf("user-provided directive text produced %q, %#v; want literal text and no button", got, buttons)
	}

	got, buttons = FormattingReplacer(
		bot,
		chat,
		&gotgbot.User{Id: 5, FirstName: "Ada"},
		"before {rules:up} after",
		[]db.Button{{Name: "Existing", Url: "https://example.com"}},
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

	got, buttons = FormattingReplacer(
		bot,
		chat,
		&gotgbot.User{Id: 5, FirstName: "Ada"},
		"show {rules:same}",
		nil,
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

func TestGetFullName(t *testing.T) {
	t.Parallel()

	t.Run("with last name", func(t *testing.T) {
		t.Parallel()
		name := GetFullName("John", "Doe")
		if name != "John Doe" {
			t.Fatalf("expected 'John Doe', got %q", name)
		}
	})

	t.Run("without last name", func(t *testing.T) {
		t.Parallel()
		name := GetFullName("Alice", "")
		if name != "Alice" {
			t.Fatalf("expected 'Alice', got %q", name)
		}
	})
}

func clearMemberCountCache() {
	memberCountCache.Range(func(key, _ any) bool {
		memberCountCache.Delete(key)
		return true
	})
}

func TestCachedMemberCountEvictsExpiredEntries(t *testing.T) {
	clearMemberCountCache()
	oldTTL := memberCountCacheTTL
	memberCountCacheTTL = 15 * time.Millisecond
	t.Cleanup(func() {
		memberCountCacheTTL = oldTTL
		clearMemberCountCache()
	})

	bot := &gotgbot.Bot{
		Token:     "123:test",
		BotClient: formattingBotClient{},
		User:      gotgbot.User{Id: 123, IsBot: true, Username: "FormatBot"},
	}
	chat := &gotgbot.Chat{Id: -100999001, Type: "supergroup", Title: "Count Chat"}

	if got := cachedMemberCount(bot, chat); got != "42" {
		t.Fatalf("cachedMemberCount() = %q, want 42", got)
	}
	if _, ok := memberCountCache.Load(chat.Id); !ok {
		t.Fatal("fresh member count was not stored")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := memberCountCache.Load(chat.Id); !ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expired member count cache entry was not evicted")
}

func TestCachedMemberCountReplacesStaleEntryOnRead(t *testing.T) {
	clearMemberCountCache()
	t.Cleanup(clearMemberCountCache)

	chatID := int64(-100999002)
	memberCountCache.Store(chatID, memberCountEntry{
		count: 1,
		at:    time.Now().Add(-time.Hour),
	})

	bot := &gotgbot.Bot{
		Token:     "123:test",
		BotClient: formattingBotClient{},
		User:      gotgbot.User{Id: 123, IsBot: true, Username: "FormatBot"},
	}
	chat := &gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Count Chat"}

	if got := cachedMemberCount(bot, chat); got != "42" {
		t.Fatalf("cachedMemberCount() = %q, want refreshed 42", got)
	}
	v, ok := memberCountCache.Load(chatID)
	if !ok {
		t.Fatal("refreshed member count was not stored")
	}
	entry, ok := v.(memberCountEntry)
	if !ok || entry.count != 42 {
		t.Fatalf("stored entry = %#v, want count 42", v)
	}
}
