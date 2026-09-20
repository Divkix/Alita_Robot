//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/lang"
)

// withAISpamTestConfig points the module at a test key and target dump chat.
func withAISpamTestConfig(t *testing.T, apiKey string, globallyEnabled bool, messageDump int64) {
	t.Helper()

	previousKey, previousEnabled, previousDump := config.AppConfig.TypeSafeAPIKey, config.AppConfig.EnableAISpam, config.AppConfig.MessageDump
	config.AppConfig.TypeSafeAPIKey = apiKey
	config.AppConfig.EnableAISpam = globallyEnabled
	config.AppConfig.MessageDump = messageDump
	t.Cleanup(func() {
		config.AppConfig.TypeSafeAPIKey = previousKey
		config.AppConfig.EnableAISpam = previousEnabled
		config.AppConfig.MessageDump = previousDump
	})
}

// resetAISpamRuntime clears the module's process-wide state so each test sees
// only its own chats and breakers. The sender window lives in Redis under keys
// scoped to one chat and user, so it needs no reset; a test that wants a fresh
// process calls this and the window survives, which is the point of it.
func resetAISpamRuntime() {
	aispamBreakers.mu.Lock()
	aispamBreakers.m = make(map[int64]*aispamBreaker)
	aispamBreakers.mu.Unlock()

	aispamDescriptions.mu.Lock()
	aispamDescriptions.entries = make(map[int64]aispamChatDescriptionEntry)
	aispamDescriptions.mu.Unlock()

	aispamChecked.Store(0)
	aispamDeleted.Store(0)
	aispamKept.Store(0)
	aispamShed.Store(0)
	aispamFailed.Store(0)
}

// aispamMessageUpdate builds the update shape the watcher sees for a plain
// group text message. Tests that need another shape mutate the result.
func aispamMessageUpdate(chat gotgbot.Chat, from gotgbot.User, messageID int64, text string) *gotgbot.Update {
	return &gotgbot.Update{
		UpdateId: messageID,
		Message: &gotgbot.Message{
			MessageId: messageID,
			Date:      1,
			Chat:      chat,
			From:      &from,
			Text:      text,
		},
	}
}

func aispamProcessUpdate(t *testing.T, dispatcher *ext.Dispatcher, bot *gotgbot.Bot, chat gotgbot.Chat, from gotgbot.User, messageID int64, text string) {
	t.Helper()

	aispamProcessRawUpdate(t, dispatcher, bot, aispamMessageUpdate(chat, from, messageID, text))
}

func aispamProcessRawUpdate(t *testing.T, dispatcher *ext.Dispatcher, bot *gotgbot.Bot, update *gotgbot.Update) {
	t.Helper()

	if err := dispatcher.ProcessUpdate(bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%d) error = %v", update.UpdateId, err)
	}
}

// aispamAdminClient returns a client whose admin list contains user 42, which
// is what chat_status consults for admin checks.
func aispamAdminClient() *moduleBotClient {
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = []byte(
		`[` +
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true,"can_restrict_members":true},` +
			`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Admin"},"can_delete_messages":true,"can_restrict_members":true}` +
			`]`,
	)
	client.responses["getChatMember"] = []byte(`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Admin"},"can_delete_messages":true}`)
	return client
}

func aispamSendMessageCount(calls []moduleBotCall, chatID int64) int {
	count := 0
	for _, call := range calls {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) {
			count++
		}
	}
	return count
}

func TestAISpamCommandTogglesChatSetting(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Toggle Chat"}
	admin := gotgbot.User{Id: 42, FirstName: "Admin"}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	aispamProcessUpdate(t, dispatcher, bot, chat, admin, 601, "/aispam on")
	if !aispam.IsAISpamEnabled(chat.Id) {
		t.Fatal("/aispam on did not enable the filter")
	}
	if len(client.callsFor("sendMessage")) == 0 {
		t.Fatal("/aispam on sent no confirmation")
	}

	aispamProcessUpdate(t, dispatcher, bot, chat, admin, 602, "/aispam off")
	if aispam.IsAISpamEnabled(chat.Id) {
		t.Fatal("/aispam off did not disable the filter")
	}
}

