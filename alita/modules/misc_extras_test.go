package modules

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestIsDeletedAccount(t *testing.T) {
	if !isDeletedAccount("", "") || !isDeletedAccount("Deleted Account", "") {
		t.Fatal("expected deleted-account detection")
	}
	if isDeletedAccount("Alice", "alice") {
		t.Fatal("live accounts must not be treated as deleted")
	}
}

func TestStickerIDRequiresStickerReply(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Misc Chat"}
	user := gotgbot.User{Id: 42, FirstName: "Member"}

	missing := newModuleMessageContext(bot, chat, user, "/stickerid")
	if err := miscModule.stickerID(bot, missing); err != ext.EndGroups {
		t.Fatalf("stickerID missing reply: %v", err)
	}

	okCtx := newModuleMessageContext(bot, chat, user, "/stickerid")
	okCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 55,
		Date:      1,
		Chat:      chat,
		Sticker:   &gotgbot.Sticker{FileId: "sticker-file-id", FileUniqueId: "u1"},
	}
	if err := miscModule.stickerID(bot, okCtx); err != ext.EndGroups {
		t.Fatalf("stickerID: %v", err)
	}
}

func TestRunsAlwaysEndsGroups(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "User"}
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "User"}
	ctx := newModuleMessageContext(bot, chat, user, "/runs")
	if err := miscModule.runs(bot, ctx); err != ext.EndGroups {
		t.Fatalf("runs: %v", err)
	}
}

func TestGitHubWikiUDRequireQueryAndAcceptMocks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "User"}
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "User"}

	if err := miscModule.github(bot, newModuleMessageContext(bot, chat, user, "/github")); err != ext.EndGroups {
		t.Fatalf("github usage: %v", err)
	}
	if err := miscModule.wiki(bot, newModuleMessageContext(bot, chat, user, "/wiki")); err != ext.EndGroups {
		t.Fatalf("wiki usage: %v", err)
	}
	if err := miscModule.urbanDict(bot, newModuleMessageContext(bot, chat, user, "/ud")); err != ext.EndGroups {
		t.Fatalf("ud usage: %v", err)
	}

	withMiscHTTPClient(t, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := "{}"
		switch {
		case strings.Contains(req.URL.Path, "/users/"):
			body = `{"login":"octocat","name":"The Octocat","html_url":"https://github.com/octocat","public_repos":8,"followers":1}`
		case strings.Contains(req.URL.Path, "/repos/"):
			body = `{"full_name":"octocat/Hello-World","html_url":"https://github.com/octocat/Hello-World","description":"demo","stargazers_count":1,"forks_count":2,"language":"Go"}`
		case strings.Contains(req.URL.Host, "wikipedia.org"):
			body = `["Go",["Go"],["A language"],["https://en.wikipedia.org/wiki/Go"]]`
		case strings.Contains(req.URL.Host, "urbandictionary.com"):
			body = `{"list":[{"word":"bot","definition":"a [program]","example":"","permalink":"https://example"}]}`
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})})

	if err := miscModule.github(bot, newModuleMessageContext(bot, chat, user, "/github octocat")); err != ext.EndGroups {
		t.Fatalf("github user: %v", err)
	}
	if err := miscModule.github(bot, newModuleMessageContext(bot, chat, user, "/github octocat/Hello-World")); err != ext.EndGroups {
		t.Fatalf("github repo: %v", err)
	}
	if err := miscModule.wiki(bot, newModuleMessageContext(bot, chat, user, "/wiki Go")); err != ext.EndGroups {
		t.Fatalf("wiki: %v", err)
	}
	if err := miscModule.urbanDict(bot, newModuleMessageContext(bot, chat, user, "/ud bot")); err != ext.EndGroups {
		t.Fatalf("ud: %v", err)
	}
}

func TestZombiesGroupAdminPath(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Zombie Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, admin, "/zombies")
	if err := miscModule.zombies(bot, ctx); err != ext.EndGroups {
		t.Fatalf("zombies: %v", err)
	}
}

func TestZombiesKicksDeletedAccounts(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatMember"] = []byte(
		`{"status":"member","user":{"id":88,"is_bot":false,"first_name":"Deleted Account"}}`,
	)
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	require.NoError(t, chats.EnsureChatInDb(chatID, "zombie chat"))
	require.NoError(t, db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatID).
		Update("users", models.Int64Array{88}).Error)

	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Zombie Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := miscModule.zombies(bot, newModuleMessageContext(bot, chat, admin, "/zombies")); err != ext.EndGroups {
		t.Fatalf("zombies deleted: %v", err)
	}
	if calls := client.callsFor("unbanChatMember"); len(calls) == 0 {
		t.Fatal("expected deleted account to be kicked")
	}
}

func TestFetchJSONUnexpectedStatus(t *testing.T) {
	withMiscHTTPClient(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
		}, nil
	})})
	var dest map[string]any
	if err := fetchJSON("https://example.invalid/x", &dest); err == nil {
		t.Fatal("expected status error")
	}
}
