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

func TestParseToggleAndFedIDHelpers(t *testing.T) {
	on, ok := parseToggleArg("YES")
	if !ok || !on {
		t.Fatal("parseToggleArg yes")
	}
	off, ok := parseToggleArg("disable")
	if !ok || off {
		t.Fatal("parseToggleArg disable")
	}
	if _, ok := parseToggleArg("maybe"); ok {
		t.Fatal("parseToggleArg maybe")
	}
	if _, ok := parseFedID("not-a-uuid"); ok {
		t.Fatal("parseFedID should reject junk")
	}
	id, ok := parseFedID("550e8400-e29b-41d4-a716-446655440000")
	if !ok || id == "" {
		t.Fatal("parseFedID uuid")
	}
}

func TestFedLifecycleCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Lifecycle Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}
	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Fed Group"}
	creator := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	if err := federationsModule.newFed(bot, newModuleMessageContext(bot, group, owner, "/newfed x")); err != ext.EndGroups {
		t.Fatalf("newFed group-only: %v", err)
	}
	if err := federationsModule.renameFed(bot, newModuleMessageContext(bot, pm, owner, "/renamefed Renamed Fed")); err != ext.EndGroups {
		t.Fatalf("renameFed: %v", err)
	}
	if got := federations.GetFed(fed.FedID); got == nil || got.Name != "Renamed Fed" {
		t.Fatalf("rename did not persist: %+v", got)
	}

	if err := federationsModule.joinFed(bot, newModuleMessageContext(bot, pm, creator, "/joinfed "+fed.FedID)); err != ext.EndGroups {
		t.Fatalf("joinFed private: %v", err)
	}
	if err := federationsModule.joinFed(bot, newModuleMessageContext(bot, group, creator, "/joinfed not-an-id")); err != ext.EndGroups {
		t.Fatalf("joinFed invalid: %v", err)
	}
	joinCtx := newModuleMessageContext(bot, group, creator, "/joinfed "+fed.FedID)
	if err := federationsModule.joinFed(bot, joinCtx); err != ext.EndGroups {
		t.Fatalf("joinFed: %v", err)
	}
	if federations.GetChatFed(group.Id) == nil {
		t.Fatal("expected join")
	}

	if err := federationsModule.quietFed(bot, newModuleMessageContext(bot, group, creator, "/quietfed")); err != ext.EndGroups {
		t.Fatalf("quietFed missing: %v", err)
	}
	if err := federationsModule.quietFed(bot, newModuleMessageContext(bot, group, creator, "/quietfed on")); err != ext.EndGroups {
		t.Fatalf("quietFed on: %v", err)
	}
	if membership := federations.GetChatFed(group.Id); membership == nil || !membership.Quiet {
		t.Fatal("quietfed on did not persist")
	}

	if err := federationsModule.chatFed(bot, newModuleMessageContext(bot, group, creator, "/chatfed")); err != ext.EndGroups {
		t.Fatalf("chatFed: %v", err)
	}
	if err := federationsModule.fedInfo(bot, newModuleMessageContext(bot, group, creator, "/fedinfo")); err != ext.EndGroups {
		t.Fatalf("fedInfo: %v", err)
	}
	if err := federationsModule.fedAdmins(bot, newModuleMessageContext(bot, group, creator, "/fedadmins")); err != ext.EndGroups {
		t.Fatalf("fedAdmins: %v", err)
	}
	if err := federationsModule.myFeds(bot, newModuleMessageContext(bot, pm, owner, "/myfeds")); err != ext.EndGroups {
		t.Fatalf("myFeds: %v", err)
	}

	if err := federationsModule.leaveFed(bot, newModuleMessageContext(bot, group, creator, "/leavefed")); err != ext.EndGroups {
		t.Fatalf("leaveFed: %v", err)
	}
	if federations.GetChatFed(group.Id) != nil {
		t.Fatal("leavefed did not remove membership")
	}
}

