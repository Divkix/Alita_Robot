//go:build testtools

package modules

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/jev"
)

type nsfwRoundTripFunc func(*http.Request) (*http.Response, error)

func (f nsfwRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func setupTestNSFWClient(t *testing.T, roundTrip nsfwRoundTripFunc) {
	t.Helper()
	oldFactory := newJevClient
	newJevClient = func(apiKey string) jev.Client {
		return jev.NewClient(apiKey, jev.WithHTTPClient(&http.Client{Transport: roundTrip}))
	}
	t.Cleanup(func() {
		newJevClient = oldFactory
	})
}

func testNSFWTranslator(t *testing.T) *i18n.Translator {
	t.Helper()
	yaml := `
nsfw_no_api_key: "TypeSafe API key is not configured. Please set <code>TYPESAFE_API_KEY</code> in the environment."
nsfw_no_target: "Please reply to a message or provide text to check for NSFW or curse words."
nsfw_no_text: "The targeted message does not contain any text to analyze."
nsfw_api_error: "Could not analyze that message right now. Please try again later."
nsfw_rate_limited: "Scan limit reached. Try again in %s."
nsfw_result: |
  <b>Content Analysis Result:</b>

  🔞 <b>NSFW Content:</b> %s (<code>%.1f%%</code>)
  🤬 <b>Curse / Profanity:</b> %s (<code>%.1f%%</code>)

  <b>Verdict:</b> %s
nsfw_detected: "Detected ⚠️"
nsfw_clean: "Clean ✅"
nsfw_verdict_unsafe: "Unsafe ❌"
nsfw_verdict_flagged: "Flagged ⚠️"
nsfw_verdict_safe: "Safe ✅"
`
	tr, err := i18n.NewTestTranslator(yaml)
	if err != nil {
		t.Fatalf("NewTestTranslator failed: %v", err)
	}
	return tr
}

func newTestCommandContext(t *testing.T, bot *gotgbot.Bot, ctx *ext.Context) *helpers.CommandContext {
	t.Helper()
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	cmdCtx.Tr = testNSFWTranslator(t)
	oldChatIDs := config.AppConfig.OwnerChatIDs
	config.AppConfig.OwnerChatIDs = []int64{cmdCtx.Chat.Id}
	t.Cleanup(func() { config.AppConfig.OwnerChatIDs = oldChatIDs })
	return cmdCtx
}

func TestLoadNSFW(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadNSFW(dispatcher)

	if !DefaultHelpRegistry().AbleMap[nsfwModule.moduleName] {
		t.Fatalf("AbleMap[%q] = false, want true", nsfwModule.moduleName)
	}
}

func TestCheckNSFWOutsideOwnerChats(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup"}
	user := gotgbot.User{Id: 456789, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/nsfw hello")
	cmdCtx := newTestCommandContext(t, bot, ctx)
	config.AppConfig.OwnerChatIDs = []int64{-1001490301388}
	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("outside chat sent %d replies, want none", len(calls))
	}
}

func TestCheckNSFWMissingAPIKey(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "NSFW Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Admin"}

	oldKey := config.AppConfig.TypeSafeAPIKey
	config.AppConfig.TypeSafeAPIKey = ""
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = oldKey
	})

	ctx := newModuleMessageContext(bot, chat, user, "/nsfw")
	cmdCtx := newTestCommandContext(t, bot, ctx)

	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "TYPESAFE_API_KEY") {
		t.Fatalf("expected missing API key error message, got: %s", text)
	}
}

func TestCheckNSFWNoTarget(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "NSFW Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Admin"}

	oldKey := config.AppConfig.TypeSafeAPIKey
	config.AppConfig.TypeSafeAPIKey = "dummy-key"
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = oldKey
	})

	ctx := newModuleMessageContext(bot, chat, user, "/nsfw")
	cmdCtx := newTestCommandContext(t, bot, ctx)

	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "reply to a message or provide text") {
		t.Fatalf("expected no target error message, got: %s", text)
	}
}

func TestCheckNSFWReplyWithNoText(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "NSFW Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Admin"}

	oldKey := config.AppConfig.TypeSafeAPIKey
	config.AppConfig.TypeSafeAPIKey = "dummy-key"
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = oldKey
	})

	ctx := newModuleMessageContext(bot, chat, user, "/nsfw")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 100,
		Chat:      chat,
	}

	cmdCtx := newTestCommandContext(t, bot, ctx)

	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "does not contain any text") {
		t.Fatalf("expected no text error message, got: %s", text)
	}
}

