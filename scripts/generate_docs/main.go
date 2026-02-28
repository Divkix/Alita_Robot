// Package main provides a documentation generator for Alita Robot.
// It parses the codebase and generates Markdown documentation for Astro/Starlight.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Config holds the generator configuration
type Config struct {
	// Source paths (relative to project root)
	LocalesPath    string
	ModulesPath    string
	ConfigPath     string
	MigrationsPath string
	SampleEnvPath  string
	ChatStatusPath string // Path to chat_status.go for permission parsing

	// Output path
	DocsOutputPath string

	// Options
	Verbose bool
	DryRun  bool
}

// Module represents a bot module with its commands and help text
type Module struct {
	Name         string
	DisplayName  string
	HelpText     string
	Commands     []Command
	Aliases      []string
	ExtendedDocs ExtendedDocs // Extended documentation fields
}

// Command represents a bot command
type Command struct {
	Name        string
	Handler     string
	Module      string
	Disableable bool
	Aliases     []string
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name        string
	Type        string
	Required    bool
	Default     string
	Description string
	Validation  string
	Category    string
}

// DBTable represents a database table
type DBTable struct {
	Name        string
	Description string
	Columns     []DBColumn
	Indexes     []string
	ForeignKeys []string
}

// DBColumn represents a database column
type DBColumn struct {
	Name       string
	Type       string
	Nullable   bool
	Default    string
	PrimaryKey bool
	Unique     bool
}

// Callback represents a callback query handler registration
type Callback struct {
	Prefix     string // e.g., "restrict."
	Handler    string // e.g., "restrictButtonHandler"
	Module     string // e.g., "bans"
	SourceFile string // e.g., "bans.go"
}

// PermissionFunc represents a permission checking function
type PermissionFunc struct {
	Name        string   // e.g., "CanUserRestrict"
	Signature   string   // Full function signature
	Parameters  []string // Parameter list
	ReturnType  string   // e.g., "bool"
	Category    string   // e.g., "User Permission Checks"
	Description string   // From comment above function
}

var config Config

