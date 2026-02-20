package modules

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

var (
	locksModule = moduleStruct{
		moduleName:        "Locks",
		permHandlerGroup:  5,
		restrHandlerGroup: 6,
	}
	arabmatch, _                 = regexp.Compile("[\u0600-\u06FF]") // the regex detects the arabic language
	GIF          filters.Message = message.Animation
	OTHER        filters.Message = func(msg *gotgbot.Message) bool {
		return msg.Game != nil || msg.Sticker != nil || message.Animation(msg)
	}
	MEDIA filters.Message = func(msg *gotgbot.Message) bool {
		return msg.Audio != nil || msg.Document != nil || msg.VideoNote != nil || msg.Video != nil || msg.Voice != nil || msg.Photo != nil
	}
	MESSAGES filters.Message = func(msg *gotgbot.Message) bool {
		return msg.Text != "" || msg.Contact != nil || msg.Location != nil || msg.Venue != nil || MEDIA(msg) || OTHER(msg)
	}
	PREVIEW filters.Message = func(msg *gotgbot.Message) bool {
		// Check message entities for URL previews
		for _, s := range msg.Entities {
			if s.Url != "" {
				return true
			}
		}
		// Also check caption entities for media with URL previews
		for _, s := range msg.CaptionEntities {
			if s.Url != "" {
				return true
			}
		}
		return false
	}

	lockMap = map[string]filters.Message{
		"sticker": message.Sticker,
		"audio":   message.Audio,
		"voice":   message.Voice,
		"document": func(msg *gotgbot.Message) bool {
			return msg.Document != nil && msg.Animation == nil
		},
		"video":     message.Video,
		"videonote": message.VideoNote,
		"contact":   message.Contact,
		"photo":     message.Photo,
		"gif":       message.Animation,
		"url":       message.Entity("url"),
		"bots":      message.NewChatMembers,
		"forward":   message.Forwarded,
		"game":      message.Game,
		"location":  message.Location,
		"rtl": func(msg *gotgbot.Message) bool {
			return arabmatch.MatchString(msg.Text)
		},
		"anonchannel": func(msg *gotgbot.Message) bool {
			sender := msg.GetSender()
			// Block messages from anonymous channels OR linked channels (channel posts forwarded to discussion)
			return sender.IsAnonymousChannel() || sender.IsLinkedChannel()
		},
	}

	restrMap = map[string]filters.Message{
		"messages": MESSAGES,
		"comments": MESSAGES,
		"media":    MEDIA,
		"other":    OTHER,
		"previews": PREVIEW,
		"all":      message.All,
	}
)

// getLockMapAsArray returns a sorted array of all available lock types
// by combining restriction types and permission lock types.
func (moduleStruct) getLockMapAsArray() (lockTypes []string) {
	tmpMap := map[string]filters.Message{}

	for r, rk := range restrMap {
		tmpMap[r] = rk
	}
	for l, lk := range lockMap {
		tmpMap[l] = lk
	}

	lockTypes = make([]string, 0, len(tmpMap))

	for k := range tmpMap {
		lockTypes = append(lockTypes, k)
	}
	slices.Sort(lockTypes)
	return
}

// buildLockTypesMessage constructs a formatted string showing all locks
// currently enabled in the specified chat.
func (moduleStruct) buildLockTypesMessage(chatID int64) (res string) {
	chatLocks := db.GetChatLocks(chatID)

	newMapLocks := chatLocks
	tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: &gotgbot.Chat{Id: chatID}}))
	res, _ = tr.GetString("locks_current_locks_header")

	keys := make([]string, 0, len(newMapLocks))
	for k := range newMapLocks {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "\n - %s = %v", k, newMapLocks[k])
	}
	res += sb.String()

	return
}