func TestAISpamCommandRefusesEnableWithoutOperatorKey(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "", true, 0)

	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "No Key Chat"}
	admin := gotgbot.User{Id: 42, FirstName: "Admin"}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	aispamProcessUpdate(t, dispatcher, bot, chat, admin, 603, "/aispam on")
	if aispam.IsAISpamEnabled(chat.Id) {
		t.Fatal("filter was enabled while the operator had no API key")
	}
	if len(client.callsFor("sendMessage")) == 0 {
		t.Fatal("refusal was not reported to the admin")
	}
}

func TestAISpamCommandRefusesEnableWhenGloballyDisabled(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", false, 0)

	client := aispamAdminClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Kill Switch Chat"}
	admin := gotgbot.User{Id: 42, FirstName: "Admin"}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	aispamProcessUpdate(t, dispatcher, bot, chat, admin, 604, "/aispam on")
	if aispam.IsAISpamEnabled(chat.Id) {
		t.Fatal("filter was enabled while the global kill switch was off")
	}
}

func TestAISpamCommandRejectsNonAdmin(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Non Admin Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	aispamProcessUpdate(t, dispatcher, bot, chat, member, 605, "/aispam on")
	if aispam.IsAISpamEnabled(chat.Id) {
		t.Fatal("a non-admin was able to enable the filter")
	}
}

func TestCheckAISpamDeletesFlaggedMessageAndMirrorsIt(t *testing.T) {
	resetAISpamRuntime()
	const dumpChatID = -100999
	withAISpamTestConfig(t, "typesafe-test-key", true, dumpChatID)

	var requests atomic.Int64
	withAISpamJevServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer typesafe-test-key" {
			t.Errorf("Authorization header = %q, want bearer key", got)
		}
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.95, "scam"))
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Spam Chat"}
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	aispamProcessUpdate(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Spammer"}, 501, "BUY CHEAP COINS NOW t.me/scam")
	DrainAISpamChecks()

	if got := requests.Load(); got != 1 {
		t.Fatalf("provider requests = %d, want 1", got)
	}

	deletes := client.callsFor("deleteMessage")
	if len(deletes) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1", len(deletes))
	}
	if got := fmt.Sprint(deletes[0].Params["message_id"]); got != "501" {
		t.Fatalf("deleteMessage message_id = %s, want 501", got)
	}
	if mirrors := aispamSendMessageCount(client.callsFor("sendMessage"), dumpChatID); mirrors != 1 {
		t.Fatalf("audit mirror messages = %d, want 1", mirrors)
	}

	checked, deleted, kept, shed, failed := GetAISpamStats()
	if checked != 1 || deleted != 1 || kept != 0 || shed != 0 || failed != 0 {
		t.Fatalf("stats = checked:%d deleted:%d kept:%d shed:%d failed:%d, want 1/1/0/0/0", checked, deleted, kept, shed, failed)
	}
}

func TestCheckAISpamKeepsMessageBelowThreshold(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.4, "none"))
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Chatty Chat"}
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	aispamProcessUpdate(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, 502, "does anyone know a good pizza place")
	DrainAISpamChecks()

	if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0", len(deletes))
	}

	checked, deleted, kept, _, _ := GetAISpamStats()
	if checked != 1 || deleted != 0 || kept != 1 {
		t.Fatalf("stats = checked:%d deleted:%d kept:%d, want 1/0/1", checked, deleted, kept)
	}
}

