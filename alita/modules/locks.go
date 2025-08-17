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
		for _, s := range msg.Entities {
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
			return msg.GetSender().IsAnonymousChannel() || !msg.GetSender().IsLinkedChannel()
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
		sb.WriteString(fmt.Sprintf("\n - %s = %v", k, newMapLocks[k]))
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
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
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
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat

	_, err := msg.Reply(b, m.buildLockTypesMessage(chat.Id), helpers.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// lockPerm handles the /lock command to enable specific lock types
// in the chat, requiring admin permissions.
func (m moduleStruct) lockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	var toLock []string

	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
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
	for _, perm := range args {
		if !string_handling.FindInStringSlice(m.getLockMapAsArray(), perm) {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("locks_invalid_lock_type")
			text := fmt.Sprintf(temp, perm)
			_, err := msg.Reply(b, text, helpers.Smarkdown())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		toLock = append(toLock, perm)
	}

	for _, perm := range toLock {
		go db.UpdateLock(chat.Id, perm, true)
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	temp, _ := tr.GetString("locks_locked_successfully")
	text := fmt.Sprintf(temp, strings.Join(toLock, "\n - "))
	_, err := msg.Reply(b, text, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// unlockPerm handles the /unlock command to disable specific lock types
// in the chat, requiring admin permissions.
func (m moduleStruct) unlockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]
	var toLock []string

	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
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

	for _, perm := range args {
		if !string_handling.FindInStringSlice(m.getLockMapAsArray(), perm) {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("locks_invalid_lock_type")
			text := fmt.Sprintf(temp, perm)
			_, err := msg.Reply(b, text, helpers.Smarkdown())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		toLock = append(toLock, perm)
	}

	for _, perm := range toLock {
		go db.UpdateLock(chat.Id, perm, false)
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	temp, _ := tr.GetString("locks_unlocked_successfully")
	text := fmt.Sprintf(temp, strings.Join(toLock, "\n - "))
	_, err := msg.Reply(b, text, helpers.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// restHandler monitors messages and deletes them if they match
// restricted content types that are locked in the chat.
func (moduleStruct) restHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User

	var err error

	// don't work on admins and approved users
	if chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	for restr, filter := range restrMap {
		if filter(msg) && db.IsPermLocked(chat.Id, restr) && chat_status.CanBotDelete(b, ctx, nil, true) {
			if restr == "comments" && msg.From.Id != 777000 {
				if !chat_status.IsUserInChat(b, chat, user.Id) {
					_, err = msg.Delete(b, nil)
					if err != nil {
						log.Error(err)
						return err
					}
				}
				break
			}
			_, err = msg.Delete(b, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}

	return ext.ContinueGroups
}

// permHandler monitors messages and deletes them if they match
// specific permission locks that are enabled in the chat.
func (moduleStruct) permHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User

	var err error

	// don't work on admins and approved users
	if chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	for perm, filter := range lockMap {
		if filter(msg) && db.IsPermLocked(chat.Id, perm) && chat_status.CanBotDelete(b, ctx, nil, true) {
			if perm == "bots" {
				continue
			}
			_, err = msg.Delete(b, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}

	return ext.ContinueGroups
}

// botLockHandler handles the bots lock by automatically banning
// bots that are added to the chat when bots lock is enabled.
func (moduleStruct) botLockHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	mem := ctx.ChatMember.NewChatMember.MergeChatMember().User

	// don't work on admins and approved users
	if chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	if !chat_status.IsBotAdmin(b, ctx, nil) {
		tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("locks_bot_lock_no_permission")
		_, err := b.SendMessage(chat.Id, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.ContinueGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, true) {
		tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("locks_bot_lock_no_ban_permission")
		_, err := b.SendMessage(chat.Id, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.ContinueGroups
	}

	if !db.IsPermLocked(chat.Id, "bots") {
		return ext.ContinueGroups
	}

	_, err := chat.BanMember(b, mem.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
	text, _ := tr.GetString("locks_bot_only_admins")
	_, err = b.SendMessage(chat.Id, text, nil)
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
