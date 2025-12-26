package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

// generateModuleDocs generates individual module pages in commands/{module}/index.md
func generateModuleDocs(modules []Module, outputPath string) error {
	for _, module := range modules {
		moduleDir := filepath.Join(outputPath, "commands", module.Name)
		moduleFile := filepath.Join(moduleDir, "index.md")

		log.Debugf("Generating module doc: %s", module.DisplayName)

		// Prepare content
		var content strings.Builder

		// Starlight frontmatter
		content.WriteString("---\n")
		content.WriteString(fmt.Sprintf("title: %s Commands\n", module.DisplayName))
		content.WriteString(fmt.Sprintf("description: Complete guide to %s module commands and features\n", module.DisplayName))
		content.WriteString("---\n\n")

		// Module header with emoji
		emoji := getModuleEmoji(module.Name)
		content.WriteString(fmt.Sprintf("# %s %s Commands\n\n", emoji, module.DisplayName))

		// Module description (converted from Telegram markdown)
		if module.HelpText != "" {
			helpText := convertTelegramMarkdown(module.HelpText)
			content.WriteString(helpText)
			content.WriteString("\n\n")
		}

		// Module aliases
		if len(module.Aliases) > 0 {
			content.WriteString("## Module Aliases\n\n")
			content.WriteString("This module can be accessed using the following aliases:\n\n")
			for _, alias := range module.Aliases {
				content.WriteString(fmt.Sprintf("- `%s`\n", alias))
			}
			content.WriteString("\n")
		}

		// Commands table
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

				content.WriteString(fmt.Sprintf("| `/%s` | %s | %s |\n",
					cmd.Name,
					description,
					disableable))
			}
			content.WriteString("\n")
		}

		// Usage examples section
		content.WriteString("## Usage Examples\n\n")
		content.WriteString(generateUsageExamples(module))
		content.WriteString("\n")

		// Permissions section
		content.WriteString("## Required Permissions\n\n")
		content.WriteString(generatePermissionsSection(module))
		content.WriteString("\n")

		// Write file
		if config.DryRun {
			log.Infof("[DRY RUN] Would write: %s (%d bytes)", moduleFile, content.Len())
			log.Debugf("Content preview:\n%s", truncateString(content.String(), 500))
		} else {
			if err := os.MkdirAll(moduleDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", moduleDir, err)
			}

			if err := os.WriteFile(moduleFile, []byte(content.String()), 0644); err != nil {
				return fmt.Errorf("failed to write module doc %s: %w", moduleFile, err)
			}

			log.Infof("Generated: commands/%s/index.md", module.Name)
		}
	}

	return nil
}

