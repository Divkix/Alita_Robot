package cache

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/constants"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

var adminCacheGeneration atomic.Uint64

const negativeAdminCacheTTL = 2 * time.Minute

func adminCacheKey(chatId int64) string {
	return fmt.Sprintf("%sadminCache:%d", DataCachePrefix, chatId)
}

// getChatAdministrators is called with ReturnBots so other administrator bots
// are included; without it Telegram omits them and IsUserAdmin treats them as
// regular members.
func LoadAdminCache(b *gotgbot.Bot, chatId int64) AdminCache {
	ac, err := LoadAdminCacheWithContext(context.Background(), b, chatId)
	RethrowAdminLoadPanic(err)
	return ac
}

// AdminLoadPanicError carries a panic in the shared admin-list load, with the
// loader's stack, to every caller waiting on it, so a crashed load never reads
// as "no admins". The caller that receives it reports it: a re-raised panic
// reaches the dispatcher's log, and other callers log it themselves.
type AdminLoadPanicError struct {
	Value any
	Stack []byte
}

func (e *AdminLoadPanicError) Error() string {
	return fmt.Sprintf("admin list load panicked: %v\nStack trace:\n%s", e.Value, e.Stack)
}

// RethrowAdminLoadPanic panics again in the caller's goroutine when err is an
// AdminLoadPanicError, as the load did before it ran on its own goroutine.
func RethrowAdminLoadPanic(err error) {
	var loadPanic *AdminLoadPanicError
	if errors.As(err, &loadPanic) {
		panic(loadPanic)
	}
}

// adminLoad is one shared admin-list load and the callers waiting on it. A
// load runs once per key at a time; it leaves adminLoads when it finishes or is
// forgotten, so the table only holds loads in flight.
type adminLoad struct {
	done     chan struct{}
	result   AdminCache
	panicErr *AdminLoadPanicError
	// Guarded by adminLoadsMu.
	waiters  int
	reported bool
}

var (
	adminLoadsMu sync.Mutex
	adminLoads   = map[string]*adminLoad{}
)

// runAdminLoad fills load and wakes its callers. A panic goes to the callers
// that receive it; the loader logs it only when no caller is waiting.
func runAdminLoad(ctx context.Context, load *adminLoad, key string, b *gotgbot.Bot, chatId int64) {
	defer error_handling.RecoverFromPanic("runAdminLoad", "cache")
	var result AdminCache
	var panicErr *AdminLoadPanicError
	returned := false
	// Deferred so a load ended by runtime.Goexit, which neither returns nor
	// panics, still wakes its callers and leaves the table; otherwise every
	// later lookup for the chat would wait on it forever.
	defer func() {
		if !returned && panicErr == nil {
			panicErr = &AdminLoadPanicError{Value: "admin list load exited via runtime.Goexit", Stack: debug.Stack()}
		}
		adminLoadsMu.Lock()
		load.result, load.panicErr = result, panicErr
		close(load.done)
		if adminLoads[key] == load {
			delete(adminLoads, key)
		}
		orphaned := panicErr != nil && load.waiters == 0
		if orphaned {
			load.reported = true
		}
		adminLoadsMu.Unlock()
		if orphaned {
			log.Errorf("[cache][LoadAdminCache] %v", panicErr)
		}
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = &AdminLoadPanicError{Value: r, Stack: debug.Stack()}
			}
		}()
		result = loadAdminCacheFromTelegram(ctx, b, chatId)
	}()
	returned = true
}