func TestFedAdminPromoteDemoteAndToggles(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Admin Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}

	if err := federationsModule.fedPromote(bot, newModuleMessageContext(bot, pm, owner, "/fedpromote 42")); err != ext.EndGroups {
		t.Fatalf("fedPromote: %v", err)
	}
	if !federations.IsFedAdmin(fed.FedID, 42) {
		t.Fatal("expected promoted admin")
	}
	if err := federationsModule.fedDemote(bot, newModuleMessageContext(bot, pm, owner, "/feddemote 42")); err != ext.EndGroups {
		t.Fatalf("fedDemote: %v", err)
	}
	if federations.IsFedAdmin(fed.FedID, 42) {
		t.Fatal("demote did not persist")
	}

	require.NoError(t, federations.PromoteFedAdmin(fed.FedID, 42))
	admin := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := federationsModule.fedDemoteMe(bot, newModuleMessageContext(bot, pm, admin, "/feddemoteme "+fed.FedID)); err != ext.EndGroups {
		t.Fatalf("fedDemoteMe: %v", err)
	}
	if federations.IsFedAdmin(fed.FedID, 42) {
		t.Fatal("feddemoteme did not persist")
	}

	if err := federationsModule.fedReason(bot, newModuleMessageContext(bot, pm, owner, "/fedreason on")); err != ext.EndGroups {
		t.Fatalf("fedReason: %v", err)
	}
	if !federations.GetFed(fed.FedID).RequireReason {
		t.Fatal("fedreason on did not persist")
	}
	if err := federationsModule.fedNotif(bot, newModuleMessageContext(bot, pm, owner, "/fednotif off")); err != ext.EndGroups {
		t.Fatalf("fedNotif: %v", err)
	}
	if federations.GetFed(fed.FedID).NotifyOwner {
		t.Fatal("fednotif off did not persist")
	}
}

func TestUnfbanAndFedStat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	owner := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Owner"}
	fed, err := federations.CreateFederation(owner.Id, "Ban Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ban Chat"}
	creator := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, federations.JoinFed(group.Id, group.Title, fed.FedID))
	require.NoError(t, federations.PromoteFedAdmin(fed.FedID, creator.Id))

	fbanCtx := newModuleMessageContext(bot, group, creator, "/fban 12345 spam")
	if err := federationsModule.fban(bot, fbanCtx); err != ext.EndGroups {
		t.Fatalf("fban: %v", err)
	}
	if federations.GetFedBan(fed.FedID, 12345) == nil {
		t.Fatal("expected fban")
	}
	if err := federationsModule.unfban(bot, newModuleMessageContext(bot, group, creator, "/unfban 12345")); err != ext.EndGroups {
		t.Fatalf("unfban: %v", err)
	}
	if federations.GetFedBan(fed.FedID, 12345) != nil {
		t.Fatal("unfban did not persist")
	}

	_, _, err = federations.Fban(fed.FedID, 12345, owner.Id, "again")
	require.NoError(t, err)
	if err := federationsModule.fedStat(bot, newModuleMessageContext(bot, group, creator, "/fedstat 12345")); err != ext.EndGroups {
		t.Fatalf("fedStat: %v", err)
	}
	if err := federationsModule.fedStat(bot, newModuleMessageContext(bot, group, creator, "/fedstat 12345 "+fed.FedID)); err != ext.EndGroups {
		t.Fatalf("fedStat reason: %v", err)
	}
}

