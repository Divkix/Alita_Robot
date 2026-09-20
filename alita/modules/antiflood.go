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
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const (
	maxConcurrentAdminChecks  = 50
	maxConcurrentMsgDeletions = 5
)

type floodKey struct {
	chatId int64
	userId int64
}

type antifloodStruct struct {
	moduleStruct
	syncHelperMap       sync.Map
	adminCheckSemaphore chan struct{}
}

type floodControl struct {
	userId       int64
	messageCount int
	messageIDs   []int64
	lastActivity int64
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

func init() {
	RegisterLegacyModule("Antiflood", 150, LoadAntiflood)
	go func() {
		defer error_handling.RecoverFromPanic("cleanupLoop", "antiflood")
		antifloodModule.cleanupLoop(context.Background())
	}()
}

func (a *antifloodStruct) cleanupOnce(now int64) {
	a.syncHelperMap.Range(func(key, value any) bool {
		// States are *floodControl so CompareAndDelete compares pointers:
		// values hold slices and are not comparable.
		floodData, ok := value.(*floodControl)
		if !ok || floodData == nil || now-floodData.lastActivity <= 600 {
			return true
		}
		// Delete only if still stale: a concurrent updateFlood CAS-stores a fresh
		// value and CompareAndDelete fails, keeping the new burst intact.
		a.syncHelperMap.CompareAndDelete(key, value)
		return true
	})
}

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

func cachedAdminStatus(chatId, userId int64) (known bool, isAdmin bool) {
	ok, cached := cache.GetAdminCacheList(chatId)
	if !ok || !cached.Cached || cached.Negative || len(cached.UserInfo) == 0 {
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

func (a *antifloodStruct) userIsFloodExempt(b *gotgbot.Bot, chatId, userId int64) bool {
	if known, isAdmin := cachedAdminStatus(chatId, userId); known {
		return isAdmin
	}
	return a.adminCheckWithTimeout(b, chatId, userId)
}

func (a *antifloodStruct) adminCheckWithTimeout(b *gotgbot.Bot, chatId, userId int64) bool {
	select {
	case a.adminCheckSemaphore <- struct{}{}:
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
		// Release the slot when the real work finishes, not when the caller
		// times out: the old defer released early, unbounding live lookups.
		defer func() { <-a.adminCheckSemaphore }()
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

func (a *antifloodStruct) updateFlood(chatId, userId, msgId int64) (shouldPunish bool, floodCrc floodControl, floodSettings *db.AntifloodSettings) {
	floodSettings = antiflood.GetFlood(chatId)
	shouldPunish, floodCrc = a.updateFloodWithSettings(chatId, userId, msgId, floodSettings)
	return
}

// updateFloodWithSettings is updateFlood with the settings row already in hand,
// so callers that loaded it (checkFlood) do not read the cache a second time.
func (a *antifloodStruct) updateFloodWithSettings(chatId, userId, msgId int64, floodSettings *db.AntifloodSettings) (shouldPunish bool, floodCrc floodControl) {
	if floodSettings.Limit != 0 {
		currentTime := time.Now().Unix()
		key := floodKey{chatId: chatId, userId: userId}

		// Lock-free RMW on *floodControl (pointers are comparable for CAS; the
		// old floodMu protocol let the cleaner mint a second mutex → lost counts).
		for {
			var cur floodControl
			old, loaded := a.syncHelperMap.Load(key)
			if loaded && old != nil {
				if prev, ok := old.(*floodControl); ok && prev != nil {
					cur = *prev
					if currentTime-cur.lastActivity > 60 {
						cur = floodControl{}
						loaded = false
					}
				} else {
					loaded = false
				}
			}
			if cur.userId == 0 {
				cur.userId = userId
				cur.messageCount = 0
				cur.messageIDs = make([]int64, 0, floodSettings.Limit+5)
			} else {
				next := make([]int64, len(cur.messageIDs), cap(cur.messageIDs))
				copy(next, cur.messageIDs)
				cur.messageIDs = next
			}
			cur.messageCount++
			cur.lastActivity = currentTime
			cur.messageIDs = append(cur.messageIDs, msgId)
			if len(cur.messageIDs) > floodSettings.Limit+5 {
				cur.messageIDs = cur.messageIDs[len(cur.messageIDs)-(floodSettings.Limit+5):]
			}
			next := cur
			punish := false
			if cur.messageCount > floodSettings.Limit {
				// Reset the stored counter but hand the burst to the caller: it
				// bulk-deletes the tracked IDs (the old reset returned an empty
				// list, deleting nothing).
				next = floodControl{lastActivity: currentTime, messageIDs: make([]int64, 0)}
				punish = true
			}
			stored := &floodControl{
				userId:       next.userId,
				messageCount: next.messageCount,
				messageIDs:   next.messageIDs,
				lastActivity: next.lastActivity,
			}
			var swapped bool
			if !loaded {
				_, swapped = a.syncHelperMap.LoadOrStore(key, stored)
				swapped = !swapped
			} else {
				swapped = a.syncHelperMap.CompareAndSwap(key, old, stored)
			}
			if swapped {
				shouldPunish = punish
				floodCrc = cur
				break
			}
		}
	}

	return
}

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

	chatId := chat.Id
	// Cheapest discriminator first: an unconfigured chat (Limit == 0) pays only
	// this one cached read, not the translator/admin/approval lookups below.
	flood := antiflood.GetFlood(chatId)
	if flood.Limit == 0 {
		return ext.ContinueGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var (
		fmode    string
		keyboard [][]gotgbot.InlineKeyboardButton
	)
	userId := user.Id()

	if antifloodModule.userIsFloodExempt(b, chatId, userId) {
		return ext.ContinueGroups
	}

	if chat_status.IsApproved(b, chatId, userId) {
		return ext.ContinueGroups
	}

	flooded, floodCrc := antifloodModule.updateFloodWithSettings(chatId, userId, msg.MessageId, flood)
	if !flooded {
		return ext.ContinueGroups
	}

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
			for _, i := range floodCrc.messageIDs {
				err := helpers.DeleteMessageWithErrorHandling(b, chatId, i)
				recordError(err, i)
			}
		} else {
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

func (m *moduleStruct) setFlood(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
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
			if err := antiflood.SetFloodContext(tracing.UpdateContext(ctx), chat.Id, 0); err != nil {
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
					if err := antiflood.SetFloodContext(tracing.UpdateContext(ctx), chat.Id, num); err != nil {
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

func (m *moduleStruct) flood(b *gotgbot.Bot, ctx *ext.Context) error {
	var text string
	msg := ctx.EffectiveMessage

	if chat_status.CheckDisabledCmd(b, msg, "flood") {
		return ext.EndGroups
	}
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	flood := antiflood.GetFloodContext(tracing.UpdateContext(ctx), chat.Id)
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

func (m *moduleStruct) setFloodMode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
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
			if err := antiflood.SetFloodModeContext(tracing.UpdateContext(ctx), chat.Id, selectedMode); err != nil {
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

func (m *moduleStruct) setFloodDeleter(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
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
			if err := antiflood.SetFloodMsgDelContext(tracing.UpdateContext(ctx), chat.Id, true); err != nil {
				log.Errorf("[Antiflood] SetFloodMsgDel failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_flood_deleter_enabled")
		case "off", "no":
			if err := antiflood.SetFloodMsgDelContext(tracing.UpdateContext(ctx), chat.Id, false); err != nil {
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
		currSet := antiflood.GetFloodContext(tracing.UpdateContext(ctx), chat.Id).DeleteAntifloodMessage
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

func LoadAntiflood(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(antifloodModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("setflood", antifloodModule.setFlood))
	dispatcher.AddHandler(handlers.NewCommand("setfloodmode", antifloodModule.setFloodMode))
	dispatcher.AddHandler(handlers.NewCommand("delflood", antifloodModule.setFloodDeleter))
	dispatcher.AddHandler(handlers.NewCommand("clearflood", antifloodModule.setFloodDeleter))
	dispatcher.AddHandler(handlers.NewCommand("flood", antifloodModule.flood))
	helpers.AddCmdToDisableable("flood")
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, antifloodModule.checkFlood), antifloodModule.handlerGroup)
}