// LoadAdminCacheWithContext is LoadAdminCache that stops waiting when ctx ends
// and then returns ctx.Err() with an empty, uncached list. A panic in the load
// comes back as an *AdminLoadPanicError. The Telegram load is
// shared with concurrent callers and keeps the initiating caller's context
// values (including its trace) without its cancellation or deadline, so one
// waiter cannot cut it short for others. Each Telegram request still has its
// own timeout; the completed load stores its result for the next lookup.
func LoadAdminCacheWithContext(ctx context.Context, b *gotgbot.Bot, chatId int64) (AdminCache, error) {
	if b == nil {
		log.Error("LoadAdminCache: bot is nil")
		return AdminCache{}, nil
	}
	if b.Id <= 0 {
		return AdminCache{}, nil
	}

	genBefore := adminCacheGeneration.Load()
	key := adminCacheKey(chatId)
	adminLoadsMu.Lock()
	load, running := adminLoads[key]
	if !running {
		load = &adminLoad{done: make(chan struct{})}
		adminLoads[key] = load
	}
	load.waiters++
	adminLoadsMu.Unlock()
	if !running {
		go runAdminLoad(context.WithoutCancel(ctx), load, key, b, chatId)
	}

	select {
	case <-load.done:
		adminLoadsMu.Lock()
		load.waiters--
		if load.panicErr != nil {
			load.reported = true
		}
		adminLoadsMu.Unlock()
		if load.panicErr != nil {
			return AdminCache{}, load.panicErr
		}
	case <-ctx.Done():
		// The load may have failed just as ctx ended. If it panicked and no
		// caller is left to receive it, the last one to leave logs it.
		adminLoadsMu.Lock()
		load.waiters--
		var unreported *AdminLoadPanicError
		if load.panicErr != nil && !load.reported && load.waiters == 0 {
			load.reported = true
			unreported = load.panicErr
		}
		adminLoadsMu.Unlock()
		if unreported != nil {
			log.Errorf("[cache][LoadAdminCache] %v", unreported)
		}
		return AdminCache{}, ctx.Err()
	}
	ac := load.result
	if genBefore != adminCacheGeneration.Load() {
		// An invalidation landed while we waited on the shared load: the value
		// predates a demotion/promote. Re-read once; on miss return empty so
		// callers fall back to per-user GetChatMember instead of trusting stale.
		if ok, fresh := GetAdminCacheList(chatId); ok {
			return fresh, nil
		}
		return AdminCache{}, nil
	}
	return ac, nil
}

func loadAdminCacheFromTelegram(ctx context.Context, b *gotgbot.Bot, chatId int64) AdminCache {
	generation := adminCacheGeneration.Load()
	const botStatusErrorAdminCacheTTL = 30 * time.Second

	storeWithTTL := func(adminCache AdminCache, ttl time.Duration) AdminCache {
		if m := GetMarshal(); m != nil && generation == adminCacheGeneration.Load() {
			if err := m.Set(Context, adminCacheKey(chatId), adminCache, store.WithExpiration(ttl)); err != nil {
				log.WithFields(log.Fields{
					"chatId": chatId,
					"error":  err,
				}).Error("LoadAdminCache: Failed to cache admin list")
			} else if generation != adminCacheGeneration.Load() {
				InvalidateAdminCache(chatId)
			}
		}
		return adminCache
	}
	storeResult := func(ac AdminCache) AdminCache {
		return storeWithTTL(ac, constants.AdminCacheTTL)
	}
	storeNegativeResult := func(ac AdminCache) AdminCache {
		return storeWithTTL(ac, negativeAdminCacheTTL)
	}

	botCtx, botCancel := context.WithTimeout(ctx, constants.DefaultTimeout)
	botMember, botErr := b.GetChatMemberWithContext(botCtx, chatId, b.Id, nil)
	botCancel()
	if botErr != nil {
		if !error_handling.IsMissingChatMemberError(botErr) {
			log.WithContext(ctx).WithFields(log.Fields{
				"chatId": chatId,
				"botId":  b.Id,
				"error":  botErr,
			}).Warning("LoadAdminCache: Could not verify bot admin status")
		}
		return storeWithTTL(AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		}, botStatusErrorAdminCacheTTL)
	}

	botStatus := botMember.GetStatus()
	if botStatus != "administrator" && botStatus != "creator" {
		return storeNegativeResult(AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
			Negative: true,
		})
	}

	log.WithFields(log.Fields{
		"chatId":    chatId,
		"botId":     b.Id,
		"botStatus": botStatus,
	}).Debug("LoadAdminCache: Bot has admin privileges")

	maxRetries := 3
	var adminList []gotgbot.ChatMember
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		attemptCtx, attemptCancel := context.WithTimeout(ctx, constants.DefaultTimeout)
		adminList, err = b.GetChatAdministratorsWithContext(
			attemptCtx,
			chatId,
			&gotgbot.GetChatAdministratorsOpts{ReturnBots: true},
		)
		attemptCancel()
		if err != nil {
			log.WithFields(log.Fields{
				"chatId":    chatId,
				"error":     err,
				"attempt":   attempt + 1,
				"errorType": fmt.Sprintf("%T", err),
			}).Warning("LoadAdminCache: Failed to get chat administrators, retrying...")

			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}

			log.WithFields(log.Fields{
				"chatId": chatId,
				"error":  err,
			}).Error("LoadAdminCache: Failed to get chat administrators after all retries")
			return AdminCache{}
		}
		break
	}

	if len(adminList) == 0 {
		log.WithFields(log.Fields{
			"chatId": chatId,
		}).Warning("LoadAdminCache: No administrators found - this is unusual for a valid group")
		return storeResult(AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		})
	}

	// A chat has at most 50 admins, so readers scan the list; a lookup map
	// would double what every read decodes.
	userList := make([]gotgbot.MergedChatMember, 0, len(adminList))
	for _, admin := range adminList {
		userList = append(userList, admin.MergeChatMember())
	}

	adminCache := AdminCache{
		ChatId:   chatId,
		UserInfo: userList,
		Cached:   true,
	}

	return storeResult(adminCache)
}