// The bar is the model's own read of the message, not the chat's setting: an
// English chat still receives posts written in other languages, and those are
// the ones Jev was not measured on.
func TestCheckAISpamThresholdFollowsMessageLanguage(t *testing.T) {
	for _, tt := range []struct {
		name          string
		chatLanguage  string
		messageAnswer string
		probability   float64
		deleted       bool
	}{
		{name: "english message keeps 0.79", chatLanguage: "en", messageAnswer: "english", probability: 0.79, deleted: false},
		{name: "english message deletes at 0.80", chatLanguage: "en", messageAnswer: "english", probability: 0.80, deleted: true},
		{name: "english message in a spanish chat deletes at 0.80", chatLanguage: "es", messageAnswer: "english", probability: 0.80, deleted: true},
		{name: "spanish message in an english chat keeps 0.89", chatLanguage: "en", messageAnswer: "other", probability: 0.89, deleted: false},
		{name: "spanish message deletes at 0.90", chatLanguage: "es", messageAnswer: "other", probability: 0.90, deleted: true},
		{name: "unreadable language keeps 0.89", chatLanguage: "en", messageAnswer: "", probability: 0.89, deleted: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAISpamRuntime()
			withAISpamTestConfig(t, "typesafe-test-key", true, 0)

			withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, aispamDecisionBodyLanguage(tt.probability, "promotion", tt.messageAnswer))
			})

			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Language Chat"}
			if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
				t.Fatalf("SetAISpamEnabled() error = %v", err)
			}
			if err := lang.ChangeGroupLanguage(chat.Id, tt.chatLanguage); err != nil {
				t.Fatalf("ChangeGroupLanguage() error = %v", err)
			}

			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadAISpam(dispatcher)

			aispamProcessUpdate(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, 503, "check out my channel t.me/promo")
			DrainAISpamChecks()

			deletes := client.callsFor("deleteMessage")
			if tt.deleted && len(deletes) != 1 {
				t.Fatalf("deleteMessage calls = %d, want 1", len(deletes))
			}
			if !tt.deleted && len(deletes) != 0 {
				t.Fatalf("deleteMessage calls = %d, want 0", len(deletes))
			}
		})
	}
}

// The sender window is what the filter remembers between messages, and it lives
// in Redis: a deploy, a restart or a second replica must not blind repetition
// detection. Clearing the process state stands in for all three.
func TestCheckAISpamSenderHistorySurvivesProcessReset(t *testing.T) {
	resetAISpamRuntime()
	withMiniredis(t)
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	var (
		mu     sync.Mutex
		states []map[string]any
	)
	withAISpamJevServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			State map[string]any `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		mu.Lock()
		states = append(states, body.State)
		mu.Unlock()
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.1, "none"))
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "History Chat"}
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	sender := gotgbot.User{Id: 42, FirstName: "Member"}
	aispamProcessUpdate(t, dispatcher, bot, chat, sender, 601, "first message about pizza")
	DrainAISpamChecks()

	resetAISpamRuntime()

	aispamProcessUpdate(t, dispatcher, bot, chat, sender, 602, "second message about pizza")
	DrainAISpamChecks()

	mu.Lock()
	defer mu.Unlock()
	if len(states) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(states))
	}
	if recent, ok := states[0]["recent_from_sender"]; ok {
		t.Fatalf("first check carried history %#v, want none: a check must not see its own message", recent)
	}
	recent, _ := states[1]["recent_from_sender"].([]any)
	if len(recent) != 1 || recent[0] != "first message about pizza" {
		t.Fatalf("second check recent_from_sender = %#v, want the first message", recent)
	}
	senderState, ok := states[1]["sender"].(map[string]any)
	if !ok {
		t.Fatalf("second check sender state = %#v, want an object", states[1]["sender"])
	}
	if got := senderState["messages_last_hour"]; got != float64(2) {
		t.Fatalf("second check messages_last_hour = %v, want 2", got)
	}
}

func TestCheckAISpamSkipsCarveOuts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		text    string
		apiKey  string
		global  bool
		enable  bool
		prepare func(*moduleBotClient, gotgbot.Chat, int64)
		mutate  func(*gotgbot.Update)
	}{
		{
			name:   "filter disabled in chat",
			text:   "BUY CHEAP COINS t.me/scam",
			apiKey: "typesafe-test-key",
			global: true,
		},
		{
			name:   "operator kill switch off",
			text:   "BUY CHEAP COINS t.me/scam",
			apiKey: "typesafe-test-key",
			global: false,
			enable: true,
		},
		{
			name:   "no API key configured",
			text:   "BUY CHEAP COINS t.me/scam",
			global: true,
			enable: true,
		},
		{
			name:   "private chat",
			text:   "BUY CHEAP COINS t.me/scam",
			apiKey: "typesafe-test-key",
			global: true,
			enable: true,
			mutate: func(update *gotgbot.Update) {
				update.Message.Chat.Type = "private"
			},
		},
		{
			name:   "channel post",
			text:   "BUY CHEAP COINS t.me/scam",
			apiKey: "typesafe-test-key",
			global: true,
			enable: true,
			mutate: func(update *gotgbot.Update) {
				update.Message.From = nil
				update.Message.SenderChat = &gotgbot.Chat{Id: -100555, Type: "channel", Title: "Promo Channel"}
			},
		},
		{
			name:   "message without text",
			text:   "",
			apiKey: "typesafe-test-key",
			global: true,
			enable: true,
			mutate: func(update *gotgbot.Update) {
				update.Message.Photo = []gotgbot.PhotoSize{{FileId: "photo", Width: 1, Height: 1}}
			},
		},
		{
			name:   "sender is solving captcha",
			text:   "BUY CHEAP COINS t.me/scam",
			apiKey: "typesafe-test-key",
			global: true,
			enable: true,
			prepare: func(_ *moduleBotClient, chat gotgbot.Chat, userID int64) {
				if _, err := captcha.CreateCaptchaAttemptPreMessage(userID, chat.Id, "7", 5); err != nil {
					t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
				}
			},
		},
		{
			name:   "sender is admin",
			text:   "BUY CHEAP COINS t.me/scam",
			apiKey: "typesafe-test-key",
			global: true,
			enable: true,
			prepare: func(client *moduleBotClient, _ gotgbot.Chat, _ int64) {
				client.responses["getChatAdministrators"] = []byte(
					`[` +
						`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"},"can_delete_messages":true},` +
						`{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Admin"},"can_delete_messages":true}` +
						`]`,
				)
			},
		},
		{
			name:   "sender is approved",
			text:   "BUY CHEAP COINS t.me/scam",
			apiKey: "typesafe-test-key",
			global: true,
			enable: true,
			prepare: func(_ *moduleBotClient, chat gotgbot.Chat, userID int64) {
				if err := approvals.AddApprovedUser(chat.Id, userID, userID, "test"); err != nil {
					t.Fatalf("AddApprovedUser() error = %v", err)
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAISpamRuntime()
			withAISpamTestConfig(t, tt.apiKey, tt.global, 0)

			var requests atomic.Int64
			withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				_, _ = fmt.Fprint(w, aispamDecisionBody(0.99, "scam"))
			})

			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Carve Out Chat"}
			user := gotgbot.User{Id: 42, FirstName: "Member"}
			if tt.enable {
				if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
					t.Fatalf("SetAISpamEnabled() error = %v", err)
				}
			}
			if tt.prepare != nil {
				tt.prepare(client, chat, user.Id)
			}

			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadAISpam(dispatcher)

			update := aispamMessageUpdate(chat, user, 504, tt.text)
			if tt.mutate != nil {
				tt.mutate(update)
			}
			aispamProcessRawUpdate(t, dispatcher, bot, update)
			DrainAISpamChecks()

			if got := requests.Load(); got != 0 {
				t.Fatalf("provider requests = %d, want 0", got)
			}
			if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
				t.Fatalf("deleteMessage calls = %d, want 0", len(deletes))
			}
		})
	}
}

