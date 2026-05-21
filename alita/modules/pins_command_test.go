package modules

import (
	"errors"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func TestUnpinWithoutReplyUnpinsLatestMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleMessageContext(bot, chat, user, "/unpin")
	cmdCtx, err := helpers.BuildCommandContext(bot, ctx)
	if err != nil {
		t.Fatalf("BuildCommandContext failed: %v", err)
	}
	if err := pinsModule.unpin(cmdCtx); err != ext.EndGroups {
		t.Fatalf("unpin() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("unpinChatMessage"); len(calls) != 1 {
		t.Fatalf("unpinChatMessage calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestUnpinReplyWithoutPinnedMessageDoesNotCallTelegramUnpin(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	if calls := client.callsFor("unpinChatMessage"); len(calls) != 0 {
		t.Fatalf("unpinChatMessage calls = %d, want 0 for non-pinned reply", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestUnpinReplyWithPinnedMessageUnpinsReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	if got, ok := calls[0].Params["message_id"].(*int64); !ok || got == nil || *got != int64(77) {
		t.Fatalf("unpinChatMessage message_id = %#v, want pointer to 77", calls[0].Params["message_id"])
	}
}

func TestUnpinAllSendsConfirmationButtons(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	yesData := encodeCallbackData("unpinallbtn", map[string]string{"a": "yes"}, "unpinallbtn(yes)")
	yesCtx := newModuleCallbackContext(bot, chat, user, yesData)
	if err := pinsModule.unpinallCallback(bot, yesCtx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback yes error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("unpinAllChatMessages"); len(calls) != 1 {
		t.Fatalf("unpinAllChatMessages calls = %d, want 1", len(calls))
	}

	noCtx := newModuleCallbackContext(bot, chat, user, "unpinallbtn(no)")
	if err := pinsModule.unpinallCallback(bot, noCtx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback no error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want 2", len(calls))
	}
}

func TestUnpinAllCallbackRejectsInvalidAction(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	ctx := newModuleCallbackContext(bot, chat, user, "unpinallbtn(maybe)")
	if err := pinsModule.unpinallCallback(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unpinallCallback invalid error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want invalid callback answer", len(calls))
	}
}

func TestPinReplyPinsMessageWithNotificationOption(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	calls := client.callsFor("pinChatMessage")
	if len(calls) != 1 {
		t.Fatalf("pinChatMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["message_id"] != int64(88) {
		t.Fatalf("pinned message_id = %v, want 88", calls[0].Params["message_id"])
	}
}

func TestPinWithoutReplyAsksForReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	pinCalls := client.callsFor("pinChatMessage")
	if len(pinCalls) != 1 {
		t.Fatalf("pinChatMessage calls = %d, want 1", len(pinCalls))
	}
	if got := pinCalls[0].Params["message_id"]; got != int64(9001) {
		t.Fatalf("pinned message_id = %v, want sent message 9001", got)
	}
}

func TestPinnedCommandLinksLatestPinnedMessage(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChat"] = []byte(
		`{"id":-1001,"type":"supergroup","title":"Pin Chat","pinned_message":{"message_id":77,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Pin Chat"},"text":"pinned"}}`,
	)
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

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

func TestAntiChannelPinAndCleanLinkedTogglePinPreferences(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	antiOnCtx := newModuleMessageContext(bot, chat, user, "/antichannelpin on")
	if err := pinsModule.antichannelpin(bot, antiOnCtx); err != ext.EndGroups {
		t.Fatalf("antichannelpin on error = %v, want EndGroups", err)
	}
	if !db.GetPinData(chatID).AntiChannelPin {
		t.Fatal("AntiChannelPin was not enabled")
	}

	cleanOnCtx := newModuleMessageContext(bot, chat, user, "/cleanlinked on")
	if err := pinsModule.cleanlinked(bot, cleanOnCtx); err != ext.EndGroups {
		t.Fatalf("cleanlinked on error = %v, want EndGroups", err)
	}
	if !db.GetPinData(chatID).CleanLinked {
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
	if db.GetPinData(chatID).CleanLinked {
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
	if db.GetPinData(chatID).AntiChannelPin {
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
	if err := db.SetCleanLinked(cleanChat.Id, true); err != nil {
		t.Fatalf("SetCleanLinked() error = %v", err)
	}
	cleanCtx := newModuleMessageContext(bot, cleanChat, user, "linked message")
	if err := pinsModule.checkPinned(bot, cleanCtx); err != ext.ContinueGroups {
		t.Fatalf("checkPinned clean error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1", len(calls))
	}

	unpinChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	if err := db.SetAntiChannelPin(unpinChat.Id, true); err != nil {
		t.Fatalf("SetAntiChannelPin() error = %v", err)
	}
	unpinCtx := newModuleMessageContext(bot, unpinChat, user, "linked message")
	if err := pinsModule.checkPinned(bot, unpinCtx); err != ext.ContinueGroups {
		t.Fatalf("checkPinned unpin error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("unpinChatMessage"); len(calls) != 1 {
		t.Fatalf("unpinChatMessage calls = %d, want 1", len(calls))
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

func TestPinTypeMediaSendersAndExtraction(t *testing.T) {
	client := newModuleBotClient()
	genericMessage := []byte(
		`{"message_id":9010,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Pin Chat"}}`,
	)
	for _, method := range []string{
		"sendSticker",
		"sendAudio",
		"sendVoice",
		"sendVideo",
		"sendVideoNote",
	} {
		client.responses[method] = genericMessage
	}
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Pin Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/permapin media")
	keyboard := &gotgbot.InlineKeyboardMarkup{}

	tests := []struct {
		name       string
		dataType   int
		fileID     string
		wantMethod string
	}{
		{name: "text", dataType: db.TEXT, wantMethod: "sendMessage"},
		{name: "sticker fallback", dataType: db.STICKER, wantMethod: "sendMessage"},
		{name: "sticker", dataType: db.STICKER, fileID: "sticker-file", wantMethod: "sendSticker"},
		{name: "document fallback", dataType: db.DOCUMENT, wantMethod: "sendMessage"},
		{name: "document", dataType: db.DOCUMENT, fileID: "doc-file", wantMethod: "sendDocument"},
		{name: "photo fallback", dataType: db.PHOTO, wantMethod: "sendMessage"},
		{name: "photo", dataType: db.PHOTO, fileID: "photo-file", wantMethod: "sendPhoto"},
		{name: "audio", dataType: db.AUDIO, fileID: "audio-file", wantMethod: "sendAudio"},
		{name: "voice", dataType: db.VOICE, fileID: "voice-file", wantMethod: "sendVoice"},
		{name: "video", dataType: db.VIDEO, fileID: "video-file", wantMethod: "sendVideo"},
		{name: "video note", dataType: db.VideoNote, fileID: "video-note-file", wantMethod: "sendVideoNote"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := len(client.callsFor(tc.wantMethod))
			sendFn := PinsEnumFuncMap[tc.dataType]
			if _, err := sendFn(bot, ctx, pinType{MsgText: "hello", FileID: tc.fileID}, keyboard, 44); err != nil {
				t.Fatalf("PinsEnumFuncMap[%d]() error = %v", tc.dataType, err)
			}
			if calls := client.callsFor(tc.wantMethod); len(calls) != before+1 {
				t.Fatalf("%s calls = %d, want %d", tc.wantMethod, len(calls), before+1)
			}
		})
	}

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
		{name: "video note", reply: &gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "video-note-file"}}, wantType: db.VideoNote, wantID: "video-note-file"},
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

	if moduleName, enabled := DefaultHelpRegistry().AbleMap.Load(pinsModule.moduleName); moduleName != pinsModule.moduleName || !enabled {
		t.Fatalf("pins help registration = (%q, %v), want enabled", moduleName, enabled)
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
			if err := db.SetCleanLinked(ctx.EffectiveChat.Id, true); err != nil {
				t.Fatalf("SetCleanLinked() error = %v", err)
			}
		}, run: pinsModule.checkPinned},
		{name: "check pinned unpin", text: "linked message", method: "unpinChatMessage", prepare: func(_ *moduleBotClient, ctx *ext.Context) {
			if err := db.SetAntiChannelPin(ctx.EffectiveChat.Id, true); err != nil {
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

	yesData := encodeCallbackData("unpinallbtn", map[string]string{"a": "yes"}, "unpinallbtn(yes)")
	for _, tt := range []struct {
		name   string
		data   string
		method string
	}{
		{name: "unpin all request", data: yesData, method: "unpinAllChatMessages"},
		{name: "unpin all edit", data: yesData, method: "editMessageText"},
		{name: "unpin all cancel edit", data: "unpinallbtn(no)", method: "editMessageText"},
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
