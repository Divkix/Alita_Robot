package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// MessageWatcher represents a message handler registration
type MessageWatcher struct {
	Handler      string
	Module       string
	HandlerGroup int
	SourceFile   string
}

// LockType represents a lock type that can be enabled/disabled
type LockType struct {
	Name        string
	Description string
	Category    string // "permission" or "restriction"
}

// ExtendedDocs holds all extended documentation for a module
type ExtendedDocs struct {
	Extended    string // Technical documentation
	Features    string // Feature lists
	Permissions string // Permission requirements
	Examples    string // Usage examples
	Notes       string // Important notes/limitations
}

// parseTranslations parses locale YAML files and extracts help messages and extended docs
func parseTranslations(localesPath string) (map[string]string, map[string]ExtendedDocs, map[string][]string, error) {
	helpTexts := make(map[string]string)
	extendedDocs := make(map[string]ExtendedDocs)
	aliases := make(map[string][]string)

	// Parse en.yml for help messages and extended docs
	enPath := filepath.Join(localesPath, "en.yml")
	data, err := os.ReadFile(filepath.Clean(enPath))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read %s: %w", enPath, err)
	}

	var translations map[string]any
	if err := yaml.Unmarshal(data, &translations); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse %s: %w", enPath, err)
	}

	// Extract help messages and extended documentation
	for key, value := range translations {
		str, ok := value.(string)
		if !ok {
			continue
		}

		// Extract help messages
		if strings.HasSuffix(key, "_help_msg") {
			helpTexts[key] = str
			continue
		}

		// Extract extended documentation
		if strings.HasSuffix(key, "_extended_docs") {
			moduleName := strings.TrimSuffix(key, "_extended_docs")
			docs := extendedDocs[moduleName]
			docs.Extended = str
			extendedDocs[moduleName] = docs
			continue
		}

		if strings.HasSuffix(key, "_features_docs") {
			moduleName := strings.TrimSuffix(key, "_features_docs")
			docs := extendedDocs[moduleName]
			docs.Features = str
			extendedDocs[moduleName] = docs
			continue
		}

		if strings.HasSuffix(key, "_permissions_docs") {
			moduleName := strings.TrimSuffix(key, "_permissions_docs")
			docs := extendedDocs[moduleName]
			docs.Permissions = str
			extendedDocs[moduleName] = docs
			continue
		}

		if strings.HasSuffix(key, "_examples_docs") {
			moduleName := strings.TrimSuffix(key, "_examples_docs")
			docs := extendedDocs[moduleName]
			docs.Examples = str
			extendedDocs[moduleName] = docs
			continue
		}

		if strings.HasSuffix(key, "_notes_docs") {
			moduleName := strings.TrimSuffix(key, "_notes_docs")
			docs := extendedDocs[moduleName]
			docs.Notes = str
			extendedDocs[moduleName] = docs
			continue
		}
	}

	// Parse config.yml for aliases
	configPath := filepath.Join(localesPath, "config.yml")
	configData, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		log.Warnf("Could not read config.yml: %v", err)
		return helpTexts, extendedDocs, aliases, nil
	}

	var configYaml map[string]any
	if err := yaml.Unmarshal(configData, &configYaml); err != nil {
		log.Warnf("Could not parse config.yml: %v", err)
		return helpTexts, extendedDocs, aliases, nil
	}

	// Extract alt_names
	if altNames, ok := configYaml["alt_names"].(map[string]any); ok {
		for module, aliasList := range altNames {
			switch v := aliasList.(type) {
			case []any:
				for _, alias := range v {
					if str, ok := alias.(string); ok {
						aliases[module] = append(aliases[module], str)
					}
				}
			case string:
				aliases[module] = []string{v}
			}
		}
	}

	return helpTexts, extendedDocs, aliases, nil
}

