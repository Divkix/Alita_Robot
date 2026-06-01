package content

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// ExtractResult holds the parsed content and metadata from a note or filter message.
type ExtractResult struct {
	KeyWord     string
	FileID      string
	Text        string
	DataType    int
	Buttons     []db.Button
	PvtOnly     bool
	GrpOnly     bool
	AdminOnly   bool
	WebPreview  bool
	IsProtected bool
	NoNotif     bool
	ErrorMsg    string
}

// WelcomeResult holds the parsed content and metadata from a welcome/greeting message.
type WelcomeResult struct {
	Text     string
	DataType int
	FileID   string
	Buttons  []db.Button
	ErrorMsg string
}

// ExtractNoteAndFilter extracts and processes note or filter content from a Telegram message.
// Handles text, media files, and reply messages with button parsing and content validation.
// Returns parsed content with metadata like data type, buttons, and special options.
//
//nolint:dupl // ExtractNoteAndFilter shares media detection logic with ExtractWelcome
func ExtractNoteAndFilter(msg *gotgbot.Message, isFilter bool, language string) ExtractResult {
	var result ExtractResult
	result.DataType = -1 // not defined datatype; invalid note
	tr := i18n.MustNewTranslator(language)

	// Check for nil message to prevent panic
	if msg == nil {
		result.ErrorMsg, _ = tr.GetString("content_invalid_message")
		if result.ErrorMsg == "" {
			result.ErrorMsg = "Invalid message: message is nil" // fallback
		}
		return result
	}

	if isFilter {
		result.ErrorMsg, _ = tr.GetString("content_need_filter_content")
		if result.ErrorMsg == "" {
			result.ErrorMsg = "You need to give the filter some content!" // fallback
		}
	} else {
		result.ErrorMsg, _ = tr.GetString("content_need_note_content")
		if result.ErrorMsg == "" {
			result.ErrorMsg = "You need to give the note some content!" // fallback
		}
	}

	var (
		rawText string
		args    = strings.Fields(msg.Text)[1:]
	)
	_buttons := make([]tgmd2html.ButtonV2, 0) // make a slice for buttons
	replyMsg := msg.ReplyToMessage

	// set rawText from helper function
	setRawText(msg, args, &rawText)

	// extract the noteword
	if len(args) >= 2 && replyMsg == nil {
		// Uses inline extraction to avoid circular dependency with the extraction package.
		if len(args) > 0 {
			result.KeyWord = args[0]
			if len(args) > 1 {
				result.Text = strings.Join(args[1:], " ")
			}
		}
		result.Text, _buttons = tgmd2html.MD2HTMLButtonsV2(result.Text)
		result.DataType = db.TEXT
	} else if replyMsg != nil && len(args) >= 1 {
		// Uses inline extraction to avoid circular dependency with the extraction package.
		result.KeyWord = strings.Join(args, " ")

		if replyMsg.ReplyMarkup == nil {
			result.Text, _buttons = tgmd2html.MD2HTMLButtonsV2(rawText)
		} else {
			result.Text, _ = tgmd2html.MD2HTMLButtonsV2(rawText)
			_buttons = inlineKeyboardToButtonV2(replyMsg.ReplyMarkup)
		}

		if replyMsg.Text != "" {
			result.DataType = db.TEXT
		} else {
			result.FileID, result.DataType = extractMediaFromReply(replyMsg)
		}
	}

	// pre-fix the data before sending it back
	preFixes(_buttons, result.KeyWord, &result.Text, &result.DataType, result.FileID, &result.Buttons, &result.ErrorMsg, language)

	// return if datatype is invalid
	if result.DataType != -1 && !isFilter {
		// parse options such as pvtOnly, adminOnly, webPrev and replace them
		result.PvtOnly, result.GrpOnly, result.AdminOnly, result.WebPreview, result.IsProtected, result.NoNotif, _ = NotesParser(result.Text)
	}

	return result
}

// extractMediaFromReply extracts media file ID and data type from a reply message.
// Checks for sticker, document, photo, audio, voice, video, animation, and video note in order.
// Returns empty fileid and -1 dataType if no media is found.
func extractMediaFromReply(replyMsg *gotgbot.Message) (fileid string, dataType int) {
	if replyMsg == nil {
		return "", -1
	}
	if replyMsg.Sticker != nil {
		return replyMsg.Sticker.FileId, db.STICKER
	} else if replyMsg.Document != nil {
		return replyMsg.Document.FileId, db.DOCUMENT
	} else if len(replyMsg.Photo) > 0 {
		return replyMsg.Photo[len(replyMsg.Photo)-1].FileId, db.PHOTO
	} else if replyMsg.Audio != nil {
		return replyMsg.Audio.FileId, db.AUDIO
	} else if replyMsg.Voice != nil {
		return replyMsg.Voice.FileId, db.VOICE
	} else if replyMsg.Video != nil {
		return replyMsg.Video.FileId, db.VIDEO
	} else if replyMsg.Animation != nil {
		return replyMsg.Animation.FileId, db.DOCUMENT
	} else if replyMsg.VideoNote != nil {
		return replyMsg.VideoNote.FileId, db.VideoNote
	}
	return "", -1
}

