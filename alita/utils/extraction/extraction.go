package extraction

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/google/uuid"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// ExtractChat extracts and validates a chat from command arguments.
// Supports both numeric chat IDs and chat usernames for chat identification.
// Returns nil if chat is not found or arguments are invalid.
func ExtractChat(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.Chat {
	msg := ctx.EffectiveMessage
	args := ctx.Args()[1:]
	if len(args) != 0 {
		if _, err := strconv.Atoi(args[0]); err == nil {
			chatId, _ := strconv.Atoi(args[0])
			chat, err := b.GetChat(int64(chatId), nil)
			if err != nil {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("extraction_chat_not_found")
				_, err := msg.Reply(b, text, nil)
				if err != nil {
					log.Error(err)
					return nil
				}
				return nil
			}
			_chat := chat.ToChat() // need to convert to Chat type
			return &_chat
		} else {
			chat, err := chat_status.GetChat(b, args[0])
			if err != nil {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("extraction_chat_not_found")
				_, err := msg.Reply(b, text, nil)
				if err != nil {
					log.Error(err)
					return nil
				}
				return nil
			}
			return chat
		}
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("extraction_need_chat_id")
	_, err := msg.Reply(b, text, nil)
	if err != nil {
		log.Error(err)
		return nil
	}
	return nil
}

// ExtractUser extracts a user ID from the message context.
// Uses ExtractUserAndText internally, returning only the user ID.
func ExtractUser(b *gotgbot.Bot, ctx *ext.Context) int64 {
	userId, _ := ExtractUserAndText(b, ctx)
	return userId
}

// ExtractUserAndText extracts both user ID and accompanying text from various message formats.
// Handles text mentions, usernames, numeric IDs, and reply messages.
// Returns user ID and associated text, with validation through the user database.
func ExtractUserAndText(b *gotgbot.Bot, ctx *ext.Context) (int64, string) {
	msg := ctx.EffectiveMessage
	args := ctx.Args()
	prevMessage := msg.ReplyToMessage

	splitText := strings.SplitN(msg.Text, " ", 2)

	if len(splitText) < 2 {
		return IdFromReply(msg)
	}

	textToParse := splitText[1]

	// func used to trim newlines from the text, fixes the pasring issues of '\n' before and after text
	trimTextNewline := func(str string) string {
		return strings.Trim(str, "\n")
	}

	text := ""

	var userId int64
	accepted := make(map[string]struct{})
	accepted["text_mention"] = struct{}{}

	entities := msg.ParseEntityTypes(accepted)

	var ent *gotgbot.ParsedMessageEntity
	isId := false
	if len(entities) > 0 {
		ent = &entities[0]
	} else {
		ent = nil
	}

	// only parse if the entity is a text mention
	if entities != nil && ent != nil && int(ent.Offset) == (len(msg.Text)-len(textToParse)) {
		ent = &entities[0]
		userId = ent.User.Id
		text = msg.Text[ent.Offset+ent.Length:]
	} else if len(args) >= 1 && args[1][0] == '@' {
		user := args[1]
		userId = GetUserId(user)
		if userId == 0 {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("extraction_user_not_found")
			_, err := msg.Reply(b, text, nil)
			error_handling.HandleErr(err)
			return -1, ""
		} else {
			res := strings.SplitN(msg.Text, " ", 3)
			if len(res) >= 3 {
				text = res[2]
			}
		}
	} else if len(args) >= 1 {
		isId = true
		if chatId, err := strconv.ParseInt(args[1], 10, 64); err != nil || !helpers.IsChannelID(chatId) {
			for _, arg := range args[1] {
				if unicode.IsDigit(arg) {
					continue
				}
				isId = false
				break
			}
		}
		if isId {
			userId, _ = strconv.ParseInt(args[1], 10, 64)
			res := strings.SplitN(msg.Text, " ", 3)
			if len(res) >= 3 {
				text = res[2]
			}
		}
	}
	if !isId && prevMessage != nil {
		_, parseErr := uuid.Parse(args[1])
		userId, text = IdFromReply(msg)
		if parseErr == nil {
			return userId, trimTextNewline(text)
		}
	} else if !isId {
		_, parseErr := uuid.Parse(args[1])
		if parseErr == nil {
			return userId, trimTextNewline(text)
		}
	}

	_, _, found := GetUserInfo(userId)
	if !found {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("extraction_get_chat_member_failed")
		_, err := msg.Reply(b, text, nil)
		error_handling.HandleErr(err)
		return -1, ""
	}

	return userId, trimTextNewline(text)
}

// GetUserId retrieves a user ID from a username string.
// Searches both user and channel databases for the username.
// Returns 0 if username is too short or not found.
func GetUserId(username string) int64 {
	if len(username) <= 5 {
		return 0
	}

	// remove '@' from the username
	username = strings.ReplaceAll(username, "@", "")

	user := db.GetUserIdByUserName(username)
	if user != 0 {
		return user
	}

	channel := db.GetChannelIdByUserName(username)
	if channel != 0 {
		return channel
	}

	return 0
}

// GetUserInfo retrieves user information (username and name) from a user ID.
// Searches both user and channel databases for the ID.
// Returns username, display name, and whether the user was found.
func GetUserInfo(userId int64) (username, name string, found bool) {
	username, name, found = db.GetUserInfoById(userId)
	if found {
		return username, name, found
	}

	username, name, found = db.GetChannelInfoById(userId)
	if found {
		return username, name, found
	}

	return "", "", false
}

// IdFromReply extracts user ID and text from a replied-to message.
// Gets the sender ID from the reply and remaining command text.
// Returns (0, "") if no reply message exists.
func IdFromReply(m *gotgbot.Message) (int64, string) {
	prevMessage := m.ReplyToMessage

	var userId int64

	if prevMessage == nil {
		return 0, ""
	}

	// get's the Id for both user and channel
	userId = prevMessage.GetSender().Id()

	res := strings.SplitN(m.Text, " ", 2)
	if len(res) < 2 {
		return userId, ""
	}
	return userId, res[1]
}

// ExtractQuotes extracts quoted text or words from a sentence using regex patterns.
// When matchQuotes is true, extracts text between double quotes.
// When matchWord is true, extracts the first word/token and remaining text.
func ExtractQuotes(sentence string, matchQuotes, matchWord bool) (inQuotes, afterWord string) {
	// Check for empty string to prevent panic
	if len(sentence) == 0 {
		return
	}

	// if first character starts with '""' and matchQutes is true
	if sentence[0] == '"' && matchQuotes {
		// regex pattern to match text between strings
		pattern, err := regexp.Compile(`(?s)(\s+)?"(.*?)"\s?(.*)?`)
		if err != nil {
			log.Error(err)
			return
		}
		if pattern.MatchString(sentence) {
			pat := pattern.FindStringSubmatch(sentence)
			// pat[0] would be the whole matched string
			// pat[1] is the spaces
			inQuotes, afterWord = pat[2], pat[3]
			return
		}
	} else if matchWord {
		// regex pattern to detect all words and special character which do not have spaces but can contain special characters
		pattern, err := regexp.Compile(`(?s)(\s+)?([A-Za-z0-9-_+=}\][{;:'",<.>?/|*\\()]+)\s?(.*)?`)
		if err != nil {
			log.Error(err)
			return
		}
		if pattern.MatchString(sentence) {
			pat := pattern.FindStringSubmatch(sentence)
			// pat[0] would be the whole matched string
			// pat[1] is the spaces
			inQuotes, afterWord = pat[2], pat[3]
			return
		}
	}

	return
}

// ExtractTime parses time duration strings for temporary actions like bans.
// Supports formats: Nm (minutes), Nh (hours), Nd (days), Nw (weeks).
// Returns Unix timestamp, formatted time string, and reason text.
func ExtractTime(b *gotgbot.Bot, ctx *ext.Context, inputVal string) (banTime int64, timeStr, reason string) {
	msg := ctx.EffectiveMessage
	timeNow := time.Now().Unix()
	yearTime := timeNow + int64(365*24*60*60)

	args := strings.Fields(inputVal)
	timeVal := args[0] // first word will be the time specification
	if len(args) >= 2 {
		reason = strings.Join(args[1:], " ")
	}

	// Check if timeVal ends with a valid time unit (m, h, d, w)
	if len(timeVal) > 0 {
		lastChar := timeVal[len(timeVal)-1]
		if lastChar == 'm' || lastChar == 'h' || lastChar == 'd' || lastChar == 'w' {
			t := timeVal[:len(timeVal)-1]
			timeNum, err := strconv.Atoi(t)
			if err != nil {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("extraction_invalid_time_amount")
				_, err := msg.Reply(b, text, nil)
				error_handling.HandleErr(err)
				return -1, "", ""
			}

			switch string(timeVal[len(timeVal)-1]) {
			case "m":
				banTime = timeNow + int64(timeNum*60)
				timeStr = fmt.Sprintf("%d minutes", timeNum)
			case "h":
				banTime = timeNow + int64(timeNum*60*60)
				timeStr = fmt.Sprintf("%d hours", timeNum)
			case "d":
				banTime = timeNow + int64(timeNum*24*60*60)
				timeStr = fmt.Sprintf("%d days", timeNum)
			case "w":
				banTime = timeNow + int64(timeNum*7*24*60*60)
				timeStr = fmt.Sprintf("%d weeks", timeNum)
			default:
				return -1, "", ""
			}

			if banTime >= yearTime {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("extraction_time_limit_exceeded")
				_, err := msg.Reply(b, text, nil)
				error_handling.HandleErr(err)
				return -1, "", ""
			}

			return banTime, timeStr, reason
		} else {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("extraction_invalid_time_type", i18n.TranslationParams{"0": timeVal})
			_, err := msg.Reply(b, text, nil)
			error_handling.HandleErr(err)
			return -1, "", ""
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("extraction_invalid_time_format")
		_, err := msg.Reply(b, text, nil)
		error_handling.HandleErr(err)
		return -1, "", ""
	}
}
