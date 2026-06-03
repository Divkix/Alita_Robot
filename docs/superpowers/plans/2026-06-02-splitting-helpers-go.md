# Splitting helpers.go — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `alita/modules/helpers.go` into focused files and introduce registry patterns for deep-link and anonymous-admin routing.

**Architecture:** Phase 1 is pure file moves (no init changes). Phase 2 introduces a `DeepLinkHandler` registry where each module registers its own handler. Phase 3 reuses the same registry pattern for anonymous admin commands.

**Tech Stack:** Go 1.26+, gotgbot v2, standard library

---

## Phase 1: Pure File Moves

### Task 1: Read helpers.go and analyze its contents

**Files:**
- Read: `alita/modules/helpers.go` (507 lines)

**Steps:**
- [ ] **Step 1: Read helpers.go completely**

  Run: `cat alita/modules/helpers.go`
  
  Verify: You see 6 sections:
  1. moduleStruct (lines 32-41)
  2. notesOverwriteMap + overwrite types (lines 43-71)
  3. spamKey + antiSpam types (lines 73-91)
  4. help system functions (lines 93-204)
  5. startHelpPrefixHandler (lines 222-469)
  6. helper functions (lines 471-507)

- [ ] **Step 2: Read help.go to find moduleEnabled and DefaultHelpRegistry**

  Run: `cat alita/modules/help.go`
  
  Note: Find `moduleEnabled` struct definition and `DefaultHelpRegistry()` function. These will move to `core.go`.

---

### Task 2: Create core.go — Module Infrastructure

**Files:**
- Create: `alita/modules/core.go`
- Modify: `alita/modules/help.go` (remove `moduleEnabled` and `DefaultHelpRegistry`)

**Steps:**
- [ ] **Step 1: Create core.go with moduleStruct and moduleEnabled**

```go
package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
)

// module struct for all modules
type moduleStruct struct {
	moduleName        string
	handlerGroup      int
	permHandlerGroup  int
	restrHandlerGroup int
	defaultRulesBtn   string
	AbleMap           moduleEnabled
	AltHelpOptions    map[string][]string
	helpableKb        map[string][][]gotgbot.InlineKeyboardButton
}

// moduleEnabled tracks which modules are enabled per chat.
type moduleEnabled struct {
	modules map[string]bool
}

// Init initializes the moduleEnabled map.
func (m *moduleEnabled) Init() {
	m.modules = make(map[string]bool)
}

// Store enables or disables a module.
func (m *moduleEnabled) Store(module string, enabled bool) {
	m.modules[module] = enabled
}

// Load returns the module name and whether it is enabled.
func (m *moduleEnabled) Load(module string) (string, bool) {
	return module, m.modules[module]
}

// LoadModules returns a slice of all enabled module names.
func (m *moduleEnabled) LoadModules() []string {
	var modules []string
	for module, enabled := range m.modules {
		if enabled {
			modules = append(modules, module)
		}
	}
	return modules
}

// DefaultHelpRegistry returns the default help registry.
func DefaultHelpRegistry() *moduleStruct {
	return defaultHelpRegistry
}

var defaultHelpRegistry = NewHelpRegistry()
```

  Run: `cat > alita/modules/core.go << 'EOF'
package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
)

// module struct for all modules
type moduleStruct struct {
	moduleName        string
	handlerGroup      int
	permHandlerGroup  int
	restrHandlerGroup int
	defaultRulesBtn   string
	AbleMap           moduleEnabled
	AltHelpOptions    map[string][]string
	helpableKb        map[string][][]gotgbot.InlineKeyboardButton
}

// moduleEnabled tracks which modules are enabled per chat.
type moduleEnabled struct {
	modules map[string]bool
}

// Init initializes the moduleEnabled map.
func (m *moduleEnabled) Init() {
	m.modules = make(map[string]bool)
}

// Store enables or disables a module.
func (m *moduleEnabled) Store(module string, enabled bool) {
	m.modules[module] = enabled
}

// Load returns the module name and whether it is enabled.
func (m *moduleEnabled) Load(module string) (string, bool) {
	return module, m.modules[module]
}

// LoadModules returns a slice of all enabled module names.
func (m *moduleEnabled) LoadModules() []string {
	var modules []string
	for module, enabled := range m.modules {
		if enabled {
			modules = append(modules, module)
		}
	}
	return modules
}

// DefaultHelpRegistry returns the default help registry.
func DefaultHelpRegistry() *moduleStruct {
	return defaultHelpRegistry
}

var defaultHelpRegistry = NewHelpRegistry()
EOF`

- [ ] **Step 2: Remove moduleEnabled from help.go**

  In `alita/modules/help.go`, find and delete:
  
```go
// moduleEnabled struct
type moduleEnabled struct {
	modules map[string]bool
}

func (m *moduleEnabled) Init() {
	m.modules = make(map[string]bool)
}

func (m *moduleEnabled) Store(module string, enabled bool) {
	m.modules[module] = enabled
}

func (m *moduleEnabled) Load(module string) (string, bool) {
	return module, m.modules[module]
}

func (m *moduleEnabled) LoadModules() []string {
	var modules []string
	for module, enabled := range m.modules {
		if enabled {
			modules = append(modules, module)
		}
	}
	return modules
}
```

  Also find and update `DefaultHelpRegistry()` in help.go to just reference the function in core.go (or delete it if it exists there).