// ExtractWelcome extracts and processes welcome/greeting content from a Telegram message.
// Similar to ExtractNoteAndFilter but specifically for greeting messages.
// Returns processed content with data type, file ID, and buttons for the greeting.
func ExtractWelcome(msg *gotgbot.Message, greetingType string, language string) WelcomeResult {
	var result WelcomeResult
	result.DataType = -1
	tr := i18n.MustNewTranslator(language)
	template, _ := tr.GetString("content_need_content")
	if template == "" {
		template = "You need to give me some content to %s users!" // fallback
	}
	result.ErrorMsg = fmt.Sprintf(template, greetingType)
	var (
		rawText string
		args    = strings.Fields(msg.Text)[1:]
	)
	_buttons := make([]tgmd2html.ButtonV2, 0)
	replyMsg := msg.ReplyToMessage

	// set rawText from helper function
	setRawText(msg, args, &rawText)

	if len(args) >= 1 && msg.ReplyToMessage == nil {
		result.FileID = ""
		result.Text, _buttons = tgmd2html.MD2HTMLButtonsV2(rawText)
		result.DataType = db.TEXT
	} else if msg.ReplyToMessage != nil {
		if replyMsg.ReplyMarkup == nil {
			result.Text, _buttons = tgmd2html.MD2HTMLButtonsV2(rawText)
		} else {
			result.Text, _ = tgmd2html.MD2HTMLButtonsV2(rawText)
			_buttons = inlineKeyboardToButtonV2(replyMsg.ReplyMarkup)
		}
		if len(args) == 0 && replyMsg.Text != "" {
			result.DataType = db.TEXT
		} else {
			result.FileID, result.DataType = extractMediaFromReply(replyMsg)
		}
	}

	// pre-fix the data before sending it back
	preFixes(_buttons, "Greeting", &result.Text, &result.DataType, result.FileID, &result.Buttons, &result.ErrorMsg, language)

	return result
}

// NotesParser parses special note options from message text using regex patterns.
// Detects {private}, {noprivate}, {admin}, {preview}, {protect}, {nonotif} tags.
// Returns boolean flags for each option and the text with tags removed.
func NotesParser(sent string) (pvtOnly, grpOnly, adminOnly, webPrev, protectedContent, noNotif bool, sentBack string) {
	pvtOnly = strings.Contains(sent, "{private}")
	grpOnly = strings.Contains(sent, "{noprivate}")
	adminOnly = strings.Contains(sent, "{admin}")
	webPrev = strings.Contains(sent, "{preview}")
	protectedContent = strings.Contains(sent, "{protect}")
	noNotif = strings.Contains(sent, "{nonotif}")

	sent = strings.NewReplacer(
		"{private}", "",
		"{admin}", "",
		"{preview}", "",
		"{noprivate}", "",
		"{protect}", "",
		"{nonotif}", "",
	).Replace(sent)

	return pvtOnly, grpOnly, adminOnly, webPrev, protectedContent, noNotif, sent
}