func main() {
	// Parse flags
	flag.StringVar(&config.LocalesPath, "locales", "locales", "Path to locales directory")
	flag.StringVar(&config.ModulesPath, "modules", "alita/modules", "Path to modules directory")
	flag.StringVar(&config.ConfigPath, "config", "alita/config/config.go", "Path to config.go")
	flag.StringVar(&config.MigrationsPath, "migrations", "migrations", "Path to migrations directory")
	flag.StringVar(&config.SampleEnvPath, "sample-env", "sample.env", "Path to sample.env")
	flag.StringVar(&config.ChatStatusPath, "chat-status", "alita/utils/chat_status/chat_status.go", "Path to chat_status.go")
	flag.StringVar(&config.DocsOutputPath, "output", "docs/src/content/docs", "Output path for generated docs")
	var inventoryMode bool
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Print what would be generated without writing files")
	flag.BoolVar(&inventoryMode, "inventory", false, "Generate canonical command inventory instead of docs")
	flag.Parse()

	if config.Verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}
	log.Debugf("Project root: %s", projectRoot)

	if inventoryMode {
		if err := generateInventory(projectRoot); err != nil {
			log.Fatalf("Failed to generate inventory: %v", err)
		}
		return
	}

	log.Info("🚀 Starting Alita Robot Documentation Generator")

	// Parse all sources
	log.Info("📖 Parsing translations...")
	translations, extendedDocs, moduleAliases, err := parseTranslations(filepath.Join(projectRoot, config.LocalesPath))
	if err != nil {
		log.Fatalf("Failed to parse translations: %v", err)
	}
	log.Infof("   Found %d module help texts", len(translations))
	log.Infof("   Found %d modules with extended documentation", len(extendedDocs))

	log.Info("🔍 Parsing commands from source...")
	commands, err := parseCommands(filepath.Join(projectRoot, config.ModulesPath))
	if err != nil {
		log.Fatalf("Failed to parse commands: %v", err)
	}
	log.Infof("   Found %d commands", len(commands))

	log.Info("⚙️  Parsing environment variables...")
	envVars, err := parseConfigStruct(filepath.Join(projectRoot, config.ConfigPath))
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}
	log.Infof("   Found %d environment variables", len(envVars))

	log.Info("🗄️  Parsing database schema...")
	tables, err := parseMigrations(filepath.Join(projectRoot, config.MigrationsPath))
	if err != nil {
		log.Fatalf("Failed to parse migrations: %v", err)
	}
	log.Infof("   Found %d database tables", len(tables))

	// Parse callbacks
	log.Info("🔔 Parsing callbacks from modules...")
	callbacks, err := parseCallbacks(filepath.Join(projectRoot, config.ModulesPath))
	if err != nil {
		log.Fatalf("Failed to parse callbacks: %v", err)
	}
	log.Infof("   Found %d callback handlers", len(callbacks))

	// Parse permissions
	log.Info("🔐 Parsing permission functions...")
	permissions, err := parsePermissions(filepath.Join(projectRoot, config.ChatStatusPath))
	if err != nil {
		log.Fatalf("Failed to parse permissions: %v", err)
	}
	log.Infof("   Found %d permission functions", len(permissions))

	// Parse lock types
	log.Info("🔒 Parsing lock types...")
	lockTypes, err := parseLockTypes(filepath.Join(projectRoot, config.ModulesPath, "locks.go"))
	if err != nil {
		log.Fatalf("Failed to parse lock types: %v", err)
	}
	log.Infof("   Found %d lock types (%d permission locks, %d restriction locks)",
		len(lockTypes),
		countLocksByCategory(lockTypes, "permission"),
		countLocksByCategory(lockTypes, "restriction"))

	// Build modules with their commands
	modules := buildModules(translations, extendedDocs, commands, moduleAliases)
	log.Infof("📦 Built %d modules with commands", len(modules))

	// Generate documentation
	outputPath := filepath.Join(projectRoot, config.DocsOutputPath)

	log.Info("📝 Generating documentation...")

	if err := generateModuleDocs(modules, outputPath); err != nil {
		log.Fatalf("Failed to generate module docs: %v", err)
	}

	if err := generateCommandReference(modules, outputPath); err != nil {
		log.Fatalf("Failed to generate command reference: %v", err)
	}

	if err := generateEnvReference(envVars, outputPath); err != nil {
		log.Fatalf("Failed to generate env reference: %v", err)
	}

	if err := generateSchemaReference(tables, outputPath); err != nil {
		log.Fatalf("Failed to generate schema reference: %v", err)
	}

	if err := generateCommandsOverview(modules, outputPath); err != nil {
		log.Fatalf("Failed to generate commands overview: %v", err)
	}

	// Generate callbacks reference
	if err := generateCallbacksReference(callbacks, outputPath); err != nil {
		log.Fatalf("Failed to generate callbacks reference: %v", err)
	}
	log.Info("Generated: api-reference/callbacks.md")

	// Generate permissions reference
	if err := generatePermissionsReference(permissions, outputPath); err != nil {
		log.Fatalf("Failed to generate permissions reference: %v", err)
	}
	log.Info("Generated: api-reference/permissions.md")

	// Generate lock types reference
	if err := generateLockTypesReference(lockTypes, outputPath); err != nil {
		log.Fatalf("Failed to generate lock types reference: %v", err)
	}
	log.Info("Generated: api-reference/lock-types.md")

	log.Info("✅ Documentation generation complete!")
	log.Infof("   Output: %s", outputPath)
}

// InventoryModule represents a module in the canonical inventory
type InventoryModule struct {
	Module           string              `json:"module"`
	SourceFile       string              `json:"source_file"`
	Internal         bool                `json:"internal,omitempty"`
	Commands         []InventoryCommand  `json:"commands"`
	Callbacks        []InventoryCallback `json:"callbacks"`
	MessageWatchers  []InventoryWatcher  `json:"message_watchers"`
	HasDocsDirectory bool                `json:"has_docs_directory"`
	DocsPath         string              `json:"docs_path,omitempty"`
}

// InventoryCommand represents a command entry in the inventory
type InventoryCommand struct {
	Command             string   `json:"command"`
	Aliases             []string `json:"aliases,omitempty"`
	HandlerGroup        int      `json:"handler_group"`
	Disableable         bool     `json:"disableable"`
	RegistrationPattern string   `json:"registration_pattern"`
}

// InventoryCallback represents a callback entry in the inventory
type InventoryCallback struct {
	Prefix  string `json:"prefix"`
	Handler string `json:"handler"`
}

// InventoryWatcher represents a message watcher entry in the inventory
type InventoryWatcher struct {
	Handler      string `json:"handler"`
	HandlerGroup int    `json:"handler_group"`
}

