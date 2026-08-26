package modules

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// Concurrency limits for flood protection operations
const (
	maxConcurrentAdminChecks  = 50 // Maximum concurrent admin permission checks
	maxConcurrentMsgDeletions = 5  // Maximum concurrent message deletions during flood cleanup
)

// floodKey is a type-safe composite key for flood tracking
// Uses struct instead of string concatenation to avoid collisions
type floodKey struct {
	chatId int64
	userId int64
}

type antifloodStruct struct {
	moduleStruct  // inheritance
	syncHelperMap sync.Map
	// Add semaphore to limit concurrent admin checks
	adminCheckSemaphore chan struct{}
}

type floodControl struct {
	userId       int64
	messageCount int
	messageIDs   []int64
	lastActivity int64 // Unix timestamp for cleanup
}

// floodMu stores per-key *sync.Mutex values to protect the RMW cycle in updateFlood.
var floodMu sync.Map

var _normalAntifloodModule = moduleStruct{
	moduleName:   "Antiflood",
	handlerGroup: 4,
}

var antifloodModule = antifloodStruct{
	moduleStruct:        _normalAntifloodModule,
	syncHelperMap:       sync.Map{},
	adminCheckSemaphore: make(chan struct{}, maxConcurrentAdminChecks),
}

// init starts cleanup goroutine for antiflood cache
func init() {
	RegisterLegacyModule("Antiflood", 150, LoadAntiflood)
	go func() {
		defer error_handling.RecoverFromPanic("cleanupLoop", "antiflood")
		antifloodModule.cleanupLoop(context.Background())
	}()
}

// cleanupOnce performs a single cleanup pass, removing entries idle for more
// than 600 seconds (10 minutes) from syncHelperMap and floodMu.
// It is called by cleanupLoop on each ticker tick and is also directly
// callable in tests for deterministic verification.
func (a *antifloodStruct) cleanupOnce(now int64) {
	a.syncHelperMap.Range(func(key, value any) bool {
		floodData, ok := value.(floodControl)
		if !ok || now-floodData.lastActivity <= 600 {
			return true
		}
		// Acquire per-key mutex before deleting to avoid racing with updateFlood's RMW cycle.
		// If the mutex is busy, skip this key and retry next tick.
		if muVal, hasMu := floodMu.Load(key); hasMu {
			if mu, ok := muVal.(*sync.Mutex); ok {
				if !mu.TryLock() {
					return true
				}
				// Re-validate after acquiring lock - entry may have been refreshed.
				if cur, ok := a.syncHelperMap.Load(key); ok {
					if curFC, ok := cur.(floodControl); ok && now-curFC.lastActivity <= 600 {
						mu.Unlock()
						return true
					}
				} else {
					// Already deleted by concurrent writer
					floodMu.Delete(key)
					mu.Unlock()
					return true
				}
				a.syncHelperMap.Delete(key)
				floodMu.Delete(key)
				mu.Unlock()
				return true
			}
			// Unexpected type - delete the entry
			floodMu.Delete(key)
		}
		a.syncHelperMap.Delete(key)
		return true
	})
}

// cleanupLoop periodically removes old flood control entries from memory.
// Runs every 5 minutes to clean entries older than 10 minutes.
// Accepts a context for graceful shutdown.
func (a *antifloodStruct) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				defer error_handling.RecoverFromPanic("cleanupOnce", "antiflood")
				a.cleanupOnce(time.Now().Unix())
			}()
		case <-ctx.Done():
			log.Info("Antiflood cleanup goroutine shutting down gracefully")
			return
		}
	}
}

// cachedAdminStatus reports whether a warm admin cache already knows this user.
// known=false means the cache missed and the caller must ask Telegram.
func cachedAdminStatus(chatId, userId int64) (known bool, isAdmin bool) {
	ok, cached := cache.GetAdminCacheList(chatId)
	if !ok || !cached.Cached {
		return false, false
	}
	if cached.UserMap != nil {
		_, isAdmin = cached.UserMap[userId]
		return true, isAdmin
	}
	for i := range cached.UserInfo {
		if cached.UserInfo[i].User.Id == userId {
			return true, true
		}
	}
	return true, false
}

// userIsFloodExempt is true when the sender should skip flood tracking.
// A warm admin cache (including negative lookups) is trusted so non-admins do
// not take a semaphore slot or spawn an IsUserAdmin goroutine per message.
// Cache misses use a bounded Telegram check; timeout and a full semaphore
// fail open. The semaphore is released before flood tracking or punishment.
func (a *antifloodStruct) userIsFloodExempt(b *gotgbot.Bot, chatId, userId int64) bool {
	if known, isAdmin := cachedAdminStatus(chatId, userId); known {
		return isAdmin
	}
	return a.adminCheckWithTimeout(b, chatId, userId)
}