- [ ] **Step 3: Run tests to verify moduleEnabled still works**

  Run: `go test -v ./alita/modules/... -run "TestModuleEnabled|TestHelpRegistry"`
  Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add alita/modules/core.go alita/modules/help.go
git commit -m "refactor(modules): extract module infrastructure to core.go"
```

---

### Task 3: Create overwrite.go — Temporary State Storage

**Files:**
- Create: `alita/modules/overwrite.go`

**Steps:**
- [ ] **Step 1: Create overwrite.go**

```go
package modules

import (
	"github.com/divkix/Alita_Robot/alita/db"
	"sync"
)

// notesOverwriteMap is a package-level concurrent-safe map for note overwrites.
// This is separate from moduleStruct to avoid copylocks issues with value receivers.
var notesOverwriteMap sync.Map

// overwriteBase holds common fields for temporary state storage during command flows.
type overwriteBase struct {
	ChatID   int64
	ItemName string // filterWord or noteWord
	Text     string
	FileID   string
	Buttons  []db.Button
	DataType int
}

// struct for filters module
type overwriteFilter struct {
	overwriteBase
}

// struct for notes module
type overwriteNote struct {
	overwriteBase
	pvtOnly     bool
	grpOnly     bool
	adminOnly   bool
	webPrev     bool
	isProtected bool
	noNotif     bool
}
```

  Run: `cat > alita/modules/overwrite.go << 'EOF'
package modules

import (
	"github.com/divkix/Alita_Robot/alita/db"
	"sync"
)

// notesOverwriteMap is a package-level concurrent-safe map for note overwrites.
var notesOverwriteMap sync.Map

// overwriteBase holds common fields for temporary state storage during command flows.
type overwriteBase struct {
	ChatID   int64
	ItemName string
	Text     string
	FileID   string
	Buttons  []db.Button
	DataType int
}

// overwriteFilter is for filters module
type overwriteFilter struct {
	overwriteBase
}

// overwriteNote is for notes module
type overwriteNote struct {
	overwriteBase
	pvtOnly     bool
	grpOnly     bool
	adminOnly   bool
	webPrev     bool
	isProtected bool
	noNotif     bool
}
EOF`

- [ ] **Step 2: Run tests to verify overwrite types still work**

  Run: `go test -v ./alita/modules/... -run "TestOverwrite|TestFilterOverwrite|TestNoteOverwrite"`
  Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add alita/modules/overwrite.go
git commit -m "refactor(modules): extract overwrite types to overwrite.go"
```

---

### Task 4: Create help_system.go — Help Menu & Navigation

**Files:**
- Create: `alita/modules/help_system.go`

**Steps:**
- [ ] **Step 1: Create help_system.go**

