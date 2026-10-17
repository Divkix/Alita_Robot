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
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

func spawnAsyncUpdate(fn func()) {
	usersAsyncWG.Add(1)
	go func() {
		defer usersAsyncWG.Done()
		fn()
	}()
}

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

	// Tracked so shutdown can drain in-flight DB writes before db.Close().
	usersAsyncWG sync.WaitGroup

	userUpdateInterval    = constants.UserUpdateInterval
	chatUpdateInterval    = constants.ChatUpdateInterval
	channelUpdateInterval = constants.ChannelUpdateInterval
)

// DrainUsersAsyncWrites blocks until fire-and-forget user/chat/channel upserts finish.
func DrainUsersAsyncWrites() {
	usersAsyncWG.Wait()
}

// throttleEntry is the value stored in the update caches: when the last write
// for a key was let through and the identity fields it carried. Fields that
// differ from the cached ones (a rename) bypass the interval.
type throttleEntry struct {
	at    time.Time
	state [2]string
}

func shouldUpdateKey(cache *sync.Map, key any, state [2]string, interval time.Duration) bool {
	now := time.Now()
	next := throttleEntry{at: now, state: state}
	for {
		prev, loaded := cache.LoadOrStore(key, next)
		if !loaded {
			return true
		}
		last := prev.(throttleEntry)
		if last.state == state && now.Sub(last.at) < interval {
			return false
		}
		if cache.CompareAndSwap(key, prev, next) {
			return true
		}
	}
}

// sweepUpdateCache drops throttle entries older than interval; shouldUpdateKey
// already treats them as expired, this only bounds memory.
func sweepUpdateCache(cache *sync.Map, now time.Time, interval time.Duration) {
	cache.Range(func(key, value any) bool {
		if entry, ok := value.(throttleEntry); ok && now.Sub(entry.at) >= interval {
			cache.CompareAndDelete(key, entry)
		}
		return true
	})
}

func usersThrottleSweepLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		func() {
			defer error_handling.RecoverFromPanic("usersThrottleSweep", "users")
			sweepUpdateCache(userUpdateCache, now, userUpdateInterval)
			sweepUpdateCache(chatUpdateCache, now, chatUpdateInterval)
			sweepUpdateCache(channelUpdateCache, now, channelUpdateInterval)
		}()
	}
}

func shouldUpdateUser(cache *sync.Map, userID int64, username, name string, interval time.Duration) bool {
	return shouldUpdateKey(cache, userID, [2]string{username, name}, interval)
}

func shouldUpdateChannel(cache *sync.Map, channelID int64, channelName, username string, interval time.Duration) bool {
	return shouldUpdateKey(cache, channelID, [2]string{channelName, username}, interval)
}

func shouldUpdateChatMember(cache *sync.Map, chatID int64, chatName string, userID int64, interval time.Duration) bool {
	return shouldUpdateKey(cache, [2]int64{chatID, userID}, [2]string{chatName}, interval)
}

func updateCurrentUser(userID int64, username, name string) {
	_, _, _ = userUpdateGroup.Do(strconv.FormatInt(userID, 10), func() (any, error) {
		if shouldUpdateUser(userUpdateCache, userID, username, name, userUpdateInterval) {
			asyncUpdateUser(userID, username, name)
		}
		return nil, nil
	})
}

func updateCurrentChat(chatID int64, chatName string, userID int64) {
	key := strconv.FormatInt(chatID, 10) + ":" + strconv.FormatInt(userID, 10)
	_, _, _ = chatUpdateGroup.Do(key, func() (any, error) {
		if shouldUpdateChatMember(chatUpdateCache, chatID, chatName, userID, chatUpdateInterval) {
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
			uid, uname, username := user.Id(), user.Name(), user.Username()
			if shouldUpdateChannel(channelUpdateCache, uid, uname, username, channelUpdateInterval) {
				log.Debugf("Updating channel %d in db", uid)
				spawnAsyncUpdate(func() { asyncUpdateChannel(uid, uname, username) })
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
				sid, sname, susername := replySender.Id(), replySender.Name(), replySender.Username()
				if shouldUpdateChannel(channelUpdateCache, sid, sname, susername, channelUpdateInterval) {
					log.Debugf("Updating channel %d in db", sid)
					spawnAsyncUpdate(func() { asyncUpdateChannel(sid, sname, susername) })
				}
			} else {
				sid, susername, sname := replySender.Id(), replySender.Username(), replySender.Name()
				if shouldUpdateUser(userUpdateCache, sid, susername, sname, userUpdateInterval) {
					log.Debugf("Updating user %d in db", sid)
					spawnAsyncUpdate(func() { asyncUpdateUser(sid, susername, sname) })
				}
			}
		}
	}

	if msg.ForwardOrigin != nil {
		forwarded := msg.ForwardOrigin.MergeMessageOrigin()
		if forwarded.Chat != nil && forwarded.Chat.Type != "group" {
			cid, ctitle, cusername := forwarded.Chat.Id, forwarded.Chat.Title, forwarded.Chat.Username
			if shouldUpdateChannel(channelUpdateCache, cid, ctitle, cusername, channelUpdateInterval) {
				spawnAsyncUpdate(func() { asyncUpdateChannel(cid, ctitle, cusername) })
			}
		} else if forwarded.SenderUser != nil {
			fuid := forwarded.SenderUser.Id
			fusername := forwarded.SenderUser.Username
			fname := formatting.GetFullName(forwarded.SenderUser.FirstName, forwarded.SenderUser.LastName)
			if shouldUpdateUser(userUpdateCache, fuid, fusername, fname, userUpdateInterval) {
				spawnAsyncUpdate(func() { asyncUpdateUser(fuid, fusername, fname) })
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
	go usersThrottleSweepLoop()
}
