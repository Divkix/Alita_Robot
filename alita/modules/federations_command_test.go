package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func uniquePositiveUserID() int64 {
	return -uniqueModuleChatID()
}

func TestParseFedBanFileCSV(t *testing.T) {
	csv := "id,reason\n111,spam\n222,flood\n"
	bans, err := parseFedBanFile("bans.csv", []byte(csv))
	require.NoError(t, err)
	require.Len(t, bans, 2)
	require.Equal(t, int64(111), bans[0].UserID)
	require.Equal(t, "spam", bans[0].Reason)
	require.Equal(t, int64(222), bans[1].UserID)
}

func TestParseFedBanFileJSON(t *testing.T) {
	raw := `[{"id":333,"reason":"raid"},{"user_id":444,"reason":"scam"}]`
	bans, err := parseFedBanFile("bans.json", []byte(raw))
	require.NoError(t, err)
	require.Len(t, bans, 2)
	require.Equal(t, int64(333), bans[0].UserID)
	require.Equal(t, int64(444), bans[1].UserID)
}

func TestParseFedBanFileJSONL(t *testing.T) {
	raw := `{"id":555,"reason":"one"}` + "\n" + `{"id":666,"reason":"two"}` + "\n"
	bans, err := parseFedBanFile("bans.jsonl", []byte(raw))
	require.NoError(t, err)
	require.Len(t, bans, 2)
	require.Equal(t, int64(555), bans[0].UserID)
	require.Equal(t, int64(666), bans[1].UserID)
}

func TestParseFedBanFileRejectsEmpty(t *testing.T) {
	_, err := parseFedBanFile("bans.txt", []byte(""))
	require.Error(t, err)
}

func TestFormatFedBanJSONLRoundtrip(t *testing.T) {
	bans := []models.FederationBan{{UserID: 1, Reason: "a"}, {UserID: 2, Reason: "b"}}
	raw, err := formatFedBanJSONL(bans)
	require.NoError(t, err)
	parsed, err := parseFedBanFile("export.jsonl", raw)
	require.NoError(t, err)
	require.Len(t, parsed, 2)
}

func TestFormatFedBanCSVRoundtrip(t *testing.T) {
	bans := []models.FederationBan{{UserID: 9, Reason: "csv reason"}}
	raw, err := formatFedBanCSV(bans, true)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(raw), "9"))
	parsed, err := parseFedBanFile("export.csv", raw)
	require.NoError(t, err)
	require.Equal(t, int64(9), parsed[0].UserID)
	require.Equal(t, "csv reason", parsed[0].Reason)
}

func TestNewFedCreatesFederation(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	ownerID := uniquePositiveUserID()
	user := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	chat := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}
	ctx := newModuleMessageContext(bot, chat, user, "/newfed Rose Parity Fed")

	if err := federationsModule.newFed(bot, ctx); err != ext.EndGroups {
		t.Fatalf("newFed() error = %v, want EndGroups", err)
	}
	fed := federations.GetFedByOwner(ownerID)
	if fed == nil {
		t.Fatal("expected federation to be created")
	}
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	if fed.Name != "Rose Parity Fed" {
		t.Fatalf("name = %q", fed.Name)
	}
}

func TestJoinFedAndFban(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	fed, err := federations.CreateFederation(owner.Id, "Join Test Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	chatID := uniqueModuleChatID()
	group := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Fed Chat"}
	joinCtx := newModuleMessageContext(bot, group, owner, "/joinfed "+fed.FedID)
	if err := federationsModule.joinFed(bot, joinCtx); err != ext.EndGroups {
		t.Fatalf("joinFed() error = %v, want EndGroups", err)
	}
	membership := federations.GetChatFed(chatID)
	if membership == nil || membership.FedID != fed.FedID {
		t.Fatalf("membership = %+v", membership)
	}

	fbanCtx := newModuleMessageContext(bot, group, owner, "/fban 12345 spam")
	if err := federationsModule.fban(bot, fbanCtx); err != ext.EndGroups {
		t.Fatalf("fban() error = %v, want EndGroups", err)
	}
	if federations.GetFedBan(fed.FedID, 12345) == nil {
		t.Fatal("expected fban to persist")
	}
}

func TestJoinFedRejectsNonOwner(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	fed, err := federations.CreateFederation(uniquePositiveUserID(), "Owner Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Fed Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, member, "/joinfed "+fed.FedID)
	if err := federationsModule.joinFed(bot, ctx); err != ext.EndGroups {
		t.Fatalf("joinFed() error = %v, want EndGroups", err)
	}
	if federations.GetChatFed(chat.Id) != nil {
		t.Fatal("non-owner must not join a federation")
	}
}
