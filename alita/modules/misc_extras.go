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
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
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
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	scanText, _ := tr.GetString("misc_zombies_scanning")
	replyHTML(b, msg, scanText)
	cleaned := 0
	users, _ := chats.GetChatUsersCachedContext(tracing.UpdateContext(ctx), chat.Id)
	for _, userID := range users {
		if !chat_status.IsValidUserId(userID) || chat_status.IsSystemUser(userID) || userID == b.Id {
			continue
		}
		member, err := b.GetChatMemberWithContext(tracing.UpdateContext(ctx), chat.Id, userID, nil)
		if err != nil || member == nil {
			continue
		}
		merged := member.MergeChatMember()
		if !isDeletedAccount(merged.User.FirstName) {
			continue
		}
		if _, err := punishment.Apply(tracing.UpdateContext(ctx), b, punishment.Request{
			ChatID: chat.Id, Target: punishment.Target{ID: userID}, Mode: punishment.Kick,
		}); err != nil {
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
	return lookupAndReply(b, ctx, "misc_github_usage", "misc_github_not_found", lookupGitHub)
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
	usageKey, failKey string,
	lookup func(string) (string, error),
) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
	return lookupAndReply(b, ctx, "misc_wiki_usage", "misc_wiki_not_found", lookupWikipedia)
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
	return lookupAndReply(b, ctx, "misc_ud_usage", "misc_ud_not_found", lookupUrbanDictionary)
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
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "stickerid", Disableable: true}, pipelineHandler(miscModule.stickerID))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "zombies",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireUserAdmin(),
			helpers.CanBotRestrict(),
		},
	}, pipelineHandler(miscModule.zombies))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "github", Disableable: true}, pipelineHandler(miscModule.github))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "wiki", Disableable: true}, pipelineHandler(miscModule.wiki))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "ud", Disableable: true}, pipelineHandler(miscModule.urbanDict))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "runs", Disableable: true}, pipelineHandler(miscModule.runs))
}