func TestCheckNSFWReplySuccess(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "NSFW Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Admin"}

	oldKey := config.AppConfig.TypeSafeAPIKey
	config.AppConfig.TypeSafeAPIKey = "valid-api-key"
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = oldKey
	})

	setupTestNSFWClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer valid-api-key" {
			t.Errorf("Authorization header = %q, want 'Bearer valid-api-key'", req.Header.Get("Authorization"))
		}

		respJSON := `{
			"model": "jev-latest",
			"answers": {
				"is_nsfw": {
					"type": "noul",
					"noul": 0.885
				},
				"is_curse": {
					"type": "noul",
					"noul": 0.120
				}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := newModuleMessageContext(bot, chat, user, "/nsfw")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 100,
		Chat:      chat,
		Text:      "explicit adult message",
	}

	cmdCtx := newTestCommandContext(t, bot, ctx)

	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "Content Analysis Result:") {
		t.Fatalf("expected result header in response, got: %s", text)
	}
	if !strings.Contains(text, "Detected ⚠️ (<code>88.5%</code>)") {
		t.Fatalf("expected NSFW detected in response, got: %s", text)
	}
	if !strings.Contains(text, "Clean ✅ (<code>12.0%</code>)") {
		t.Fatalf("expected curse clean in response, got: %s", text)
	}
	if !strings.Contains(text, "Unsafe ❌") {
		t.Fatalf("expected Unsafe verdict in response, got: %s", text)
	}

	// Verify reply parameters pointed to the replied message
	replyParams, ok := calls[0].Params["reply_parameters"].(*gotgbot.ReplyParameters)
	if !ok || replyParams == nil || replyParams.MessageId != 100 {
		t.Fatalf("expected reply_parameters to target message 100, got: %v", calls[0].Params["reply_parameters"])
	}
}

func TestCheckNSFWArgumentsTextSuccess(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "NSFW Chat"}
	user := gotgbot.User{Id: 456789, FirstName: "Member"}

	oldKey := config.AppConfig.TypeSafeAPIKey
	config.AppConfig.TypeSafeAPIKey = "valid-api-key"
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = oldKey
	})

	setupTestNSFWClient(t, func(req *http.Request) (*http.Response, error) {
		respJSON := `{
			"model": "jev-latest",
			"answers": {
				"is_nsfw": {
					"type": "noul",
					"noul": 0.05
				},
				"is_curse": {
					"type": "noul",
					"noul": 0.02
				}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := newModuleMessageContext(bot, chat, user, "/nsfw hello world this is clean")
	cmdCtx := newTestCommandContext(t, bot, ctx)
	config.AppConfig.OwnerChatIDs = []int64{-1001490301388, chat.Id}

	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "Safe ✅") {
		t.Fatalf("expected Safe verdict in response, got: %s", text)
	}
	if !strings.Contains(text, "Clean ✅ (<code>5.0%</code>)") {
		t.Fatalf("expected NSFW clean in response, got: %s", text)
	}
}

func TestCheckNSFWCaptionSuccess(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "NSFW Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Admin"}

	oldKey := config.AppConfig.TypeSafeAPIKey
	config.AppConfig.TypeSafeAPIKey = "valid-api-key"
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = oldKey
	})

	setupTestNSFWClient(t, func(req *http.Request) (*http.Response, error) {
		respJSON := `{
			"model": "jev-latest",
			"answers": {
				"is_nsfw": {
					"type": "noul",
					"noul": 0.45
				},
				"is_curse": {
					"type": "noul",
					"noul": 0.60
				}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := newModuleMessageContext(bot, chat, user, "/nsfw")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 101,
		Chat:      chat,
		Caption:   "photo with curse words",
	}

	cmdCtx := newTestCommandContext(t, bot, ctx)

	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "Detected ⚠️ (<code>60.0%</code>)") {
		t.Fatalf("expected curse detected in response, got: %s", text)
	}
	if !strings.Contains(text, "Flagged ⚠️") {
		t.Fatalf("expected Flagged verdict in response, got: %s", text)
	}
}

func TestCheckNSFWAPIError(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "NSFW Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Admin"}

	oldKey := config.AppConfig.TypeSafeAPIKey
	config.AppConfig.TypeSafeAPIKey = "valid-api-key"
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = oldKey
	})

	setupTestNSFWClient(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error": "Internal server error"}`)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := newModuleMessageContext(bot, chat, user, "/nsfw bad content")
	cmdCtx := newTestCommandContext(t, bot, ctx)

	if err := checkNSFW(cmdCtx); err != ext.EndGroups {
		t.Fatalf("checkNSFW() = %v, want EndGroups", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text, _ := calls[0].Params["text"].(string)
	if !strings.Contains(text, "Could not analyze that message right now") {
		t.Fatalf("expected API error in response, got: %s", text)
	}
}
