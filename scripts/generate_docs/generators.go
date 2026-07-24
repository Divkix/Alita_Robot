package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// manualMaintenanceSentinel marks files that should not be overwritten by the generator.
// Files containing this sentinel in their first 512 bytes are skipped during generation.
const manualMaintenanceSentinel = "<!-- MANUALLY MAINTAINED: do not regenerate -->"

// skipIfManuallyMaintained checks if a file has the sentinel comment
// indicating it should not be overwritten by the generator.
func skipIfManuallyMaintained(filePath string) bool {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return false // File doesn't exist, proceed with generation
	}
	checkLen := min(512, len(data))
	return strings.Contains(string(data[:checkLen]), manualMaintenanceSentinel)
}

// generateModuleDocs generates individual module pages in commands/{module}/index.md
func generateModuleDocs(modules []Module, outputPath string) error {
	for _, module := range modules {
		moduleDir := filepath.Join(outputPath, "commands", module.Name)
		moduleFile := filepath.Join(moduleDir, "index.md")

		if skipIfManuallyMaintained(moduleFile) {
			log.Infof("Skipped: commands/%s/index.md (manually maintained)", module.Name)
			continue
		}

		log.Debugf("Generating module doc: %s", module.DisplayName)

		// Prepare content
		var content strings.Builder

		// Starlight frontmatter
		content.WriteString("---\n")
		fmt.Fprintf(&content, "title: %s Commands\n", module.DisplayName)
		fmt.Fprintf(&content, "description: Complete guide to %s module commands and features\n", module.DisplayName)
		content.WriteString("---\n\n")

		// Module header with emoji
		emoji := getModuleEmoji(module.Name)
		fmt.Fprintf(&content, "# %s %s Commands\n\n", emoji, module.DisplayName)

		// Module description (converted from Telegram markdown)
		if module.HelpText != "" {
			helpText := formatHelpText(module.HelpText)
			content.WriteString(helpText)
			content.WriteString("\n\n")
		}

		// Extended documentation - append after help text
		if module.ExtendedDocs.Extended != "" {
			extendedDocs := convertTelegramMarkdown(module.ExtendedDocs.Extended)
			content.WriteString(extendedDocs)
			content.WriteString("\n\n")
		}

		// Module aliases
		if len(module.Aliases) > 0 {
			content.WriteString("## Module Aliases\n\n")
			content.WriteString("This module can be accessed using the following aliases:\n\n")
			for _, alias := range module.Aliases {
				fmt.Fprintf(&content, "- `%s`\n", alias)
			}
			content.WriteString("\n")
		}

		// Commands table - only if module has commands
		if len(module.Commands) > 0 {
			content.WriteString("## Available Commands\n\n")
			content.WriteString("| Command | Description | Disableable |\n")
			content.WriteString("|---------|-------------|-------------|\n")

			for _, cmd := range module.Commands {
				description := extractCommandDescription(cmd.Name, module.HelpText)
				disableable := "✅"
				if !cmd.Disableable {
					disableable = "❌"
				}

				// Add command aliases to description if present
				if len(cmd.Aliases) > 0 {
					aliasStr := strings.Join(cmd.Aliases, ", ")
					description += fmt.Sprintf(" (Aliases: `%s`)", aliasStr)
				}

				fmt.Fprintf(&content, "| `/%s` | %s | %s |\n",
					cmd.Name,
					description,
					disableable)
			}
			content.WriteString("\n")
		}

		// Features section - from *_features_docs
		if module.ExtendedDocs.Features != "" {
			content.WriteString("## Features\n\n")
			featureDocs := convertTelegramMarkdown(module.ExtendedDocs.Features)
			content.WriteString(featureDocs)
			content.WriteString("\n\n")
		}

		// Usage examples section
		if module.ExtendedDocs.Examples != "" {
			// Use custom examples from translation if available
			content.WriteString("## Usage Examples\n\n")
			exampleDocs := convertTelegramMarkdown(module.ExtendedDocs.Examples)
			content.WriteString(exampleDocs)
			content.WriteString("\n\n")
		} else if len(module.Commands) > 0 {
			// Generate basic examples only if module has commands and no custom examples
			content.WriteString("## Usage Examples\n\n")
			content.WriteString(generateUsageExamples(module))
			content.WriteString("\n")
		}

		// Permissions section
		if module.ExtendedDocs.Permissions != "" {
			// Use custom permissions docs from translation if available
			content.WriteString("## Required Permissions\n\n")
			permissionDocs := convertTelegramMarkdown(module.ExtendedDocs.Permissions)
			content.WriteString(permissionDocs)
			content.WriteString("\n\n")
		} else if len(module.Commands) > 0 {
			// Generate generic permissions section only if module has commands
			content.WriteString("## Required Permissions\n\n")
			content.WriteString(generatePermissionsSection(module))
			content.WriteString("\n")
		}

		// Technical Notes section - from *_notes_docs
		if module.ExtendedDocs.Notes != "" {
			content.WriteString("## Technical Notes\n\n")
			notesDocs := convertTelegramMarkdown(module.ExtendedDocs.Notes)
			content.WriteString(notesDocs)
			content.WriteString("\n\n")
		}

		// Write file
		if config.DryRun {
			log.Infof("[DRY RUN] Would write: %s (%d bytes)", moduleFile, content.Len())
			log.Debugf("Content preview:\n%s", truncateString(content.String(), 500))
		} else {
			if err := os.MkdirAll(moduleDir, 0o750); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", moduleDir, err)
			}

			if err := os.WriteFile(moduleFile, []byte(content.String()), 0o600); err != nil {
				return fmt.Errorf("failed to write module doc %s: %w", moduleFile, err)
			}

			log.Infof("Generated: commands/%s/index.md", module.Name)
		}
	}

	return nil
}