// generateCommandReference generates api-reference/commands.md with all commands
func generateCommandReference(modules []Module, outputPath string) error {
	refDir := filepath.Join(outputPath, "api-reference")
	refFile := filepath.Join(refDir, "commands.md")

	log.Debug("Generating command reference")

	var content strings.Builder

	// Starlight frontmatter
	content.WriteString("---\n")
	content.WriteString("title: Command Reference\n")
	content.WriteString("description: Complete reference of all Alita Robot commands\n")
	content.WriteString("---\n\n")

	content.WriteString("# 🤖 Command Reference\n\n")
	content.WriteString("This page provides a complete reference of all commands available in Alita Robot.\n\n")

	// Summary statistics
	totalCommands := 0
	for _, module := range modules {
		totalCommands += len(module.Commands)
	}

	content.WriteString("## Overview\n\n")
	content.WriteString(fmt.Sprintf("- **Total Modules**: %d\n", len(modules)))
	content.WriteString(fmt.Sprintf("- **Total Commands**: %d\n", totalCommands))
	content.WriteString("\n")

	// Commands by module
	content.WriteString("## Commands by Module\n\n")

	for _, module := range modules {
		if len(module.Commands) == 0 {
			continue
		}

		emoji := getModuleEmoji(module.Name)
		content.WriteString(fmt.Sprintf("### %s %s\n\n", emoji, module.DisplayName))

		// Sort commands alphabetically
		sortedCmds := make([]Command, len(module.Commands))
		copy(sortedCmds, module.Commands)
		sort.Slice(sortedCmds, func(i, j int) bool {
			return sortedCmds[i].Name < sortedCmds[j].Name
		})

		content.WriteString("| Command | Handler | Disableable | Aliases |\n")
		content.WriteString("|---------|---------|-------------|----------|\n")

		for _, cmd := range sortedCmds {
			disableable := "✅"
			if !cmd.Disableable {
				disableable = "❌"
			}

			aliases := "—"
			if len(cmd.Aliases) > 0 {
				aliases = strings.Join(cmd.Aliases, ", ")
			}

			content.WriteString(fmt.Sprintf("| `/%s` | `%s` | %s | %s |\n",
				cmd.Name,
				cmd.Handler,
				disableable,
				aliases))
		}
		content.WriteString("\n")
	}

	// Alphabetical index
	content.WriteString("## Alphabetical Index\n\n")

	// Collect all commands
	var allCommands []Command
	for _, module := range modules {
		allCommands = append(allCommands, module.Commands...)
	}

	// Sort alphabetically
	sort.Slice(allCommands, func(i, j int) bool {
		return allCommands[i].Name < allCommands[j].Name
	})

	content.WriteString("| Command | Module | Handler |\n")
	content.WriteString("|---------|--------|----------|\n")

	for _, cmd := range allCommands {
		content.WriteString(fmt.Sprintf("| `/%s` | %s | `%s` |\n",
			cmd.Name,
			cmd.Module,
			cmd.Handler))
	}
	content.WriteString("\n")

	// Write file
	if config.DryRun {
		log.Infof("[DRY RUN] Would write: %s (%d bytes)", refFile, content.Len())
	} else {
		if err := os.MkdirAll(refDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", refDir, err)
		}

		if err := os.WriteFile(refFile, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("failed to write command reference: %w", err)
		}

		log.Info("Generated: api-reference/commands.md")
	}

	return nil
}