// parseCommands parses Go source files to extract command registrations
func parseCommands(modulesPath string) ([]Command, error) {
	var commands []Command

	// Regex patterns for command extraction
	// Matches handlers.NewCommand("cmd", module.handler) or handlers.NewCommand("cmd", func(...) {...})
	newCommandPattern := regexp.MustCompile(`handlers\.NewCommand\s*\(\s*"([^"]+)"\s*,\s*(?:(\w+)(?:\(\))?\.(\w+)|func\b)`)
	disableablePattern := regexp.MustCompile(`(?:misc|helpers)\.AddCmdToDisableable\s*\(\s*"([^"]+)"\s*\)`)
	moduleNamePattern := regexp.MustCompile(`(\w+)Module\s*=\s*moduleStruct\s*\{\s*moduleName:\s*"([^"]+)"`)

	// Track disableable commands
	disableableCmds := make(map[string]bool)

	// Track module names from struct declarations
	moduleNames := make(map[string]string)

	files, err := filepath.Glob(filepath.Join(modulesPath, "*.go"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		data, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			log.Warnf("Could not read %s: %v", file, err)
			continue
		}

		content := string(data)
		fileName := filepath.Base(file)
		moduleName := strings.TrimSuffix(fileName, ".go")

		// Find module name declarations
		moduleMatches := moduleNamePattern.FindAllStringSubmatch(content, -1)
		for _, match := range moduleMatches {
			if len(match) >= 3 {
				moduleNames[match[1]+"Module"] = match[2]
			}
		}

		// Special case: DefaultHelpRegistry is used in help.go
		if strings.Contains(content, "func DefaultHelpRegistry()") {
			moduleNames["DefaultHelpRegistry"] = "Help"
		}

		// Find disableable commands
		disableMatches := disableablePattern.FindAllStringSubmatch(content, -1)
		for _, match := range disableMatches {
			if len(match) >= 2 {
				disableableCmds[match[1]] = true
			}
		}

		// Find command registrations
		cmdMatches := newCommandPattern.FindAllStringSubmatch(content, -1)
		for _, match := range cmdMatches {
			if len(match) >= 2 {
				cmdName := match[1]
				moduleVar := match[2]
				handler := match[3]
				if handler == "" {
					handler = "anonymous"
				}

				// Try to get module name from struct declaration
				modName := moduleName
				if name, ok := moduleNames[moduleVar]; ok {
					modName = strings.ToLower(name)
				}

				commands = append(commands, Command{
					Name:        cmdName,
					Handler:     handler,
					Module:      modName,
					Disableable: disableableCmds[cmdName],
				})
			}
		}

		// Extract MultiCommand registrations (helpers.MultiCommand(dispatcher, []string{...}, handler))
		multiCommandPattern := regexp.MustCompile(
			`helpers\.MultiCommand\s*\(\s*\w+\s*,\s*\[\]string\s*\{([^}]+)\}\s*,\s*(\w+)\.(\w+)\s*\)`,
		)
		multiMatches := multiCommandPattern.FindAllStringSubmatch(content, -1)
		for _, match := range multiMatches {
			if len(match) >= 4 {
				aliasesRaw := match[1] // e.g., `"remallbl", "rmallbl"`
				moduleVar := match[2]  // e.g., "blacklistsModule"
				handler := match[3]    // e.g., "rmAllBlacklists"

				// Parse individual alias strings from the slice literal
				aliasPattern := regexp.MustCompile(`"([^"]+)"`)
				aliasMatches := aliasPattern.FindAllStringSubmatch(aliasesRaw, -1)

				var aliases []string
				for _, a := range aliasMatches {
					if len(a) > 1 {
						aliases = append(aliases, a[1])
					}
				}

			// Resolve module name from the module variable
			modName := moduleName
			if name, ok := moduleNames[moduleVar]; ok {
				modName = strings.ToLower(name)
			}

			// Register each alias as a command
			for _, alias := range aliases {
				commands = append(commands, Command{
					Name:        alias,
					Handler:     handler,
					Module:      modName,
					Disableable: disableableCmds[alias],
					Aliases:     aliases,
				})
			}
			}
		}

		// Extract WrapCommand registrations (helpers.WrapCommand/WrapCommandRaw)
		// First, find all CommandDescriptor definitions with their command names
		wrapDescPattern := regexp.MustCompile(`(?s)(\w+)Desc\s*=\s*helpers\.CommandDescriptor\s*\{.*?Name:\s*"([^"]+)"`)
		wrapDescMatches := wrapDescPattern.FindAllStringSubmatch(content, -1)
		wrapDescriptors := make(map[string]string) // map[descVar]cmdName
		for _, match := range wrapDescMatches {
			if len(match) >= 3 {
				wrapDescriptors[match[1]+"Desc"] = match[2]
			}
		}

		// Find WrapCommand and WrapCommandRaw calls
		wrapCommandPattern := regexp.MustCompile(
			`helpers\.WrapCommand(?:Raw)?\s*\(\s*\w+\s*,\s*(\w+)Desc\s*,\s*(\w+)\.(\w+)\s*\)`,
		)
		wrapMatches := wrapCommandPattern.FindAllStringSubmatch(content, -1)
		for _, match := range wrapMatches {
			if len(match) >= 4 {
				descVar := match[1] + "Desc"
				moduleVar := match[2]
				handler := match[3]

				// Resolve command name from descriptor
				cmdName := ""
				if name, ok := wrapDescriptors[descVar]; ok {
					cmdName = name
				}
				if cmdName == "" {
					continue
				}

			// Resolve module name
			modName := moduleName
			if name, ok := moduleNames[moduleVar]; ok {
				modName = strings.ToLower(name)
			}

			commands = append(commands, Command{
				Name:        cmdName,
				Handler:     handler,
				Module:      modName,
				Disableable: disableableCmds[cmdName],
			})
			}
		}
	}

	// Sort commands
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Module != commands[j].Module {
			return commands[i].Module < commands[j].Module
		}
		return commands[i].Name < commands[j].Name
	})

	return commands, nil
}