var sectionHeaderRe = regexp.MustCompile(`^\*([^*]+)\*\s*:?\s*$`)

// inlineBoldRe matches *text* mid-sentence (Telegram bold → Markdown bold).
var inlineBoldRe = regexp.MustCompile(`\*([^*\n]+)\*`)

// crossBulletCommandRe matches × lines that start with a /command.
var crossBulletCommandRe = regexp.MustCompile(`^×\s+(/\S+)`)

// arrowSubExampleRe matches -> lines (sub-examples in filters/notes help).
var arrowSubExampleRe = regexp.MustCompile(`^->\s+(.+)`)

// convertTelegramMarkdown converts Telegram markdown to standard Markdown.
// Handles HTML tags, bullet characters, × command bullets, -> sub-examples,
// and HTML entity decoding.
func convertTelegramMarkdown(text string) string {
	// Convert HTML tags to Markdown equivalents
	text = strings.ReplaceAll(text, "<b>", "**")
	text = strings.ReplaceAll(text, "</b>", "**")
	text = strings.ReplaceAll(text, "<i>", "*")
	text = strings.ReplaceAll(text, "</i>", "*")
	text = strings.ReplaceAll(text, "<code>", "`")
	text = strings.ReplaceAll(text, "</code>", "`")

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// × bullet conversion
		if strings.HasPrefix(trimmed, "×") {
			if crossBulletCommandRe.MatchString(trimmed) {
				// × /cmd ... : desc → - `/cmd ...`: desc
				rest := strings.TrimPrefix(trimmed, "× ")
				// Split on first colon to wrap the command part in backticks
				if colonIdx := strings.Index(rest, ":"); colonIdx != -1 {
					cmdPart := strings.TrimSpace(rest[:colonIdx])
					descPart := strings.TrimSpace(rest[colonIdx+1:])
					cmdPart = strings.ReplaceAll(cmdPart, "`", "")
					lines[i] = "- `" + cmdPart + "`: " + descPart
				} else {
					rest = strings.ReplaceAll(rest, "`", "")
					lines[i] = "- `" + rest + "`"
				}
			} else {
				// × plain text → - plain text
				rest := strings.TrimPrefix(trimmed, "×")
				lines[i] = "-" + rest
			}
			continue
		}

		// -> sub-example conversion
		if m := arrowSubExampleRe.FindStringSubmatch(trimmed); m != nil {
			lines[i] = "  - `" + m[1] + "`"
			continue
		}

		// • and · bullet conversion
		if strings.HasPrefix(trimmed, "•") || strings.HasPrefix(trimmed, "·") {
			lines[i] = strings.Replace(line, "•", "-", 1)
			lines[i] = strings.Replace(lines[i], "·", "-", 1)
		}
	}

	text = strings.Join(lines, "\n")

	// Decode HTML entities last (after tag conversion)
	text = html.UnescapeString(text)

	return text
}

