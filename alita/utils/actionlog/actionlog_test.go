package actionlog

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
)

func TestLogSkipsInvalidAndChannelChats(t *testing.T) {
	bot := &gotgbot.Bot{User: gotgbot.User{Id: 999, IsBot: true}}
	channel := &gotgbot.Chat{Id: -1001, Type: "channel", Title: "Broadcast"}
	Log(nil, channel, logchannels.CategoryAdmin, "hello")
	Log(bot, nil, logchannels.CategoryAdmin, "hello")
	Log(bot, channel, logchannels.CategoryAdmin, "")
	Log(bot, channel, logchannels.CategoryAdmin, "hello")
}

func TestHelpersSkipNilActorsOrChannelChats(t *testing.T) {
	bot := &gotgbot.Bot{User: gotgbot.User{Id: 999, IsBot: true}}
	channel := &gotgbot.Chat{Id: -1001, Type: "channel", Title: "Broadcast"}
	actor := &gotgbot.User{Id: 7, FirstName: "Admin"}

	Admin(bot, channel, nil, "BAN", 42, "spam")
	Admin(bot, channel, actor, "BAN", 42, "spam")
	User(bot, channel, nil, "KICKME")
	User(bot, channel, actor, "KICKME")
	Reports(bot, channel, nil, 42)
	Reports(bot, channel, actor, 42)
	Settings(bot, channel, nil, "changed")
	Settings(bot, channel, actor, "changed")
}
