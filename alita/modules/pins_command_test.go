package modules

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/pins"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func TestUnpinWithoutReplyUnpinsLatestMessage(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/unpin")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.unpin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("unpin() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "unpinChatMessage"), map[string]any{"chat_id": chat.Id})
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestUnpinReplyUsesReplyMessageID(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/unpin")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 77,
		Date:      1,
		Chat:      chat,
		Text:      "not pinned",
	}
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.unpin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("unpin() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("unpinChatMessage")
	if len(calls) != 1 {
		t.Fatalf("unpinChatMessage calls = %d, want 1", len(calls))
	}
	assertParams(t, calls[0], map[string]any{"chat_id": chat.Id})
	if got, ok := calls[0].Params["message_id"].(*int64); !ok || got == nil || *got != int64(77) {
		t.Fatalf("unpinChatMessage message_id = %#v, want pointer to 77", calls[0].Params["message_id"])
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestUnpinReplyWithPinnedMessageUnpinsReply(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/unpin")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId:      77,
		Date:           1,
		Chat:           chat,
		Text:           "pinned",
		PinnedMessage:  &gotgbot.Message{MessageId: 77, Date: 1, Chat: chat, Text: "pinned"},
		ReplyToMessage: nil,
	}
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.unpin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("unpin(pinned reply) error = %v, want EndGroups", err)
	}
	calls := client.callsFor("unpinChatMessage")
	if len(calls) != 1 {
		t.Fatalf("unpinChatMessage calls = %d, want 1", len(calls))
	}
	assertParams(t, calls[0], map[string]any{"chat_id": chat.Id})
	if got, ok := calls[0].Params["message_id"].(*int64); !ok || got == nil || *got != int64(77) {
		t.Fatalf("unpinChatMessage message_id = %#v, want pointer to 77", calls[0].Params["message_id"])
	}
}

func TestUnpinAllSendsConfirmationButtons(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/unpinall")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.unpinAll(cmdCtx); err != ext.EndGroups {
		t.Fatalf("unpinAll() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("unpinAll confirmation did not include reply_markup")
	}
}

func TestUnpinAllCallbackExecutesAndCancels(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	yesData := encodeCallbackData("unpinallbtn", map[string]string{"a": "yes"})
	yesCtx := newModuleCallbackContext(bot, chat, user, yesData)
	if err := pinsModule.unpinallCallback(bot, yesCtx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback yes error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "unpinAllChatMessages"), map[string]any{"chat_id": chat.Id})

	noCtx := newModuleCallbackContext(bot, chat, user, encodeCallbackData("unpinallbtn", map[string]string{"a": "no"}))
	if err := pinsModule.unpinallCallback(bot, noCtx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback no error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want 2", len(calls))
	}
}

func TestUnpinAllCallbackRejectsInvalidAction(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleCallbackContext(bot, chat, user, "unpinallbtn(maybe)")
	if err := pinsModule.unpinallCallback(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback invalid error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want invalid callback answer", len(calls))
	}
}

func TestPinReplyPinsMessageWithNotificationOption(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/pin loud")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 88,
		Date:      1,
		Chat:      chat,
		Text:      "pin me",
	}
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.pin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("pin() error = %v, want EndGroups", err)
	}
	assertParams(t, client.onlyCall(t, "pinChatMessage"), map[string]any{"chat_id": chat.Id, "message_id": 88})
}