// formatHelpText processes _help_msg content with help-specific patterns
// (section headers, inline bold) before calling convertTelegramMarkdown for
// the remaining patterns.
func formatHelpText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Convert *Section Name*: at line start → ### Section Name
		if m := sectionHeaderRe.FindStringSubmatch(trimmed); m != nil {
			lines[i] = "### " + m[1]
			continue
		}
		// Convert remaining *text* (Telegram bold) → **text** (Markdown bold)
		if inlineBoldRe.MatchString(trimmed) {
			lines[i] = inlineBoldRe.ReplaceAllString(line, "**$1**")
		}
	}
	text = strings.Join(lines, "\n")
	return convertTelegramMarkdown(text)
}

// extractCommandDescription attempts to extract description for a command from help text
func extractCommandDescription(cmdName, helpText string) string {
	lowerCmdName := "/" + strings.ToLower(cmdName)
	for line := range strings.SplitSeq(helpText, "\n") {
		lowerLine := strings.ToLower(line)
		start := 0
		matched := false
		for {
			idx := strings.Index(lowerLine[start:], lowerCmdName)
			if idx == -1 {
				break
			}
			idx += start

			// Ensure the character after cmdName is not a letter or number to prevent substring matching
			afterIdx := idx + len(lowerCmdName)
			isValidBoundary := true
			if afterIdx < len(line) {
				nextChar := line[afterIdx]
				if (nextChar >= 'a' && nextChar <= 'z') || (nextChar >= 'A' && nextChar <= 'Z') || (nextChar >= '0' && nextChar <= '9') {
					isValidBoundary = false
				}
			}

			if isValidBoundary {
				matched = true
				break
			}

			// Prefix false-positive: advance start past this occurrence
			start = idx + len(lowerCmdName)
		}

		if !matched {
			continue
		}

		for _, sep := range []string{": ", " - "} {
			parts := strings.SplitN(line, sep, 2)
			if len(parts) == 2 {
				desc := strings.TrimSpace(parts[1])
				desc = strings.ReplaceAll(desc, "<b>", "")
				desc = strings.ReplaceAll(desc, "</b>", "")
				desc = strings.ReplaceAll(desc, "<code>", "`")
				desc = strings.ReplaceAll(desc, "</code>", "`")
				return desc
			}
		}
	}
	return "No description available"
}

// generateUsageExamples generates usage examples for a module
func generateUsageExamples(module Module) string {
	var content strings.Builder

	content.WriteString("### Basic Usage\n\n")

	if len(module.Commands) > 0 {
		// Show first few commands as examples
		limit := min(3, len(module.Commands))

		content.WriteString("```text\n")
		for i := range limit {
			fmt.Fprintf(&content, "/%s\n", module.Commands[i].Name)
		}
		content.WriteString("```\n\n")
	}

	content.WriteString("For detailed command usage, refer to the commands table above.\n")

	return content.String()
}

// generatePermissionsSection generates permissions section for a module
func generatePermissionsSection(module Module) string {
	var content strings.Builder

	// Determine permission level based on module name
	adminModules := map[string]bool{
		"admin": true, "bans": true, "locks": true, "muting": true,
		"purges": true, "warnings": true, "antiflood": true,
	}

	if adminModules[module.Name] {
		content.WriteString("Most commands in this module require **admin permissions** in the group.\n\n")
		content.WriteString("**Bot Permissions Required:**\n\n")
		content.WriteString("- Delete messages\n")
		content.WriteString("- Ban users\n")
		content.WriteString("- Restrict users\n")
		content.WriteString("- Pin messages (if applicable)\n")
	} else {
		content.WriteString("Commands in this module are available to all users unless otherwise specified.\n")
	}

	return content.String()
}