// generateInventory produces the canonical command inventory
func generateInventory(projectRoot string) error {
	log.Info("Generating canonical command inventory...")

	modulesPath := filepath.Join(projectRoot, config.ModulesPath)

	// Parse all data
	commands, err := parseCommands(modulesPath)
	if err != nil {
		return fmt.Errorf("failed to parse commands: %w", err)
	}
	log.Infof("Parsed %d commands", len(commands))

	callbacks, err := parseCallbacks(modulesPath)
	if err != nil {
		return fmt.Errorf("failed to parse callbacks: %w", err)
	}
	log.Infof("Parsed %d callbacks", len(callbacks))

	watchers, err := parseMessageWatchers(modulesPath)
	if err != nil {
		return fmt.Errorf("failed to parse message watchers: %w", err)
	}
	log.Infof("Parsed %d message watchers", len(watchers))

	// Non-module helper files to skip
	helperFiles := map[string]bool{
		"helpers.go":                  true,
		"moderation_input.go":         true,
		"callback_codec.go":           true,
		"callback_parse_overwrite.go": true,
		"chat_permissions.go":         true,
		"connections_auth.go":         true,
		"rules_format.go":             true,
	}

	// Discover module files
	files, err := filepath.Glob(filepath.Join(modulesPath, "*.go"))
	if err != nil {
		return fmt.Errorf("failed to glob modules: %w", err)
	}

	moduleSet := make(map[string]string) // module name -> source file
	for _, file := range files {
		fileName := filepath.Base(file)

		// Skip test files and helper files
		if strings.HasSuffix(fileName, "_test.go") || helperFiles[fileName] {
			continue
		}

		moduleName := strings.TrimSuffix(fileName, ".go")
		moduleSet[moduleName] = fileName
	}

	// Group commands by module
	cmdsByModule := make(map[string][]Command)
	for _, cmd := range commands {
		cmdsByModule[cmd.Module] = append(cmdsByModule[cmd.Module], cmd)
	}

	// Group callbacks by module
	cbsByModule := make(map[string][]Callback)
	for _, cb := range callbacks {
		cbsByModule[cb.Module] = append(cbsByModule[cb.Module], cb)
	}

	// Group watchers by module
	watchersByModule := make(map[string][]MessageWatcher)
	for _, w := range watchers {
		watchersByModule[w.Module] = append(watchersByModule[w.Module], w)
	}

	// Docs directory path
	docsBasePath := filepath.Join(projectRoot, "docs", "src", "content", "docs", "commands")

	// Build inventory
	var inventory []InventoryModule
	for moduleName, sourceFile := range moduleSet {
		invMod := InventoryModule{
			Module:     moduleName,
			SourceFile: filepath.Join("alita", "modules", sourceFile),
		}

		// Mark bot_updates as internal
		if moduleName == "bot_updates" {
			invMod.Internal = true
		}

		// Check for docs directory (handle naming mismatches)
		docsName := moduleName
		namingMap := map[string]string{
			"mute":     "mutes",
			"language": "languages",
		}
		if mapped, ok := namingMap[moduleName]; ok {
			docsName = mapped
		}

		docsDir := filepath.Join(docsBasePath, docsName)
		if _, statErr := os.Stat(docsDir); statErr == nil {
			invMod.HasDocsDirectory = true
			invMod.DocsPath = filepath.Join("docs", "src", "content", "docs", "commands", docsName)
		}

		// Add commands
		// Try exact match first, then try with variations
		modCmds := cmdsByModule[moduleName]
		if len(modCmds) == 0 {
			// Try plural/singular variations
			for modKey, cmds := range cmdsByModule {
				if strings.ToLower(modKey) == moduleName ||
					strings.ToLower(modKey)+"s" == moduleName ||
					strings.ToLower(modKey) == moduleName+"s" {
					modCmds = append(modCmds, cmds...)
				}
			}
		}

		for _, cmd := range modCmds {
			regPattern := "NewCommand"
			if len(cmd.Aliases) > 0 {
				regPattern = "MultiCommand"
			}
			invCmd := InventoryCommand{
				Command:             cmd.Name,
				Aliases:             cmd.Aliases,
				HandlerGroup:        0, // Default group for commands
				Disableable:         cmd.Disableable,
				RegistrationPattern: regPattern,
			}
			invMod.Commands = append(invMod.Commands, invCmd)
		}

		// Add callbacks
		modCbs := cbsByModule[moduleName]
		if len(modCbs) == 0 {
			for modKey, cbs := range cbsByModule {
				if strings.ToLower(modKey) == moduleName ||
					strings.ToLower(modKey)+"s" == moduleName ||
					strings.ToLower(modKey) == moduleName+"s" {
					modCbs = append(modCbs, cbs...)
				}
			}
		}
		for _, cb := range modCbs {
			invMod.Callbacks = append(invMod.Callbacks, InventoryCallback{
				Prefix:  cb.Prefix,
				Handler: cb.Handler,
			})
		}

		// Add message watchers
		modWatchers := watchersByModule[moduleName]
		if len(modWatchers) == 0 {
			for modKey, ws := range watchersByModule {
				if strings.ToLower(modKey) == moduleName ||
					strings.ToLower(modKey)+"s" == moduleName ||
					strings.ToLower(modKey) == moduleName+"s" {
					modWatchers = append(modWatchers, ws...)
				}
			}
		}
		for _, w := range modWatchers {
			invMod.MessageWatchers = append(invMod.MessageWatchers, InventoryWatcher{
				Handler:      w.Handler,
				HandlerGroup: w.HandlerGroup,
			})
		}

		inventory = append(inventory, invMod)
	}

	// Sort by module name
	sort.Slice(inventory, func(i, j int) bool {
		return inventory[i].Module < inventory[j].Module
	})

	// Ensure .planning directory exists
	planningDir := filepath.Join(projectRoot, ".planning")
	if err := os.MkdirAll(planningDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .planning directory: %w", err)
	}

	// Write INVENTORY.json
	jsonData, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal inventory: %w", err)
	}

	jsonPath := filepath.Join(planningDir, "INVENTORY.json")
	if err := os.WriteFile(jsonPath, jsonData, 0o644); err != nil {
		return fmt.Errorf("failed to write INVENTORY.json: %w", err)
	}
	log.Infof("Written: %s", jsonPath)

	// Write INVENTORY.md
	mdContent := generateInventoryMarkdown(inventory)
	mdPath := filepath.Join(planningDir, "INVENTORY.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		return fmt.Errorf("failed to write INVENTORY.md: %w", err)
	}
	log.Infof("Written: %s", mdPath)

	log.Info("Inventory generation complete!")
	return nil
}