func GetAdminCacheList(chatId int64) (bool, AdminCache) {
	m := GetMarshal()
	if m == nil {
		return false, AdminCache{}
	}
	gotAdminlist, err := m.Get(
		Context,
		adminCacheKey(chatId),
		new(AdminCache),
	)
	if err != nil {
		log.WithFields(log.Fields{
			"chatId": chatId,
			"error":  err,
		}).Debug("GetAdminCacheList: Cache miss, will attempt fallback")
		return false, AdminCache{}
	}
	if gotAdminlist == nil {
		log.WithFields(log.Fields{
			"chatId": chatId,
		}).Debug("GetAdminCacheList: Cache empty, will attempt fallback")
		return false, AdminCache{}
	}
	return true, *gotAdminlist.(*AdminCache)
}

func GetAdminCacheUser(chatId, userId int64) (bool, gotgbot.MergedChatMember) {
	m := GetMarshal()
	if m == nil {
		return false, gotgbot.MergedChatMember{}
	}
	adminList, err := m.Get(Context, adminCacheKey(chatId), new(AdminCache))
	if err != nil || adminList == nil {
		return false, gotgbot.MergedChatMember{}
	}

	adminCache, ok := adminList.(*AdminCache)
	if !ok || adminCache == nil {
		return false, gotgbot.MergedChatMember{}
	}

	for i := range adminCache.UserInfo {
		admin := &adminCache.UserInfo[i]
		if admin.User.Id == userId {
			return true, *admin
		}
	}
	return false, gotgbot.MergedChatMember{}
}

func InvalidateAdminCache(chatId int64) {
	if err := ClearAdminCache(chatId); err != nil {
		log.Debugf("[AdminCache] Failed to invalidate cache for chat %d: %v", chatId, err)
	}
}

// MarkMemberLookupForbidden records on the chat's Negative admin list that
// Telegram refused a member lookup there. A missing or non-Negative list is
// left alone: the bot's rights changed since it was stored.
func MarkMemberLookupForbidden(chatId int64) {
	m := GetMarshal()
	if m == nil {
		return
	}
	generation := adminCacheGeneration.Load()
	found, adminCache := GetAdminCacheList(chatId)
	if !found || !adminCache.Negative || adminCache.MemberLookupForbidden {
		return
	}
	adminCache.MemberLookupForbidden = true
	if err := m.Set(Context, adminCacheKey(chatId), adminCache, store.WithExpiration(negativeAdminCacheTTL)); err != nil {
		log.WithFields(log.Fields{"chatId": chatId, "error": err}).Debug("[AdminCache] Failed to mark member lookups forbidden")
		return
	}
	if generation != adminCacheGeneration.Load() {
		InvalidateAdminCache(chatId)
	}
}

// ClearAdminCache invalidates pending lookups and removes stored permissions.
func ClearAdminCache(chatId int64) error {
	adminCacheGeneration.Add(1)
	adminLoadsMu.Lock()
	delete(adminLoads, adminCacheKey(chatId))
	adminLoadsMu.Unlock()
	if m := GetMarshal(); m != nil {
		return m.Delete(Context, adminCacheKey(chatId))
	}
	return nil
}