// parseLockTypes extracts lock types from locks.go by parsing the lockMap and restrMap
func parseLockTypes(locksPath string) ([]LockType, error) {
	data, err := os.ReadFile(filepath.Clean(locksPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", locksPath, err)
	}

	content := string(data)
	var lockTypes []LockType

	// Regex patterns to extract map keys
	// Matches: "key": value,
	mapEntryPattern := regexp.MustCompile(`"(\w+)":\s*[^,}]+,?`)

	// Find lockMap section
	lockMapStart := strings.Index(content, "lockMap = map[string]filters.Message{")
	lockMapEnd := -1
	if lockMapStart != -1 {
		depth := 0
		for i := lockMapStart; i < len(content); i++ {
			if content[i] == '{' {
				depth++
			} else if content[i] == '}' {
				depth--
				if depth == 0 {
					lockMapEnd = i
					break
				}
			}
		}
	}

	// Extract permission locks from lockMap
	if lockMapStart != -1 && lockMapEnd != -1 {
		lockMapSection := content[lockMapStart:lockMapEnd]
		matches := mapEntryPattern.FindAllStringSubmatch(lockMapSection, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				lockName := match[1]
				description := getLockDescription(lockName, "permission")
				lockTypes = append(lockTypes, LockType{
					Name:        lockName,
					Description: description,
					Category:    "permission",
				})
			}
		}
	}

	// Find restrMap section
	restrMapStart := strings.Index(content, "restrMap = map[string]filters.Message{")
	restrMapEnd := -1
	if restrMapStart != -1 {
		depth := 0
		for i := restrMapStart; i < len(content); i++ {
			if content[i] == '{' {
				depth++
			} else if content[i] == '}' {
				depth--
				if depth == 0 {
					restrMapEnd = i
					break
				}
			}
		}
	}

	// Extract restriction locks from restrMap
	if restrMapStart != -1 && restrMapEnd != -1 {
		restrMapSection := content[restrMapStart:restrMapEnd]
		matches := mapEntryPattern.FindAllStringSubmatch(restrMapSection, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				lockName := match[1]
				description := getLockDescription(lockName, "restriction")
				lockTypes = append(lockTypes, LockType{
					Name:        lockName,
					Description: description,
					Category:    "restriction",
				})
			}
		}
	}

	// Sort lock types: restrictions first, then permissions, alphabetically within each category
	sort.Slice(lockTypes, func(i, j int) bool {
		if lockTypes[i].Category != lockTypes[j].Category {
			return lockTypes[i].Category == "restriction" // Restrictions come first
		}
		return lockTypes[i].Name < lockTypes[j].Name
	})

	return lockTypes, nil
}

// getLockDescription returns a human-readable description for a lock type
func getLockDescription(lockName, category string) string {
	// Descriptions based on the lock implementation in locks.go
	descriptions := map[string]string{
		// Permission locks (from lockMap)
		"sticker":     "Blocks sticker messages",
		"audio":       "Blocks audio file messages",
		"voice":       "Blocks voice messages",
		"document":    "Blocks document files (excludes GIFs/animations)",
		"video":       "Blocks video messages",
		"videonote":   "Blocks video note messages (round videos)",
		"contact":     "Blocks contact card messages",
		"photo":       "Blocks photo messages",
		"gif":         "Blocks GIF/animation messages",
		"url":         "Blocks messages containing URLs",
		"bots":        "Prevents non-admins from adding bots to the group",
		"forward":     "Blocks forwarded messages",
		"game":        "Blocks game messages",
		"location":    "Blocks location/venue messages",
		"rtl":         "Blocks messages containing right-to-left (Arabic) text",
		"anonchannel": "Blocks messages from anonymous channels and linked channel posts",

		// Restriction locks (from restrMap)
		"messages": "Blocks all text, media, contacts, locations, games, stickers, and GIFs",
		"comments": "Blocks messages from non-members (discussion comments)",
		"media":    "Blocks all media files (audio, document, video, photo, video note, voice)",
		"other":    "Blocks games, stickers, and GIFs",
		"previews": "Blocks messages with URL previews",
		"all":      "Blocks all messages from non-admins",
	}

	if desc, exists := descriptions[lockName]; exists {
		return desc
	}

	return "No description available"
}