func TestCheckAISpamCommandCarveOut(t *testing.T) {
	for _, tt := range []struct {
		name      string
		text      string
		judged    bool
		messageID int64
	}{
		{name: "bare command", text: "/help", judged: false, messageID: 506},
		{name: "command for this bot", text: "/help@AlitaTestBot", judged: false, messageID: 507},
		{name: "command for another bot", text: "/promo@SomeOtherBot buy now", judged: true, messageID: 508},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetAISpamRuntime()
			withAISpamTestConfig(t, "typesafe-test-key", true, 0)

			var requests atomic.Int64
			withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				_, _ = fmt.Fprint(w, aispamDecisionBody(0.99, "promotion"))
			})

			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Command Chat"}
			if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
				t.Fatalf("SetAISpamEnabled() error = %v", err)
			}

			dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
			LoadAISpam(dispatcher)

			aispamProcessUpdate(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, tt.messageID, tt.text)
			DrainAISpamChecks()

			wantRequests := int64(0)
			if tt.judged {
				wantRequests = 1
			}
			if got := requests.Load(); got != wantRequests {
				t.Fatalf("provider requests = %d, want %d", got, wantRequests)
			}
			if got := len(client.callsFor("deleteMessage")); int64(got) != wantRequests {
				t.Fatalf("deleteMessage calls = %d, want %d", got, wantRequests)
			}
		})
	}
}