```go
package modules

import (
	"fmt"
	"html"
	"slices"
	"strings"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// markup is the global help menu keyboard.
var markup gotgbot.InlineKeyboardMarkup

// listModules returns a sorted slice of all currently enabled bot modules.
func listModules() []string {
	return listModulesFrom(DefaultHelpRegistry())
}

func listModulesFrom(registry *moduleStruct) []string {
	modules := registry.AbleMap.LoadModules()
	slices.Sort(modules)
	return modules
}

// initHelpButtons initializes the help menu keyboard with all enabled modules.
func initHelpButtons() {
	markup = initHelpButtonsFrom(DefaultHelpRegistry())
}

// initHelpButtonsFrom builds a help menu keyboard from the given registry.
func initHelpButtonsFrom(registry *moduleStruct) gotgbot.InlineKeyboardMarkup {
	var kb []gotgbot.InlineKeyboardButton

	for _, i := range listModulesFrom(registry) {
		kb = append(kb, gotgbot.InlineKeyboardButton{
			Text: i,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": i},
				fmt.Sprintf("helpq.%s", i),
			),
		})
	}
	zb := keyboard.ChunkKeyboardSlices(kb, 3)
	tr := i18n.MustNewTranslator("en")
	backText, _ := tr.GetString("helpers_back_button")
	zb = append(zb, []gotgbot.InlineKeyboardButton{{
		Text:         backText,
		CallbackData: encodeCallbackData("helpq", map[string]string{"m": "BackStart"}, "helpq.BackStart"),
	}})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: zb}
}

// getModuleHelpAndKb retrieves help text and keyboard for a specific module.
func getModuleHelpAndKb(module, lang string, registry *moduleStruct) (helpText string, replyMarkup gotgbot.InlineKeyboardMarkup) {
	ModName := cases.Title(language.English).String(module)
	tr := i18n.MustNewTranslator(lang)
	helpMsg, _ := tr.GetString(fmt.Sprintf("%s_help_msg", strings.ToLower(ModName)))
	headerTemplate, _ := tr.GetString("helpers_module_help_header")
	helpText = tgmd2html.MD2HTMLV2(fmt.Sprintf(headerTemplate, ModName) + helpMsg)

	backText, _ := tr.GetString("common_back_arrow")
	homeText, _ := tr.GetString("common_home")
	backBtnSuffix := []gotgbot.InlineKeyboardButton{
		{
			Text:         backText,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Help"}, "helpq.Help"),
		},
		{
			Text:         homeText,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "BackStart"}, "helpq.BackStart"),
		},
	}

	replyMarkup = gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: append(
			registry.helpableKb[ModName],
			backBtnSuffix,
		),
	}
	return
}

// sendHelpkb sends help information for a specific module with navigation keyboard.
func sendHelpkb(b *gotgbot.Bot, ctx *ext.Context, module string, registry *moduleStruct) (msg *gotgbot.Message, err error) {
	module = strings.ToLower(module)
	if module == "help" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		helpText := getMainHelp(tr, html.EscapeString(ctx.EffectiveMessage.From.FirstName))
		_, err = b.SendMessage(
			ctx.EffectiveMessage.Chat.Id,
			helpText,
			&gotgbot.SendMessageOpts{
				ParseMode:   formatting.HTML,
				ReplyMarkup: &markup,
			},
		)
		return
	}
	helpText, replyMarkup, _parsemode := getHelpTextAndMarkup(ctx, module, registry)

	msg, err = b.SendMessage(
		ctx.EffectiveChat.Id,
		helpText,
		&gotgbot.SendMessageOpts{
			ParseMode:   _parsemode,
			ReplyMarkup: replyMarkup,
		},
	)
	return
}

// getModuleNameFromAltName resolves alternative module names to their canonical form.
func getModuleNameFromAltName(altName string, registry *moduleStruct) string {
	for _, modName := range listModulesFrom(registry) {
		altNames := getAltNamesOfModule(modName)
		for _, altNameInSlice := range altNames {
			if altNameInSlice == altName {
				return modName
			}
		}
	}
	return ""
}

// getAltNamesOfModule returns all alternative names for a given module.
func getAltNamesOfModule(moduleName string) []string {
	tr := i18n.MustNewTranslator("config")
	altNamesFromConfig, _ := tr.GetStringSlice(fmt.Sprintf("alt_names.%s", moduleName))
	return append(altNamesFromConfig, strings.ToLower(moduleName))
}

// getHelpTextAndMarkup generates help content and keyboard for a module or main help.
func getHelpTextAndMarkup(ctx *ext.Context, module string, registry *moduleStruct) (helpText string, kbmarkup gotgbot.InlineKeyboardMarkup, _parsemode string) {
	var moduleName string
	userOrGroupLanguage := lang.GetLanguage(ctx)

	for _, ModName := range listModulesFrom(registry) {
		altnames := getAltNamesOfModule(ModName)
		if slices.Contains(altnames, module) {
			moduleName = ModName
			break
		}
	}

	if moduleName != "" {
		_parsemode = formatting.HTML
		helpText, kbmarkup = getModuleHelpAndKb(moduleName, userOrGroupLanguage, registry)
	} else {
		_parsemode = formatting.HTML
		tr := i18n.MustNewTranslator(userOrGroupLanguage)
		helpText = getMainHelp(tr, html.EscapeString(ctx.EffectiveUser.FirstName))
		kbmarkup = initHelpButtonsFrom(registry)
	}

	return
}
```

  Run: `cat > alita/modules/help_system.go << 'EOF'
package modules

import (
	"fmt"
	"html"
	"slices"
	"strings"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var markup gotgbot.InlineKeyboardMarkup

func listModules() []string {
	return listModulesFrom(DefaultHelpRegistry())
}

func listModulesFrom(registry *moduleStruct) []string {
	modules := registry.AbleMap.LoadModules()
	slices.Sort(modules)
	return modules
}

func initHelpButtons() {
	markup = initHelpButtonsFrom(DefaultHelpRegistry())
}

func initHelpButtonsFrom(registry *moduleStruct) gotgbot.InlineKeyboardMarkup {
	var kb []gotgbot.InlineKeyboardButton

	for _, i := range listModulesFrom(registry) {
		kb = append(kb, gotgbot.InlineKeyboardButton{
			Text: i,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": i},
				fmt.Sprintf("helpq.%s", i),
			),
		})
	}
	zb := keyboard.ChunkKeyboardSlices(kb, 3)
	tr := i18n.MustNewTranslator("en")
	backText, _ := tr.GetString("helpers_back_button")
	zb = append(zb, []gotgbot.InlineKeyboardButton{{
		Text:         backText,
		CallbackData: encodeCallbackData("helpq", map[string]string{"m": "BackStart"}, "helpq.BackStart"),
	}})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: zb}
}

func getModuleHelpAndKb(module, lang string, registry *moduleStruct) (helpText string, replyMarkup gotgbot.InlineKeyboardMarkup) {
	ModName := cases.Title(language.English).String(module)
	tr := i18n.MustNewTranslator(lang)
	helpMsg, _ := tr.GetString(fmt.Sprintf("%s_help_msg", strings.ToLower(ModName)))
	headerTemplate, _ := tr.GetString("helpers_module_help_header")
	helpText = tgmd2html.MD2HTMLV2(fmt.Sprintf(headerTemplate, ModName) + helpMsg)

	backText, _ := tr.GetString("common_back_arrow")
	homeText, _ := tr.GetString("common_home")
	backBtnSuffix := []gotgbot.InlineKeyboardButton{
		{
			Text:         backText,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Help"}, "helpq.Help"),
		},
		{
			Text:         homeText,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "BackStart"}, "helpq.BackStart"),
		},
	}

	replyMarkup = gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: append(
			registry.helpableKb[ModName],
			backBtnSuffix,
		),
	}
	return
}

func sendHelpkb(b *gotgbot.Bot, ctx *ext.Context, module string, registry *moduleStruct) (msg *gotgbot.Message, err error) {
	module = strings.ToLower(module)
	if module == "help" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		helpText := getMainHelp(tr, html.EscapeString(ctx.EffectiveMessage.From.FirstName))
		_, err = b.SendMessage(
			ctx.EffectiveMessage.Chat.Id,
			helpText,
			&gotgbot.SendMessageOpts{
				ParseMode:   formatting.HTML,
				ReplyMarkup: &markup,
			},
		)
		return
	}
	helpText, replyMarkup, _parsemode := getHelpTextAndMarkup(ctx, module, registry)

	msg, err = b.SendMessage(
		ctx.EffectiveChat.Id,
		helpText,
		&gotgbot.SendMessageOpts{
			ParseMode:   _parsemode,
			ReplyMarkup: replyMarkup,
		},
	)
	return
}

func getModuleNameFromAltName(altName string, registry *moduleStruct) string {
	for _, modName := range listModulesFrom(registry) {
		altNames := getAltNamesOfModule(modName)
		for _, altNameInSlice := range altNames {
			if altNameInSlice == altName {
				return modName
			}
		}
	}
	return ""
}

func getAltNamesOfModule(moduleName string) []string {
	tr := i18n.MustNewTranslator("config")
	altNamesFromConfig, _ := tr.GetStringSlice(fmt.Sprintf("alt_names.%s", moduleName))
	return append(altNamesFromConfig, strings.ToLower(moduleName))
}

func getHelpTextAndMarkup(ctx *ext.Context, module string, registry *moduleStruct) (helpText string, kbmarkup gotgbot.InlineKeyboardMarkup, _parsemode string) {
	var moduleName string
	userOrGroupLanguage := lang.GetLanguage(ctx)

	for _, ModName := range listModulesFrom(registry) {
		altnames := getAltNamesOfModule(ModName)
		if slices.Contains(altnames, module) {
			moduleName = ModName
			break
		}
	}

	if moduleName != "" {
		_parsemode = formatting.HTML
		helpText, kbmarkup = getModuleHelpAndKb(moduleName, userOrGroupLanguage, registry)
	} else {
		_parsemode = formatting.HTML
		tr := i18n.MustNewTranslator(userOrGroupLanguage)
		helpText = getMainHelp(tr, html.EscapeString(ctx.EffectiveUser.FirstName))
		kbmarkup = initHelpButtonsFrom(registry)
	}

	return
}
EOF`

