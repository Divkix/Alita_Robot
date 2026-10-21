package modules

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/extraction"
)

// buildModerationMatchText builds a unified searchable string for moderation.
// It includes message text, caption text, and URL entities from both.
func buildModerationMatchText(msg *gotgbot.Message) string {
	if msg == nil {
		return ""
	}

	// No entities means no URL can be extracted: skip the slice/map allocations
	// and reproduce the trim + dedupe of Text and Caption directly.
	if len(msg.Entities) == 0 && len(msg.CaptionEntities) == 0 {
		text := strings.TrimSpace(msg.Text)
		caption := strings.TrimSpace(msg.Caption)
		switch {
		case text == "":
			return caption
		case caption == "" || caption == text:
			return text
		default:
			return text + "\n" + caption
		}
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

	// Allocate lazily: most messages carry entities that are not URLs.
	var urls []string
	for _, entity := range entities {
		if entity.Url != "" {
			urls = append(urls, entity.Url)
			continue
		}
		// "url" entities store raw links in the text/caption itself.
		if entity.Type == "url" {
			if extracted := extraction.ExtractEntityText(source, entity.Offset, entity.Length); extracted != "" {
				urls = append(urls, extracted)
			}
		}
	}
	if urls == nil {
		return []string{}
	}
	return urls
}
