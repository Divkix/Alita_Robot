package modules

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/channels"
	"github.com/divkix/Alita_Robot/alita/db/devs"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withMiscHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	previous := httpClient
	httpClient = client
	t.Cleanup(func() {
		httpClient = previous
	})
}

func TestPingSendsAndEditsLatencyMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/ping")

	if err := miscModule.ping(bot, ctx); err != ext.EndGroups {
		t.Fatalf("ping() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("getMe"); len(calls) != 1 {
		t.Fatalf("getMe calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(calls))
	}
}

func TestStatRepliesWithGroupMessageCount(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/stat")
	ctx.EffectiveMessage.MessageId = 123

	if err := miscModule.stat(bot, ctx); err != ext.EndGroups {
		t.Fatalf("stat() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestRemoveBotKeyboardSendsKeyboardRemoval(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/removebotkeyboard")

	if err := miscModule.removeBotKeyboard(bot, ctx); err != ext.EndGroups {
		t.Fatalf("removeBotKeyboard() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if _, ok := calls[0].Params["reply_markup"].(*gotgbot.ReplyKeyboardRemove); !ok {
		t.Fatalf("reply_markup = %#v, want ReplyKeyboardRemove", calls[0].Params["reply_markup"])
	}

	waitForModuleCondition(t, func() bool {
		return len(client.callsFor("deleteMessage")) == 1
	})
}

func TestEchoMessageRequiresReplyAndContent(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	noReplyCtx := newModuleMessageContext(bot, chat, user, "/tell hi")
	if err := miscModule.echomsg(bot, noReplyCtx); err != ext.EndGroups {
		t.Fatalf("echomsg no-reply error = %v, want EndGroups", err)
	}

	noContentCtx := newModuleMessageContext(bot, chat, user, "/tell")
	noContentCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 55,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 88, FirstName: "Target"},
		Text:      "target",
	}
	if err := miscModule.echomsg(bot, noContentCtx); err != ext.EndGroups {
		t.Fatalf("echomsg no-content error = %v, want EndGroups", err)
	}

	echoCtx := newModuleMessageContext(bot, chat, user, "/tell hello there")
	echoCtx.EffectiveMessage.ReplyToMessage = noContentCtx.EffectiveMessage.ReplyToMessage
	if err := miscModule.echomsg(bot, echoCtx); err != ext.EndGroups {
		t.Fatalf("echomsg echo error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want 3", len(calls))
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1 for successful echo", len(calls))
	}
}

func TestTranslateMissingInputReplies(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/tr")

	if err := miscModule.translate(bot, ctx); err != ext.EndGroups {
		t.Fatalf("translate() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestParseTranslateResponseHandlesShapes(t *testing.T) {
	detected, translated, err := parseTranslateResponse([]byte(`[["hola","en"]]`))
	if err != nil {
		t.Fatalf("parseTranslateResponse valid error = %v", err)
	}
	if detected != "en" || translated != "hola" {
		t.Fatalf("parseTranslateResponse = (%q, %q), want (en, hola)", detected, translated)
	}

	for _, body := range [][]byte{
		[]byte(`not-json`),
		[]byte(`[]`),
		[]byte(`[[]]`),
		[]byte(`[[null,"en"]]`),
		[]byte(`[["hola",null]]`),
	} {
		if _, _, err := parseTranslateResponse(body); err == nil {
			t.Fatalf("parseTranslateResponse(%s) error = nil, want error", body)
		}
	}
}

func TestTranslateHandlesReplyAndHTTPBranches(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		replyText  string
		replyCap   string
		body       string
		wantCalls  int
		wantTarget string
	}{
		{name: "missing direct text", text: "/tr es", wantCalls: 1},
		{name: "reply without text", text: "/tr es", wantCalls: 1},
		{name: "reply text defaults target", text: "/tr", replyText: "hello", body: `[["hola","en"]]`, wantCalls: 1, wantTarget: "en"},
		{name: "reply caption target", text: "/tr hi", replyCap: "hello", body: `[["namaste","en"]]`, wantCalls: 1, wantTarget: "hi"},
		{name: "parse error", text: "/tr es hello", body: `[[]]`, wantCalls: 1, wantTarget: "es"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
			user := gotgbot.User{Id: 42, FirstName: "Member"}
			var seenURL string
			withMiscHTTPClient(t, &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					seenURL = req.URL.String()
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(tc.body)),
						Header:     make(http.Header),
					}, nil
				}),
			})

			ctx := newModuleMessageContext(bot, chat, user, tc.text)
			if tc.replyText != "" || tc.replyCap != "" || tc.name == "reply without text" {
				ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
					MessageId: 55,
					Date:      1,
					Chat:      chat,
					From:      &gotgbot.User{Id: 88, FirstName: "Target"},
					Text:      tc.replyText,
					Caption:   tc.replyCap,
				}
			}

			if err := miscModule.translate(bot, ctx); err != ext.EndGroups {
				t.Fatalf("translate() error = %v, want EndGroups", err)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != tc.wantCalls {
				t.Fatalf("sendMessage calls = %d, want %d", len(calls), tc.wantCalls)
			}
			if tc.wantTarget != "" && !strings.Contains(seenURL, "tl="+tc.wantTarget) {
				t.Fatalf("translate URL = %q, want target %q", seenURL, tc.wantTarget)
			}
		})
	}
}

