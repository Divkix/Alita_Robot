package main

import (
	"fmt"
	"html"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

// handWrittenPages lists the committed pages the generator must never
// overwrite, keyed by their docs-relative path. A page can stay hand-written
// only by being named here; a page without an entry is regenerated even if it
// carries an old opt-out comment. The value is a short reason the page owns
// content the generator cannot produce.
var handWrittenPages = map[string]string{
	"commands/admin/index.md":       "curated command semantics, anonymous-admin and security notes",
	"commands/aispam/index.md":      "model pipeline, thresholds, privacy, failure behaviour and operator setup",
	"commands/antiflood/index.md":   "curated action modes, defaults and per-command permissions",
	"commands/antiraid/index.md":    "duration bounds and examples newer than en.yml",
	"commands/approvals/index.md":   "corrected descriptions, examples and admin-only permission",
	"commands/backup/index.md":      "supported module list, confirmation flow and format compatibility",
	"commands/bans/index.md":        "curated ban/kick variants, examples and permissions",
	"commands/blacklists/index.md":  "action modes, aliases and temporary-duration rules",
	"commands/bot_updates/index.md": "no registered commands; documents handler groups and anonymous-admin security",
	"commands/captcha/index.md":     "join-request flow, pending messages and bot requirements",
	"commands/connections/index.md": "PM group arguments, connection modes and settings summary",
	"commands/devs/index.md":        "team tiers, argument requirements and permission matrix",
	"commands/disabling/index.md":   "per-command exceptions and disabled-command semantics",
	"commands/filters/index.md":     "noformat, random responses and PM group arguments",
	"commands/formatting/index.md":  "markdown, filling and random-content guide",
	"commands/greetings/index.md":   "welcome/goodbye settings, join requests and noformat",
	"commands/help/index.md":        "deep links, configuration wizard and navigation notes",
	"commands/languages/index.md":   "connected-group language semantics",
	"commands/locks/index.md":       "nightmode, media lock behaviour and permissions",
	"commands/misc/index.md":        "ping metrics and stat field details",
	"commands/mutes/index.md":       "time formats, reasons and permissions",
	"commands/notes/index.md":       "note visibility, noformat and examples",
	"commands/pins/index.md":        "pin limits, anti-channel-pin and technical notes",
	"commands/purges/index.md":      "purge ranges, examples and permissions",
	"commands/reactions/index.md":   "message watcher and inline help callbacks",
	"commands/reports/index.md":     "report routing, auto-mute and delivery notes",
	"commands/rules/index.md":       "private rules, buttons and reset behaviour",
	"commands/spamdb/index.md":      "databases, actions, privacy and failure behaviour",
	"commands/warns/index.md":       "warn limits, punishment/expiry notes and permissions",
}

// isHandWritten reports whether the docs-relative page is allowlisted.
func isHandWritten(pagePath string) bool {
	_, ok := handWrittenPages[pagePath]
	return ok
}

// commandRowRe matches the first command in a Markdown table row such as
// "| `/ban` | Ban a user |". Argument rows ("| `/spamdb on` | ...") yield the
// base command.
var commandRowRe = regexp.MustCompile("^\\|\\s*`/([a-z0-9_]+)")

// verifyHandWrittenCommandCoverage checks allowlisted command pages against
// the same parsed manifest the generator uses. Every command name and alias
// registered for the page's module must appear as /name, and every command row
// in the page must still be registered by some module.
//
// False-positive limits: a /name mention anywhere (prose, examples or an
// alias list) satisfies the presence check, so a command can be mentioned
// without a row of its own. Only the first command in a multi-command table
// cell is checked, argument variants are reduced to their base command, and
// prose is never judged stale. The manifest comes from static parsing, so a
// registration form parseCommands does not understand looks missing.
func verifyHandWrittenCommandCoverage(docsRoot string, commands []Command, pages map[string]string) error {
	commandsByModule := make(map[string][]Command)
	known := make(map[string]bool)
	for _, cmd := range commands {
		module := strings.ToLower(cmd.Module)
		commandsByModule[module] = append(commandsByModule[module], cmd)
		known[cmd.Name] = true
		for _, alias := range cmd.Aliases {
			known[alias] = true
		}
	}

	pagePaths := make([]string, 0, len(pages))
	for pagePath := range pages {
		if strings.HasPrefix(pagePath, "commands/") && strings.HasSuffix(pagePath, "/index.md") {
			pagePaths = append(pagePaths, pagePath)
		}
	}
	sort.Strings(pagePaths)

	var problems []string
	for _, pagePath := range pagePaths {
		data, err := os.ReadFile(filepath.Join(docsRoot, filepath.FromSlash(pagePath)))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", pagePath, err))
			continue
		}
		content := string(data)
		module := strings.TrimSuffix(strings.TrimPrefix(pagePath, "commands/"), "/index.md")

		// Match the generator's module resolution: exact lowercase name or a
		// singular/plural variant (admin.go commands feed the admin page).
		var registered []Command
		for name, cmds := range commandsByModule {
			if name == module || name == module+"s" || name+"s" == module {
				registered = append(registered, cmds...)
			}
		}

		var missing []string
		seen := make(map[string]bool)
		for _, cmd := range registered {
			names := append([]string{cmd.Name}, cmd.Aliases...)
			for _, name := range names {
				if seen[name] {
					continue
				}
				seen[name] = true
				if !mentionsCommand(content, name) {
					missing = append(missing, "/"+name)
				}
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf(
				"%s: module %q does not document registered command(s)/alias(es): %s",
				pagePath, module, strings.Join(missing, ", ")))
		}

		var stale []string
		staleSeen := make(map[string]bool)
		for _, line := range strings.Split(content, "\n") {
			match := commandRowRe.FindStringSubmatch(line)
			if match == nil || known[match[1]] || staleSeen[match[1]] {
				continue
			}
			staleSeen[match[1]] = true
			stale = append(stale, "/"+match[1])
		}
		sort.Strings(stale)
		if len(stale) > 0 {
			problems = append(problems, fmt.Sprintf(
				"%s: command table lists command(s) no module registers: %s",
				pagePath, strings.Join(stale, ", ")))
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

// mentionsCommand reports whether content references /name as a whole command
// word, so /ban does not match /banall.
func mentionsCommand(content, name string) bool {
	re := regexp.MustCompile(`(^|[^a-z0-9_])/` + regexp.QuoteMeta(name) + `($|[^a-z0-9_])`)
	return re.MatchString(content)
}

// generateModuleDocs generates individual module pages in commands/{module}/index.md
func generateModuleDocs(modules []Module, outputPath string) error {
	for _, module := range modules {
		moduleDir := filepath.Join(outputPath, "commands", module.Name)
		moduleFile := filepath.Join(moduleDir, "index.md")
		pagePath := path.Join("commands", module.Name, "index.md")

		if isHandWritten(pagePath) {
			log.Infof("Skipped: %s (hand-written: %s)", pagePath, handWrittenPages[pagePath])
			continue
		}

		log.Debugf("Generating module doc: %s", module.DisplayName)

		// Prepare content
		var content strings.Builder

		// Blume frontmatter (title/description — also valid for any Markdown docs framework)
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
		"admin": true, "bans": true, "locks": true, "mutes": true,
		"purges": true, "warns": true, "antiflood": true,
	}

	if module.Name == "federations" {
		content.WriteString("Most commands are limited to the federation owner or federation admins, not group admins. `/joinfed` and `/leavefed` require the group **creator**, and `/quietfed` requires a group admin. `/newfed` (in PM), `/fedinfo`, `/fedadmins`, `/chatfed`, `/myfeds` and `/fedstat` are open to everyone.\n\n")
		content.WriteString("**Bot Permissions Required:**\n\n")
		content.WriteString("- Ban users (to enforce and lift federation bans)\n")
		return content.String()
	}

	if module.Name == "logchannels" {
		content.WriteString("`/setlog` is posted in the log channel by one of its admins, then that post is forwarded to the group by a group admin. `/unsetlog`, `/log` and `/nolog` require a group admin. `/logchannel` and `/logcategories` are open to everyone.\n\n")
		content.WriteString("**Bot Permissions Required:**\n\n")
		content.WriteString("- Admin in the log channel with permission to post messages (to receive `/setlog` and send the logs)\n")
		return content.String()
	}

	if module.Name == "digest" {
		content.WriteString("Every `/digest` form requires **admin permissions** in the group. The digest itself is sent by the bot.\n\n")
		content.WriteString("**Bot Permissions Required:**\n\n")
		content.WriteString("- Send messages (in the group, or in the log channel when the digest goes there)\n")
		return content.String()
	}

	if module.Name == "recurring" {
		content.WriteString("Every `/recurring` form requires **admin permissions** with the right to change group info, the same right `/save` and `/setwelcome` need.\n\n")
		content.WriteString("**Bot Permissions Required:**\n\n")
		content.WriteString("- Send messages (and send media, for media content)\n")
		content.WriteString("- Delete messages (only for `delete=on` when the previous post is older than 48 hours)\n")
		return content.String()
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
		"admin":       "👑",
		"bans":        "🔨",
		"locks":       "🔒",
		"mutes":       "🔇",
		"purges":      "🧹",
		"warns":       "⚠️",
		"filters":     "🔍",
		"notes":       "📝",
		"rules":       "📋",
		"greetings":   "👋",
		"reports":     "📢",
		"languages":   "🌐",
		"antiflood":   "🌊",
		"blacklists":  "🚫",
		"approvals":   "✅",
		"captcha":     "🔐",
		"connections": "🔗",
		"disabling":   "❌",
		"extras":      "⭐",
		"misc":        "🔧",
		"formatting":  "📄",
		"federations": "🛡️",
		"logchannels": "📜",
		"info":        "ℹ️",
		"karma":       "⭐",
		"gtranslate":  "🌍",
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
	content.WriteString("Locks prevent users who are neither admins nor approved from posting specific types of content. ")
	content.WriteString("When a lock is enabled, the bot will automatically delete matching messages from regular users.\n\n")

	content.WriteString("### Usage\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock <lock_type> [lock_type2 ...]\n")
	content.WriteString("/unlock <lock_type> [lock_type2 ...]\n")
	content.WriteString("/locks - View the lock settings stored for this chat\n")
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
	content.WriteString("**Behavior**: When enabled, the bot will automatically ban any bot added by a user who is neither an admin nor approved.\n\n")

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
	content.WriteString("Blocks all media files (audio, documents, GIFs, videos, photos, video notes, and voice messages).\n\n")

	content.WriteString("### Create Read-Only Chat\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock all\n")
	content.WriteString("```\n")
	content.WriteString("Prevents all users who are neither admins nor approved from sending any messages.\n\n")

	content.WriteString("### Block Forwarded Content\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock forward\n")
	content.WriteString("```\n")
	content.WriteString("Prevents users from forwarding messages from other chats.\n\n")

	content.WriteString("### Prevent Bot Addition\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock bots\n")
	content.WriteString("```\n")
	content.WriteString("Only admins and approved users can add bots to the group.\n\n")

	content.WriteString("### Multiple Locks at Once\n\n")
	content.WriteString("```\n")
	content.WriteString("/lock sticker gif video\n")
	content.WriteString("```\n")
	content.WriteString("Lock multiple content types in a single command.\n\n")

	// Admin exemption
	content.WriteString("## Important Notes\n\n")
	content.WriteString("### Admin and Approved User Exemption\n\n")
	content.WriteString("All locks automatically exempt administrators and approved users (see /approve). They can always post any content type, ")
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
	content.WriteString("- `/locks` - View the lock settings stored for this chat (true = locked, false = unlocked)\n")
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
