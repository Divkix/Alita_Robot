package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var reactionsModule = moduleStruct{
	moduleName:   "Reactions",
	handlerGroup: 8,
}

// reactionKey generates a Redis key for storing reactions for a chat
func reactionKey(chatID int64) string {
	return fmt.Sprintf("alita:reactions:%d", chatID)
}

// LoadReactions loads the reactions module with all command handlers
func LoadReactions(dispatcher *ext.Dispatcher) {
	// Admin commands
	dispatcher.AddHandler(handlers.NewCommand("addreaction", reactionsModule.addReaction))
	dispatcher.AddHandler(handlers.NewCommand("removereaction", reactionsModule.removeReaction))
	dispatcher.AddHandler(handlers.NewCommand("reactions", reactionsModule.listReactions))
	dispatcher.AddHandler(handlers.NewCommand("resetreactions", reactionsModule.resetReactions))

	// Message watcher for reactions (positive handler group for monitoring)
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, reactionsModule.checkReactions), reactionsModule.handlerGroup)

	// Register module as disableable
	HelpModule.AbleMap.Store(reactionsModule.moduleName, true)

	// Add help text
	HelpModule.AltHelpOptions["Reactions"] = []string{"reaction"}
	HelpModule.helpableKb["Reactions"] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         "Add Reaction",
				CallbackData: encodeCallbackData("reactions_help", map[string]string{"action": "add"}, "reactions_help.add"),
			},
			{
				Text:         "Remove Reaction",
				CallbackData: encodeCallbackData("reactions_help", map[string]string{"action": "remove"}, "reactions_help.remove"),
			},
		},
	}

	log.Info("[Modules] Reactions module loaded")
}

// addReaction handles /addreaction <keyword> <emoji> command
func (m moduleStruct) addReaction(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][addReaction] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	// Check permission - only admins can add reactions
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 3 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_add_usage")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyword := strings.ToLower(strings.TrimSpace(args[1]))
	emoji := strings.TrimSpace(args[2])

	// Validate emoji (basic check - should be a single emoji or emoji sequence)
	if emoji == "" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_invalid_emoji")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Store in Redis using SET
	key := reactionKey(chat.Id)

	// Get existing reactions
	existing, err := cache.Marshal.Get(cache.Context, key, new(map[string]string))
	if err != nil {
		// Create new map if doesn't exist
		existing = &map[string]string{}
	}

	reactionsMap := *existing.(*map[string]string)
	if reactionsMap == nil {
		reactionsMap = make(map[string]string)
	}

	// Add or update reaction
	reactionsMap[keyword] = emoji

	// Save back to cache
	if err := cache.Marshal.Set(cache.Context, key, reactionsMap); err != nil {
		log.Errorf("[Reactions] Failed to save reaction: %v", err)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_add_error")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_add_success", i18n.TranslationParams{
		"keyword": keyword,
		"emoji":   emoji,
	})
	_, err = msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// removeReaction handles /removereaction <keyword> command
func (m moduleStruct) removeReaction(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][removeReaction] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	// Check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 2 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_remove_usage")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyword := strings.ToLower(strings.TrimSpace(args[1]))
	key := reactionKey(chat.Id)

	// Get existing reactions
	existing, err := cache.Marshal.Get(cache.Context, key, new(map[string]string))
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_not_found")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	reactionsMap := *existing.(*map[string]string)
	if reactionsMap == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_not_found")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Check if keyword exists
	if _, exists := reactionsMap[keyword]; !exists {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_keyword_not_found", i18n.TranslationParams{
			"keyword": keyword,
		})
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Remove reaction
	delete(reactionsMap, keyword)

	// Save back to cache (or delete if empty)
	if len(reactionsMap) == 0 {
		if err := cache.Marshal.Delete(cache.Context, key); err != nil {
			log.Errorf("[Reactions] Failed to delete empty reactions: %v", err)
		}
	} else {
		if err := cache.Marshal.Set(cache.Context, key, reactionsMap); err != nil {
			log.Errorf("[Reactions] Failed to update reactions: %v", err)
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("reactions_remove_error")
			_, _ = msg.Reply(b, text, helpers.Shtml())
			return ext.EndGroups
		}
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_remove_success", i18n.TranslationParams{
		"keyword": keyword,
	})
	_, err = msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// listReactions handles /reactions command
func (m moduleStruct) listReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][listReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	key := reactionKey(chat.Id)

	// Get existing reactions
	existing, err := cache.Marshal.Get(cache.Context, key, new(map[string]string))
	if err != nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_none")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	reactionsMap := *existing.(*map[string]string)
	if len(reactionsMap) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_none")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return ext.EndGroups
	}

	// Build list
	var sb strings.Builder
	for keyword, emoji := range reactionsMap {
		sb.WriteString(fmt.Sprintf("• %s → %s\n", keyword, emoji))
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_list_header", i18n.TranslationParams{
		"list": sb.String(),
	})
	_, err = msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetReactions handles /resetreactions command
func (m moduleStruct) resetReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][resetReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	// Check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	key := reactionKey(chat.Id)

	// Delete all reactions
	if err := cache.Marshal.Delete(cache.Context, key); err != nil {
		log.Debugf("[Reactions] Failed to delete reactions (may not exist): %v", err)
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_reset_success")
	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// checkReactions checks incoming messages and reacts with emojis when keywords match
func (m moduleStruct) checkReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][checkReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return ext.ContinueGroups
	}

	chat := ctx.EffectiveChat
	if chat == nil {
		return ext.ContinueGroups
	}

	// Skip if module is disabled for this chat
	_, enabled := HelpModule.AbleMap.Load(reactionsModule.moduleName)
	if !enabled {
		return ext.ContinueGroups
	}

	// Get reactions for this chat
	key := reactionKey(chat.Id)
	existing, err := cache.Marshal.Get(cache.Context, key, new(map[string]string))
	if err != nil {
		// No reactions configured, continue silently
		return ext.ContinueGroups
	}

	reactionsMap := *existing.(*map[string]string)
	if len(reactionsMap) == 0 {
		return ext.ContinueGroups
	}

	// Check if message text contains any keywords (case-insensitive)
	lowerText := strings.ToLower(msg.Text)
	for keyword, emoji := range reactionsMap {
		if strings.Contains(lowerText, keyword) {
			// Set reaction on the message
			_, err := b.SetMessageReaction(
				chat.Id,
				msg.MessageId,
				&gotgbot.SetMessageReactionOpts{
					Reaction: []gotgbot.ReactionType{
						gotgbot.ReactionTypeEmoji{
							Emoji: emoji,
						},
					},
				},
			)
			if err != nil {
				log.Debugf("[Reactions] Failed to set reaction: %v", err)
				// Continue to next keyword even if this one failed
				continue
			}
			// Only react with first matching keyword to avoid rate limits
			break
		}
	}

	return ext.ContinueGroups
}