func TestPinWithoutReplyAsksForReply(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/pin")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.pin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("pin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("pinChatMessage"); len(calls) != 0 {
		t.Fatalf("pinChatMessage calls = %d, want 0 without reply", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestPermaPinValidationBranches(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	missingCtx := newModuleMessageContext(bot, chat, user, "/permapin")
	cmdCtx, err := helpers.BuildCommandContext(bot, missingCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.permaPin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("permaPin missing error = %v, want EndGroups", err)
	}

	unsupportedCtx := newModuleMessageContext(bot, chat, user, "/permapin")
	unsupportedCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 44,
		Date:      1,
		Chat:      chat,
	}
	cmdCtx2, err := helpers.BuildCommandContext(bot, unsupportedCtx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.permaPin(cmdCtx2); err != ext.EndGroups {
		t.Fatalf("permaPin unsupported error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want validation replies", len(calls))
	}
	if calls := client.callsFor("pinChatMessage"); len(calls) != 0 {
		t.Fatalf("pinChatMessage calls = %d, want none for validation branches", len(calls))
	}
}

func TestPermaPinSendsNewMessageThenPinsIt(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/permapin keep this pinned")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.permaPin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("permaPin() error = %v, want EndGroups", err)
	}
	sendCalls := client.callsFor("sendMessage")
	if len(sendCalls) != 2 {
		t.Fatalf("sendMessage calls = %d, want pin message and confirmation", len(sendCalls))
	}
	assertParams(t, client.onlyCall(t, "pinChatMessage"), map[string]any{"chat_id": chat.Id, "message_id": 9001})
}

func TestPinnedCommandLinksLatestPinnedMessage(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	client.chatsByID[fmt.Sprint(chat.Id)] = []byte(fmt.Sprintf(
		`{"id":%d,"type":"supergroup","title":"Pin Chat","pinned_message":{"message_id":77,"date":1,"chat":{"id":%d,"type":"supergroup","title":"Pin Chat"},"text":"pinned"}}`,
		chat.Id, chat.Id,
	))

	ctx := newModuleMessageContext(bot, chat, user, "/pinned")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.pinned(cmdCtx); err != ext.EndGroups {
		t.Fatalf("pinned() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want pinned link reply", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("pinned reply did not include link button markup")
	}
}

func TestPinnedCommandReportsMissingPinnedMessage(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin

	ctx := newModuleMessageContext(bot, chat, user, "/pinned")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.pinned(cmdCtx); err != ext.EndGroups {
		t.Fatalf("pinned() error = %v, want EndGroups for missing pinned message reply", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want missing pinned message reply", len(calls))
	}
}

func TestPinSettingStatusEscapesChatTitle(t *testing.T) {
	useRealLocales(t)
	for _, tc := range []struct {
		name    string
		handler func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "antichannelpin", handler: pinsModule.antichannelpin},
		{name: "cleanlinked", handler: pinsModule.cleanlinked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			chat := f.chat
			chat.Title = "A <B> & C"
			ctx := newModuleMessageContext(f.bot, chat, f.admin, "/"+tc.name)
			if err := tc.handler(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s() error = %v, want EndGroups", tc.name, err)
			}
			text := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"])
			if !strings.Contains(text, "A &lt;B&gt; &amp; C") {
				t.Fatalf("%s status = %q, want the chat title HTML-escaped", tc.name, text)
			}
		})
	}
}

func TestAntiChannelPinAndCleanLinkedTogglePinPreferences(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, user := f.bot, f.chat, f.admin
	chatID := chat.Id

	antiOnCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin on")
	if err := pinsModule.antichannelpin(bot, antiOnCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin on error = %v, want EndGroups", err)
	}
	if !pins.GetPinData(chatID).AntiChannelPin {
		t.Fatal("AntiChannelPin was not enabled")
	}

	cleanOnCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked on")
	if err := pinsModule.cleanlinked(bot, cleanOnCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked on error = %v, want EndGroups", err)
	}
	if !pins.GetPinData(chatID).CleanLinked {
		t.Fatal("CleanLinked was not enabled")
	}

	antiInvalidCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin maybe")
	if err := pinsModule.antichannelpin(bot, antiInvalidCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin invalid error = %v, want EndGroups", err)
	}

	cleanOffCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked off")
	if err := pinsModule.cleanlinked(bot, cleanOffCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked off error = %v, want EndGroups", err)
	}
	if pins.GetPinData(chatID).CleanLinked {
		t.Fatal("CleanLinked stayed enabled")
	}

	antiCurrentCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin")
	if err := pinsModule.antichannelpin(bot, antiCurrentCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin current error = %v, want EndGroups", err)
	}
	antiOffCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin off")
	if err := pinsModule.antichannelpin(bot, antiOffCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin off error = %v, want EndGroups", err)
	}
	if pins.GetPinData(chatID).AntiChannelPin {
		t.Fatal("AntiChannelPin stayed enabled")
	}
	antiCurrentDisabledCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin")
	if err := pinsModule.antichannelpin(bot, antiCurrentDisabledCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin current disabled error = %v, want EndGroups", err)
	}

	cleanEnabledCurrentCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked on")
	if err := pinsModule.cleanlinked(bot, cleanEnabledCurrentCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked re-enable error = %v, want EndGroups", err)
	}
	cleanCurrentEnabledCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked")
	if err := pinsModule.cleanlinked(bot, cleanCurrentEnabledCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked current enabled error = %v, want EndGroups", err)
	}
	cleanInvalidCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked maybe")
	if err := pinsModule.cleanlinked(bot, cleanInvalidCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked invalid error = %v, want EndGroups", err)
	}
	cleanOffAgainCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked off")
	if err := pinsModule.cleanlinked(bot, cleanOffAgainCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked off again error = %v, want EndGroups", err)
	}
	cleanCurrentCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked")
	if err := pinsModule.cleanlinked(bot, cleanCurrentCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked current error = %v, want EndGroups", err)
	}
}

func TestCheckPinnedDeletesOrUnpinsLinkedChannelMessages(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	cleanChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := pins.SetCleanLinked(cleanChat.Id, true); err != nil {
		t.Fatalf("SetCleanLinked() error = %v", err)
	}
	cleanCtx := newModuleMessageContext(bot, cleanChat, user, "linked message")
	if err := pinsModule.checkPinned(bot, cleanCtx); err != ext.ContinueGroups {
		t.Fatalf("checkPinned clean error = %v, want ContinueGroups", err)
	}
	assertParams(t, client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": cleanChat.Id, "message_id": cleanCtx.EffectiveMessage.MessageId})

	unpinChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	if err := pins.SetAntiChannelPin(unpinChat.Id, true); err != nil {
		t.Fatalf("SetAntiChannelPin() error = %v", err)
	}
	unpinCtx := newModuleMessageContext(bot, unpinChat, user, "linked message")
	if err := pinsModule.checkPinned(bot, unpinCtx); err != ext.ContinueGroups {
		t.Fatalf("checkPinned unpin error = %v, want ContinueGroups", err)
	}
	unpin := client.onlyCall(t, "unpinChatMessage")
	assertParams(t, unpin, map[string]any{"chat_id": unpinChat.Id})
	if got, ok := unpin.Params["message_id"].(*int64); !ok || got == nil || *got != unpinCtx.EffectiveMessage.MessageId {
		t.Fatalf("unpinChatMessage message_id = %#v, want the linked message %d", unpin.Params["message_id"], unpinCtx.EffectiveMessage.MessageId)
	}

	noopChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	noopCtx := newModuleMessageContext(bot, noopChat, user, "linked message")
	if err := pinsModule.checkPinned(bot, noopCtx); err != ext.ContinueGroups {
		t.Fatalf("checkPinned noop error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want unchanged after noop", len(calls))
	}
	if calls := client.callsFor("unpinChatMessage"); len(calls) != 1 {
		t.Fatalf("unpinChatMessage calls = %d, want unchanged after noop", len(calls))
	}
}

func TestGetPinTypeExtractsMedia(t *testing.T) {
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}

	extractCases := []struct {
		name     string
		reply    *gotgbot.Message
		wantType int
		wantID   string
	}{
		{name: "sticker", reply: &gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "sticker-file"}}, wantType: db.STICKER, wantID: "sticker-file"},
		{name: "document", reply: &gotgbot.Message{Document: &gotgbot.Document{FileId: "doc-file"}}, wantType: db.DOCUMENT, wantID: "doc-file"},
		{name: "animation", reply: &gotgbot.Message{Animation: &gotgbot.Animation{FileId: "anim-file"}}, wantType: db.DOCUMENT, wantID: "anim-file"},
		{name: "photo", reply: &gotgbot.Message{Photo: []gotgbot.PhotoSize{{FileId: "small"}, {FileId: "large"}}}, wantType: db.PHOTO, wantID: "large"},
		{name: "audio", reply: &gotgbot.Message{Audio: &gotgbot.Audio{FileId: "audio-file"}}, wantType: db.AUDIO, wantID: "audio-file"},
		{name: "voice", reply: &gotgbot.Message{Voice: &gotgbot.Voice{FileId: "voice-file"}}, wantType: db.VOICE, wantID: "voice-file"},
		{name: "video", reply: &gotgbot.Message{Video: &gotgbot.Video{FileId: "video-file"}}, wantType: db.VIDEO, wantID: "video-file"},
		{name: "video note", reply: &gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "video-note-file"}}, wantType: db.VIDEO_NOTE, wantID: "video-note-file"},
	}
	for _, tc := range extractCases {
		t.Run("extract "+tc.name, func(t *testing.T) {
			msg := &gotgbot.Message{Text: "/permapin", ReplyToMessage: tc.reply}
			tc.reply.Chat = chat
			fileID, _, dataType, _ := pinsModule.GetPinType(msg)
			if dataType != tc.wantType || fileID != tc.wantID {
				t.Fatalf("GetPinType() = (%q, %d), want (%q, %d)", fileID, dataType, tc.wantID, tc.wantType)
			}
		})
	}
}

func TestLoadPinRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadPin(dispatcher)

	if !DefaultHelpRegistry().AbleMap[pinsModule.moduleName] {
		t.Fatal("pins help registration = false, want enabled")
	}
}

func TestPinCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name    string
		text    string
		method  string
		prepare func(*moduleBotClient, *ext.Context)
		run     func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "check pinned delete", text: "linked message", method: "deleteMessage", prepare: func(_ *moduleBotClient, ctx *ext.Context) {
			if err := pins.SetCleanLinked(ctx.EffectiveChat.Id, true); err != nil {
				t.Fatalf("SetCleanLinked() error = %v", err)
			}
		}, run: pinsModule.checkPinned},
		{name: "check pinned unpin", text: "linked message", method: "unpinChatMessage", prepare: func(_ *moduleBotClient, ctx *ext.Context) {
			if err := pins.SetAntiChannelPin(ctx.EffectiveChat.Id, true); err != nil {
				t.Fatalf("SetAntiChannelPin() error = %v", err)
			}
		}, run: pinsModule.checkPinned},
		{name: "unpin latest request", text: "/unpin", method: "unpinChatMessage", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.unpin(cmdCtx)
		}},
		{name: "unpin reply result", text: "/unpin", method: "sendMessage", prepare: func(_ *moduleBotClient, ctx *ext.Context) {
			ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
				MessageId:     77,
				Date:          1,
				Chat:          *ctx.EffectiveChat,
				Text:          "pinned",
				PinnedMessage: &gotgbot.Message{MessageId: 77, Date: 1, Chat: *ctx.EffectiveChat},
			}
		}, run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.unpin(cmdCtx)
		}},
		{name: "unpin all confirmation", text: "/unpinall", method: "sendMessage", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.unpinAll(cmdCtx)
		}},
		{name: "permapin missing target reply", text: "/permapin", method: "sendMessage", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.permaPin(cmdCtx)
		}},
		{name: "permapin send request", text: "/permapin keep pinned", method: "sendMessage", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.permaPin(cmdCtx)
		}},
		{name: "permapin pin request", text: "/permapin keep pinned", method: "pinChatMessage", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.permaPin(cmdCtx)
		}},
		{name: "pin missing reply", text: "/pin", method: "sendMessage", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.pin(cmdCtx)
		}},
		{name: "pin request", text: "/pin", method: "pinChatMessage", prepare: func(_ *moduleBotClient, ctx *ext.Context) {
			ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
				MessageId: 88,
				Date:      1,
				Chat:      *ctx.EffectiveChat,
				Text:      "pin me",
			}
		}, run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.pin(cmdCtx)
		}},
		{name: "pin confirmation reply", text: "/pin", method: "sendMessage", prepare: func(_ *moduleBotClient, ctx *ext.Context) {
			ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
				MessageId: 88,
				Date:      1,
				Chat:      *ctx.EffectiveChat,
				Text:      "pin me",
			}
		}, run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.pin(cmdCtx)
		}},
		{name: "anti channel pin status reply", text: "/antichannelpin", method: "sendMessage", run: pinsModule.antichannelpin},
		{name: "anti channel pin toggle reply", text: "/antichannelpin on", method: "sendMessage", run: pinsModule.antichannelpin},
		{name: "clean linked status reply", text: "/cleanlinked", method: "sendMessage", run: pinsModule.cleanlinked},
		{name: "clean linked toggle reply", text: "/cleanlinked on", method: "sendMessage", run: pinsModule.cleanlinked},
		{name: "pinned get chat request", text: "/pinned", method: "getChat", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.pinned(cmdCtx)
		}},
		{name: "pinned missing message reply", text: "/pinned", method: "sendMessage", run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.pinned(cmdCtx)
		}},
		{name: "pinned result reply", text: "/pinned", method: "sendMessage", prepare: func(client *moduleBotClient, _ *ext.Context) {
			client.responses["getChat"] = []byte(
				`{"id":-1001,"type":"supergroup","title":"Pin Chat","pinned_message":{"message_id":77,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Pin Chat"},"text":"pinned"}}`,
			)
		}, run: func(bot *gotgbot.Bot, ctx *ext.Context) error {
			cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
			if err != nil {
				t.Fatalf("BuildCommandContext failed: %v", err)
			}
			return pinsModule.pinned(cmdCtx)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
			ctx := newModuleMessageContext(bot, chat, user, tt.text)
			if tt.prepare != nil {
				tt.prepare(client, ctx)
			}

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestPinCallbacksPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	yesData := encodeCallbackData("unpinallbtn", map[string]string{"a": "yes"})
	for _, tt := range []struct {
		name   string
		data   string
		method string
	}{
		{name: "unpin all request", data: yesData, method: "unpinAllChatMessages"},
		{name: "unpin all edit", data: yesData, method: "editMessageText"},
		{name: "unpin all cancel edit", data: encodeCallbackData("unpinallbtn", map[string]string{"a": "no"}), method: "editMessageText"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
			ctx := newModuleCallbackContext(bot, chat, user, tt.data)

			err := pinsModule.unpinallCallback(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("unpinallCallback returned error %v, want request error", err)
			}
		})
	}
}

func TestPinByAnonymousAdminSendsOnlyTheProofKeyboard(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	c := newAnonAdminCommandContext(t, f, "/pin")

	if helpers.RunChecks(c, pinDesc.RequiredChecks) {
		t.Fatal("RunChecks() = true, want the anonymous admin held for proof")
	}
	calls := f.client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want only the proof keyboard", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatalf("sendMessage = %#v, want the proof keyboard", calls[0].Params)
	}
	refusal, _ := i18n.MustNewTranslator("en").GetString("chat_status_pin_user_error")
	if text, _ := calls[0].Params["text"].(string); refusal != "" && strings.Contains(text, refusal) {
		t.Fatalf("sendMessage text = %q, want no pin refusal", text)
	}
}
