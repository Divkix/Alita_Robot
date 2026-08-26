package actionlog

import (
	"fmt"
	"html"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// Log sends htmlText to the chat's log channel when the category is enabled.
// Failures are swallowed so logging never blocks a user-facing command.
func Log(b *gotgbot.Bot, chat *gotgbot.Chat, category, htmlText string) {
	if b == nil || chat == nil || htmlText == "" {
		return
	}
	// Skip channel *chats* (broadcast channels). Supergroup IDs are also
	// channel-shaped numerically, so do not use IsChannelId here.
	if chat.Type == "channel" {
		return
	}
	settings := logchannels.Get(chat.Id)
	if settings == nil || !logchannels.CategoryEnabled(settings, category) {
		return
	}
	title := html.EscapeString(chat.Title)
	header := fmt.Sprintf("<b>%s</b> (<code>%d</code>)\n", title, chat.Id)
	if _, err := helpers.SendMessageWithErrorHandling(b, settings.LogChannelID, header+htmlText, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
	}); err != nil {
		log.Debugf("[ActionLog] failed to send %s log for chat %d: %v", category, chat.Id, err)
	}
}

// Admin logs a human admin action (ban/mute/kick/warn).
func Admin(b *gotgbot.Bot, chat *gotgbot.Chat, actor *gotgbot.User, action string, targetID int64, reason string) {
	if actor == nil {
		return
	}
	text := fmt.Sprintf(
		"#%s\nAdmin: %s\nUser: <code>%d</code>",
		html.EscapeString(action),
		formatting.MentionHtml(actor.Id, actor.FirstName),
		targetID,
	)
	if reason != "" {
		text += "\nReason: " + html.EscapeString(reason)
	}
	Log(b, chat, logchannels.CategoryAdmin, text)
}

// User logs a user-initiated action such as kickme.
func User(b *gotgbot.Bot, chat *gotgbot.Chat, actor *gotgbot.User, action string) {
	if actor == nil {
		return
	}
	text := fmt.Sprintf(
		"#%s\nUser: %s",
		html.EscapeString(action),
		formatting.MentionHtml(actor.Id, actor.FirstName),
	)
	Log(b, chat, logchannels.CategoryUser, text)
}

// Reports logs a /report or @admin event.
func Reports(b *gotgbot.Bot, chat *gotgbot.Chat, reporter *gotgbot.User, targetID int64) {
	if reporter == nil {
		return
	}
	text := fmt.Sprintf(
		"#REPORT\nReporter: %s\nTarget: <code>%d</code>",
		formatting.MentionHtml(reporter.Id, reporter.FirstName),
		targetID,
	)
	Log(b, chat, logchannels.CategoryReports, text)
}

// Settings logs a settings change.
func Settings(b *gotgbot.Bot, chat *gotgbot.Chat, actor *gotgbot.User, summary string) {
	actorBit := "unknown"
	if actor != nil {
		actorBit = formatting.MentionHtml(actor.Id, actor.FirstName)
	}
	Log(b, chat, logchannels.CategorySettings, fmt.Sprintf("#SETTINGS\nAdmin: %s\n%s", actorBit, html.EscapeString(summary)))
}
