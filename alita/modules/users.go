package modules

import (
	"strconv"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/divkix/Alita_Robot/alita/db/channels"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/constants"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

func spawnAsyncUpdate(fn func()) {
	usersAsyncWG.Add(1)
	go func() {
		defer usersAsyncWG.Done()
		defer error_handling.RecoverFromPanic("spawnAsyncUpdate", "users")
		fn()
	}()
}

func asyncUpdateUser(userId int64, username, name string) {
	if err := user.UpdateUser(userId, username, name); err != nil {
		userUpdateCache.Delete(userId)
		log.Warnf("[Users] Failed to update user %d: %v", userId, err)
	}
}

func asyncTouchChat(chatId int64, chatname string) {
	if err := chats.TouchChat(chatId, chatname); err != nil {
		chatUpdateCache.Delete(chatId)
		log.Warnf("[Users] Failed to update chat %d: %v", chatId, err)
	}
}

func asyncAddChatMember(chatId, userid int64) {
	if err := chats.AddChatMember(chatId, userid); err != nil {
		chatMemberUpdateCache.Delete([2]int64{chatId, userid})
		log.Warnf("[Users] Failed to add user %d to chat %d: %v", userid, chatId, err)
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

	userUpdateCache = &sync.Map{}
	// chatUpdateCache throttles the chats row touch per chat;
	// chatMemberUpdateCache throttles membership appends per (chat, user).
	chatUpdateCache       = &sync.Map{}
	chatMemberUpdateCache = &sync.Map{}
	channelUpdateCache    = &sync.Map{}
	userUpdateGroup       singleflight.Group
	chatUpdateGroup       singleflight.Group
	chatMemberUpdateGroup singleflight.Group

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

func sweepUsersThrottleCaches(now time.Time) {
	sweepUpdateCache(userUpdateCache, now, userUpdateInterval)
	sweepUpdateCache(chatUpdateCache, now, chatUpdateInterval)
	sweepUpdateCache(chatMemberUpdateCache, now, chatUpdateInterval)
	sweepUpdateCache(channelUpdateCache, now, channelUpdateInterval)
}

func usersThrottleSweepLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		func() {
			defer error_handling.RecoverFromPanic("usersThrottleSweep", "users")
			sweepUsersThrottleCaches(now)
		}()
	}
}

func shouldUpdateUser(cache *sync.Map, userID int64, username, name string, interval time.Duration) bool {
	return shouldUpdateKey(cache, userID, [2]string{username, name}, interval)
}

func shouldUpdateChannel(cache *sync.Map, channelID int64, channelName, username string, interval time.Duration) bool {
	return shouldUpdateKey(cache, channelID, [2]string{channelName, username}, interval)
}

func shouldTouchChat(cache *sync.Map, chatID int64, chatName string, interval time.Duration) bool {
	return shouldUpdateKey(cache, chatID, [2]string{chatName}, interval)
}

func shouldUpdateChatMember(cache *sync.Map, chatID, userID int64, interval time.Duration) bool {
	return shouldUpdateKey(cache, [2]int64{chatID, userID}, [2]string{}, interval)
}

func updateCurrentUser(userID int64, username, name string) {
	_, _, _ = userUpdateGroup.Do(strconv.FormatInt(userID, 10), func() (any, error) {
		if shouldUpdateUser(userUpdateCache, userID, username, name, userUpdateInterval) {
			asyncUpdateUser(userID, username, name)
		}
		return nil, nil
	})
}

// touchCurrentChat touches the chats row once per chat per interval (or on a
// rename).
func touchCurrentChat(chatID int64, chatName string) {
	_, _, _ = chatUpdateGroup.Do(strconv.FormatInt(chatID, 10), func() (any, error) {
		if shouldTouchChat(chatUpdateCache, chatID, chatName, chatUpdateInterval) {
			asyncTouchChat(chatID, chatName)
		}
		return nil, nil
	})
}

// updateCurrentChat touches the chats row, then records membership once per
// (chat, user) per interval. The touch runs first so a chat seen for the first
// time exists before the append.
func updateCurrentChat(chatID int64, chatName string, userID int64) {
	touchCurrentChat(chatID, chatName)
	memberKey := strconv.FormatInt(chatID, 10) + ":" + strconv.FormatInt(userID, 10)
	_, _, _ = chatMemberUpdateGroup.Do(memberKey, func() (any, error) {
		if shouldUpdateChatMember(chatMemberUpdateCache, chatID, userID, chatUpdateInterval) {
			asyncAddChatMember(chatID, userID)
		}
		return nil, nil
	})
}

// isChannelSender reports a channel posting into a group: send-as-channel or a
// linked channel's automatic forward.
func isChannelSender(s *gotgbot.Sender) bool {
	return s.Chat != nil && s.Chat.Type == "channel" && !s.IsChannelPost()
}

func (moduleStruct) logUsers(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender
	repliedMsg := msg.ReplyToMessage

	// Every group message counts once, whatever its sender: a real user, an
	// anonymous admin (GroupAnonymousBot) or a channel. This watcher matches
	// new messages only (handlers.NewMessage excludes edited ones), so an edit
	// never reaches this line. The increment is in process memory; see the
	// stats package for the batched flush.
	if chat != nil && (chat.Type == "group" || chat.Type == "supergroup") {
		stats.RecordMessage(chat.Id)
		// The per-member count ranks real users only: a channel or an
		// anonymous admin posts with sender_chat set and has no user to rank.
		if user != nil && user.IsUser() {
			stats.RecordMemberMessage(chat.Id, user.User.Id)
		}
	}

	if user != nil {
		if isChannelSender(user) {
			uid, uname, username := user.Id(), user.Name(), user.Username()
			if shouldUpdateChannel(channelUpdateCache, uid, uname, username, channelUpdateInterval) {
				log.Debugf("Updating channel %d in db", uid)
				spawnAsyncUpdate(func() { asyncUpdateChannel(uid, uname, username) })
			}
		} else if user.Chat != nil {
			// Anonymous admin (or other chat sender): the chat is not a user or a member.
			if chat_status.RequireGroup(bot, ctx, chat) {
				touchCurrentChat(chat.Id, chat.Title)
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
			if isChannelSender(replySender) {
				sid, sname, susername := replySender.Id(), replySender.Name(), replySender.Username()
				if shouldUpdateChannel(channelUpdateCache, sid, sname, susername, channelUpdateInterval) {
					log.Debugf("Updating channel %d in db", sid)
					spawnAsyncUpdate(func() { asyncUpdateChannel(sid, sname, susername) })
				}
			} else if replySender.Chat == nil {
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