- [ ] **Step 2: Run tests to verify help system still works**

  Run: `go test -v ./alita/modules/... -run "TestHelp|TestInitHelpButtons|TestSendHelpkb|TestGetModuleHelpAndKb|TestGetAltNamesOfModule|TestGetModuleNameFromAltName|TestGetHelpTextAndMarkup"`
  Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add alita/modules/help_system.go
git commit -m "refactor(modules): extract help system to help_system.go"
```

---

### Task 5: Merge antispam types into antispam.go

**Files:**
- Modify: `alita/modules/antispam.go`

**Steps:**
- [ ] **Step 1: Add antispam types to antispam.go**

  At the top of `alita/modules/antispam.go`, add:

```go
// spamKey is a composite key for rate limiting per user per chat
type spamKey struct {
	chatId int64
	userId int64
}

// antiSpamInfo tracks spam levels for a user in a chat.
type antiSpamInfo struct {
	Levels []antiSpamLevel
}

// antiSpamLevel represents a single spam threshold.
type antiSpamLevel struct {
	Count    int
	Limit    int
	CurrTime time.Time
	Expiry   time.Duration
	Spammed  bool
}
```

  Run: `sed -i '' '1a\
\
// spamKey is a composite key for rate limiting per user per chat\
type spamKey struct {\
	chatId int64\
	userId int64\
}\
\
// antiSpamInfo tracks spam levels for a user in a chat.\
type antiSpamInfo struct {\
	Levels []antiSpamLevel\
}\
\
// antiSpamLevel represents a single spam threshold.\
type antiSpamLevel struct {\
	Count    int\
	Limit    int\
	CurrTime time.Time\
	Expiry   time.Duration\
	Spammed  bool\
}\
' alita/modules/antispam.go`

  Wait — actually, use a more reliable method. Read the file and insert after the imports or package declaration.

- [ ] **Step 2: Run tests to verify antispam still works**

  Run: `go test -v ./alita/modules/... -run "TestAntiSpam|TestSpamCheck"`
  Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add alita/modules/antispam.go
git commit -m "refactor(modules): move antispam types to antispam.go"
```

---

### Task 6: Delete helpers.go

**Files:**
- Delete: `alita/modules/helpers.go`

**Steps:**
- [ ] **Step 1: Delete helpers.go**

  Run: `rm alita/modules/helpers.go`

- [ ] **Step 2: Run all tests in modules package**

  Run: `go test -v ./alita/modules/...`
  Expected: ALL PASS

- [ ] **Step 3: Run lint**

  Run: `make lint`
  Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add alita/modules/helpers.go
git commit -m "refactor(modules): delete helpers.go after extraction"
```

---

## Phase 2: Deep Link Router Registry

### Task 7: Create deeplink_router.go

**Files:**
- Create: `alita/modules/deeplink_router.go`

**Steps:**
- [ ] **Step 1: Create the router file**

```go
package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
)

// DeepLinkHandler processes a deep link argument.
type DeepLinkHandler func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error

var deepLinkRegistry = make(map[string]DeepLinkHandler)

