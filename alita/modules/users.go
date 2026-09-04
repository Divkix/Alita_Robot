package modules

import (
	"strconv"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"golang.org/x/sync/singleflight"

	"github.com/divkix/Alita_Robot/alita/db/channels"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/constants"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

func asyncUpdateUser(userId int64, username, name string) {
	if err := user.UpdateUser(userId, username, name); err != nil {
		userUpdateCache.Delete(userId)
		log.Warnf("[Users] Failed to update user %d: %v", userId, err)
	}
}

func asyncUpdateChat(chatId int64, chatname string, userid int64) {
	if err := chats.UpdateChat(chatId, chatname, userid); err != nil {
		chatUpdateCache.Delete([2]int64{chatId, userid})
		log.Warnf("[Users] Failed to update chat %d: %v", chatId, err)
	}
}

func asyncUpdateChannel(channelId int64, channelName, username string) {
	if err := channels.UpdateChannel(channelId, channelName, username); err != nil {
		channelUpdateCache.Delete(channelId)
		log.Warnf("[Users] Failed to update channel %d: %v", channelId, err)
	}
}

var (
	usersModule = moduleStruct{
		moduleName:   "Users",
		handlerGroup: -1,
	}

	userUpdateCache    = &sync.Map{}
	chatUpdateCache    = &sync.Map{}
	channelUpdateCache = &sync.Map{}
	userUpdateGroup    singleflight.Group
	chatUpdateGroup    singleflight.Group

	userUpdateInterval    = constants.UserUpdateInterval
	chatUpdateInterval    = constants.ChatUpdateInterval
	channelUpdateInterval = constants.ChannelUpdateInterval
)

func shouldUpdateKey(cache *sync.Map, key any, interval time.Duration) bool {
	now := time.Now()
	for {
		lastUpdate, loaded := cache.LoadOrStore(key, now)
		if !loaded {
			expireUpdateKey(cache, key, now, interval)
			return true
		}
		if now.Sub(lastUpdate.(time.Time)) < interval {
			return false
		}
		if cache.CompareAndSwap(key, lastUpdate, now) {
			expireUpdateKey(cache, key, now, interval)
			return true
		}
	}
}

func expireUpdateKey(cache *sync.Map, key any, timestamp time.Time, interval time.Duration) {
	time.AfterFunc(interval, func() {
		cache.CompareAndDelete(key, timestamp)
	})
}

func shouldUpdate(cache *sync.Map, id int64, interval time.Duration) bool {
	return shouldUpdateKey(cache, id, interval)
}

func shouldUpdateChatMember(cache *sync.Map, chatID, userID int64, interval time.Duration) bool {
	return shouldUpdateKey(cache, [2]int64{chatID, userID}, interval)
}

func updateCurrentUser(userID int64, username, name string) {
	_, _, _ = userUpdateGroup.Do(strconv.FormatInt(userID, 10), func() (any, error) {
		if shouldUpdate(userUpdateCache, userID, userUpdateInterval) {
			asyncUpdateUser(userID, username, name)
		}
		return nil, nil
	})
}

func updateCurrentChat(chatID int64, chatName string, userID int64) {
	key := strconv.FormatInt(chatID, 10) + ":" + strconv.FormatInt(userID, 10)
	_, _, _ = chatUpdateGroup.Do(key, func() (any, error) {
		if shouldUpdateChatMember(chatUpdateCache, chatID, userID, chatUpdateInterval) {
			asyncUpdateChat(chatID, chatName, userID)
		}
		return nil, nil
	})
}

func (moduleStruct) logUsers(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender
	repliedMsg := msg.ReplyToMessage

	if user != nil {
		if user.IsAnonymousChannel() {
			if shouldUpdate(channelUpdateCache, user.Id(), channelUpdateInterval) {
				log.Debugf("Updating channel %d in db", user.Id())
				go asyncUpdateChannel(
					user.Id(),
					user.Name(),
					user.Username(),
				)
			}
		} else {
			if chat_status.RequireGroup(bot, ctx, chat) {
				updateCurrentChat(chat.Id, chat.Title, user.Id())
			}

			updateCurrentUser(user.Id(), user.Username(), user.Name())
		}
	}

	if repliedMsg != nil {
		replySender := repliedMsg.GetSender()
		if replySender != nil {
			if replySender.IsAnonymousChannel() {
				if shouldUpdate(channelUpdateCache, replySender.Id(), channelUpdateInterval) {
					log.Debugf("Updating channel %d in db", replySender.Id())
					go asyncUpdateChannel(
						replySender.Id(),
						replySender.Name(),
						replySender.Username(),
					)
				}
			} else {
				if shouldUpdate(userUpdateCache, replySender.Id(), userUpdateInterval) {
					log.Debugf("Updating user %d in db", replySender.Id())
					go asyncUpdateUser(
						replySender.Id(),
						replySender.Username(),
						replySender.Name(),
					)
				}
			}
		}
	}

	if msg.ForwardOrigin != nil {
		forwarded := msg.ForwardOrigin.MergeMessageOrigin()
		if forwarded.Chat != nil && forwarded.Chat.Type != "group" {
			if shouldUpdate(channelUpdateCache, forwarded.Chat.Id, channelUpdateInterval) {
				go asyncUpdateChannel(
					forwarded.Chat.Id,
					forwarded.Chat.Title,
					forwarded.Chat.Username,
				)
			}
		} else if forwarded.SenderUser != nil {
			if shouldUpdate(userUpdateCache, forwarded.SenderUser.Id, userUpdateInterval) {
				go asyncUpdateUser(
					forwarded.SenderUser.Id,
					forwarded.SenderUser.Username,
					formatting.GetFullName(
						forwarded.SenderUser.FirstName,
						forwarded.SenderUser.LastName,
					),
				)
			}
		}
	}

	return ext.ContinueGroups
}

func LoadUsers(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, usersModule.logUsers), usersModule.handlerGroup)
}

func init() {
	RegisterLegacyModule("Users", 100, LoadUsers)
}