// preFixes validates and preprocesses message content before database storage.
// Checks message length limits using UTF-8 character count (not bytes), validates button URLs,
// sets default button names, and filters invalid content. Modifies parameters by reference.
func preFixes(buttons []tgmd2html.ButtonV2, defaultNameButton string, text *string, dataType *int, fileid string, dbButtons *[]db.Button, errorMsg *string, language string) {
	tr := i18n.MustNewTranslator(language)

	// Use utf8.RuneCountInString to count UTF-8 characters instead of len() for bytes
	textRuneCount := utf8.RuneCountInString(*text)

	if *dataType == db.TEXT && textRuneCount > 4096 {
		*dataType = -1
		template, _ := tr.GetString("content_text_too_long")
		*errorMsg = fmt.Sprintf(template, textRuneCount)
	} else if *dataType != db.TEXT && textRuneCount > 1024 {
		*dataType = -1
		template, _ := tr.GetString("content_caption_too_long")
		*errorMsg = fmt.Sprintf(template, textRuneCount)
	} else {
		for i, button := range buttons {
			if button.Name == "" {
				buttons[i].Name = defaultNameButton
			}
		}

		// buttonUrlFixer filters out non-URL buttons from the keyboard, keeping only valid URL buttons.
		buttonUrlFixer := func(_buttons *[]tgmd2html.ButtonV2) {
			// Validate URLs using Go's net/url parser instead of regex for proper validation
			validButtons := make([]tgmd2html.ButtonV2, 0, len(*_buttons))
			for _, btn := range *_buttons {
				u, err := url.Parse(btn.Content)
				if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
					validButtons = append(validButtons, btn)
				}
			}
			*_buttons = validButtons
		}

		buttonUrlFixer(&buttons)
		*dbButtons = ConvertButtonV2ToDbButton(buttons)

		// trim the characters \n, \t, \r and space from the text
		// also, set the dataType to -1 to make note invalid
		*text = strings.Trim(*text, "\n\t\r ")
		if *text == "" && fileid == "" {
			*dataType = -1
		}
	}
}

// setRawText extracts raw markdown text from a Telegram message.
// Handles both direct message text/caption and replied message content.
// Sets rawText parameter by reference with the extracted content.
func setRawText(msg *gotgbot.Message, args []string, rawText *string) {
	replyMsg := msg.ReplyToMessage
	if replyMsg == nil {
		if msg.Text == "" && msg.Caption != "" {
			parts := strings.SplitN(msg.OriginalCaptionMDV2(), " ", 2)
			if len(parts) >= 2 {
				*rawText = parts[1] // remove the command
			}
		} else if msg.Text != "" && msg.Caption == "" {
			parts := strings.SplitN(msg.OriginalMDV2(), " ", 2)
			if len(parts) >= 2 {
				*rawText = parts[1] // remove the command
			}
		}
	} else {
		if replyMsg.Text == "" && replyMsg.Caption != "" {
			*rawText = replyMsg.OriginalCaptionMDV2()
		} else if replyMsg.Caption == "" && len(args) >= 2 {
			parts := strings.SplitN(msg.OriginalMDV2(), " ", 3)
			if len(parts) >= 3 {
				*rawText = parts[2] // remove the command and first arg
			}
		} else {
			*rawText = replyMsg.OriginalMDV2()
		}
	}
}

// convertButtonV2ToDbButton converts markdown parser button format to database button format.
// Maps ButtonV2 fields to corresponding db.Button fields.
func ConvertButtonV2ToDbButton(buttons []tgmd2html.ButtonV2) (btns []db.Button) {
	btns = make([]db.Button, len(buttons))
	for i, btn := range buttons {
		btns[i] = db.Button{
			Name:     btn.Name,
			Url:      btn.Content,
			SameLine: btn.SameLine,
		}
	}
	return
}

// RevertButtons converts database button format back to markdown button string format.
// Generates markdown buttonurl syntax for each button with proper same-line handling.
func RevertButtons(buttons []db.Button) string {
	res := ""
	for _, btn := range buttons {
		if btn.SameLine {
			res += fmt.Sprintf("\n[%s](buttonurl://%s:same)", btn.Name, btn.Url)
		} else {
			res += fmt.Sprintf("\n[%s](buttonurl://%s)", btn.Name, btn.Url)
		}
	}
	return res
}

// inlineKeyboardToButtonV2 converts Telegram inline keyboard to markdown button format.
// Filters out non-URL buttons and handles same-line button positioning.
func inlineKeyboardToButtonV2(replyMarkup *gotgbot.InlineKeyboardMarkup) (btns []tgmd2html.ButtonV2) {
	btns = make([]tgmd2html.ButtonV2, 0)
	for _, inlineKeyboard := range replyMarkup.InlineKeyboard {
		if len(inlineKeyboard) > 1 {
			for i, button := range inlineKeyboard {
				// if any button has anything other than url, it's not a valid button
				// skip options such as CallbackData, CallbackUrl, etc.
				if button.Url == "" {
					continue
				}

				sameline := true
				if i == 0 {
					sameline = false
				}
				btns = append(
					btns,
					tgmd2html.ButtonV2{
						Name:     button.Text,
						Content:  button.Url,
						SameLine: sameline,
					},
				)
			}
		} else if len(inlineKeyboard) > 0 && inlineKeyboard[0].Url != "" {
			btns = append(btns,
				tgmd2html.ButtonV2{
					Name:     inlineKeyboard[0].Text,
					Content:  inlineKeyboard[0].Url,
					SameLine: false,
				},
			)
		}
	}
	return
}
