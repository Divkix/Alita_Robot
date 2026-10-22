package modules

import (
	"html"
	"regexp"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// commandHelpModules maps each command that answers a wrong call with its own
// help to the module whose <module>_help_msg documents it. The warn commands
// (/warn, /swarn, /rmwarn, /resetwarns, /warns) do so only from PM: in a group
// they answer a call without a user with common_no_user_specified alone, as
// before PM support.
var commandHelpModules = map[string]string{
	"ban":        "bans",
	"sban":       "bans",
	"tban":       "bans",
	"dban":       "bans",
	"unban":      "bans",
	"kick":       "bans",
	"dkick":      "bans",
	"skick":      "bans",
	"restrict":   "bans",
	"unrestrict": "bans",
	"mute":       "mutes",
	"smute":      "mutes",
	"tmute":      "mutes",
	"dmute":      "mutes",
	"unmute":     "mutes",
	"digest":     "digest",
	"warn":       "warns",
	"swarn":      "warns",
	"rmwarn":     "warns",
	"resetwarns": "warns",
	"warns":      "warns",
	"warnings":   "warns",
	"rules":      "rules",
	"notes":      "notes",
	"saved":      "notes",
	"filters":    "filters",
	"connect":    "connections",
	"setlog":     "logchannels",
	"unsetlog":   "logchannels",
	"logchannel": "logchannels",
	"log":        "logchannels",
	"nolog":      "logchannels",

	"locks":            "locks",
	"locktypes":        "locks",
	"lock":             "locks",
	"unlock":           "locks",
	"flood":            "antiflood",
	"setflood":         "antiflood",
	"setfloodmode":     "antiflood",
	"delflood":         "antiflood",
	"disabled":         "disabling",
	"disable":          "disabling",
	"enable":           "disabling",
	"disabledel":       "disabling",
	"reactions":        "reactions",
	"addreaction":      "reactions",
	"removereaction":   "reactions",
	"resetreactions":   "reactions",
	"antiraid":         "antiraid",
	"raidtime":         "antiraid",
	"raidactiontime":   "antiraid",
	"autoantiraid":     "antiraid",
	"antichannelpin":   "pins",
	"cleanlinked":      "pins",
	"allowconnect":     "connections",
	"setrules":         "rules",
	"resetrules":       "rules",
	"privaterules":     "rules",
	"rulesbtn":         "rules",
	"resetrulesbutton": "rules",
	"blacklists":       "blacklists",
	"addblacklist":     "blacklists",
	"rmblacklist":      "blacklists",
	"blaction":         "blacklists",
	"approved":         "approvals",
	"approve":          "approvals",
	"unapprove":        "approvals",
	"approval":         "approvals",
	// WrapCommand settings commands, with the group from PM.
	"welcome":            "greetings",
	"setwelcome":         "greetings",
	"resetwelcome":       "greetings",
	"goodbye":            "greetings",
	"setgoodbye":         "greetings",
	"resetgoodbye":       "greetings",
	"cleanwelcome":       "greetings",
	"cleangoodbye":       "greetings",
	"cleanservice":       "greetings",
	"autoapprove":        "greetings",
	"captcha":            "captcha",
	"captchamode":        "captcha",
	"captchatime":        "captcha",
	"captchaaction":      "captcha",
	"captchamaxattempts": "captcha",
	"captchapending":     "captcha",
	"captchaclear":       "captcha",
	"setwarnlimit":       "warns",
	"setwarnmode":        "warns",
	"setwarntime":        "warns",
	"aispam":             "aispam",
	"spamdb":             "spamdb",
	"anonadmin":          "admin",
	// Group resets, with the group from PM.
	"privnote":      "notes",
	"clearall":      "notes",
	"stopall":       "filters",
	"remallbl":      "blacklists",
	"unapproveall":  "approvals",
	"resetallwarns": "warns",
	"unbanall":      "bans",
	// Group-only admin commands, with the group from PM.
	"adminlist":       "admin",
	"invitelink":      "admin",
	"setgtitle":       "admin",
	"setgdesc":        "admin",
	"admincache":      "admin",
	"clearadmincache": "admin",
	"timezone":        "admin",
}

// menuCommand is one command of the help menu: the module whose
// <module>_help_msg documents it, and whether only admins can use it.
type menuCommand struct {
	Module    string
	AdminOnly bool
}

// menuCommands lists every command of every en.yml <module>_extend_msg, and
// nothing else. A command is AdminOnly only when no form of it works for a
// member: /warns without a user shows your own warns, /lang in PM sets your
// own language, /invitelink answers a member in a public group, and the
// federation commands are gated by federation roles, not group admin rights.
// Dev commands have no help module, so they are not listed.
var menuCommands = map[string]menuCommand{
	"adminlist":       {Module: "admin"},
	"promote":         {Module: "admin", AdminOnly: true},
	"demote":          {Module: "admin", AdminOnly: true},
	"title":           {Module: "admin", AdminOnly: true},
	"invitelink":      {Module: "admin"},
	"anonadmin":       {Module: "admin"},
	"admincache":      {Module: "admin", AdminOnly: true},
	"clearadmincache": {Module: "admin", AdminOnly: true},
	"setgtitle":       {Module: "admin", AdminOnly: true},
	"setgpic":         {Module: "admin", AdminOnly: true},
	"setgdesc":        {Module: "admin", AdminOnly: true},
	"timezone":        {Module: "admin", AdminOnly: true},

	"aispam": {Module: "aispam", AdminOnly: true},

	"flood":        {Module: "antiflood"},
	"setflood":     {Module: "antiflood", AdminOnly: true},
	"setfloodmode": {Module: "antiflood", AdminOnly: true},
	"delflood":     {Module: "antiflood", AdminOnly: true},

	"antiraid":       {Module: "antiraid", AdminOnly: true},
	"raidtime":       {Module: "antiraid", AdminOnly: true},
	"raidactiontime": {Module: "antiraid", AdminOnly: true},
	"autoantiraid":   {Module: "antiraid", AdminOnly: true},

	"approve":      {Module: "approvals", AdminOnly: true},
	"unapprove":    {Module: "approvals", AdminOnly: true},
	"approval":     {Module: "approvals", AdminOnly: true},
	"approved":     {Module: "approvals", AdminOnly: true},
	"unapproveall": {Module: "approvals", AdminOnly: true},

	"export": {Module: "backup", AdminOnly: true},
	"import": {Module: "backup", AdminOnly: true},
	"reset":  {Module: "backup", AdminOnly: true},

	"ban":        {Module: "bans", AdminOnly: true},
	"sban":       {Module: "bans", AdminOnly: true},
	"dban":       {Module: "bans", AdminOnly: true},
	"tban":       {Module: "bans", AdminOnly: true},
	"unban":      {Module: "bans", AdminOnly: true},
	"kick":       {Module: "bans", AdminOnly: true},
	"skick":      {Module: "bans", AdminOnly: true},
	"dkick":      {Module: "bans", AdminOnly: true},
	"kickme":     {Module: "bans"},
	"banme":      {Module: "bans"},
	"restrict":   {Module: "bans", AdminOnly: true},
	"unrestrict": {Module: "bans", AdminOnly: true},
	"unbanall":   {Module: "bans", AdminOnly: true},

	"blacklists":      {Module: "blacklists"},
	"addblacklist":    {Module: "blacklists", AdminOnly: true},
	"rmblacklist":     {Module: "blacklists", AdminOnly: true},
	"blaction":        {Module: "blacklists", AdminOnly: true},
	"blacklistaction": {Module: "blacklists", AdminOnly: true},
	"remallbl":        {Module: "blacklists", AdminOnly: true},

	"captcha":            {Module: "captcha", AdminOnly: true},
	"captchamode":        {Module: "captcha", AdminOnly: true},
	"captchatime":        {Module: "captcha", AdminOnly: true},
	"captchaaction":      {Module: "captcha", AdminOnly: true},
	"captchamaxattempts": {Module: "captcha", AdminOnly: true},
	"captchapending":     {Module: "captcha", AdminOnly: true},
	"captchaclear":       {Module: "captcha", AdminOnly: true},

	"connect":      {Module: "connections"},
	"disconnect":   {Module: "connections"},
	"reconnect":    {Module: "connections"},
	"connection":   {Module: "connections"},
	"settings":     {Module: "connections", AdminOnly: true},
	"allowconnect": {Module: "connections", AdminOnly: true},

	"digest": {Module: "digest", AdminOnly: true},

	"disable":     {Module: "disabling", AdminOnly: true},
	"enable":      {Module: "disabling", AdminOnly: true},
	"disableable": {Module: "disabling"},
	"disabled":    {Module: "disabling"},
	"disabledel":  {Module: "disabling", AdminOnly: true},

	"newfed":      {Module: "federations"},
	"renamefed":   {Module: "federations"},
	"delfed":      {Module: "federations"},
	"fedinfo":     {Module: "federations"},
	"fedadmins":   {Module: "federations"},
	"fedpromote":  {Module: "federations"},
	"feddemote":   {Module: "federations"},
	"feddemoteme": {Module: "federations"},
	"joinfed":     {Module: "federations", AdminOnly: true},
	"leavefed":    {Module: "federations", AdminOnly: true},
	"chatfed":     {Module: "federations"},
	"myfeds":      {Module: "federations"},
	"fban":        {Module: "federations"},
	"unfban":      {Module: "federations"},
	"funban":      {Module: "federations"},
	"fedstat":     {Module: "federations"},
	"fbanstat":    {Module: "federations"},
	"fedreason":   {Module: "federations"},
	"fednotif":    {Module: "federations"},
	"quietfed":    {Module: "federations", AdminOnly: true},
	"subfed":      {Module: "federations"},
	"unsubfed":    {Module: "federations"},
	"fedsubs":     {Module: "federations"},
	"fbanlist":    {Module: "federations"},
	"importfbans": {Module: "federations"},
	"setfedlog":   {Module: "federations"},
	"unsetfedlog": {Module: "federations"},

	"filter":  {Module: "filters", AdminOnly: true},
	"filters": {Module: "filters"},
	"stop":    {Module: "filters", AdminOnly: true},
	"stopall": {Module: "filters", AdminOnly: true},

	"markdownhelp": {Module: "formatting"},
	"formatting":   {Module: "formatting"},

	"welcome":      {Module: "greetings", AdminOnly: true},
	"setwelcome":   {Module: "greetings", AdminOnly: true},
	"resetwelcome": {Module: "greetings", AdminOnly: true},
	"goodbye":      {Module: "greetings", AdminOnly: true},
	"setgoodbye":   {Module: "greetings", AdminOnly: true},
	"resetgoodbye": {Module: "greetings", AdminOnly: true},
	"cleanwelcome": {Module: "greetings", AdminOnly: true},
	"cleangoodbye": {Module: "greetings", AdminOnly: true},
	"cleanservice": {Module: "greetings", AdminOnly: true},
	"autoapprove":  {Module: "greetings", AdminOnly: true},

	"lang": {Module: "languages"},

	"lock":      {Module: "locks", AdminOnly: true},
	"unlock":    {Module: "locks", AdminOnly: true},
	"locks":     {Module: "locks", AdminOnly: true},
	"locktypes": {Module: "locks"},

	"setlog":        {Module: "logchannels", AdminOnly: true},
	"unsetlog":      {Module: "logchannels", AdminOnly: true},
	"logchannel":    {Module: "logchannels"},
	"logcategories": {Module: "logchannels"},
	"log":           {Module: "logchannels", AdminOnly: true},
	"nolog":         {Module: "logchannels", AdminOnly: true},

	"info":              {Module: "misc"},
	"id":                {Module: "misc"},
	"ping":              {Module: "misc"},
	"stat":              {Module: "misc"},
	"tell":              {Module: "misc", AdminOnly: true},
	"tr":                {Module: "misc"},
	"removebotkeyboard": {Module: "misc"},
	"stickerid":         {Module: "misc"},
	"zombies":           {Module: "misc", AdminOnly: true},
	"github":            {Module: "misc"},
	"wiki":              {Module: "misc"},
	"ud":                {Module: "misc"},
	"runs":              {Module: "misc"},

	"mute":   {Module: "mutes", AdminOnly: true},
	"smute":  {Module: "mutes", AdminOnly: true},
	"dmute":  {Module: "mutes", AdminOnly: true},
	"tmute":  {Module: "mutes", AdminOnly: true},
	"unmute": {Module: "mutes", AdminOnly: true},

	"save":         {Module: "notes", AdminOnly: true},
	"addnote":      {Module: "notes", AdminOnly: true},
	"get":          {Module: "notes"},
	"notes":        {Module: "notes"},
	"saved":        {Module: "notes"},
	"clear":        {Module: "notes", AdminOnly: true},
	"rmnote":       {Module: "notes", AdminOnly: true},
	"clearall":     {Module: "notes", AdminOnly: true},
	"privatenotes": {Module: "notes", AdminOnly: true},
	"privnote":     {Module: "notes", AdminOnly: true},

	"pin":            {Module: "pins", AdminOnly: true},
	"unpin":          {Module: "pins", AdminOnly: true},
	"unpinall":       {Module: "pins", AdminOnly: true},
	"pinned":         {Module: "pins"},
	"permapin":       {Module: "pins", AdminOnly: true},
	"antichannelpin": {Module: "pins", AdminOnly: true},
	"cleanlinked":    {Module: "pins", AdminOnly: true},

	"del":       {Module: "purges", AdminOnly: true},
	"purge":     {Module: "purges", AdminOnly: true},
	"purgefrom": {Module: "purges", AdminOnly: true},
	"purgeto":   {Module: "purges", AdminOnly: true},

	"addreaction":    {Module: "reactions", AdminOnly: true},
	"removereaction": {Module: "reactions", AdminOnly: true},
	"reactions":      {Module: "reactions"},
	"resetreactions": {Module: "reactions", AdminOnly: true},

	"report":  {Module: "reports"},
	"reports": {Module: "reports", AdminOnly: true},

	"rules":            {Module: "rules"},
	"setrules":         {Module: "rules", AdminOnly: true},
	"resetrules":       {Module: "rules", AdminOnly: true},
	"privaterules":     {Module: "rules", AdminOnly: true},
	"rulesbtn":         {Module: "rules", AdminOnly: true},
	"resetrulesbutton": {Module: "rules", AdminOnly: true},
	"resetrulesbtn":    {Module: "rules", AdminOnly: true},

	"spamdb": {Module: "spamdb", AdminOnly: true},

	"warn":          {Module: "warns", AdminOnly: true},
	"swarn":         {Module: "warns", AdminOnly: true},
	"dwarn":         {Module: "warns", AdminOnly: true},
	"warns":         {Module: "warns"},
	"rmwarn":        {Module: "warns", AdminOnly: true},
	"resetwarns":    {Module: "warns", AdminOnly: true},
	"resetallwarns": {Module: "warns", AdminOnly: true},
	"warnings":      {Module: "warns", AdminOnly: true},
	"setwarnlimit":  {Module: "warns", AdminOnly: true},
	"setwarntime":   {Module: "warns", AdminOnly: true},
	"setwarnmode":   {Module: "warns", AdminOnly: true},
}

// helpCommandLine matches a help line that defines commands: an optional
// bullet followed by one or more /commands ("× /fban /unfban: ..."). Commands
// mentioned inside prose ("use the /id command") are cross-references.
var (
	helpCommandLine = regexp.MustCompile(`(?m)^\s*(?:[×•-]>?\s*)?((?:/[a-z][a-z_]*\s*)+)`)
	helpCommandName = regexp.MustCompile(`/([a-z][a-z_]*)`)
)

// commandHelpBlock returns the entry for cmd in a module's help: the line that
// defines it and the unmarked lines that continue it, up to a blank line, a
// heading, or the next entry. It returns "" when the help has no such entry.
func commandHelpBlock(helpMsg, cmd string) string {
	lines := strings.Split(helpMsg, "\n")
	for i, line := range lines {
		if !definesCommand(line, cmd) {
			continue
		}
		block := []string{strings.TrimSpace(line)}
		for _, next := range lines[i+1:] {
			next = strings.TrimSpace(next)
			if next == "" || startsHelpEntry(next) {
				break
			}
			block = append(block, next)
		}
		return strings.Join(block, "\n")
	}
	return ""
}

func definesCommand(line, cmd string) bool {
	m := helpCommandLine.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	for _, name := range helpCommandName.FindAllStringSubmatch(m[1], -1) {
		if name[1] == cmd {
			return true
		}
	}
	return false
}

func startsHelpEntry(line string) bool {
	return helpCommandLine.MatchString(line) || strings.HasPrefix(line, "*") ||
		strings.HasPrefix(line, "×") || strings.HasPrefix(line, "•") || strings.HasPrefix(line, "- ")
}

// commandExample returns the sample call for cmd from locales/config.yml, or ""
// when the command has none.
func commandExample(cmd string) string {
	text, _ := i18n.MustNewTranslator("config").GetString(cmd + "_example")
	return text
}

// commandPMExample returns the sample call for cmd shown in PM help without a
// group: <cmd>_pm_example when the group's call (commandExample) names no
// group, else commandExample.
func commandPMExample(cmd string) string {
	if text, _ := i18n.MustNewTranslator("config").GetString(cmd + "_pm_example"); text != "" {
		return text
	}
	return commandExample(cmd)
}

// commandHelpTextWith puts errText (HTML) above the help for cmd: its own
// entry and example, the sample call, when the chat's language documents it,
// else the whole module help.
func commandHelpTextWith(tr *i18n.Translator, cmd, errText, example string) string {
	module, ok := commandHelpModules[cmd]
	if !ok {
		return errText
	}
	body := commandHelpBody(tr, module, cmd, example)
	if body == "" {
		return errText
	}
	return errText + "\n\n" + body
}

// commandHelpBody is the help for cmd (HTML) from module's help: its own entry
// and example when the chat's language documents it, else the whole module
// help. It is "" when the language has no help for module.
func commandHelpBody(tr *i18n.Translator, module, cmd, example string) string {
	helpMsg, _ := tr.GetString(module + "_help_msg")
	if helpMsg == "" {
		return ""
	}
	block := commandHelpBlock(helpMsg, cmd)
	if block == "" {
		header, _ := tr.GetString("helpers_module_help_header")
		return renderModuleHelp(header, strings.ToUpper(module[:1])+module[1:], helpMsg)
	}
	text := formatting.ToTelegramHTML(block)
	if example != "" {
		line, _ := tr.GetString("command_help_example", i18n.TranslationParams{
			"example": "<code>" + html.EscapeString(example) + "</code>",
		})
		if line != "" {
			text += "\n\n" + line
		}
	}
	return text
}

// replyCommandHelp answers a call that cmd cannot run as given with errText
// (HTML) and the help that shows how to call it: from PM, with the PM example
// (commandPMExample).
func replyCommandHelp(b *gotgbot.Bot, msg *gotgbot.Message, tr *i18n.Translator, cmd, errText string) error {
	example := commandExample(cmd)
	if inPM(msg) {
		example = commandPMExample(cmd)
	}
	_, err := msg.Reply(b, commandHelpTextWith(tr, cmd, errText, example), formatting.Shtml())
	return err
}
