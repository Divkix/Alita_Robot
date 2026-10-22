package actionlog

import (
	"fmt"
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// Failures are swallowed so logging never blocks a user-facing command.
func Log(b *gotgbot.Bot, chat *gotgbot.Chat, category, htmlText string) {
	if b == nil || chat == nil || htmlText == "" || chat.Type == "channel" {
		return
	}
	logTo(b, chat, logchannels.Get(chat.Id), category, htmlText)
}

// logTo sends to the log channel in settings rather than looking it up, so a
// change that removes the settings can still be logged to them.
func logTo(b *gotgbot.Bot, chat *gotgbot.Chat, settings *models.LogChannel, category, htmlText string) {
	if b == nil || chat == nil || htmlText == "" || chat.Type == "channel" {
		return
	}
	if settings == nil || !logchannels.CategoryEnabled(settings, category) {
		return
	}
	title := chat.Title
	if title == "" {
		// A button from PM names its group by ID alone; the header takes the
		// title from the chats table.
		if info, err := chats.GetChatBasicInfoCached(chat.Id); err == nil && info != nil {
			title = info.ChatName
		}
	}
	header := fmt.Sprintf("<b>%s</b> (<code>%d</code>)\n", html.EscapeString(title), chat.Id)
	if _, err := helpers.SendMessageWithErrorHandling(b, settings.LogChannelID, header+htmlText, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
	}); err != nil {
		log.Debugf("[ActionLog] failed to send %s log for chat %d: %v", category, chat.Id, err)
	}
}

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

// Field is one "Name: value" line of an automated entry; an empty value is left out.
type Field struct {
	Name  string
	Value string
}

// Automated logs an action the bot took on its own: the tag, the user it hit
// (none when userID is 0) and the given fields. Call it only after Telegram
// accepted the action.
func Automated(b *gotgbot.Bot, chat *gotgbot.Chat, tag string, userID int64, userName string, fields ...Field) {
	Log(b, chat, logchannels.CategoryAutomated, entryText(tag, nil, userID, userName, fields))
}

// Warn logs an admin's warn action under the warns category: the tag, the
// admin, the user it concerns (none when userID is 0) and the given fields.
// Call it only after the warn was written.
func Warn(b *gotgbot.Bot, chat *gotgbot.Chat, actor *gotgbot.User, tag string, userID int64, userName string, fields ...Field) {
	if actor == nil {
		return
	}
	Log(b, chat, logchannels.CategoryWarns, entryText(tag, actor, userID, userName, fields))
}

// Join logs a member joining under the joins category: the tag, the member
// and the given fields. Call it only after the join was let through.
func Join(b *gotgbot.Bot, chat *gotgbot.Chat, tag string, userID int64, userName string, fields ...Field) {
	Log(b, chat, logchannels.CategoryJoins, entryText(tag, nil, userID, userName, fields))
}

// SettingChange logs a chat setting an admin changed under the settings
// category, with its own tag and the given fields. Call it only after the
// change was made.
func SettingChange(b *gotgbot.Bot, chat *gotgbot.Chat, actor *gotgbot.User, tag string, fields ...Field) {
	if actor == nil {
		return
	}
	Log(b, chat, logchannels.CategorySettings, entryText(tag, actor, 0, "", fields))
}

// entryText renders an entry: the tag, the admin (none when actor is nil),
// the user (none when userID is 0) and one line per field with a value.
func entryText(tag string, actor *gotgbot.User, userID int64, userName string, fields []Field) string {
	var text strings.Builder
	text.WriteString("#" + html.EscapeString(tag))
	if actor != nil {
		fmt.Fprintf(&text, "\nAdmin: %s", formatting.MentionHtml(actor.Id, actor.FirstName))
	}
	if userID != 0 {
		who := html.EscapeString(userName)
		if userID > 0 {
			who = formatting.MentionHtml(userID, userName)
		}
		fmt.Fprintf(&text, "\nUser: %s (<code>%d</code>)", who, userID)
	}
	for _, f := range fields {
		if f.Value != "" {
			fmt.Fprintf(&text, "\n%s: %s", html.EscapeString(f.Name), html.EscapeString(f.Value))
		}
	}
	return text.String()
}

func Settings(b *gotgbot.Bot, chat *gotgbot.Chat, actor *gotgbot.User, summary string) {
	Log(b, chat, logchannels.CategorySettings, settingsText(actor, summary))
}

// SettingsTo logs to the given settings, for changes that remove them.
func SettingsTo(b *gotgbot.Bot, chat *gotgbot.Chat, settings *models.LogChannel, actor *gotgbot.User, summary string) {
	logTo(b, chat, settings, logchannels.CategorySettings, settingsText(actor, summary))
}

func settingsText(actor *gotgbot.User, summary string) string {
	actorBit := "unknown"
	if actor != nil {
		actorBit = formatting.MentionHtml(actor.Id, actor.FirstName)
	}
	return fmt.Sprintf("#SETTINGS\nAdmin: %s\n%s", actorBit, html.EscapeString(summary))
}