func TestCheckAISpamFailsOpenOnProviderErrors(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Broken Provider Chat"}
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	aispamProcessUpdate(t, dispatcher, bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, 505, "BUY CHEAP COINS t.me/scam")
	DrainAISpamChecks()

	if deletes := client.callsFor("deleteMessage"); len(deletes) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0 (fail open)", len(deletes))
	}
	if !aispam.IsAISpamEnabled(chat.Id) {
		t.Fatal("a provider error disabled the chat's filter")
	}
	_, _, _, _, failed := GetAISpamStats()
	if failed != 1 {
		t.Fatalf("failed checks = %d, want 1", failed)
	}
}

func TestAISpamBreakerPausesChatAfterRepeatedFailures(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	var requests atomic.Int64
	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		// A backoff longer than the client is willing to wait drops the retry,
		// so each check costs exactly one request.
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Breaker Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	for i := range aispamBreakerFailures {
		aispamProcessUpdate(t, dispatcher, bot, chat, user, int64(700+i), fmt.Sprintf("BUY CHEAP COINS %d", i))
	}
	DrainAISpamChecks()

	if got := requests.Load(); got != aispamBreakerFailures {
		t.Fatalf("provider requests = %d, want %d", got, aispamBreakerFailures)
	}
	if !aispamBreakerPaused(chat.Id) {
		t.Fatal("breaker did not pause the chat")
	}
	// The notice fires once per trip, so a further failure must not ask for one.
	if aispamBreakerRecordFailure(chat.Id) {
		t.Fatal("breaker asked for a second notice in the same pause")
	}

	aispamProcessUpdate(t, dispatcher, bot, chat, user, 799, "BUY CHEAP COINS AGAIN")
	DrainAISpamChecks()

	if got := requests.Load(); got != aispamBreakerFailures {
		t.Fatalf("provider requests = %d, want %d (paused chat must not call the provider)", got, aispamBreakerFailures)
	}
}

func TestAISpamBreakerResumesAfterCooldown(t *testing.T) {
	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	previousCooldown := aispamBreakerCooldown
	aispamBreakerCooldown = 50 * time.Millisecond
	t.Cleanup(func() { aispamBreakerCooldown = previousCooldown })

	var fail atomic.Bool
	fail.Store(true)
	var requests atomic.Int64
	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if fail.Load() {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.99, "scam"))
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Recovering Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	for i := range aispamBreakerFailures {
		aispamProcessUpdate(t, dispatcher, bot, chat, user, int64(800+i), fmt.Sprintf("BUY CHEAP COINS %d", i))
	}
	DrainAISpamChecks()

	if !aispamBreakerPaused(chat.Id) {
		t.Fatal("breaker did not pause the chat")
	}

	time.Sleep(aispamBreakerCooldown + 20*time.Millisecond)
	fail.Store(false)

	aispamProcessUpdate(t, dispatcher, bot, chat, user, 899, "BUY CHEAP COINS AFTER COOLDOWN")
	DrainAISpamChecks()

	if aispamBreakerPaused(chat.Id) {
		t.Fatal("breaker stayed paused after its cooldown elapsed")
	}
	if got := requests.Load(); got != aispamBreakerFailures+1 {
		t.Fatalf("provider requests = %d, want %d (checks must resume after the cooldown)", got, aispamBreakerFailures+1)
	}
	if deletes := client.callsFor("deleteMessage"); len(deletes) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1", len(deletes))
	}
}

// TestAISpamIsReachableFromHelp pins the user-visible integration: a watcher
// alone does not put a module in the /help menu, and the name an admin types
// must resolve to the module's own page rather than the main help.
func TestAISpamIsReachableFromHelp(t *testing.T) {
	registry := DefaultHelpRegistry()
	previousEnabled, hadEnabled := registry.AbleMap[aispamModule.moduleName]
	t.Cleanup(func() {
		if hadEnabled {
			registry.AbleMap[aispamModule.moduleName] = previousEnabled
			return
		}
		delete(registry.AbleMap, aispamModule.moduleName)
	})

	LoadAISpam(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))

	var button bool
	for _, row := range initHelpButtonsFrom(registry).InlineKeyboard {
		for _, each := range row {
			button = button || each.Text == aispamModule.moduleName
		}
	}
	if !button {
		t.Fatalf("%s is missing from the /help menu", aispamModule.moduleName)
	}

	if got := getModuleNameFromAltName("aispam", registry); got != aispamModule.moduleName {
		t.Fatalf("/help aispam resolves to %q, want %q", got, aispamModule.moduleName)
	}
}