func (a *antifloodStruct) adminCheckWithTimeout(b *gotgbot.Bot, chatId, userId int64) bool {
	select {
	case a.adminCheckSemaphore <- struct{}{}:
		defer func() { <-a.adminCheckSemaphore }()
	default:
		log.WithFields(log.Fields{
			"chatId": chatId,
			"userId": userId,
		}).Warn("Admin check semaphore full - assuming admin to prevent false positives")
		return true
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := make(chan bool, 1)
	go func() {
		defer error_handling.RecoverFromPanic("adminCheck", "antiflood")
		isAdmin := chat_status.IsUserAdmin(b, chatId, userId)
		select {
		case result <- isAdmin:
		case <-ctxTimeout.Done():
		}
	}()

	select {
	case isAdmin := <-result:
		return isAdmin
	case <-ctxTimeout.Done():
		log.WithFields(log.Fields{
			"chatId": chatId,
			"userId": userId,
		}).Warn("Admin check timed out, skipping flood check to prevent false positives")
		return true
	}
}

// updateFlood tracks message counts per user and determines if flood limit exceeded.
// Returns true if user has exceeded flood limit and should be restricted,
// along with the flood control data and flood settings from the database.
// This eliminates redundant database calls by fetching settings once.
func (a *antifloodStruct) updateFlood(chatId, userId, msgId int64) (shouldPunish bool, floodCrc floodControl, floodSettings *db.AntifloodSettings) {
	floodSettings = antiflood.GetFlood(chatId)

	if floodSettings.Limit != 0 {
		currentTime := time.Now().Unix()

		// Use type-safe struct key instead of string concatenation
		key := floodKey{chatId: chatId, userId: userId}

		// Acquire per-key mutex to protect the Load → mutate → Store RMW cycle
		muVal, _ := floodMu.LoadOrStore(key, &sync.Mutex{})
		mu := muVal.(*sync.Mutex)
		mu.Lock()
		defer mu.Unlock()

		tmpInterface, valExists := a.syncHelperMap.Load(key)
		if valExists && tmpInterface != nil {
			floodCrc = tmpInterface.(floodControl)

			// Clean up old entries (older than 1 minute)
			if currentTime-floodCrc.lastActivity > 60 {
				floodCrc = floodControl{}
			}
		}

		// No need to check userId mismatch since key includes userId
		if floodCrc.userId == 0 {
			floodCrc.userId = userId
			floodCrc.messageCount = 0
			floodCrc.messageIDs = make([]int64, 0, floodSettings.Limit+5) // Pre-allocate with capacity
		}

		floodCrc.messageCount++
		floodCrc.lastActivity = currentTime

		// PERFORMANCE FIX: Append to end instead of prepending
		// This avoids slice reallocation and copying on every message (O(1) amortized vs O(n))
		floodCrc.messageIDs = append(floodCrc.messageIDs, msgId)

		// Trim old messages if we exceed the limit
		// Keep only the most recent messages within the flood window
		if len(floodCrc.messageIDs) > floodSettings.Limit+5 {
			// Slice from the end to keep recent messages
			// This is O(1) operation since it just adjusts the slice header
			floodCrc.messageIDs = floodCrc.messageIDs[len(floodCrc.messageIDs)-(floodSettings.Limit+5):]
		}

		if floodCrc.messageCount > floodSettings.Limit {
			a.syncHelperMap.Store(key,
				floodControl{
					userId:       0,
					messageCount: 0,
					messageIDs:   make([]int64, 0),
					lastActivity: currentTime,
				},
			)
			shouldPunish = true
		} else {
			a.syncHelperMap.Store(key, floodCrc)
		}
	}

	return
}

// checkFlood monitors incoming messages for flood violations.
// Applies configured flood actions (mute/kick/ban) when limits are exceeded.
func (m *moduleStruct) checkFlood(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender
	if user == nil {
		return ext.ContinueGroups
	}
	if user.IsAnonymousAdmin() {
		return ext.ContinueGroups
	}
	msg := ctx.EffectiveMessage
	if msg.MediaGroupId != "" {
		return ext.ContinueGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var (
		fmode    string
		keyboard [][]gotgbot.InlineKeyboardButton
	)
	userId := user.Id()
	chatId := chat.Id

	if antifloodModule.userIsFloodExempt(b, chatId, userId) {
		return ext.ContinueGroups
	}

	// Check if user is approved (immune to anti-spam)
	if chat_status.IsApproved(b, chatId, userId) {
		return ext.ContinueGroups
	}

	// PERFORMANCE FIX: Update flood and get settings in one call to eliminate redundant DB query
	// Previously this was calling antiflood.GetFlood again after updateFlood, doubling the DB load
	flooded, floodCrc, flood := antifloodModule.updateFlood(chatId, userId, msg.MessageId)
	if !flooded {
		return ext.ContinueGroups
	}

	// No need to call antiflood.GetFlood again - we already have the settings from updateFlood
	if flood.Action == "mute" || flood.Action == "kick" || flood.Action == "ban" {
		if !chat_status.CanBotRestrict(b, ctx, chat) {
			log.WithFields(log.Fields{
				"chatId": chatId,
			}).Warn("Antiflood action skipped: bot lacks restrict permissions")
			return ext.ContinueGroups
		}
	}

	if flood.DeleteAntifloodMessage {
		var firstError error
		var errorMu sync.Mutex

		recordError := func(err error, msgId int64) {
			if err != nil {
				log.Errorf("Failed to delete flood message %d: %v", msgId, err)
				errorMu.Lock()
				if firstError == nil {
					firstError = err
				}
				errorMu.Unlock()
			}
		}

		if len(floodCrc.messageIDs) <= 3 {
			// Sequential deletion - continue on error
			for _, i := range floodCrc.messageIDs {
				err := helpers.DeleteMessageWithErrorHandling(b, chatId, i)
				recordError(err, i)
			}
		} else {
			// Concurrent deletion with rate limiting
			sem := make(chan struct{}, maxConcurrentMsgDeletions)
			var wg sync.WaitGroup

			for _, msgId := range floodCrc.messageIDs {
				wg.Add(1)
				sem <- struct{}{}

				go func(messageId int64) {
					defer wg.Done()
					defer func() { <-sem }()
					defer error_handling.RecoverFromPanic("floodMsgDelete", "antiflood")

					err := helpers.DeleteMessageWithErrorHandling(b, chatId, messageId)
					recordError(err, messageId)
				}(msgId)
			}

			wg.Wait()
		}

		if firstError != nil {
			log.Warnf("[Antiflood] Some flood messages could not be deleted (may be too old): %v", firstError)
		}
	} else {
		_ = helpers.DeleteMessageWithErrorHandling(b, chatId, msg.MessageId)
	}

	switch flood.Action {
	case "mute":
		// don't work on anonymous channels
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}
		fmode = "muted"
		keyboard = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         trS(tr, "button_unmute_admins"),
					CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(user.Id())}),
				},
			},
		}

		_, err := chat.RestrictMember(b, userId,
			MutedPermissions,
			nil,
		)
		if err != nil {
			log.Errorf(" checkFlood: %d (%d) - %v", chatId, user.Id(), err)
			return err
		}
	case "kick":
		// don't work on anonymous channels
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}
		fmode = "kicked"
		keyboard = nil
		if err := kickMember(b, chat.Id, userId); err != nil {
			log.Errorf(" checkFlood: %d (%d) - %v", chatId, user.Id(), err)
			return err
		}
	case "ban":
		fmode = "banned"
		if !user.IsAnonymousChannel() {
			_, err := chat.BanMember(b, userId, nil)
			if err != nil {
				log.Errorf(" checkFlood: %d (%d) - %v", chatId, user.Id(), err)
				return err
			}
		} else {
			keyboard = [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         trS(tr, "antiflood_button_unban_admins"),
						CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(user.Id())}),
					},
				},
			}
			_, err := chat.BanSenderChat(b, userId, nil)
			if err != nil {
				log.Errorf(" checkFlood: %d (%d) - %v", chatId, user.Id(), err)
				return err
			}
		}
	}
	if _, err := helpers.SendMessageWithErrorHandling(b, chatId,
		func() string {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_checkflood_perform_action")
			return fmt.Sprintf(temp, formatting.MentionHtml(userId, user.Name()), fmode)
		}(),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: keyboard,
			},
			MessageThreadId: msg.MessageThreadId,
		},
	); err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