// generateInventoryMarkdown produces the human-readable inventory
func generateInventoryMarkdown(inventory []InventoryModule) string {
	var sb strings.Builder

	// Count totals
	totalCommands := 0
	totalCallbacks := 0
	totalWatchers := 0
	totalDisableable := 0
	modulesWithDocs := 0
	modulesWithoutDocs := 0

	for _, mod := range inventory {
		totalCommands += len(mod.Commands)
		totalCallbacks += len(mod.Callbacks)
		totalWatchers += len(mod.MessageWatchers)
		for _, cmd := range mod.Commands {
			if cmd.Disableable {
				totalDisableable++
			}
		}
		if mod.HasDocsDirectory {
			modulesWithDocs++
		} else {
			modulesWithoutDocs++
		}
	}

	sb.WriteString("# Canonical Command Inventory\n\n")
	sb.WriteString("**Generated:** auto-generated by `make inventory`\n\n")
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Modules:** %d\n", len(inventory)))
	sb.WriteString(fmt.Sprintf("- **Commands:** %d\n", totalCommands))
	sb.WriteString(fmt.Sprintf("- **Callbacks:** %d\n", totalCallbacks))
	sb.WriteString(fmt.Sprintf("- **Message Watchers:** %d\n", totalWatchers))
	sb.WriteString(fmt.Sprintf("- **Disableable Commands:** %d\n", totalDisableable))
	sb.WriteString(fmt.Sprintf("- **Modules with Docs:** %d\n", modulesWithDocs))
	sb.WriteString(fmt.Sprintf("- **Modules without Docs:** %d\n", modulesWithoutDocs))
	sb.WriteString("\n")

	// Summary table
	sb.WriteString("## Module Overview\n\n")
	sb.WriteString("| Module | Commands | Aliases | Callbacks | Watchers | Disableable | Has Docs |\n")
	sb.WriteString("|--------|----------|---------|-----------|----------|-------------|----------|\n")

	for _, mod := range inventory {
		aliasCount := 0
		disableCount := 0
		for _, cmd := range mod.Commands {
			if len(cmd.Aliases) > 0 {
				aliasCount++
			}
			if cmd.Disableable {
				disableCount++
			}
		}

		hasDocs := "No"
		if mod.HasDocsDirectory {
			hasDocs = "Yes"
		}
		if mod.Internal {
			hasDocs = "N/A (internal)"
		}

		moduleName := mod.Module
		if mod.Internal {
			moduleName = fmt.Sprintf("%s (internal)", mod.Module)
		}

		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %s |\n",
			moduleName,
			len(mod.Commands),
			aliasCount,
			len(mod.Callbacks),
			len(mod.MessageWatchers),
			disableCount,
			hasDocs,
		))
	}

	sb.WriteString("\n")

	// Module-to-Docs Mapping
	sb.WriteString("## Module-to-Docs Mapping\n\n")
	sb.WriteString("| Module | Source File | Docs Directory | Status |\n")
	sb.WriteString("|--------|-----------|----------------|--------|\n")

	for _, mod := range inventory {
		status := "Documented"
		docsDir := mod.DocsPath
		if mod.Internal {
			status = "Internal (no docs needed)"
			docsDir = "N/A"
		} else if !mod.HasDocsDirectory {
			status = "**Missing docs**"
			docsDir = "N/A"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			mod.Module, mod.SourceFile, docsDir, status))
	}

	sb.WriteString("\n")

	// Modules without docs detail
	sb.WriteString("## Modules Without Documentation\n\n")
	for _, mod := range inventory {
		if !mod.HasDocsDirectory && !mod.Internal {
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`): No docs directory found\n", mod.Module, mod.SourceFile))
		}
	}

	sb.WriteString("\n")

	// Naming mismatches
	sb.WriteString("## Naming Mismatches\n\n")
	sb.WriteString("These modules have source file names that don't exactly match their docs directory names:\n\n")
	namingMismatches := map[string]string{
		"mute":     "mutes",
		"language": "languages",
	}
	for src, docs := range namingMismatches {
		fmt.Fprintf(&sb, "- `%s.go` -> `docs/commands/%s/`\n", src, docs)
	}

	return sb.String()
}

// findProjectRoot finds the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// buildModules combines translations and commands into Module structs
func buildModules(translations map[string]string, extendedDocs map[string]ExtendedDocs, commands []Command, aliases map[string][]string) []Module {
	// Group commands by module
	commandsByModule := make(map[string][]Command)
	for _, cmd := range commands {
		commandsByModule[cmd.Module] = append(commandsByModule[cmd.Module], cmd)
	}

	// Track which modules we've already added
	addedModules := make(map[string]bool)
	var modules []Module

	// First, add modules from translation help texts
	for name, helpText := range translations {
		// Extract module name from key (e.g., "admin_help_msg" -> "Admin")
		moduleName := strings.TrimSuffix(name, "_help_msg")
		displayName := cases.Title(language.English).String(moduleName)

		// Normalize module name for lookup
		normalizedName := strings.ToLower(moduleName)

		// Find commands for this module
		var moduleCommands []Command
		for modName, cmds := range commandsByModule {
			if strings.ToLower(modName) == normalizedName ||
				strings.ToLower(modName) == normalizedName+"s" ||
				strings.ToLower(modName)+"s" == normalizedName {
				moduleCommands = append(moduleCommands, cmds...)
			}
		}

		// Sort commands by name
		sort.Slice(moduleCommands, func(i, j int) bool {
			return moduleCommands[i].Name < moduleCommands[j].Name
		})

		// Get extended docs for this module
		var moduleDocs ExtendedDocs
		if docs, ok := extendedDocs[normalizedName]; ok {
			moduleDocs = docs
		}

		modules = append(modules, Module{
			Name:         normalizedName,
			DisplayName:  displayName,
			HelpText:     helpText,
			Commands:     moduleCommands,
			Aliases:      aliases[displayName],
			ExtendedDocs: moduleDocs,
		})

		addedModules[normalizedName] = true
	}

	// Now check for modules that only have extended docs but no help_msg
	for moduleName, docs := range extendedDocs {
		normalizedName := strings.ToLower(moduleName)
		if !addedModules[normalizedName] {
			displayName := cases.Title(language.English).String(moduleName)

			// Find commands for this module
			var moduleCommands []Command
			for modName, cmds := range commandsByModule {
				if strings.ToLower(modName) == normalizedName ||
					strings.ToLower(modName) == normalizedName+"s" ||
					strings.ToLower(modName)+"s" == normalizedName {
					moduleCommands = append(moduleCommands, cmds...)
				}
			}

			// Sort commands by name
			sort.Slice(moduleCommands, func(i, j int) bool {
				return moduleCommands[i].Name < moduleCommands[j].Name
			})

			modules = append(modules, Module{
				Name:         normalizedName,
				DisplayName:  displayName,
				HelpText:     "", // No help message
				Commands:     moduleCommands,
				Aliases:      aliases[displayName],
				ExtendedDocs: docs,
			})
		}
	}

	// Sort modules by name
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})

	return modules
}

// countLocksByCategory returns the count of locks in a specific category
func countLocksByCategory(locks []LockType, category string) int {
	count := 0
	for _, lock := range locks {
		if lock.Category == category {
			count++
		}
	}
	return count
}
