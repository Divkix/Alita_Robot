package formatting

import (
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

const (
	Markdown             = "Markdown"
	HTML                 = "HTML"
	MaxMessageLength int = 4096
)

var (
	linkRegex        = regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`)
	rulesBtnRegex    = regexp.MustCompile(`(?s){rules(:(same|up))?}`)
	htmlToMdReplacer = strings.NewReplacer(
		"<b>", "*",
		"</b>", "*",
		"<i>", "_",
		"</i>", "_",
		"<u>", "__",
		"</u>", "__",
		"<s>", "~",
		"</s>", "~",
		"<code>", "`",
		"</code>", "`",
		"<pre>", "```",
		"</pre>", "```",
	)
)

type memberCountEntry struct {
	count int
	at    time.Time
}

const defaultMemberCountCacheTTL = 60 * time.Second

var (
	memberCountCache    sync.Map
	memberCountCacheTTL = defaultMemberCountCacheTTL
)

func cachedMemberCount(b *gotgbot.Bot, chat *gotgbot.Chat) string {
	if v, ok := memberCountCache.Load(chat.Id); ok {
		if e, ok := v.(memberCountEntry); ok && time.Since(e.at) < memberCountCacheTTL {
			return strconv.Itoa(e.count)
		}
		memberCountCache.Delete(chat.Id)
	}
	count, err := chat.GetMemberCount(b, nil)
	if err != nil {
		return "0"
	}
	entry := memberCountEntry{count: int(count), at: time.Now()}
	memberCountCache.Store(chat.Id, entry)
	expireMemberCount(chat.Id, entry)
	return strconv.Itoa(int(count))
}

func expireMemberCount(chatID int64, entry memberCountEntry) {
	time.AfterFunc(memberCountCacheTTL, func() {
		memberCountCache.CompareAndDelete(chatID, entry)
	})
}

func Shtml() *gotgbot.SendMessageOpts {
	return &gotgbot.SendMessageOpts{
		ParseMode: HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyParameters: &gotgbot.ReplyParameters{
			AllowSendingWithoutReply: true,
		},
	}
}

func Smarkdown() *gotgbot.SendMessageOpts {
	return &gotgbot.SendMessageOpts{
		ParseMode: Markdown,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyParameters: &gotgbot.ReplyParameters{
			AllowSendingWithoutReply: true,
		},
	}
}

func SplitMessage(msg string) []string {
	totalRunes := utf8.RuneCountInString(msg)
	if totalRunes <= MaxMessageLength {
		return []string{msg}
	}

	lines := strings.Split(msg, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	result := make([]string, 0, totalRunes/MaxMessageLength+1)

	smallMsg := ""
	smallMsgRunes := 0

	for _, line := range lines {
		lineRunes := utf8.RuneCountInString(line)
		potentialRunes := smallMsgRunes + lineRunes + 1

		if potentialRunes <= MaxMessageLength {
			smallMsg += line + "\n"
			smallMsgRunes = potentialRunes
			continue
		}

		if lineRunes > MaxMessageLength {
			if smallMsg != "" {
				result = append(result, smallMsg)
				smallMsg = ""
				smallMsgRunes = 0
			}
			for chunk := range slices.Chunk([]rune(line), MaxMessageLength) {
				result = append(result, string(chunk))
			}
		} else {
			if smallMsg != "" {
				result = append(result, smallMsg)
			}
			smallMsg = line + "\n"
			smallMsgRunes = lineRunes + 1
		}
	}

	if smallMsg != "" {
		result = append(result, smallMsg)
	}

	return result
}

func MentionHtml(userId int64, name string) string {
	return MentionUrl(fmt.Sprintf("tg://user?id=%d", userId), name)
}

func MentionUrl(url, name string) string {
	return fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(url), html.EscapeString(name))
}

func HtmlEscape(s string) string {
	return html.EscapeString(s)
}

func ReverseHTML2MD(text string) string {
	if linkRegex.MatchString(text) {
		matches := linkRegex.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				oldLink := match[0]
				newLink := fmt.Sprintf("[%s](%s)", match[2], match[1])
				text = strings.Replace(text, oldLink, newLink, 1)
			}
		}
	}

	return htmlToMdReplacer.Replace(text)
}

func FormattingReplacer(b *gotgbot.Bot, chat *gotgbot.Chat, user *gotgbot.User, oldMsg string, buttons []db.Button) (res string, btns []db.Button) {
	const language = "en"
	var (
		firstName string
		lastName  string
		fullName  string
		username  string
		userId    int64
	)

	if user == nil {
		tr := i18n.MustNewTranslator(language)
		personNoName, _ := tr.GetString("helpers_person_no_name")
		if personNoName == "" {
			personNoName = "PersonWithNoName"
		}
		firstName = personNoName
		fullName = personNoName
		username = personNoName
		userId = 0
	} else {
		firstName = user.FirstName
		if len(user.FirstName) <= 0 {
			tr := i18n.MustNewTranslator(language)
			personNoName, _ := tr.GetString("helpers_person_no_name")
			if personNoName == "" {
				personNoName = "PersonWithNoName"
			}
			firstName = personNoName
		}

		lastName = user.LastName
		fullName = GetFullName(firstName, user.LastName)
		mention := MentionHtml(user.Id, firstName)

		if user.Username != "" {
			username = "@" + html.EscapeString(user.Username)
		} else {
			username = mention
		}
		userId = user.Id
	}

	countStr := "0"
	if strings.Contains(oldMsg, "{count}") {
		countStr = cachedMemberCount(b, chat)
	}

	r := strings.NewReplacer(
		"{first}", html.EscapeString(firstName),
		"{last}", html.EscapeString(lastName),
		"{fullname}", html.EscapeString(fullName),
		"{username}", username,
		"{mention}", username,
		"{count}", countStr,
		"{chatname}", html.EscapeString(chat.Title),
		"{id}", strconv.Itoa(int(userId)),
	)

	response := rulesBtnRegex.FindStringSubmatch(oldMsg)
	if response == nil {
		return r.Replace(oldMsg), buttons
	}

	res = r.Replace(rulesBtnRegex.ReplaceAllString(oldMsg, ""))
	btns = append([]db.Button(nil), buttons...)

	rulesDb := rules.GetChatRulesInfo(chat.Id)
	if rulesDb.Rules == "" {
		return res, btns
	}

	rulesBtnText := rulesDb.RulesBtn
	if rulesBtnText == "" {
		tr := i18n.MustNewTranslator(language)
		defaultRulesText, _ := tr.GetString("button_rules_default")
		if defaultRulesText == "" {
			defaultRulesText = "Rules"
		}
		rulesBtnText = defaultRulesText
	}

	sameline := response[2] == "same"
	rulesButton := db.Button{
		Name:     rulesBtnText,
		Url:      fmt.Sprintf("https://t.me/%s?start=rules_%d", b.Username, chat.Id),
		SameLine: sameline,
	}

	if response[2] == "up" {
		btns = append([]db.Button{rulesButton}, buttons...)
	} else {
		btns = append(btns, rulesButton)
	}

	return res, btns
}

func GetFullName(firstName, lastName string) string {
	if lastName != "" {
		return firstName + " " + lastName
	}
	return firstName
}