func TestGetIdRepliesForCurrentGroupUserAndReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}

	groupCtx := newModuleMessageContext(bot, chat, user, "/id")
	if err := miscModule.getId(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("getId group error = %v, want EndGroups", err)
	}

	replyCtx := newModuleMessageContext(bot, chat, user, "/id")
	replyCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 55,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 88, FirstName: "Target"},
		Text:      "target",
		Sticker:   &gotgbot.Sticker{FileId: "sticker-file-id"},
	}
	if err := miscModule.getId(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("getId reply error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
}

func TestGetIdHandlesPrivateAnonymousAndMediaReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42, FirstName: "Member"}

	privateChat := gotgbot.Chat{Id: 42, Type: "private", FirstName: "Member"}
	privateCtx := newModuleMessageContext(bot, privateChat, user, "/id")
	if err := miscModule.getId(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("getId private error = %v, want EndGroups", err)
	}

	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	anonymousCtx := newModuleMessageContext(bot, groupChat, user, "/id")
	anonymousCtx.EffectiveMessage.From = nil
	if err := miscModule.getId(bot, anonymousCtx); err != ext.EndGroups {
		t.Fatalf("getId anonymous error = %v, want EndGroups", err)
	}

	replyCtx := newModuleMessageContext(bot, groupChat, user, "/id")
	replyCtx.EffectiveMessage.IsTopicMessage = true
	replyCtx.EffectiveMessage.MessageThreadId = 77
	replyCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 55,
		Date:      1,
		Chat:      groupChat,
		From:      &gotgbot.User{Id: 88, FirstName: "Target"},
		Text:      "target",
		Animation: &gotgbot.Animation{FileId: "gif-file-id"},
	}
	if err := miscModule.getId(bot, replyCtx); err != ext.EndGroups {
		t.Fatalf("getId media reply error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want 3", len(calls))
	}
}

func TestInfoRepliesForUnknownNumericUser(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/info 123456789")

	if err := miscModule.info(bot, ctx); err != ext.EndGroups {
		t.Fatalf("info() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestInfoRepliesForKnownUserWithRoles(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	u := gotgbot.User{Id: 42, FirstName: "Member"}
	requireUserID := time.Now().UnixNano()

	if err := user.EnsureUserInDb(requireUserID, "knownuser", "Known User"); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}
	if err := devs.AddDev(requireUserID); err != nil {
		t.Fatalf("AddDev() error = %v", err)
	}
	previousOwnerID := config.AppConfig.OwnerId
	config.AppConfig.OwnerId = requireUserID
	t.Cleanup(func() {
		config.AppConfig.OwnerId = previousOwnerID
		_ = devs.RemDev(requireUserID)
	})

	ctx := newModuleMessageContext(bot, chat, u, "/info "+strconv.FormatInt(requireUserID, 10))

	if err := miscModule.info(bot, ctx); err != ext.EndGroups {
		t.Fatalf("info() error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text := calls[0].Params["text"].(string)
	for _, want := range []string{"knownuser", "Known User", strconv.FormatInt(requireUserID, 10)} {
		if !strings.Contains(text, want) {
			t.Fatalf("info text %q missing %q", text, want)
		}
	}
}

func TestInfoRepliesForKnownChannel(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	channelID := int64(-1001234567890)
	if err := channels.UpdateChannel(channelID, "News Channel", "newsroom"); err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, user, "/info "+strconv.FormatInt(channelID, 10))

	if err := miscModule.info(bot, ctx); err != ext.EndGroups {
		t.Fatalf("info() error = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text := calls[0].Params["text"].(string)
	for _, want := range []string{"News Channel", "newsroom", strconv.FormatInt(channelID, 10)} {
		if !strings.Contains(text, want) {
			t.Fatalf("info text %q missing %q", text, want)
		}
	}
}

func TestLoadMiscRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadMisc(dispatcher)

	if moduleName, enabled := DefaultHelpRegistry().AbleMap.Load(miscModule.moduleName); moduleName != miscModule.moduleName || !enabled {
		t.Fatalf("misc help registration = (%q, %v), want enabled", moduleName, enabled)
	}
}