// getModuleEmoji returns an appropriate emoji for a module
//
//nolint:dupl // getModuleEmoji and categorizeModule have similar structure by design
func getModuleEmoji(moduleName string) string {
	emojiMap := map[string]string{
		"admin":      "👑",
		"bans":       "🔨",
		"locks":      "🔒",
		"muting":     "🔇",
		"purges":     "🧹",
		"warnings":   "⚠️",
		"filters":    "🔍",
		"notes":      "📝",
		"rules":      "📋",
		"greetings":  "👋",
		"reporting":  "📢",
		"language":   "🌐",
		"antiflood":  "🌊",
		"blacklist":  "🚫",
		"approval":   "✅",
		"captcha":    "🔐",
		"connection": "🔗",
		"disabling":  "❌",
		"extras":     "⭐",
		"misc":       "🔧",
		"formatting": "📄",
		"info":       "ℹ️",
		"karma":      "⭐",
		"gtranslate": "🌍",
	}

	if emoji, exists := emojiMap[moduleName]; exists {
		return emoji
	}

	return "📦"
}

// truncateString truncates a string to maxLen
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// generateLockTypesReference generates the lock types API reference documentation
func generateLockTypesReference(lockTypes []LockType, outputPath string) error {
	filePath := filepath.Join(outputPath, "api-reference", "lock-types.md")

	log.Debug("Generating lock types reference")

	var content strings.Builder

	// Frontmatter
	content.WriteString("---\n")
	content.WriteString("title: Lock Types\n")
	content.WriteString("description: Complete reference of all available lock types in the Locks module\n")
	content.WriteString("---\n\n")

	// Overview
	content.WriteString("# 🔒 Lock Types Reference\n\n")
	content.WriteString("This page documents all available lock types in the Locks module. ")
	content.WriteString("Locks allow administrators to restrict specific types of content or actions in their groups.\n\n")

	// Count lock types by category
	permissionCount := 0
	restrictionCount := 0
	for _, lock := range lockTypes {
		switch lock.Category {
		case "permission":
			permissionCount++
		case "restriction":
			restrictionCount++
		}
	}

	content.WriteString("## Overview\n\n")
	fmt.Fprintf(&content, "- **Total Lock Types**: %d\n", len(lockTypes))
	fmt.Fprintf(&content, "- **Permission Locks**: %d (specific content types)\n", permissionCount)
	fmt.Fprintf(&content, "- **Restriction Locks**: %d (broad categories)\n", restrictionCount)
	content.WriteString("\n")

	// How locks work
	content.WriteString("## How Locks Work\n\n")
	content.WriteString("Locks prevent non-admin users from posting specific types of content. ")
	content.WriteString("When a lock is enabled, the bot will automatically delete matching messages from regular users.\n\n")

	content.WriteString("### Usage\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock <lock_type> [lock_type2 ...]\n")
	content.WriteString("/unlock <lock_type> [lock_type2 ...]\n")
	content.WriteString("/locks - View all currently enabled locks\n")
	content.WriteString("/locktypes - View all available lock types\n")
	content.WriteString("```\n\n")

	// Restriction Locks
	content.WriteString("## Restriction Locks\n\n")
	content.WriteString("Restriction locks affect broad categories of messages. ")
	content.WriteString("These are powerful locks that can block multiple content types at once.\n\n")

	content.WriteString("| Lock Type | Description |\n")
	content.WriteString("|-----------|-------------|\n")

	for _, lock := range lockTypes {
		if lock.Category == "restriction" {
			fmt.Fprintf(&content, "| `%s` | %s |\n", lock.Name, lock.Description)
		}
	}
	content.WriteString("\n")

	// Permission Locks
	content.WriteString("## Permission Locks\n\n")
	content.WriteString("Permission locks target specific types of content or actions. ")
	content.WriteString("Use these for fine-grained control over what users can post.\n\n")

	content.WriteString("| Lock Type | Description |\n")
	content.WriteString("|-----------|-------------|\n")

	for _, lock := range lockTypes {
		if lock.Category == "permission" {
			fmt.Fprintf(&content, "| `%s` | %s |\n", lock.Name, lock.Description)
		}
	}
	content.WriteString("\n")

	// Media types section
	content.WriteString("## Media Type Locks\n\n")
	content.WriteString("These locks control specific types of media content:\n\n")
	mediaTypes := []string{"photo", "video", "audio", "voice", "document", "gif", "sticker", "videonote"}
	for _, mediaType := range mediaTypes {
		for _, lock := range lockTypes {
			if lock.Name == mediaType && lock.Category == "permission" {
				fmt.Fprintf(&content, "- **`%s`**: %s\n", lock.Name, lock.Description)
			}
		}
	}
	content.WriteString("\n")

	// Content behavior locks
	content.WriteString("## Content Behavior Locks\n\n")
	content.WriteString("These locks control how content behaves:\n\n")
	behaviorTypes := []string{"forward", "url", "previews", "rtl", "anonchannel", "comments"}
	for _, behaviorType := range behaviorTypes {
		for _, lock := range lockTypes {
			if lock.Name == behaviorType {
				fmt.Fprintf(&content, "- **`%s`**: %s\n", lock.Name, lock.Description)
			}
		}
	}
	content.WriteString("\n")

	// Special locks
	content.WriteString("## Special Locks\n\n")

	content.WriteString("### `bots`\n\n")
	for _, lock := range lockTypes {
		if lock.Name == "bots" {
			fmt.Fprintf(&content, "%s\n\n", lock.Description)
		}
	}
	content.WriteString("**Behavior**: When enabled, the bot will automatically ban any bot added by non-admins.\n\n")

	content.WriteString("### `all`\n\n")
	for _, lock := range lockTypes {
		if lock.Name == "all" {
			fmt.Fprintf(&content, "%s\n\n", lock.Description)
		}
	}
	content.WriteString("**Use Case**: Useful for temporarily freezing chat activity or creating read-only channels.\n\n")

	// Examples
	content.WriteString("## Examples\n\n")

	content.WriteString("### Prevent Media Spam\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock media\n")
	content.WriteString("```\n")
	content.WriteString("Blocks all media files (audio, documents, videos, photos, video notes, and voice messages).\n\n")

	content.WriteString("### Create Read-Only Chat\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock all\n")
	content.WriteString("```\n")
	content.WriteString("Prevents all non-admin users from sending any messages.\n\n")

	content.WriteString("### Block Forwarded Content\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock forward\n")
	content.WriteString("```\n")
	content.WriteString("Prevents users from forwarding messages from other chats.\n\n")

	content.WriteString("### Prevent Bot Addition\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock bots\n")
	content.WriteString("```\n")
	content.WriteString("Only admins can add bots to the group.\n\n")

	content.WriteString("### Multiple Locks at Once\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock sticker gif video\n")
	content.WriteString("```\n")
	content.WriteString("Lock multiple content types in a single command.\n\n")

	// Admin exemption
	content.WriteString("## Important Notes\n\n")
	content.WriteString("### Admin Exemption\n\n")
	content.WriteString("All locks automatically exempt administrators. Admins can always post any content type, ")
	content.WriteString("regardless of which locks are enabled.\n\n")

	content.WriteString("### Bot Permissions\n\n")
	content.WriteString("The bot requires the following permissions to enforce locks:\n\n")
	content.WriteString("- **Delete Messages**: Required to remove locked content\n")
	content.WriteString("- **Ban Users**: Only for the `bots` lock (to ban unauthorized bot additions)\n\n")

	content.WriteString("### Interaction with Other Modules\n\n")
	content.WriteString("Locks work independently but complement other moderation modules:\n\n")
	content.WriteString("- **Antiflood**: Locks control content types, antiflood controls message frequency\n")
	content.WriteString("- **Filters**: Custom filters can block specific words/patterns, locks block content types\n")
	content.WriteString("- **Blacklist**: Blacklist blocks specific words, locks block entire categories\n\n")

	// Related commands
	content.WriteString("## Related Commands\n\n")
	content.WriteString("- `/lock <type>` - Enable one or more locks\n")
	content.WriteString("- `/unlock <type>` - Disable one or more locks\n")
	content.WriteString("- `/locks` - View currently enabled locks\n")
	content.WriteString("- `/locktypes` - View all available lock types\n\n")

	content.WriteString("For more information, see the [Locks module documentation](/commands/locks/).\n\n")

	// Write file
	if config.DryRun {
		log.Infof("[DRY RUN] Would write: %s (%d bytes)", filePath, content.Len())
	} else {
		refDir := filepath.Join(outputPath, "api-reference")
		if err := os.MkdirAll(refDir, 0o750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", refDir, err)
		}

		if err := os.WriteFile(filePath, []byte(content.String()), 0o600); err != nil {
			return fmt.Errorf("failed to write lock types reference: %w", err)
		}

		log.Info("Generated: api-reference/lock-types.md")
	}

	return nil
}
