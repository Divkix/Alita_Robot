package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
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
	go func() {
		defer error_handling.RecoverFromPanic("cleanupLoop", "antiflood")
		antifloodModule.cleanupLoop(context.Background())
	}()
}

// cleanupLoop periodically cleans up old entries from the flood cache
// cleanupLoop periodically removes old flood control entries from memory.
// Runs every 5 minutes to clean entries older than 10 minutes.
// Accepts a context for graceful shutdown.
func (a *antifloodStruct) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentTime := time.Now().Unix()
			a.syncHelperMap.Range(func(key, value any) bool {
				if floodData, ok := value.(floodControl); ok {
					// Remove entries older than 10 minutes
					if currentTime-floodData.lastActivity > 600 {
						a.syncHelperMap.Delete(key)
					}
				}
				return true
			})
		case <-ctx.Done():
			log.Info("Antiflood cleanup goroutine shutting down gracefully")
			return
		}
	}
}

// updateFlood tracks message counts per user and determines if flood limit exceeded.
// Returns true if user has exceeded flood limit and should be restricted,
// along with the flood control data and flood settings from the database.
// This eliminates redundant database calls by fetching settings once.
func (a *antifloodStruct) updateFlood(chatId, userId, msgId int64) (shouldPunish bool, floodCrc floodControl, floodSettings *db.AntifloodSettings) {
	floodSettings = db.GetFlood(chatId)

	if floodSettings.Limit != 0 {
		currentTime := time.Now().Unix()

		// Use type-safe struct key instead of string concatenation
		key := floodKey{chatId: chatId, userId: userId}
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

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	var (
		fmode    string
		keyboard [][]gotgbot.InlineKeyboardButton
	)
	userId := user.Id()
	chatId := chat.Id

	// Use semaphore to limit concurrent admin checks and add timeout
	select {
	case antifloodModule.adminCheckSemaphore <- struct{}{}:
		// Got semaphore, proceed with admin check
		defer func() { <-antifloodModule.adminCheckSemaphore }()

		// Create context with timeout for admin check
		ctx_timeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Check if user is admin with timeout and proper goroutine cleanup
		isAdmin := make(chan bool, 1)
		done := make(chan struct{})

		go func() {
			defer func() {
				close(done) // Signal completion to prevent goroutine leak
			}()
			defer error_handling.RecoverFromPanic("adminCheck", "antiflood")

			select {
			case isAdmin <- chat_status.IsUserAdmin(b, chatId, userId):
				// Successfully sent result
			case <-ctx_timeout.Done():
				// Context cancelled, exit goroutine early
				return
			}
		}()

		select {
		case admin := <-isAdmin:
			if admin {
				// Admins are exempt from flood tracking
				return ext.ContinueGroups
			}
		case <-ctx_timeout.Done():
			// Admin check timed out, fail open to prevent false positives
			// It's better to occasionally miss a flood from an admin than to ban actual admins on timeout
			log.WithFields(log.Fields{
				"chatId": chatId,
				"userId": userId,
			}).Warn("Admin check timed out, skipping flood check to prevent false positives")

			// Wait for goroutine cleanup with timeout to prevent indefinite blocking
			select {
			case <-done:
				// Goroutine completed cleanly
			case <-time.After(1 * time.Second):
				// Log if goroutine takes too long to cleanup
				log.WithFields(log.Fields{
					"chatId": chatId,
					"userId": userId,
				}).Warn("Admin check goroutine cleanup timeout")
			}

			// Skip flood check on timeout - fail open like semaphore full case
			return ext.ContinueGroups
		}
	default:
		// CRITICAL FIX: Semaphore full - fail open to prevent false positives
		// It's better to occasionally miss a flood from an admin than to ban actual admins under load
		log.WithFields(log.Fields{
			"chatId": chatId,
			"userId": userId,
		}).Warn("Admin check semaphore full - assuming admin to prevent false positives")
		return ext.ContinueGroups
	}

	// PERFORMANCE FIX: Update flood and get settings in one call to eliminate redundant DB query
	// Previously this was calling db.GetFlood again after updateFlood, doubling the DB load
	flooded, floodCrc, flood := antifloodModule.updateFlood(chatId, userId, msg.MessageId)
	if !flooded {
		return ext.ContinueGroups
	}

	// No need to call db.GetFlood again - we already have the settings from updateFlood
	if flood.Action == "mute" || flood.Action == "kick" || flood.Action == "ban" {
		if !chat_status.CanBotRestrict(b, ctx, chat, true) {
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

					err := helpers.DeleteMessageWithErrorHandling(b, chatId, messageId)
					recordError(err, messageId)
				}(msgId)
			}

			wg.Wait()
		}

		if firstError != nil {
			return firstError
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
					Text:         func() string { t, _ := tr.GetString("button_unmute_admins"); return t }(),
					CallbackData: fmt.Sprintf("unrestrict.unmute.%d", user.Id()),
				},
			},
		}

		_, err := chat.RestrictMember(b, userId,
			gotgbot.ChatPermissions{
				CanSendMessages:       false,
				CanSendPhotos:         false,
				CanSendVideos:         false,
				CanSendAudios:         false,
				CanSendDocuments:      false,
				CanSendVideoNotes:     false,
				CanSendVoiceNotes:     false,
				CanAddWebPagePreviews: false,
				CanChangeInfo:         false,
				CanInviteUsers:        false,
				CanPinMessages:        false,
				CanManageTopics:       false,
				CanSendPolls:          false,
				CanSendOtherMessages:  false,
			},
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
		_, err := chat.BanMember(b, userId, nil)
		if err != nil {
			log.Errorf(" checkFlood: %d (%d) - %v", chatId, user.Id(), err)
			return err
		}
		// Use non-blocking delayed unban for kick action
		go func() {
			defer error_handling.RecoverFromPanic("delayedUnban", "antiflood")

			time.Sleep(3 * time.Second)

			_, unbanErr := chat.UnbanMember(b, userId, nil)
			if unbanErr != nil {
				log.WithFields(log.Fields{
					"chatId": chatId,
					"userId": userId,
					"error":  unbanErr,
				}).Error("Failed to unban user after antiflood kick")
			}
		}()
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
						Text:         func() string { t, _ := tr.GetString("antiflood_button_unban_admins"); return t }(),
						CallbackData: fmt.Sprintf("unrestrict.unban.%d", user.Id()),
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
			return fmt.Sprintf(temp, helpers.MentionHtml(userId, user.Name()), fmode)
		}(),
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
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
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := ctx.Args()[1:]

	var replyText string

	if len(args) == 0 {
		replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_expected_args")
	} else {
		if string_handling.FindInStringSlice([]string{"off", "no", "false", "0"}, strings.ToLower(args[0])) {
			db.SetFlood(chat.Id, 0)
			replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_setflood_disabled")
		} else {
			num, err := strconv.Atoi(args[0])
			if err != nil {
				replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_invalid_int")
			} else {
				if num < 3 || num > 100 {
					replyText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_errors_set_in_limit")
				} else {
					db.SetFlood(chat.Id, num)
					temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setflood_success")
					replyText = fmt.Sprintf(temp, num)
				}
			}
		}
	}

	_, err := msg.Reply(b, replyText, helpers.Shtml())
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
	connectedChat := helpers.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	flood := db.GetFlood(chat.Id)
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
	_, err := msg.Reply(b, text, helpers.Shtml())
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
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := ctx.Args()[1:]

	if len(args) > 0 {
		selectedMode := strings.ToLower(args[0])
		if string_handling.FindInStringSlice([]string{"ban", "kick", "mute"}, selectedMode) {
			db.SetFloodMode(chat.Id, selectedMode)
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_success")
			_, err := msg.Reply(b, fmt.Sprintf(temp, selectedMode), helpers.Shtml())
			if err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_unknown_type")
			_, err := msg.Reply(b, fmt.Sprintf(temp, args[0]), helpers.Shtml())
			if err != nil {
				return err
			}
		}
	} else {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_setfloodmode_specify_action")
		_, err := msg.Reply(b, text, helpers.Smarkdown())
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
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := ctx.Args()[1:]
	var text string

	if len(args) > 0 {
		selectedMode := strings.ToLower(args[0])
		switch selectedMode {
		case "on", "yes":
			db.SetFloodMsgDel(chat.Id, true)
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_enabled")
		case "off", "no":
			db.SetFloodMsgDel(chat.Id, false)
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_disabled")
		default:
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_invalid_option")
		}
	} else {
		currSet := db.GetFlood(chat.Id).DeleteAntifloodMessage
		if currSet {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_already_enabled")
		} else {
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_already_disabled")
		}
	}
	_, err := msg.Reply(b, text, helpers.Smarkdown())
	if err != nil {
		return err
	}

	return ext.EndGroups
}

// LoadAntiflood registers all antiflood module handlers with the dispatcher.
// Sets up flood detection commands and message monitoring handlers.
func LoadAntiflood(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(antifloodModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("setflood", antifloodModule.setFlood))
	dispatcher.AddHandler(handlers.NewCommand("setfloodmode", antifloodModule.setFloodMode))
	dispatcher.AddHandler(handlers.NewCommand("delflood", antifloodModule.setFloodDeleter))
	dispatcher.AddHandler(handlers.NewCommand("flood", antifloodModule.flood))
	misc.AddCmdToDisableable("flood")
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, antifloodModule.checkFlood), antifloodModule.handlerGroup)
}