// generateEnvReference generates api-reference/environment.md with environment variables
func generateEnvReference(envVars []EnvVar, outputPath string) error {
	refDir := filepath.Join(outputPath, "api-reference")
	refFile := filepath.Join(refDir, "environment.md")

	log.Debug("Generating environment reference")

	var content strings.Builder

	// Starlight frontmatter
	content.WriteString("---\n")
	content.WriteString("title: Environment Variables\n")
	content.WriteString("description: Configuration reference for all environment variables\n")
	content.WriteString("---\n\n")

	content.WriteString("# ⚙️ Environment Variables\n\n")
	content.WriteString("This page documents all environment variables used to configure Alita Robot.\n\n")

	// Group by category
	categories := make(map[string][]EnvVar)
	categoryOrder := []string{}

	for _, env := range envVars {
		category := env.Category
		if category == "" {
			category = "General"
		}

		if _, exists := categories[category]; !exists {
			categoryOrder = append(categoryOrder, category)
		}

		categories[category] = append(categories[category], env)
	}

	// Sort categories (keep General first if it exists)
	sort.Slice(categoryOrder, func(i, j int) bool {
		if categoryOrder[i] == "General" {
			return true
		}
		if categoryOrder[j] == "General" {
			return false
		}
		return categoryOrder[i] < categoryOrder[j]
	})

	// Generate content for each category
	for _, category := range categoryOrder {
		vars := categories[category]

		// Sort variables within category
		sort.Slice(vars, func(i, j int) bool {
			// Required variables first
			if vars[i].Required && !vars[j].Required {
				return true
			}
			if !vars[i].Required && vars[j].Required {
				return false
			}
			return vars[i].Name < vars[j].Name
		})

		emoji := getCategoryEmoji(category)
		content.WriteString(fmt.Sprintf("## %s %s\n\n", emoji, category))

		for _, env := range vars {
			// Variable heading
			required := ""
			if env.Required {
				required = " (Required)"
			}
			content.WriteString(fmt.Sprintf("### `%s`%s\n\n", env.Name, required))

			// Description
			if env.Description != "" {
				content.WriteString(fmt.Sprintf("%s\n\n", env.Description))
			}

			// Details table
			content.WriteString("| Property | Value |\n")
			content.WriteString("|----------|-------|\n")
			content.WriteString(fmt.Sprintf("| **Type** | `%s` |\n", env.Type))
			content.WriteString(fmt.Sprintf("| **Required** | %s |\n", boolToYesNo(env.Required)))

			if env.Default != "" {
				content.WriteString(fmt.Sprintf("| **Default** | `%s` |\n", env.Default))
			}

			if env.Validation != "" {
				content.WriteString(fmt.Sprintf("| **Validation** | %s |\n", env.Validation))
			}

			content.WriteString("\n")
		}
	}

	// Quick reference section
	content.WriteString("## Quick Reference\n\n")
	content.WriteString("### Required Variables\n\n")

	var requiredVars []EnvVar
	for _, env := range envVars {
		if env.Required {
			requiredVars = append(requiredVars, env)
		}
	}

	if len(requiredVars) > 0 {
		sort.Slice(requiredVars, func(i, j int) bool {
			return requiredVars[i].Name < requiredVars[j].Name
		})

		content.WriteString("```bash\n")
		for _, env := range requiredVars {
			content.WriteString(fmt.Sprintf("%s=\n", env.Name))
		}
		content.WriteString("```\n\n")
	}

	content.WriteString("### Optional Variables\n\n")

	var optionalVars []EnvVar
	for _, env := range envVars {
		if !env.Required {
			optionalVars = append(optionalVars, env)
		}
	}

	if len(optionalVars) > 0 {
		sort.Slice(optionalVars, func(i, j int) bool {
			return optionalVars[i].Name < optionalVars[j].Name
		})

		content.WriteString("```bash\n")
		for _, env := range optionalVars {
			defaultValue := env.Default
			if defaultValue == "" {
				defaultValue = "# (optional)"
			}
			content.WriteString(fmt.Sprintf("%s=%s\n", env.Name, defaultValue))
		}
		content.WriteString("```\n\n")
	}

	// Write file
	if config.DryRun {
		log.Infof("[DRY RUN] Would write: %s (%d bytes)", refFile, content.Len())
	} else {
		if err := os.MkdirAll(refDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", refDir, err)
		}

		if err := os.WriteFile(refFile, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("failed to write environment reference: %w", err)
		}

		log.Info("Generated: api-reference/environment.md")
	}

	return nil
}

