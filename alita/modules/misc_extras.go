package modules

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func fetchJSON(rawURL string, dest any) error {
	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func isDeletedAccount(firstName string) bool {
	name := strings.TrimSpace(firstName)
	return name == "" || strings.EqualFold(name, "Deleted Account")
}

func (moduleStruct) stickerID(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if chat_status.CheckDisabledCmd(b, msg, "stickerid") {
		return ext.EndGroups
	}
	if msg == nil || msg.ReplyToMessage == nil || msg.ReplyToMessage.Sticker == nil {
		text, _ := tr.GetString("misc_stickerid_need_sticker")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("misc_stickerid_result", i18n.TranslationParams{
		"id": html.EscapeString(msg.ReplyToMessage.Sticker.FileId),
	})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) zombies(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}
	scanText, _ := tr.GetString("misc_zombies_scanning")
	replyHTML(b, msg, scanText)
	cleaned := 0
	for _, userID := range chats.GetChatSettings(chat.Id).Users {
		if !chat_status.IsValidUserId(userID) || userID == b.Id {
			continue
		}
		member, err := b.GetChatMember(chat.Id, userID, nil)
		if err != nil || member == nil {
			continue
		}
		merged := member.MergeChatMember()
		if !isDeletedAccount(merged.User.FirstName) {
			continue
		}
		if err := kickMember(b, chat.Id, userID); err != nil {
			log.Debugf("[Misc] zombies kick %d: %v", userID, err)
			continue
		}
		cleaned++
	}
	if cleaned == 0 {
		text, _ := tr.GetString("misc_zombies_none")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("misc_zombies_cleaned", i18n.TranslationParams{"count": cleaned})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) github(b *gotgbot.Bot, ctx *ext.Context) error {
	return lookupAndReply(b, ctx, "github", "misc_github_usage", "misc_github_not_found", lookupGitHub)
}

func lookupGitHub(query string) (string, error) {
	query = strings.TrimPrefix(query, "https://github.com/")
	query = strings.Trim(query, "/")
	if strings.Contains(query, "/") {
		parts := strings.SplitN(query, "/", 2)
		return lookupGitHubRepo(parts[0], parts[1])
	}
	return lookupGitHubUser(query)
}

func lookupGitHubUser(login string) (string, error) {
	var payload struct {
		Login       string `json:"login"`
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		Bio         string `json:"bio"`
		PublicRepos int    `json:"public_repos"`
		Followers   int    `json:"followers"`
		Message     string `json:"message"`
	}
	if err := fetchJSON("https://api.github.com/users/"+url.PathEscape(login), &payload); err != nil {
		return "", err
	}
	if payload.Login == "" {
		return "", fmt.Errorf("not found")
	}
	name := payload.Name
	if name == "" {
		name = payload.Login
	}
	return fmt.Sprintf(
		"<b>%s</b> (<code>%s</code>)\n%s\nRepos: %d · Followers: %d",
		html.EscapeString(name),
		html.EscapeString(payload.Login),
		html.EscapeString(payload.HTMLURL),
		payload.PublicRepos,
		payload.Followers,
	), nil
}

func lookupGitHubRepo(owner, repo string) (string, error) {
	var payload struct {
		FullName    string `json:"full_name"`
		HTMLURL     string `json:"html_url"`
		Description string `json:"description"`
		Stargazers  int    `json:"stargazers_count"`
		Forks       int    `json:"forks_count"`
		Language    string `json:"language"`
	}
	path := url.PathEscape(owner) + "/" + url.PathEscape(repo)
	if err := fetchJSON("https://api.github.com/repos/"+path, &payload); err != nil {
		return "", err
	}
	if payload.FullName == "" {
		return "", fmt.Errorf("not found")
	}
	return fmt.Sprintf(
		"<b>%s</b>\n%s\n%s\n★ %d · forks %d · %s",
		html.EscapeString(payload.FullName),
		html.EscapeString(payload.HTMLURL),
		html.EscapeString(payload.Description),
		payload.Stargazers,
		payload.Forks,
		html.EscapeString(payload.Language),
	), nil
}

func lookupAndReply(
	b *gotgbot.Bot,
	ctx *ext.Context,
	cmd, usageKey, failKey string,
	lookup func(string) (string, error),
) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if chat_status.CheckDisabledCmd(b, msg, cmd) {
		return ext.EndGroups
	}
	query := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	if query == "" {
		text, _ := tr.GetString(usageKey)
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, err := lookup(query)
	if err != nil {
		fail, _ := tr.GetString(failKey)
		replyHTML(b, msg, fail)
		return ext.EndGroups
	}
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) wiki(b *gotgbot.Bot, ctx *ext.Context) error {
	return lookupAndReply(b, ctx, "wiki", "misc_wiki_usage", "misc_wiki_not_found", lookupWikipedia)
}

func lookupWikipedia(query string) (string, error) {
	api := "https://en.wikipedia.org/w/api.php?action=opensearch&limit=1&namespace=0&format=json&search=" +
		url.QueryEscape(query)
	var payload []any
	if err := fetchJSON(api, &payload); err != nil {
		return "", err
	}
	if len(payload) < 4 {
		return "", fmt.Errorf("unexpected wikipedia payload")
	}
	descs, _ := payload[2].([]any)
	links, _ := payload[3].([]any)
	if len(links) == 0 {
		return "", fmt.Errorf("not found")
	}
	title := query
	if titles, ok := payload[1].([]any); ok && len(titles) > 0 {
		if s, ok := titles[0].(string); ok {
			title = s
		}
	}
	desc := ""
	if len(descs) > 0 {
		desc, _ = descs[0].(string)
	}
	link, _ := links[0].(string)
	out := "<b>" + html.EscapeString(title) + "</b>"
	if desc != "" {
		out += "\n" + html.EscapeString(desc)
	}
	if link != "" {
		out += "\n" + html.EscapeString(link)
	}
	return out, nil
}

func (moduleStruct) urbanDict(b *gotgbot.Bot, ctx *ext.Context) error {
	return lookupAndReply(b, ctx, "ud", "misc_ud_usage", "misc_ud_not_found", lookupUrbanDictionary)
}

func lookupUrbanDictionary(query string) (string, error) {
	var payload struct {
		List []struct {
			Word       string `json:"word"`
			Definition string `json:"definition"`
			Example    string `json:"example"`
			Permalink  string `json:"permalink"`
		} `json:"list"`
	}
	if err := fetchJSON("https://api.urbandictionary.com/v0/define?term="+url.QueryEscape(query), &payload); err != nil {
		return "", err
	}
	if len(payload.List) == 0 {
		return "", fmt.Errorf("not found")
	}
	entry := payload.List[0]
	def := strings.ReplaceAll(entry.Definition, "[", "")
	def = strings.ReplaceAll(def, "]", "")
	if len(def) > 700 {
		def = def[:700] + "…"
	}
	return fmt.Sprintf(
		"<b>%s</b>\n%s",
		html.EscapeString(entry.Word),
		html.EscapeString(def),
	), nil
}

func (moduleStruct) runs(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if chat_status.CheckDisabledCmd(b, msg, "runs") {
		return ext.EndGroups
	}
	lines, err := tr.GetStringSlice("misc_runs")
	if err != nil || len(lines) == 0 {
		lines = []string{"Run."}
	}
	idx, err := secureIntn(len(lines))
	if err != nil {
		idx = 0
	}
	replyHTML(b, msg, html.EscapeString(lines[idx]))
	return ext.EndGroups
}

func registerMiscExtras(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("stickerid", miscModule.stickerID))
	helpers.AddCmdToDisableable("stickerid")
	dispatcher.AddHandler(handlers.NewCommand("zombies", miscModule.zombies))
	dispatcher.AddHandler(handlers.NewCommand("github", miscModule.github))
	helpers.AddCmdToDisableable("github")
	dispatcher.AddHandler(handlers.NewCommand("wiki", miscModule.wiki))
	helpers.AddCmdToDisableable("wiki")
	dispatcher.AddHandler(handlers.NewCommand("ud", miscModule.urbanDict))
	helpers.AddCmdToDisableable("ud")
	dispatcher.AddHandler(handlers.NewCommand("runs", miscModule.runs))
	helpers.AddCmdToDisableable("runs")
}