// parseCallbacks parses Go source files to extract callback handler registrations
func parseCallbacks(modulesPath string) ([]Callback, error) {
	var callbacks []Callback

	// Regex pattern for callback extraction
	// Matches: handlers.NewCallback(callbackquery.Prefix("prefix"), module.handler)
	callbackPattern := regexp.MustCompile(`handlers\.NewCallback\s*\(\s*callbackquery\.Prefix\s*\(\s*"([^"]+)"\s*\)\s*,\s*(\w+)\.(\w+)\s*\)`)

	// Module name pattern
	moduleNamePattern := regexp.MustCompile(`(\w+)Module\s*=\s*moduleStruct\s*\{\s*moduleName:\s*"([^"]+)"`)

	files, err := filepath.Glob(filepath.Join(modulesPath, "*.go"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		data, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			log.Warnf("Could not read %s: %v", file, err)
			continue
		}

		content := string(data)
		fileName := filepath.Base(file)
		moduleName := strings.TrimSuffix(fileName, ".go")

		// Find module name from struct declaration
		moduleMatches := moduleNamePattern.FindAllStringSubmatch(content, -1)
		moduleNames := make(map[string]string)
		for _, match := range moduleMatches {
			if len(match) >= 3 {
				moduleNames[match[1]+"Module"] = match[2]
			}
		}

		// Find callback registrations
		cbMatches := callbackPattern.FindAllStringSubmatch(content, -1)
		for _, match := range cbMatches {
			if len(match) >= 4 {
				prefix := match[1]
				moduleVar := match[2]
				handler := match[3]

				// Try to get module name from struct declaration
				modName := moduleName
				if name, ok := moduleNames[moduleVar]; ok {
					modName = name
				}

				callbacks = append(callbacks, Callback{
					Prefix:     prefix,
					Handler:    handler,
					Module:     modName,
					SourceFile: fileName,
				})
			}
		}
	}

	// Sort callbacks by module then prefix
	sort.Slice(callbacks, func(i, j int) bool {
		if callbacks[i].Module != callbacks[j].Module {
			return callbacks[i].Module < callbacks[j].Module
		}
		return callbacks[i].Prefix < callbacks[j].Prefix
	})

	return callbacks, nil
}