// generateSchemaReference generates api-reference/database-schema.md
func generateSchemaReference(tables []DBTable, outputPath string) error {
	refDir := filepath.Join(outputPath, "api-reference")
	refFile := filepath.Join(refDir, "database-schema.md")

	log.Debug("Generating database schema reference")

	var content strings.Builder

	// Starlight frontmatter
	content.WriteString("---\n")
	content.WriteString("title: Database Schema\n")
	content.WriteString("description: Complete reference of the PostgreSQL database schema\n")
	content.WriteString("---\n\n")

	content.WriteString("# 🗄️ Database Schema\n\n")
	content.WriteString("This page documents the complete PostgreSQL database schema for Alita Robot.\n\n")

	// Overview
	content.WriteString("## Overview\n\n")
	content.WriteString(fmt.Sprintf("- **Total Tables**: %d\n", len(tables)))
	content.WriteString("- **Database Type**: PostgreSQL\n")
	content.WriteString("- **ORM**: GORM\n")
	content.WriteString("- **Migration Tool**: golang-migrate\n\n")

	// Key patterns
	content.WriteString("## Design Patterns\n\n")
	content.WriteString("### Surrogate Key Pattern\n\n")
	content.WriteString("All tables use an auto-incremented `id` field as the primary key (internal identifier), ")
	content.WriteString("while external identifiers like `user_id` and `chat_id` (Telegram IDs) are stored with unique constraints.\n\n")

	content.WriteString("**Benefits:**\n\n")
	content.WriteString("- Decouples internal schema from external systems\n")
	content.WriteString("- Provides stability if external IDs change\n")
	content.WriteString("- Simplifies GORM operations with consistent integer primary keys\n")
	content.WriteString("- Better performance for joins and indexing\n\n")

	// Sort tables alphabetically
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})

	// Generate table documentation
	content.WriteString("## Tables\n\n")

	for _, table := range tables {
		content.WriteString(fmt.Sprintf("### `%s`\n\n", table.Name))

		if table.Description != "" {
			content.WriteString(fmt.Sprintf("%s\n\n", table.Description))
		}

		// Columns table
		content.WriteString("#### Columns\n\n")
		content.WriteString("| Column | Type | Nullable | Default | Constraints |\n")
		content.WriteString("|--------|------|----------|---------|-------------|\n")

		for _, col := range table.Columns {
			constraints := []string{}
			if col.PrimaryKey {
				constraints = append(constraints, "PRIMARY KEY")
			}
			if col.Unique {
				constraints = append(constraints, "UNIQUE")
			}

			nullable := "❌"
			if col.Nullable {
				nullable = "✅"
			}

			defaultVal := "—"
			if col.Default != "" {
				defaultVal = fmt.Sprintf("`%s`", col.Default)
			}

			constraintStr := "—"
			if len(constraints) > 0 {
				constraintStr = strings.Join(constraints, ", ")
			}

			content.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s | %s |\n",
				col.Name,
				col.Type,
				nullable,
				defaultVal,
				constraintStr))
		}
		content.WriteString("\n")

		// Indexes
		if len(table.Indexes) > 0 {
			content.WriteString("#### Indexes\n\n")
			for _, index := range table.Indexes {
				content.WriteString(fmt.Sprintf("- %s\n", index))
			}
			content.WriteString("\n")
		}

		// Foreign keys
		if len(table.ForeignKeys) > 0 {
			content.WriteString("#### Foreign Keys\n\n")
			for _, fk := range table.ForeignKeys {
				content.WriteString(fmt.Sprintf("- %s\n", fk))
			}
			content.WriteString("\n")
		}
	}

	// Entity Relationship Diagram section
	content.WriteString("## Entity Relationships\n\n")
	content.WriteString("### Core Entities\n\n")
	content.WriteString("- **Users**: Telegram users who interact with the bot\n")
	content.WriteString("- **Chats**: Telegram groups/channels managed by the bot\n")
	content.WriteString("- **ChatUsers**: Join table linking users to chats\n\n")

	content.WriteString("### Relationship Patterns\n\n")
	content.WriteString("- User ↔ Chat: Many-to-many through `chat_users`\n")
	content.WriteString("- Chat → Settings: One-to-one (module-specific settings)\n")
	content.WriteString("- Chat → Content: One-to-many (filters, notes, rules, etc.)\n\n")

	// Write file
	if config.DryRun {
		log.Infof("[DRY RUN] Would write: %s (%d bytes)", refFile, content.Len())
	} else {
		if err := os.MkdirAll(refDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", refDir, err)
		}

		if err := os.WriteFile(refFile, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("failed to write schema reference: %w", err)
		}

		log.Info("Generated: api-reference/database-schema.md")
	}

	return nil
}