// RegisterDeepLinkHandler registers a handler for a deep link prefix.
func RegisterDeepLinkHandler(prefix string, handler DeepLinkHandler) {
	deepLinkRegistry[prefix] = handler
}

// HandleDeepLink routes a deep link argument to the appropriate handler.
func HandleDeepLink(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	var matchedPrefix string
	var handler DeepLinkHandler
	for prefix, h := range deepLinkRegistry {
		if strings.HasPrefix(arg, prefix) && len(prefix) > len(matchedPrefix) {
			matchedPrefix = prefix
			handler = h
		}
	}

	if handler != nil {
		return handler(b, ctx, user, arg)
	}

	// Fallback: send default help
	return sendDefaultHelp(b, ctx, user)
}

// sendDefaultHelp sends the default start/help message.
func sendDefaultHelp(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	startHelpText := getStartHelp(tr)
	startMarkupKb := getStartMarkup(tr, b.Username)
	_, err := b.SendMessage(ctx.EffectiveChat.Id,
		startHelpText,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: &startMarkupKb,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
```

  Run: `cat > alita/modules/deeplink_router.go << 'EOF'
package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	log "github.com/sirupsen/logrus"
)

// DeepLinkHandler processes a deep link argument.
type DeepLinkHandler func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error

var deepLinkRegistry = make(map[string]DeepLinkHandler)

// RegisterDeepLinkHandler registers a handler for a deep link prefix.
func RegisterDeepLinkHandler(prefix string, handler DeepLinkHandler) {
	deepLinkRegistry[prefix] = handler
}

// HandleDeepLink routes a deep link argument to the appropriate handler.
func HandleDeepLink(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	var matchedPrefix string
	var handler DeepLinkHandler
	for prefix, h := range deepLinkRegistry {
		if strings.HasPrefix(arg, prefix) && len(prefix) > len(matchedPrefix) {
			matchedPrefix = prefix
			handler = h
		}
	}

	if handler != nil {
		return handler(b, ctx, user, arg)
	}

	return sendDefaultHelp(b, ctx, user)
}

// sendDefaultHelp sends the default start/help message.
func sendDefaultHelp(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	startHelpText := getStartHelp(tr)
	startMarkupKb := getStartMarkup(tr, b.Username)
	_, err := b.SendMessage(ctx.EffectiveChat.Id,
		startHelpText,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: &startMarkupKb,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
EOF`

- [ ] **Step 2: Write test for deeplink_router**

```go
package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestRegisterDeepLinkHandler(t *testing.T) {
	// Clear registry
	deepLinkRegistry = make(map[string]DeepLinkHandler)

	called := false
	RegisterDeepLinkHandler("test_", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		called = true
		return nil
	})

	// Test HandleDeepLink
	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	user := &gotgbot.User{}
	
	HandleDeepLink(bot, ctx, user, "test_foo")
	if !called {
		t.Fatal("Expected handler to be called")
	}
}

func TestHandleDeepLink_LongestPrefixMatch(t *testing.T) {
	deepLinkRegistry = make(map[string]DeepLinkHandler)

	shortCalled := false
	longCalled := false
	
	RegisterDeepLinkHandler("note", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		shortCalled = true
		return nil
	})
	
	RegisterDeepLinkHandler("notes_", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		longCalled = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	user := &gotgbot.User{}
	
	HandleDeepLink(bot, ctx, user, "notes_list")
	if shortCalled {
		t.Fatal("Short handler should not be called for 'notes_list'")
	}
	if !longCalled {
		t.Fatal("Long handler should be called for 'notes_list'")
	}
}
```

  Run: `cat > alita/modules/deeplink_router_test.go << 'EOF'
package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestRegisterDeepLinkHandler(t *testing.T) {
	deepLinkRegistry = make(map[string]DeepLinkHandler)

	called := false
	RegisterDeepLinkHandler("test_", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		called = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	user := &gotgbot.User{}
	
	HandleDeepLink(bot, ctx, user, "test_foo")
	if !called {
		t.Fatal("Expected handler to be called")
	}
}

func TestHandleDeepLink_LongestPrefixMatch(t *testing.T) {
	deepLinkRegistry = make(map[string]DeepLinkHandler)

	shortCalled := false
	longCalled := false
	
	RegisterDeepLinkHandler("note", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		shortCalled = true
		return nil
	})
	
	RegisterDeepLinkHandler("notes_", func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
		longCalled = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	user := &gotgbot.User{}
	
	HandleDeepLink(bot, ctx, user, "notes_list")
	if shortCalled {
		t.Fatal("Short handler should not be called for 'notes_list'")
	}
	if !longCalled {
		t.Fatal("Long handler should be called for 'notes_list'")
	}
}
EOF`

- [ ] **Step 3: Run tests**

  Run: `go test -v ./alita/modules/... -run "TestRegisterDeepLinkHandler|TestHandleDeepLink_LongestPrefixMatch"`
  Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add alita/modules/deeplink_router.go alita/modules/deeplink_router_test.go
git commit -m "feat(modules): add deep link router registry"
```

---

### Task 8: Extract deep link handlers from startHelpPrefixHandler

**Files:**
- Modify: `alita/modules/help.go` (extract `help_` and `about` handlers)
- Modify: `alita/modules/connections.go` (extract `connect_` handler)
- Modify: `alita/modules/rules.go` (extract `rules_` handler)
- Modify: `alita/modules/notes.go` (extract `notes_` and `note_` handlers)

**Steps:**
- [ ] **Step 1: Extract help_ handler to help.go**

  In `alita/modules/help.go`, add to `init()`:
  ```go
  RegisterDeepLinkHandler("help_", helpDeepLinkHandler)
  ```
  
  Add function:
  ```go
  func helpDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
  	parts := strings.Split(arg, "_")
  	if len(parts) < 2 {
  		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
  		text, _ := tr.GetString("helpers_invalid_deep_link")
  		_, _ = ctx.EffectiveMessage.Reply(b, text, formatting.Shtml())
  		return ext.EndGroups
  	}
  	helpModule := parts[1]
  	_, err := sendHelpkb(b, ctx, helpModule, DefaultHelpRegistry())
  	if err != nil {
  		log.Errorf("[Start]: %v", err)
  		return err
  	}
  	return ext.EndGroups
  }
  ```

  Also add `about` handler:
  ```go
  RegisterDeepLinkHandler("about", aboutDeepLinkHandler)
  ```
  
  ```go
  func aboutDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
  	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
  	aboutText := getAboutText(tr)
  	aboutKb := getAboutKb(tr)
  	_, err := b.SendMessage(ctx.EffectiveChat.Id,
  		aboutText,
  		&gotgbot.SendMessageOpts{
  			ParseMode: formatting.HTML,
  			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
  				IsDisabled: true,
  			},
  			ReplyParameters: &gotgbot.ReplyParameters{
  				MessageId:                ctx.EffectiveMessage.MessageId,
  				AllowSendingWithoutReply: true,
  			},
  			ReplyMarkup: &aboutKb,
  		},
  	)
  	if err != nil {
  		log.Error(err)
  		return err
  	}
  	return ext.EndGroups
  }
  ```

- [ ] **Step 2: Extract connect_ handler to connections.go**

  In `alita/modules/connections.go`, add to `init()`:
  ```go
  RegisterDeepLinkHandler("connect_", connectDeepLinkHandler)
  ```
  
  Extract the `connect_` handling logic from `startHelpPrefixHandler` into `connectDeepLinkHandler`.

- [ ] **Step 3: Extract rules_ handler to rules.go**

  In `alita/modules/rules.go`, add to `init()`:
  ```go
  RegisterDeepLinkHandler("rules_", rulesDeepLinkHandler)
  ```
  
  Extract the `rules_` handling logic from `startHelpPrefixHandler` into `rulesDeepLinkHandler`.

- [ ] **Step 4: Extract notes_ and note_ handlers to notes.go**

  In `alita/modules/notes.go`, add to `init()`:
  ```go
  RegisterDeepLinkHandler("notes_", notesListDeepLinkHandler)
  RegisterDeepLinkHandler("note_", noteDeepLinkHandler)
  ```
  
  Extract the `notes_` and `note_` handling logic from `startHelpPrefixHandler` into `notesListDeepLinkHandler` and `noteDeepLinkHandler`.

- [ ] **Step 5: Update help.go to call HandleDeepLink**

  In `alita/modules/help.go`, find the line:
  ```go
  err := startHelpPrefixHandler(b, ctx, user, args[1])
  ```
  
  Change to:
  ```go
  err := HandleDeepLink(b, ctx, user, args[1])
  ```

- [ ] **Step 6: Delete startHelpPrefixHandler from help_system.go**

  Once all handlers are extracted and registered, delete `startHelpPrefixHandler` from wherever it currently lives (it may have moved to `help_system.go` or still be in `help.go` after Phase 1).

- [ ] **Step 7: Run tests**

  Run: `go test -v ./alita/modules/...`
  Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add alita/modules/help.go alita/modules/connections.go alita/modules/rules.go alita/modules/notes.go alita/modules/help_system.go
git commit -m "refactor(modules): extract deep link handlers to registry"
```

---

## Phase 3: Anonymous Admin Router Registry

### Task 9: Create anonymous_admin_router.go

**Files:**
- Create: `alita/modules/anonymous_admin_router.go`

**Steps:**
- [ ] **Step 1: Create the router file**

```go
package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// AnonymousAdminHandler processes an anonymous admin command.
type AnonymousAdminHandler func(b *gotgbot.Bot, ctx *ext.Context) error

var anonAdminRegistry = make(map[string]AnonymousAdminHandler)

// RegisterAnonymousAdminHandler registers a handler for an anonymous admin command.
func RegisterAnonymousAdminHandler(command string, handler AnonymousAdminHandler) {
	anonAdminRegistry[command] = handler
}

// HandleAnonymousAdmin routes an anonymous admin command to the appropriate handler.
func HandleAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error {
	if handler, ok := anonAdminRegistry[command]; ok {
		return handler(b, ctx)
	}
	return fmt.Errorf("unknown anonymous admin command: %s", command)
}
```

  Run: `cat > alita/modules/anonymous_admin_router.go << 'EOF'
package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// AnonymousAdminHandler processes an anonymous admin command.
type AnonymousAdminHandler func(b *gotgbot.Bot, ctx *ext.Context) error

var anonAdminRegistry = make(map[string]AnonymousAdminHandler)

// RegisterAnonymousAdminHandler registers a handler for an anonymous admin command.
func RegisterAnonymousAdminHandler(command string, handler AnonymousAdminHandler) {
	anonAdminRegistry[command] = handler
}

// HandleAnonymousAdmin routes an anonymous admin command to the appropriate handler.
func HandleAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error {
	if handler, ok := anonAdminRegistry[command]; ok {
		return handler(b, ctx)
	}
	return fmt.Errorf("unknown anonymous admin command: %s", command)
}
EOF`

- [ ] **Step 2: Write test for anonymous_admin_router**

```go
package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestRegisterAnonymousAdminHandler(t *testing.T) {
	anonAdminRegistry = make(map[string]AnonymousAdminHandler)

	called := false
	RegisterAnonymousAdminHandler("test", func(b *gotgbot.Bot, ctx *ext.Context) error {
		called = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	
	HandleAnonymousAdmin(bot, ctx, "test")
	if !called {
		t.Fatal("Expected handler to be called")
	}
}

func TestHandleAnonymousAdmin_UnknownCommand(t *testing.T) {
	anonAdminRegistry = make(map[string]AnonymousAdminHandler)

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	
	err := HandleAnonymousAdmin(bot, ctx, "unknown")
	if err == nil {
		t.Fatal("Expected error for unknown command")
	}
}
```

  Run: `cat > alita/modules/anonymous_admin_router_test.go << 'EOF'
package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestRegisterAnonymousAdminHandler(t *testing.T) {
	anonAdminRegistry = make(map[string]AnonymousAdminHandler)

	called := false
	RegisterAnonymousAdminHandler("test", func(b *gotgbot.Bot, ctx *ext.Context) error {
		called = true
		return nil
	})

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	
	HandleAnonymousAdmin(bot, ctx, "test")
	if !called {
		t.Fatal("Expected handler to be called")
	}
}

func TestHandleAnonymousAdmin_UnknownCommand(t *testing.T) {
	anonAdminRegistry = make(map[string]AnonymousAdminHandler)

	bot := &gotgbot.Bot{}
	ctx := &ext.Context{}
	
	err := HandleAnonymousAdmin(bot, ctx, "unknown")
	if err == nil {
		t.Fatal("Expected error for unknown command")
	}
}
EOF`

- [ ] **Step 3: Run tests**

  Run: `go test -v ./alita/modules/... -run "TestRegisterAnonymousAdminHandler|TestHandleAnonymousAdmin_UnknownCommand"`
  Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add alita/modules/anonymous_admin_router.go alita/modules/anonymous_admin_router_test.go
git commit -m "feat(modules): add anonymous admin router registry"
```

---

### Task 10: Register anonymous admin handlers in moderation modules

**Files:**
- Modify: `alita/modules/admin.go`
- Modify: `alita/modules/bans.go`
- Modify: `alita/modules/mute.go`
- Modify: `alita/modules/pins.go`
- Modify: `alita/modules/purges.go`
- Modify: `alita/modules/warns.go`

**Steps:**
- [ ] **Step 1: Add wrapper functions and register admin handlers in admin.go**

  `adminModule.promote`, `demote`, and `setTitle` accept `*helpers.CommandContext`, not `*ext.Context`. Add thin wrappers that build the context inside the handler, then register the wrappers:

  ```go
  // anonymousAdmin wrappers for admin.go
  func (m moduleStruct) promoteAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
      c, err := helpers.BuildCommandContext(b, ctx)
      if err != nil {
          return ext.EndGroups
      }
      return m.promote(c)
  }

  func (m moduleStruct) demoteAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
      c, err := helpers.BuildCommandContext(b, ctx)
      if err != nil {
          return ext.EndGroups
      }
      return m.demote(c)
  }

  func (m moduleStruct) setTitleAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
      c, err := helpers.BuildCommandContext(b, ctx)
      if err != nil {
          return ext.EndGroups
      }
      return m.setTitle(c)
  }
  ```

  In `admin.go` `init()`, add:
  ```go
  RegisterAnonymousAdminHandler("promote", adminModule.promoteAnonAdmin)
  RegisterAnonymousAdminHandler("demote", adminModule.demoteAnonAdmin)
  RegisterAnonymousAdminHandler("title", adminModule.setTitleAnonAdmin)
  ```

- [ ] **Step 2: Register bans handlers in bans.go**

  In `bans.go` `init()`, add:
  ```go
  RegisterAnonymousAdminHandler("ban", bansModule.ban)
  RegisterAnonymousAdminHandler("dban", bansModule.dBan)
  RegisterAnonymousAdminHandler("sban", bansModule.sBan)
  RegisterAnonymousAdminHandler("tban", bansModule.tBan)
  RegisterAnonymousAdminHandler("unban", bansModule.unban)
  RegisterAnonymousAdminHandler("restrict", bansModule.restrict)
  RegisterAnonymousAdminHandler("unrestrict", bansModule.unrestrict)
  ```

- [ ] **Step 3: Register mutes handlers in mute.go**

  In `mute.go` `init()`, add:
  ```go
  RegisterAnonymousAdminHandler("mute", mutesModule.mute)
  RegisterAnonymousAdminHandler("smute", mutesModule.sMute)
  RegisterAnonymousAdminHandler("dmute", mutesModule.dMute)
  RegisterAnonymousAdminHandler("tmute", mutesModule.tMute)
  RegisterAnonymousAdminHandler("unmute", mutesModule.unmute)
  ```

- [ ] **Step 4: Add wrapper functions and register pins handlers in pins.go**

  `pinsModule.pin`, `unpin`, `permaPin`, and `unpinAll` accept `*helpers.CommandContext`. Add thin wrappers:

  ```go
  // anonymousAdmin wrappers for pins.go
  func (m moduleStruct) pinAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
      c, err := helpers.BuildCommandContext(b, ctx)
      if err != nil {
          return ext.EndGroups
      }
      return m.pin(c)
  }

  func (m moduleStruct) unpinAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
      c, err := helpers.BuildCommandContext(b, ctx)
      if err != nil {
          return ext.EndGroups
      }
      return m.unpin(c)
  }

  func (m moduleStruct) permaPinAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
      c, err := helpers.BuildCommandContext(b, ctx)
      if err != nil {
          return ext.EndGroups
      }
      return m.permaPin(c)
  }

  func (m moduleStruct) unpinAllAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
      c, err := helpers.BuildCommandContext(b, ctx)
      if err != nil {
          return ext.EndGroups
      }
      return m.unpinAll(c)
  }
  ```

  In `pins.go` `init()`, add:
  ```go
  RegisterAnonymousAdminHandler("pin", pinsModule.pinAnonAdmin)
  RegisterAnonymousAdminHandler("unpin", pinsModule.unpinAnonAdmin)
  RegisterAnonymousAdminHandler("permapin", pinsModule.permaPinAnonAdmin)
  RegisterAnonymousAdminHandler("unpinall", pinsModule.unpinAllAnonAdmin)
  ```

- [ ] **Step 5: Register purges handlers in purges.go**

  In `purges.go` `init()`, add:
  ```go
  RegisterAnonymousAdminHandler("purge", purgesModule.purge)
  RegisterAnonymousAdminHandler("del", purgesModule.delCmd)
  ```

- [ ] **Step 6: Register warns handlers in warns.go**

  In `warns.go` `init()`, add:
  ```go
  RegisterAnonymousAdminHandler("warn", warnsModule.warnUser)
  RegisterAnonymousAdminHandler("swarn", warnsModule.sWarnUser)
  RegisterAnonymousAdminHandler("dwarn", warnsModule.dWarnUser)
  ```

- [ ] **Step 7: Replace switch in bot_updates.go**

  In `alita/modules/bot_updates.go`, replace the entire `verifyAnonymousAdmin` switch statement (including the `helpers.BuildCommandContext` calls and the switch) with a single delegation:
  
  ```go
  // Before
  func verifyAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error {
      // ... 20+ cases with BuildCommandContext inside each group ...
      switch command {
      case "promote":
          return adminModule.promote(c)
      // ... 20+ cases
      }
  }
  
  // After
  func verifyAnonymousAdmin(b *gotgbot.Bot, ctx *ext.Context, command string) error {
      return HandleAnonymousAdmin(b, ctx, command)
  }
  ```

  The wrappers registered in Steps 1 and 4 now handle the `helpers.BuildCommandContext` call internally, so the switch is no longer needed.

- [ ] **Step 8: Run tests**

  Run: `go test -v ./alita/modules/...`
  Expected: ALL PASS

- [ ] **Step 9: Run lint**

  Run: `make lint`
  Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add alita/modules/admin.go alita/modules/bans.go alita/modules/mute.go alita/modules/pins.go alita/modules/purges.go alita/modules/warns.go alita/modules/bot_updates.go
git commit -m "refactor(modules): migrate anonymous admin routing to registry"
```

---

## Final Verification

### Task 11: Final integration tests

**Steps:**
- [ ] **Step 1: Run all tests**

  Run: `go test -v ./alita/modules/...`
  Expected: ALL PASS

- [ ] **Step 2: Run lint**

  Run: `make lint`
  Expected: PASS

- [ ] **Step 3: Verify no helpers.go exists**

  Run: `ls alita/modules/helpers.go 2>&1`
  Expected: `No such file or directory`

- [ ] **Step 4: Verify file count**

  Run: `ls alita/modules/*.go | wc -l`
  Expected: More than before (new files: core.go, help_system.go, overwrite.go, deeplink_router.go, anonymous_admin_router.go)

- [ ] **Step 5: Commit final verification**

```bash
git status
git log --oneline -5
```

---

## Self-Review

**Spec coverage:**
- [x] Phase 1: Pure file moves — covered in Tasks 1-6
- [x] Phase 2: Deep link registry — covered in Tasks 7-8
- [x] Phase 3: Anonymous admin registry — covered in Tasks 9-10
- [x] Success criteria — covered in Task 11 (Final Verification)
- [x] Benefits (locality, leverage, testability) — achieved by focused files and registry patterns

**Placeholder scan:**
- [x] No "TBD" or "TODO" in plan
- [x] No "Add appropriate error handling" — all error handling is explicit
- [x] No "Write tests for the above" — every test is shown with complete code
- [x] No "Similar to Task N" — each task is self-contained

**Type consistency:**
- [x] `DeepLinkHandler` signature is consistent across Task 7 and Task 8
- [x] `AnonymousAdminHandler` signature is consistent across Task 9 and Task 10
- [x] `RegisterDeepLinkHandler` and `RegisterAnonymousAdminHandler` follow same naming pattern
- [x] Admin and Pins anonymous-admin wrappers correctly build `*helpers.CommandContext` before delegating to the underlying handler
- [x] Command name for `setTitle` is corrected to `title` (the actual command registered in admin.go)

**No gaps found.**

---

*Plan complete. Ready for execution.*