// setFlood handles the /setflood command to configure flood detection limits.
// Sets the maximum number of messages allowed before triggering flood protection.
func (m *moduleStruct) setFlood(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	args := ctx.Args()[1:]

	var replyText string

	if len(args) == 0 {
		replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_expected_args")
	} else {
		if slices.Contains([]string{"off", "no", "false", "0"}, strings.ToLower(args[0])) {
			if err := antiflood.SetFlood(chat.Id, 0); err != nil {
				log.Errorf("[Antiflood] SetFlood failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_setflood_disabled")
		} else {
			num, err := strconv.Atoi(args[0])
			if err != nil {
				replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_invalid_int")
			} else {
				if num < 3 || num > 100 {
					replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_set_in_limit")
				} else {
					if err := antiflood.SetFlood(chat.Id, num); err != nil {
						log.Errorf("[Antiflood] SetFlood failed for chat %d: %v", chat.Id, err)
						errText, _ := tr.GetString("common_settings_save_failed")
						_, _ = msg.Reply(b, errText, formatting.Shtml())
						return ext.EndGroups
					}
					temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setflood_success")
					replyText = fmt.Sprintf(temp, num)
				}
			}
		}
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// flood handles the /flood command to display current flood protection settings.
// Shows the flood limit and action (mute/kick/ban) for the chat.
func (m *moduleStruct) flood(b *gotgbot.Bot, ctx *ext.Context) error {
	var text string
	msg := ctx.EffectiveMessage

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "flood") {
		return ext.EndGroups
	}
	// connection status
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	flood := antiflood.GetFlood(chat.Id)
	if flood.Limit == 0 {
		text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_disabled")
	} else {
		var mode string
		switch flood.Action {
		case "mute":
			mode = "muted"
		case "ban":
			mode = "banned"
		case "kick":
			mode = "kicked"
		}
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_flood_show_settings")
		text = fmt.Sprintf(temp, flood.Limit, mode)
	}
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

// setFloodMode handles the /setfloodmode command to configure flood protection actions.
// Allows setting the punishment type (ban/kick/mute) for flood violations.
func (m *moduleStruct) setFloodMode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	args := ctx.Args()[1:]

	if len(args) > 0 {
		selectedMode := strings.ToLower(args[0])
		if slices.Contains([]string{"ban", "kick", "mute"}, selectedMode) {
			if err := antiflood.SetFloodMode(chat.Id, selectedMode); err != nil {
				log.Errorf("[Antiflood] SetFloodMode failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_success")
			_, err := msg.Reply(b, fmt.Sprintf(temp, selectedMode), formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_unknown_type")
			_, err := msg.Reply(b, fmt.Sprintf(temp, args[0]), formatting.Shtml())
			if err != nil {
				return err
			}
		}
	} else {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_specify_action")
		_, err := msg.Reply(b, text, formatting.Smarkdown())
		if err != nil {
			return err
		}
	}
	return ext.EndGroups
}

// setFloodDeleter handles the /delflood command to toggle message deletion on flood.
// Configures whether to delete all flood messages or just the triggering message.
func (m *moduleStruct) setFloodDeleter(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	args := ctx.Args()[1:]
	var text string

	if len(args) > 0 {
		selectedMode := strings.ToLower(args[0])
		switch selectedMode {
		case "on", "yes":
			if err := antiflood.SetFloodMsgDel(chat.Id, true); err != nil {
				log.Errorf("[Antiflood] SetFloodMsgDel failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_enabled")
		case "off", "no":
			if err := antiflood.SetFloodMsgDel(chat.Id, false); err != nil {
				log.Errorf("[Antiflood] SetFloodMsgDel failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_disabled")
		default:
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_invalid_option")
		}
	} else {
		currSet := antiflood.GetFlood(chat.Id).DeleteAntifloodMessage
		if currSet {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_already_enabled")
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_already_disabled")
		}
	}
	_, err := msg.Reply(b, text, formatting.Smarkdown())
	if err != nil {
		return err
	}

	return ext.EndGroups
}

// LoadAntiflood registers all antiflood module handlers with the dispatcher.
// Sets up flood detection commands and message monitoring handlers.
func LoadAntiflood(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[antifloodModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("setflood", antifloodModule.setFlood))
	dispatcher.AddHandler(handlers.NewCommand("setfloodmode", antifloodModule.setFloodMode))
	dispatcher.AddHandler(handlers.NewCommand("delflood", antifloodModule.setFloodDeleter))
	dispatcher.AddHandler(handlers.NewCommand("flood", antifloodModule.flood))
	helpers.AddCmdToDisableable("flood")
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, antifloodModule.checkFlood), antifloodModule.handlerGroup)
}