// generateCommandsOverview generates commands/index.md with overview
func generateCommandsOverview(modules []Module, outputPath string) error {
	commandsDir := filepath.Join(outputPath, "commands")
	overviewFile := filepath.Join(commandsDir, "index.md")

	log.Debug("Generating commands overview")

	var content strings.Builder

	// Starlight frontmatter
	content.WriteString("---\n")
	content.WriteString("title: Commands Overview\n")
	content.WriteString("description: Overview of all command modules and categories\n")
	content.WriteString("---\n\n")

	content.WriteString("# 📚 Commands Overview\n\n")
	content.WriteString("Alita Robot provides a comprehensive set of commands organized into modules. ")
	content.WriteString("Each module handles a specific aspect of group management.\n\n")

	// Statistics
	totalCommands := 0
	for _, module := range modules {
		totalCommands += len(module.Commands)
	}

	content.WriteString("## Quick Stats\n\n")
	content.WriteString(fmt.Sprintf("- **Total Modules**: %d\n", len(modules)))
	content.WriteString(fmt.Sprintf("- **Total Commands**: %d\n", totalCommands))
	content.WriteString("\n")

	// Module categories
	content.WriteString("## Module Categories\n\n")

	// Group modules by category
	categories := map[string][]Module{
		"Administration": {},
		"Moderation":     {},
		"Content":        {},
		"User Tools":     {},
		"Bot Management": {},
	}

	for _, module := range modules {
		category := categorizeModule(module.Name)
		categories[category] = append(categories[category], module)
	}

	// Define category order
	categoryOrder := []string{"Administration", "Moderation", "Content", "User Tools", "Bot Management"}

	for _, category := range categoryOrder {
		modules := categories[category]
		if len(modules) == 0 {
			continue
		}

		// Sort modules within category
		sort.Slice(modules, func(i, j int) bool {
			return modules[i].DisplayName < modules[j].DisplayName
		})

		emoji := getCategoryEmoji(category)
		content.WriteString(fmt.Sprintf("### %s %s\n\n", emoji, category))

		for _, module := range modules {
			moduleEmoji := getModuleEmoji(module.Name)
			commandCount := len(module.Commands)

			content.WriteString(fmt.Sprintf("#### [%s %s](./%s/)\n\n",
				moduleEmoji,
				module.DisplayName,
				module.Name))

			// Extract first line of help text as summary
			summary := extractFirstSentence(module.HelpText)
			content.WriteString(fmt.Sprintf("%s\n\n", summary))

			content.WriteString(fmt.Sprintf("**Commands**: %d", commandCount))
			if len(module.Aliases) > 0 {
				content.WriteString(fmt.Sprintf(" | **Aliases**: %s", strings.Join(module.Aliases, ", ")))
			}
			content.WriteString("\n\n")
		}
	}

	// Usage guide
	content.WriteString("## Getting Started\n\n")
	content.WriteString("### Basic Command Syntax\n\n")
	content.WriteString("All commands follow this format:\n\n")
	content.WriteString("```\n")
	content.WriteString("/command [required_argument] [optional_argument]\n")
	content.WriteString("```\n\n")

	content.WriteString("### Command Prefixes\n\n")
	content.WriteString("Commands can be used with or without the bot username:\n\n")
	content.WriteString("- `/start` - Works in private chat or group\n")
	content.WriteString("- `/start@AlitaRobot` - Explicitly targets this bot in groups\n\n")

	content.WriteString("### Getting Help\n\n")
	content.WriteString("- `/help` - Show general help and module list\n")
	content.WriteString("- `/help <module>` - Show detailed help for a specific module\n")
	content.WriteString("- `/cmds <module>` - List all commands in a module\n\n")

	content.WriteString("### Permission Levels\n\n")
	content.WriteString("Commands require different permission levels:\n\n")
	content.WriteString("- **Everyone**: All group members can use\n")
	content.WriteString("- **Admin**: Requires admin rights in the group\n")
	content.WriteString("- **Owner**: Requires group creator/owner status\n")
	content.WriteString("- **Dev**: Requires bot developer access\n\n")

	// Write file
	if config.DryRun {
		log.Infof("[DRY RUN] Would write: %s (%d bytes)", overviewFile, content.Len())
	} else {
		if err := os.MkdirAll(commandsDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", commandsDir, err)
		}

		if err := os.WriteFile(overviewFile, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("failed to write commands overview: %w", err)
		}

		log.Info("Generated: commands/index.md")
	}

	return nil
}