// parseMessageWatchers parses Go source files to extract message handler registrations
func parseMessageWatchers(modulesPath string) ([]MessageWatcher, error) {
	var watchers []MessageWatcher

	// Pattern 1: Single line with module.handler and literal group number
	// e.g., dispatcher.AddHandlerToGroup(handlers.NewMessage(filter, module.handler), 5)
	watcherPatternLiteral := regexp.MustCompile(
		`dispatcher\.AddHandlerToGroup\s*\(\s*handlers\.NewMessage\s*\([^,]+,\s*(\w+)\.(\w+)\s*\)\s*,\s*(-?\d+)\s*\)`,
	)

	// Pattern 2: Module.handler with variable group reference
	// e.g., dispatcher.AddHandlerToGroup(handlers.NewMessage(filter, module.handler), module.handlerGroup)
	watcherPatternVar := regexp.MustCompile(
		`dispatcher\.AddHandlerToGroup\s*\(\s*handlers\.NewMessage\s*\([^,]+,\s*(\w+)\.(\w+)\s*\)\s*,\s*(\w+)\.(\w+)\s*\)`,
	)

	// Pattern 3: Multi-line with anonymous function handler and literal group
	// e.g., dispatcher.AddHandlerToGroup(\n\thandlers.NewMessage(..., func(...)...),\n\t-2,\n)
	// We search for AddHandlerToGroup blocks and extract group numbers
	watcherPatternMultiline := regexp.MustCompile(
		`(?s)dispatcher\.AddHandlerToGroup\s*\(\s*handlers\.NewMessage\s*\([\s\S]*?\)\s*,\s*(-?\d+)\s*,?\s*\)`,
	)

	// Module name pattern
	moduleNamePattern := regexp.MustCompile(`(\w+)Module\s*=\s*moduleStruct\s*\{\s*moduleName:\s*"([^"]+)"`)

	// Handler group field pattern (to extract default handler group numbers)
	handlerGroupPattern := regexp.MustCompile(`(\w+)Module\s*=\s*moduleStruct\s*\{[^}]*handlerGroup:\s*(-?\d+)`)

	files, err := filepath.Glob(filepath.Join(modulesPath, "*.go"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		data, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			log.Warnf("Could not read %s: %v", file, err)
			continue
		}

		content := string(data)
		fileName := filepath.Base(file)
		moduleName := strings.TrimSuffix(fileName, ".go")

		// Find module name from struct declaration
		moduleMatches := moduleNamePattern.FindAllStringSubmatch(content, -1)
		moduleNames := make(map[string]string)
		for _, match := range moduleMatches {
			if len(match) >= 3 {
				moduleNames[match[1]+"Module"] = match[2]
			}
		}

		// Extract handler group numbers from struct declarations
		handlerGroups := make(map[string]int)
		hgMatches := handlerGroupPattern.FindAllStringSubmatch(content, -1)
		for _, match := range hgMatches {
			if len(match) >= 3 {
				groupNum, parseErr := strconv.Atoi(match[2])
				if parseErr == nil {
					handlerGroups[match[1]+"Module"] = groupNum
				}
			}
		}

		// Track found handlers to avoid duplicates
		foundHandlers := make(map[string]bool)

		// Pattern 1: Literal group number with named handler
		watcherMatches := watcherPatternLiteral.FindAllStringSubmatch(content, -1)
		for _, match := range watcherMatches {
			if len(match) >= 4 {
				moduleVar := match[1]
				handler := match[2]
				groupStr := match[3]

				groupNum, parseErr := strconv.Atoi(groupStr)
				if parseErr != nil {
					log.Warnf("Could not parse handler group number '%s' in %s", groupStr, fileName)
					continue
				}

				modName := moduleName
				if name, ok := moduleNames[moduleVar]; ok {
					modName = name
				}

				key := modName + "." + handler
				if !foundHandlers[key] {
					foundHandlers[key] = true
					watchers = append(watchers, MessageWatcher{
						Handler:      handler,
						Module:       modName,
						HandlerGroup: groupNum,
						SourceFile:   fileName,
					})
				}
			}
		}

		// Pattern 2: Variable group reference with named handler
		varMatches := watcherPatternVar.FindAllStringSubmatch(content, -1)
		for _, match := range varMatches {
			if len(match) >= 5 {
				moduleVar := match[1]
				handler := match[2]
				groupVar := match[3]
				// match[4] is the field name like "handlerGroup"

				modName := moduleName
				if name, ok := moduleNames[moduleVar]; ok {
					modName = name
				}

				// Try to resolve the group number from struct declaration
				groupNum := 0
				if g, ok := handlerGroups[groupVar]; ok {
					groupNum = g
				}

				key := modName + "." + handler
				if !foundHandlers[key] {
					foundHandlers[key] = true
					watchers = append(watchers, MessageWatcher{
						Handler:      handler,
						Module:       modName,
						HandlerGroup: groupNum,
						SourceFile:   fileName,
					})
				}
			}
		}

		// Pattern 3: Multiline with anonymous function - extract group from block
		multiMatches := watcherPatternMultiline.FindAllStringSubmatch(content, -1)
		for _, match := range multiMatches {
			if len(match) >= 2 {
				groupStr := match[1]
				groupNum, parseErr := strconv.Atoi(groupStr)
				if parseErr != nil {
					continue
				}

				// For anonymous handlers, use module name as handler description
				modName := moduleName

				// Try to find module handler in the match by looking for module.method pattern
				handlerName := "anonymous"
				handlerRe := regexp.MustCompile(`(\w+Module)\.(\w+)`)
				if hMatch := handlerRe.FindStringSubmatch(match[0]); hMatch != nil {
					if name, ok := moduleNames[hMatch[1]]; ok {
						modName = name
					}
					handlerName = hMatch[2]
				}

				key := modName + "." + handlerName
				if !foundHandlers[key] {
					foundHandlers[key] = true
					watchers = append(watchers, MessageWatcher{
						Handler:      handlerName,
						Module:       modName,
						HandlerGroup: groupNum,
						SourceFile:   fileName,
					})
				}
			}
		}
	}

	// Sort by handler group then module
	sort.Slice(watchers, func(i, j int) bool {
		if watchers[i].HandlerGroup != watchers[j].HandlerGroup {
			return watchers[i].HandlerGroup < watchers[j].HandlerGroup
		}
		return watchers[i].Module < watchers[j].Module
	})

	return watchers, nil
}
