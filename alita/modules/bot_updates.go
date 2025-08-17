package modules

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// function used to get status of bot when it joined a group and send a message to the group
// also send a message to MESSAGE_DUMP telling that it joined a group
// botJoinedGroup handles bot addition to new groups.
// Sends welcome message and ensures the group is a supergroup before staying.
func botJoinedGroup(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat

	// don't log if it's a private chat
	if chat.Type == "private" {
		return ext.EndGroups
	}

	// check if group is supergroup or not
	// if not a supergroup, send a message and leave it
	if chat.Type == "group" || chat.Type == "channel" {
		if chat.Type == "group" {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("bot_updates_need_supergroup")
			convertInstr, _ := tr.GetString("bot_updates_convert_instruction")
			convertHowto, _ := tr.GetString("bot_updates_convert_howto")
			_, err := b.SendMessage(
				chat.Id,
				fmt.Sprint(
					text,
					convertInstr,
					convertHowto,
					"https://telegra.ph/Convert-group-to-Supergroup-07-29",
				),
				helpers.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
		}

		_, err := b.LeaveChat(chat.Id, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	msgAdmin := "\n\nMake me admin to use me with my full abilities!"

	// used to check if bot was added as admin or not
	if chat_status.IsBotAdmin(b, ctx, chat) {
		msgAdmin = ""
	}

	// send a message to group itself
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	thanksText, _ := tr.GetString("bot_updates_thanks_for_adding")
	_, err := b.SendMessage(
		chat.Id,
		fmt.Sprint(
			thanksText,
			func() string { plug, _ := tr.GetString("bot_updates_creators_plug"); return plug }(),
			msgAdmin,
		),
		nil,
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

// adminCacheAutoUpdate automatically refreshes admin cache when admin status changes.
// Reloads admin permissions cache if it's not already available.
func adminCacheAutoUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat

	adminsAvail, _ := cache.GetAdminCacheList(chat.Id)

	if !adminsAvail {
		cache.LoadAdminCache(b, chat.Id)
		log.Info(fmt.Sprintf("Reloaded admin cache for %d (%s)", chat.Id, chat.Title))
	}

	return ext.ContinueGroups
}

// function used to verify anonymous admins when they press to verify admin button
// verifyAnonyamousAdmin handles callback verification for anonymous admins.
// Verifies admin status and executes the original command from cached context.
func verifyAnonyamousAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	qmsg := query.Message

	data := strings.Split(query.Data, ".")
	chatId, _ := strconv.ParseInt(data[1], 10, 64)
	msgId, _ := strconv.ParseInt(data[2], 10, 64)

	// if non-admins try to press it
	// using this func because it's the only one that can be called by taking chatId from callback query
	if !chat_status.IsUserAdmin(b, chatId, query.From.Id) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("bot_updates_need_admin")
		_, err := query.Answer(b,
			&gotgbot.AnswerCallbackQueryOpts{
				Text: text,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	chatIdData, errCache := setAdminCache(chatId, msgId)

	if errCache != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		expiredText, _ := tr.GetString("bot_updates_button_expired")
		_, _, err := qmsg.EditText(b, expiredText, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	msg := chatIdData.(*gotgbot.Message)

	_, err := qmsg.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	ctx.EffectiveMessage = msg                     // set the message to the message that was originally used when command was given
	ctx.EffectiveMessage.SenderChat = nil          // make senderChat nil to avoid chat_status.isAnonAdmin to mistaken user for GroupAnonymousBot
	ctx.CallbackQuery = nil                        // callback query is not needed anymore
	command := strings.Split(msg.Text, " ")[0][1:] // get the command, with or without the bot username and without '/'
	command = strings.Split(command, "@")[0]       // separate the command from the bot username

	switch command {

	// admin
	case "promote":
		return adminModule.promote(b, ctx)
	case "demote":
		return adminModule.demote(b, ctx)
	case "title":
		return adminModule.setTitle(b, ctx)

	// bans (restrictions)
	case "ban":
		return bansModule.ban(b, ctx)
	case "dban":
		return bansModule.dBan(b, ctx)
	case "sban":
		return bansModule.sBan(b, ctx)
	case "tban":
		return bansModule.tBan(b, ctx)
	case "unban":
		return bansModule.unban(b, ctx)
	case "restrict":
		return bansModule.restrict(b, ctx)
	case "unrestrict":
		return bansModule.unrestrict(b, ctx)

	// mutes (restrictions)
	case "mute":
		return mutesModule.mute(b, ctx)
	case "smute":
		return mutesModule.sMute(b, ctx)
	case "dmute":
		return mutesModule.dMute(b, ctx)
	case "tmute":
		return mutesModule.tMute(b, ctx)
	case "unmute":
		return mutesModule.unmute(b, ctx)

	// pins
	case "pin":
		return pinsModule.pin(b, ctx)
	case "unpin":
		return pinsModule.unpin(b, ctx)
	case "permapin":
		return pinsModule.permaPin(b, ctx)
	case "unpinall":
		return pinsModule.unpinAll(b, ctx)

	// purges
	case "purge":
		return purgesModule.purge(b, ctx)
	case "del":
		return purgesModule.delCmd(b, ctx)

	// warns
	case "warn":
		return warnsModule.warnUser(b, ctx)
	case "swarn":
		return warnsModule.sWarnUser(b, ctx)
	case "dwarn":
		return warnsModule.dWarnUser(b, ctx)
	}

	return ext.EndGroups
}

// setAdminCache retrieves cached message data for anonymous admin verification.
// Returns the original message context stored during anonymous admin command execution.
func setAdminCache(chatId, msgId int64) (any, error) {
	return cache.Marshal.Get(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msgId), new(gotgbot.Message))
}

// LoadBotUpdates registers bot event handlers for group management.
// Sets up handlers for bot joins, admin updates, and anonymous admin verification.
func LoadBotUpdates(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandlerToGroup(
		handlers.NewMyChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				wasMember, isMember := helpers.ExtractJoinLeftStatusChange(u)
				return !wasMember && isMember
			},
			botJoinedGroup,
		),
		-1, // process before all other handlers
	)

	dispatcher.AddHandler(
		handlers.NewChatMember(
			helpers.ExtractAdminUpdateStatusChange,
			adminCacheAutoUpdate,
		),
	)

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("alita:anonAdmin:"), verifyAnonyamousAdmin))
}