// Helper functions

// convertTelegramMarkdown converts Telegram markdown to standard Markdown
func convertTelegramMarkdown(text string) string {
	// Remove HTML tags that might be in help text
	text = strings.ReplaceAll(text, "<b>", "**")
	text = strings.ReplaceAll(text, "</b>", "**")
	text = strings.ReplaceAll(text, "<i>", "*")
	text = strings.ReplaceAll(text, "</i>", "*")
	text = strings.ReplaceAll(text, "<code>", "`")
	text = strings.ReplaceAll(text, "</code>", "`")

	// Convert bullet points
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "•") || strings.HasPrefix(trimmed, "·") {
			lines[i] = strings.Replace(line, "•", "-", 1)
			lines[i] = strings.Replace(lines[i], "·", "-", 1)
		}
	}

	return strings.Join(lines, "\n")
}

// extractCommandDescription attempts to extract description for a command from help text
func extractCommandDescription(cmdName, helpText string) string {
	// Try to find command in help text with various patterns
	lines := strings.Split(helpText, "\n")

	for _, line := range lines {
		// Look for lines mentioning the command
		if strings.Contains(strings.ToLower(line), "/"+strings.ToLower(cmdName)) {
			// Extract description after command
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				desc := strings.TrimSpace(parts[1])
				// Clean up markdown
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
		limit := 3
		if len(module.Commands) < limit {
			limit = len(module.Commands)
		}

		content.WriteString("```\n")
		for i := 0; i < limit; i++ {
			cmd := module.Commands[i]
			content.WriteString(fmt.Sprintf("/%s\n", cmd.Name))
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

// getCategoryEmoji returns an appropriate emoji for a category
func getCategoryEmoji(category string) string {
	emojiMap := map[string]string{
		"Administration": "👑",
		"Moderation":     "🛡️",
		"Content":        "📝",
		"User Tools":     "🔧",
		"Bot Management": "⚙️",
		"General":        "🌐",
		"Core":           "💎",
		"Database":       "🗄️",
		"Performance":    "⚡",
		"Security":       "🔒",
		"HTTP Server":    "🌐",
		"Webhook":        "🔗",
		"Monitoring":     "📊",
	}

	if emoji, exists := emojiMap[category]; exists {
		return emoji
	}

	return "📂"
}

// categorizeModule assigns a module to a category
func categorizeModule(moduleName string) string {
	categories := map[string]string{
		"admin":      "Administration",
		"bans":       "Moderation",
		"locks":      "Moderation",
		"muting":     "Moderation",
		"purges":     "Moderation",
		"warnings":   "Moderation",
		"antiflood":  "Moderation",
		"blacklist":  "Moderation",
		"approval":   "Moderation",
		"captcha":    "Moderation",
		"filters":    "Content",
		"notes":      "Content",
		"rules":      "Content",
		"greetings":  "Content",
		"reporting":  "User Tools",
		"language":   "User Tools",
		"info":       "User Tools",
		"gtranslate": "User Tools",
		"karma":      "User Tools",
		"formatting": "User Tools",
		"connection": "Bot Management",
		"disabling":  "Bot Management",
		"extras":     "Bot Management",
		"misc":       "Bot Management",
	}

	if category, exists := categories[moduleName]; exists {
		return category
	}

	return "Bot Management"
}

// extractFirstSentence extracts the first sentence from text
func extractFirstSentence(text string) string {
	// Clean up markdown first
	text = convertTelegramMarkdown(text)

	// Find first sentence (up to . or newline)
	sentences := strings.SplitN(text, ".", 2)
	if len(sentences) > 0 {
		first := strings.TrimSpace(sentences[0])
		// Also check for newline
		lines := strings.SplitN(first, "\n", 2)
		return strings.TrimSpace(lines[0]) + "."
	}

	return text
}

// boolToYesNo converts boolean to Yes/No
func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// truncateString truncates a string to maxLen
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
