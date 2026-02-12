package modules

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// buildModerationMatchText builds a unified searchable string for moderation.
// It includes message text, caption text, and URL entities from both.
func buildModerationMatchText(msg *gotgbot.Message) string {
	if msg == nil {
		return ""
	}

	parts := make([]string, 0, 4)
	seen := make(map[string]struct{})
	appendUnique := func(s string) {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		parts = append(parts, trimmed)
	}

	appendUnique(msg.Text)
	appendUnique(msg.Caption)

	for _, url := range extractEntityURLs(msg.Text, msg.Entities) {
		appendUnique(url)
	}
	for _, url := range extractEntityURLs(msg.Caption, msg.CaptionEntities) {
		appendUnique(url)
	}

	return strings.Join(parts, "\n")
}

func extractEntityURLs(source string, entities []gotgbot.MessageEntity) []string {
	if len(entities) == 0 {
		return nil
	}

	urls := make([]string, 0, len(entities))
	for _, entity := range entities {
		if entity.Url != "" {
			urls = append(urls, entity.Url)
			continue
		}
		// "url" entities store raw links in the text/caption itself.
		if entity.Type == "url" {
			if extracted := extractEntityText(source, entity.Offset, entity.Length); extracted != "" {
				urls = append(urls, extracted)
			}
		}
	}
	return urls
}

func extractEntityText(source string, offset, length int64) string {
	if source == "" || offset < 0 || length <= 0 {
		return ""
	}
	runes := []rune(source)
	start := int(offset)
	end := start + int(length)
	if start >= len(runes) || end > len(runes) || start >= end {
		return ""
	}
	return string(runes[start:end])
}