// locktypes handles the /locktypes command by displaying all available
// lock types that can be used in the chat.
func (m moduleStruct) locktypes(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "locktypes") {
		return ext.EndGroups
	}
	// connection status - also sets ctx.EffectiveChat
	if helpers.IsUserConnected(b, ctx, false, true) == nil {
		return ext.EndGroups
	}
	_locktypes := m.getLockMapAsArray()

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	header, _ := tr.GetString("locks_locktypes_header")
	_, err := msg.Reply(b, header+strings.Join(_locktypes, "\n - "), helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// locks handles the /locks command by showing all currently enabled
// locks in the chat with their status.
func (m moduleStruct) locks(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "locks") {
		return ext.EndGroups
	}
	// connection status - also sets ctx.EffectiveChat
	if helpers.IsUserConnected(b, ctx, true, true) == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat

	_, err := msg.Reply(b, m.buildLockTypesMessage(chat.Id), helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// lockPerm handles the /lock command to enable specific lock types
// in the chat, requiring admin permissions.
func (m moduleStruct) lockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status - also sets ctx.EffectiveChat
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	// Get sender for admin check
	sender := ctx.EffectiveSender
	if sender == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, sender.Id(), false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("locks_what_to_lock")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Validate all lock types first
	var toLock []string
	for _, perm := range args {
		if !string_handling.FindInStringSlice(m.getLockMapAsArray(), perm) {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("locks_invalid_lock_type")
			text := fmt.Sprintf(temp, perm)
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		toLock = append(toLock, perm)
	}

	// Update locks synchronously to ensure success before sending confirmation
	var failedLocks []string
	for _, perm := range toLock {
		if err := db.UpdateLock(chat.Id, perm, true); err != nil {
			log.Warnf("[Locks] Failed to lock %s in chat %d: %v", perm, chat.Id, err)
			failedLocks = append(failedLocks, perm)
		}
	}

	// Send appropriate response based on success/failure
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if len(failedLocks) > 0 {
		// Some locks failed
		text, _ := tr.GetString("locks_lock_failed")
		_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(failedLocks, ", ")), helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		// All locks succeeded
		temp, _ := tr.GetString("locks_locked_successfully")
		text := fmt.Sprintf(temp, strings.Join(toLock, "\n - "))
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

// unlockPerm handles the /unlock command to disable specific lock types
// in the chat, requiring admin permissions.
func (m moduleStruct) unlockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status - also sets ctx.EffectiveChat
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	// Get sender for admin check
	sender := ctx.EffectiveSender
	if sender == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, sender.Id(), false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("locks_what_to_unlock")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Validate all lock types first
	var toUnlock []string
	for _, perm := range args {
		if !string_handling.FindInStringSlice(m.getLockMapAsArray(), perm) {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("locks_invalid_lock_type")
			text := fmt.Sprintf(temp, perm)
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		toUnlock = append(toUnlock, perm)
	}

	// Update locks synchronously to ensure success before sending confirmation
	var failedLocks []string
	for _, perm := range toUnlock {
		if err := db.UpdateLock(chat.Id, perm, false); err != nil {
			log.Warnf("[Locks] Failed to unlock %s in chat %d: %v", perm, chat.Id, err)
			failedLocks = append(failedLocks, perm)
		}
	}

	// Send appropriate response based on success/failure
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if len(failedLocks) > 0 {
		// Some unlocks failed
		text, _ := tr.GetString("locks_unlock_failed")
		_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(failedLocks, ", ")), helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		// All unlocks succeeded
		temp, _ := tr.GetString("locks_unlocked_successfully")
		text := fmt.Sprintf(temp, strings.Join(toUnlock, "\n - "))
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

// restHandler monitors messages and deletes them if they match
// restricted content types that are locked in the chat.
func (moduleStruct) restHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender

	// Skip if sender is nil (shouldn't happen but be safe)
	if sender == nil {
		return ext.ContinueGroups
	}

	// Get sender ID - works for both users and channels
	senderID := sender.Id()

	// Skip for admins and approved users (IsUserAdmin handles channel IDs safely)
	if chat_status.IsUserAdmin(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	for restr, filter := range restrMap {
		if !filter(msg) || !db.IsPermLocked(chat.Id, restr) || !chat_status.CanBotDelete(b, ctx, nil, true) {
			continue
		}

		// Special handling for comments lock:
		// Delete messages from users who aren't members of the chat (discussion comments)
		// but skip Telegram's system account (777000) which forwards channel posts
		if restr == "comments" {
			// Skip if from Telegram's system account
			if msg.From != nil && msg.From.Id == 777000 {
				continue
			}
			// Only delete if sender is not a member of the chat
			if chat_status.IsUserInChat(b, chat, senderID) {
				continue
			}
		}

		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		// Message deleted, no need to check other restrictions
		break
	}

	return ext.ContinueGroups
}

// permHandler monitors messages and deletes them if they match
// specific permission locks that are enabled in the chat.
func (moduleStruct) permHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender

	// Skip if sender is nil (shouldn't happen but be safe)
	if sender == nil {
		return ext.ContinueGroups
	}

	// Get sender ID - works for both users and channels
	senderID := sender.Id()

	// Skip for admins and approved users (IsUserAdmin handles channel IDs safely)
	if chat_status.IsUserAdmin(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	for perm, filter := range lockMap {
		if !filter(msg) || !db.IsPermLocked(chat.Id, perm) || !chat_status.CanBotDelete(b, ctx, nil, true) {
			continue
		}

		// Skip "bots" lock - handled separately by botLockHandler for new member joins
		if perm == "bots" {
			continue
		}

		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		// Message deleted, no need to check other locks
		break
	}

	return ext.ContinueGroups
}

// botLockHandler handles the bots lock by automatically banning
// bots that are added to the chat when bots lock is enabled.
func (moduleStruct) botLockHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	sender := ctx.EffectiveSender
	mem := ctx.ChatMember.NewChatMember.MergeChatMember().User

	// Check if bots lock is enabled first (most common case: it's not)
	if !db.IsPermLocked(chat.Id, "bots") {
		return ext.ContinueGroups
	}

	// Get sender ID for admin check - the person who added the bot
	var senderID int64
	if sender != nil {
		senderID = sender.Id()
	}

	// Allow admins to add bots even when bots lock is enabled
	if senderID > 0 && chat_status.IsUserAdmin(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	// Check if bot has necessary permissions
	if !chat_status.IsBotAdmin(b, ctx, nil) {
		tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("locks_bot_lock_no_permission")
		_, err := b.SendMessage(chat.Id, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.ContinueGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, true) {
		tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("locks_bot_lock_no_ban_permission")
		_, err := b.SendMessage(chat.Id, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.ContinueGroups
	}

	// Ban the bot that was added
	_, err := chat.BanMember(b, mem.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
	text, _ := tr.GetString("locks_bot_only_admins")
	_, err = b.SendMessage(chat.Id, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

// LoadLocks registers all locks module handlers with the dispatcher,
// including commands and message filters for lock enforcement.
func LoadLocks(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(locksModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("lock", locksModule.lockPerm))
	dispatcher.AddHandler(handlers.NewCommand("unlock", locksModule.unlockPerm))
	dispatcher.AddHandler(handlers.NewCommand("locktypes", locksModule.locktypes))
	misc.AddCmdToDisableable("locktypes")
	dispatcher.AddHandler(handlers.NewCommand("locks", locksModule.locks))
	misc.AddCmdToDisableable("locks")
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, locksModule.permHandler), locksModule.permHandlerGroup)
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, locksModule.restHandler), locksModule.restrHandlerGroup)
	dispatcher.AddHandler(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				mem := u.NewChatMember.MergeChatMember()
				oldMem := u.OldChatMember.MergeChatMember()
				return mem.User.IsBot && mem.Status == "member" && oldMem.Status == "left" // new bot being added to group
			},
			locksModule.botLockHandler,
		),
	)
}