func TestFedSubscribeLogExportAndDelete(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Export Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	other, err := federations.CreateFederation(uniquePositiveUserID(), "Target Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(other.FedID) })
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}

	if err := federationsModule.subFed(bot, newModuleMessageContext(bot, pm, owner, "/subfed "+other.FedID)); err != ext.EndGroups {
		t.Fatalf("subFed: %v", err)
	}
	if len(federations.ListSubscribedFedIDs(fed.FedID)) != 1 {
		t.Fatal("expected subscription")
	}
	if err := federationsModule.fedSubs(bot, newModuleMessageContext(bot, pm, owner, "/fedsubs")); err != ext.EndGroups {
		t.Fatalf("fedSubs: %v", err)
	}
	if err := federationsModule.unsubFed(bot, newModuleMessageContext(bot, pm, owner, "/unsubfed "+other.FedID)); err != ext.EndGroups {
		t.Fatalf("unsubFed: %v", err)
	}

	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Log Group"}
	if err := federationsModule.setFedLog(bot, newModuleMessageContext(bot, group, owner, "/setfedlog")); err != ext.EndGroups {
		t.Fatalf("setFedLog: %v", err)
	}
	if federations.GetFed(fed.FedID).LogChatID != group.Id {
		t.Fatal("setfedlog did not persist")
	}
	if err := federationsModule.unsetFedLog(bot, newModuleMessageContext(bot, pm, owner, "/unsetfedlog")); err != ext.EndGroups {
		t.Fatalf("unsetFedLog: %v", err)
	}
	if federations.GetFed(fed.FedID).LogChatID != 0 {
		t.Fatal("unsetfedlog did not clear")
	}

	_, _, err = federations.Fban(fed.FedID, 99, ownerID, "export")
	require.NoError(t, err)
	if err := federationsModule.fbanList(bot, newModuleMessageContext(bot, pm, owner, "/fbanlist json")); err != ext.EndGroups {
		t.Fatalf("fbanList: %v", err)
	}

	if err := federationsModule.delFed(bot, newModuleMessageContext(bot, pm, owner, "/delfed")); err != ext.EndGroups {
		t.Fatalf("delFed: %v", err)
	}
	data := encodeCallbackData(fedCallbackNamespace, map[string]string{"a": "del"})
	cb := newModuleCallbackContext(bot, pm, owner, data)
	if err := federationsModule.fedCallback(bot, cb); err != ext.EndGroups {
		t.Fatalf("fedCallback del: %v", err)
	}
	if federations.GetFed(fed.FedID) != nil {
		t.Fatal("expected federation to be deleted")
	}
}

func TestEnforceFedBanOnJoinAndMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	ownerID := uniquePositiveUserID()
	fed, err := federations.CreateFederation(ownerID, "Enforce Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Enforce Chat"}
	require.NoError(t, federations.JoinFed(group.Id, group.Title, fed.FedID))
	_, _, err = federations.Fban(fed.FedID, 42, ownerID, "raid")
	require.NoError(t, err)

	member := gotgbot.User{Id: 42, FirstName: "Member"}
	joinCtx := newModuleMessageContext(bot, group, member, "hello")
	joinCtx.EffectiveMessage.NewChatMembers = []gotgbot.User{member}
	if err := federationsModule.enforceFedBan(bot, joinCtx); err != ext.ContinueGroups {
		t.Fatalf("enforceFedBan join: %v", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) == 0 {
		t.Fatal("expected ban on fban join")
	}

	msgCtx := newModuleMessageContext(bot, group, member, "still here")
	if err := federationsModule.enforceFedBan(bot, msgCtx); err != ext.ContinueGroups {
		t.Fatalf("enforceFedBan message: %v", err)
	}
}

func TestParseFedBanCSVWithoutHeader(t *testing.T) {
	bans, err := parseFedBanFile("bans.csv", []byte("888,no header\n"))
	require.NoError(t, err)
	require.Equal(t, int64(888), bans[0].UserID)
}

func TestAnyToInt64ViaJSON(t *testing.T) {
	raw := `{"id":"777","reason":"str"}` + "\n" + `{"user_id":12,"reason":"n"}` + "\n"
	bans, err := parseFedBanFile("bans.jsonl", []byte(raw))
	require.NoError(t, err)
	require.Equal(t, int64(777), bans[0].UserID)
	require.Equal(t, int64(12), bans[1].UserID)
}
