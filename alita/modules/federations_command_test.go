package modules

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
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

func TestImportFBansNeedsFileAndOwnedFedLookup(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "IO Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}

	if err := federationsModule.importFBans(bot, newModuleMessageContext(bot, pm, owner, "/importfbans")); err != ext.EndGroups {
		t.Fatalf("importFBans: %v", err)
	}
	if _, err := downloadTelegramFile(nil, ""); err == nil {
		t.Fatal("downloadTelegramFile should reject missing file")
	}

	if ownedFedOrReply(bot, newModuleMessageContext(bot, pm, owner, "/noop")) == nil {
		t.Fatal("ownedFedOrReply should find the fed")
	}
	stranger := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Other"}
	if ownedFedOrReply(bot, newModuleMessageContext(bot, pm, stranger, "/noop")) != nil {
		t.Fatal("ownedFedOrReply should be nil for a non-owner")
	}

	notifyFedAction(bot, federations.GetFed(fed.FedID), &stranger, "notify")
	applyActiveFban(bot, fed.FedID, 42)
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

// The owner cannot step down; /feddemoteme tells them so and keeps the fed.
func TestFedDemoteMeFromOwnerRepliesOwnerCannotStepDown(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Demoteme Owner Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}

	if err := federationsModule.fedDemoteMe(bot, newModuleMessageContext(bot, pm, owner, "/feddemoteme "+fed.FedID)); err != ext.EndGroups {
		t.Fatalf("fedDemoteMe: %v, want EndGroups", err)
	}
	want, err := i18n.MustNewTranslator("en").GetString("feds_demoteme_owner")
	require.NoError(t, err)
	require.NotEmpty(t, want)
	calls := client.callsFor("sendMessage")
	require.Len(t, calls, 1)
	require.Equal(t, want, calls[0].Params["text"])
	require.True(t, federations.IsFedOwner(fed.FedID, ownerID), "owner must keep the federation")
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

// seedFedChat registers a chat in the federation and records the given users as
// its known members, the way the users tracker does in production.
func seedFedChat(t *testing.T, fedID string, members ...int64) gotgbot.Chat {
	t.Helper()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Fed Member Chat"}
	require.NoError(t, chats.EnsureChatInDb(chat.Id, chat.Title))
	t.Cleanup(func() { _ = db.DB.Where("chat_id = ?", chat.Id).Delete(&models.Chat{}).Error })
	require.NoError(t, db.DB.Model(&models.Chat{}).Where("chat_id = ?", chat.Id).
		Update("users", models.Int64Array(members)).Error)
	require.NoError(t, federations.JoinFed(chat.Id, chat.Title, fedID))
	return chat
}

func unbanTargets(client *moduleBotClient, userID int64) []int64 {
	var chatIDs []int64
	for _, call := range client.callsFor("unbanChatMember") {
		if uid, _ := call.Params["user_id"].(int64); uid != userID {
			continue
		}
		if cid, ok := call.Params["chat_id"].(int64); ok {
			chatIDs = append(chatIDs, cid)
		}
	}
	return chatIDs
}

func TestUnfbanLiftsBanInFederationChats(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	owner := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Owner"}
	fed, err := federations.CreateFederation(owner.Id, "Unfban Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	target := uniquePositiveUserID()
	chatA := seedFedChat(t, fed.FedID, target)
	chatB := seedFedChat(t, fed.FedID, target)
	chatOther := seedFedChat(t, fed.FedID, uniquePositiveUserID())
	_, _, err = federations.Fban(fed.FedID, target, owner.Id, "spam")
	require.NoError(t, err)

	ctx := newModuleMessageContext(bot, chatA, owner, "/unfban "+strconv.FormatInt(target, 10))
	if err := federationsModule.unfban(bot, ctx); err != ext.EndGroups {
		t.Fatalf("unfban: %v", err)
	}

	require.Eventually(t, func() bool { return len(unbanTargets(client, target)) == 2 },
		2*time.Second, 10*time.Millisecond, "unfban must unban the user in every federation chat that knows them")
	require.ElementsMatch(t, []int64{chatA.Id, chatB.Id}, unbanTargets(client, target))
	require.NotContains(t, unbanTargets(client, target), chatOther.Id)
	for _, call := range client.callsFor("unbanChatMember") {
		require.Equal(t, true, call.Params["only_if_banned"], "unfban must not kick members who are not banned")
	}
}

// withUnfbanLocale loads the /unfban replies so tests can tell them apart.
func withUnfbanLocale(t *testing.T) {
	t.Helper()
	restore, err := i18n.OverrideManagerForTest(`
feds_unfban_not_banned: "Not fbanned."
feds_unfban_ok: "Unfbanned {user} from <b>{name}</b>."
`)
	require.NoError(t, err)
	t.Cleanup(restore)
}

// This pins a known gap, not the behaviour we want: an unfban in a source
// federation walks only its own chats, so a subscriber federation whose own
// unfban walk was skipped keeps the user banned in its chats.
func TestSourceFedUnfbanLeavesSubscriberChatsBannedAfterSkippedWalk(t *testing.T) {
	withUnfbanLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	ownerA := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Owner A"}
	ownerB := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Owner B"}
	fedA, err := federations.CreateFederation(ownerA.Id, "Subscriber Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fedA.FedID) })
	fedB, err := federations.CreateFederation(ownerB.Id, "Source Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fedB.FedID) })
	require.NoError(t, federations.SubscribeFed(fedA.FedID, fedB.FedID))

	target := uniquePositiveUserID()
	chatA := seedFedChat(t, fedA.FedID, target)
	chatB := seedFedChat(t, fedB.FedID, target)
	_, _, err = federations.Fban(fedA.FedID, target, ownerA.Id, "local")
	require.NoError(t, err)
	_, _, err = federations.Fban(fedB.FedID, target, ownerB.Id, "upstream")
	require.NoError(t, err)
	unfbanText := "/unfban " + strconv.FormatInt(target, 10)

	if err := federationsModule.unfban(bot, newModuleMessageContext(bot, chatA, ownerA, unfbanText)); err != ext.EndGroups {
		t.Fatalf("unfban in A: %v", err)
	}
	require.Never(t, func() bool { return len(unbanTargets(client, target)) > 0 },
		200*time.Millisecond, 10*time.Millisecond, "A's walk must skip while the source federation still bans the user")

	if err := federationsModule.unfban(bot, newModuleMessageContext(bot, chatB, ownerB, unfbanText)); err != ext.EndGroups {
		t.Fatalf("unfban in B: %v", err)
	}
	require.Eventually(t, func() bool { return slices.Contains(unbanTargets(client, target), chatB.Id) },
		2*time.Second, 10*time.Millisecond, "B's walk must unban the user in B's chats")
	require.Never(t, func() bool { return slices.Contains(unbanTargets(client, target), chatA.Id) },
		200*time.Millisecond, 10*time.Millisecond, "known gap: B's walk does not reach A's chats")
	require.Equal(t, []int64{chatB.Id}, unbanTargets(client, target))

	ban, _ := federations.FindBanInFedTree(fedA.FedID, target)
	require.Nil(t, ban, "no federation in A's tree bans the user any more")
	if err := federationsModule.unfban(bot, newModuleMessageContext(bot, chatA, ownerA, unfbanText)); err != ext.EndGroups {
		t.Fatalf("second unfban in A: %v", err)
	}
	notBanned, _ := i18n.MustNewTranslator("en").GetString("feds_unfban_not_banned")
	texts := sentTexts(client)
	require.NotEmpty(t, texts, "the second unfban in A must reply")
	require.Equal(t, notBanned, texts[len(texts)-1], "a repeated unfban in A cannot recover the chat ban")
}

func TestUnfbanOfUserWithoutFbanRepliesNotBannedAndUnbansNowhere(t *testing.T) {
	withUnfbanLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	owner := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Owner"}
	fed, err := federations.CreateFederation(owner.Id, "Clean Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	target := uniquePositiveUserID()
	chat := seedFedChat(t, fed.FedID, target)

	if err := federationsModule.unfban(bot, newModuleMessageContext(bot, chat, owner, "/unfban "+strconv.FormatInt(target, 10))); err != ext.EndGroups {
		t.Fatalf("unfban: %v", err)
	}
	notBanned, _ := i18n.MustNewTranslator("en").GetString("feds_unfban_not_banned")
	require.Equal(t, []string{notBanned}, sentTexts(client))
	require.Never(t, func() bool { return len(unbanTargets(client, target)) > 0 },
		200*time.Millisecond, 10*time.Millisecond, "unfban of a user who is not fbanned must not unban them anywhere")
}

// Telegram keeps one ban per chat and does not record who set it, so the walk
// cannot spare a ban a chat admin set by hand. The fake client keeps no ban
// state: this pins the policy (the walk sends the unban in a chat whose admin
// banned the user), not that the bot tells the two bans apart.
func TestUnfbanSendsUnbanEvenWhereAChatAdminBannedTheUser(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	owner := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Owner"}
	fed, err := federations.CreateFederation(owner.Id, "Local Ban Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	target := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Member"}
	chat := seedFedChat(t, fed.FedID, target.Id)
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := bansModule.ban(bot, newBanReplyContext(bot, chat, admin, target, "/ban spam")); err != ext.EndGroups {
		t.Fatalf("ban: %v", err)
	}
	require.Len(t, client.callsFor("banChatMember"), 1)
	_, _, err = federations.Fban(fed.FedID, target.Id, owner.Id, "spam")
	require.NoError(t, err)

	if err := federationsModule.unfban(bot, newModuleMessageContext(bot, chat, owner, "/unfban "+strconv.FormatInt(target.Id, 10))); err != ext.EndGroups {
		t.Fatalf("unfban: %v", err)
	}
	require.Eventually(t, func() bool { return slices.Contains(unbanTargets(client, target.Id), chat.Id) },
		2*time.Second, 10*time.Millisecond, "the unfban walk must send the unban whoever set the chat ban")
}

func TestUnfbanChatFailureDoesNotStopOtherChats(t *testing.T) {
	client := newModuleBotClient()
	client.errors["unbanChatMember"] = errors.New("bot was kicked from the chat")
	bot := newModuleTestBot(client)
	owner := uniquePositiveUserID()
	fed, err := federations.CreateFederation(owner, "Failing Unfban Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	target := uniquePositiveUserID()
	chatA := seedFedChat(t, fed.FedID, target)
	chatB := seedFedChat(t, fed.FedID, target)

	applyActiveUnfban(bot, fed.FedID, target)
	require.ElementsMatch(t, []int64{chatA.Id, chatB.Id}, unbanTargets(client, target),
		"an error in one chat must not skip the remaining chats")
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

func TestImportFBansFromCSVDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("id,reason\n4242,imported\n"))
	}))
	t.Cleanup(server.Close)
	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
	})

	client := newModuleBotClient()
	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"fed-bans","file_path":"fbans/bans.csv"}`,
	)
	bot := newModuleTestBot(client)
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Import CSV Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}

	ctx := newModuleMessageContext(bot, pm, owner, "/importfbans")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 77,
		Date:      1,
		Chat:      pm,
		Document:  &gotgbot.Document{FileId: "fed-bans", FileName: "bans.csv", FileSize: 24},
	}
	if err := federationsModule.importFBans(bot, ctx); err != ext.EndGroups {
		t.Fatalf("importFBans: %v", err)
	}
	if federations.GetFedBan(fed.FedID, 4242) == nil {
		t.Fatal("imported ban missing")
	}

	tr := i18n.MustNewTranslator("en")
	tooBig, _ := downloadFedBanDocument(bot, &gotgbot.Document{
		FileId: "x", FileName: "bans.csv", FileSize: maxBackupFileSize + 1,
	}, tr)
	if tooBig != nil {
		t.Fatal("oversized import should be rejected")
	}
}

// An imported list cannot ban the owner, a fed admin or the bot: /fban
// refuses them and /unfban could not lift such a ban.
func TestImportFBansSkipsOwnerAdminsAndBot(t *testing.T) {
	ownerID := uniquePositiveUserID()
	adminID := uniquePositiveUserID()
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	csvBody := "id,reason\n" +
		strconv.FormatInt(ownerID, 10) + ",x\n" +
		strconv.FormatInt(adminID, 10) + ",x\n" +
		strconv.FormatInt(bot.Id, 10) + ",x\n" +
		"4343,ok\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(csvBody))
	}))
	t.Cleanup(server.Close)
	oldBaseURL := backupDownloadBaseURL
	oldHTTPClient := backupDownloadHTTPClient
	backupDownloadBaseURL = server.URL + "/file/bot"
	backupDownloadHTTPClient = server.Client()
	t.Cleanup(func() {
		backupDownloadBaseURL = oldBaseURL
		backupDownloadHTTPClient = oldHTTPClient
	})

	client.responses["getFile"] = json.RawMessage(
		`{"file_id":"fed-bans","file_path":"fbans/bans.csv"}`,
	)
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Import Guard Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	require.NoError(t, federations.PromoteFedAdmin(fed.FedID, adminID))
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}

	ctx := newModuleMessageContext(bot, pm, owner, "/importfbans")
	ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 77,
		Date:      1,
		Chat:      pm,
		Document:  &gotgbot.Document{FileId: "fed-bans", FileName: "bans.csv", FileSize: int64(len(csvBody))},
	}
	if err := federationsModule.importFBans(bot, ctx); err != ext.EndGroups {
		t.Fatalf("importFBans: %v", err)
	}
	for _, id := range []int64{ownerID, adminID, bot.Id} {
		require.Nil(t, federations.GetFedBan(fed.FedID, id), "user %d must not be fbanned by an import", id)
	}
	require.NotNil(t, federations.GetFedBan(fed.FedID, 4343), "ordinary user must be imported")
}

func TestFbanInPrivateAndEmptyExport(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Private Fban Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}

	if err := federationsModule.fbanList(bot, newModuleMessageContext(bot, pm, owner, "/fbanlist")); err != ext.EndGroups {
		t.Fatalf("empty fbanList: %v", err)
	}
	if err := federationsModule.fban(bot, newModuleMessageContext(bot, pm, owner, "/fban 888 spam")); err != ext.EndGroups {
		t.Fatalf("private fban: %v", err)
	}
	if federations.GetFedBan(fed.FedID, 888) == nil {
		t.Fatal("private fban did not persist")
	}
	if err := federationsModule.fbanList(bot, newModuleMessageContext(bot, pm, owner, "/fbanlist csv")); err != ext.EndGroups {
		t.Fatalf("csv fbanList: %v", err)
	}
	if err := federationsModule.fbanList(bot, newModuleMessageContext(bot, pm, owner, "/fbanlist minicsv")); err != ext.EndGroups {
		t.Fatalf("minicsv fbanList: %v", err)
	}

	require.NoError(t, federations.SetRequireReason(fed.FedID, true))
	if err := federationsModule.fban(bot, newModuleMessageContext(bot, pm, owner, "/fban 889")); err != ext.EndGroups {
		t.Fatalf("fban missing reason: %v", err)
	}
	if federations.GetFedBan(fed.FedID, 889) != nil {
		t.Fatal("reason-required fban should not persist")
	}
}

func TestJoinFedAlreadyJoinedOtherFederation(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	first, err := federations.CreateFederation(uniquePositiveUserID(), "First")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(first.FedID) })
	second, err := federations.CreateFederation(uniquePositiveUserID(), "Second")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(second.FedID) })

	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Two Feds"}
	creator := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	require.NoError(t, federations.JoinFed(group.Id, group.Title, first.FedID))
	if err := federationsModule.joinFed(bot, newModuleMessageContext(bot, group, creator, "/joinfed "+second.FedID)); err != ext.EndGroups {
		t.Fatalf("joinFed other: %v", err)
	}
	if got := federations.GetChatFed(group.Id); got == nil || got.FedID != first.FedID {
		t.Fatalf("should stay in first fed, got %+v", got)
	}
}

func TestFedCommandErrorPaths(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	ownerID := uniquePositiveUserID()
	owner := gotgbot.User{Id: ownerID, FirstName: "Owner"}
	fed, err := federations.CreateFederation(ownerID, "Errors Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	pm := gotgbot.Chat{Id: ownerID, Type: "private", FirstName: "Owner"}
	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Err Group"}
	quietGroup := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Quiet Group"}
	require.NoError(t, federations.JoinFed(quietGroup.Id, quietGroup.Title, fed.FedID))
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	creator := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	cases := []struct {
		name string
		chat gotgbot.Chat
		from gotgbot.User
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{"newfed empty", pm, owner, "/newfed", federationsModule.newFed},
		{"newfed already owns", pm, owner, "/newfed Another", federationsModule.newFed},
		{"rename empty", pm, owner, "/renamefed", federationsModule.renameFed},
		{"rename missing", pm, member, "/renamefed X", federationsModule.renameFed},
		{"delfed group", group, owner, "/delfed", federationsModule.delFed},
		{"delfed none", pm, member, "/delfed", federationsModule.delFed},
		{"join missing fed", group, creator, "/joinfed 550e8400-e29b-41d4-a716-446655440000", federationsModule.joinFed},
		{"leave private", pm, creator, "/leavefed", federationsModule.leaveFed},
		{"leave member", group, member, "/leavefed", federationsModule.leaveFed},
		{"leave none", group, creator, "/leavefed", federationsModule.leaveFed},
		{"quiet private", pm, creator, "/quietfed on", federationsModule.quietFed},
		{"quiet member", group, member, "/quietfed on", federationsModule.quietFed},
		{"quiet none", group, creator, "/quietfed on", federationsModule.quietFed},
		{"quiet junk", quietGroup, creator, "/quietfed maybe", federationsModule.quietFed},
		{"info invalid", pm, owner, "/fedinfo nope", federationsModule.fedInfo},
		{"info missing", pm, member, "/fedinfo", federationsModule.fedInfo},
		{"admins invalid", pm, owner, "/fedadmins nope", federationsModule.fedAdmins},
		{"admins none", pm, member, "/fedadmins", federationsModule.fedAdmins},
		{"chatfed private", pm, owner, "/chatfed", federationsModule.chatFed},
		{"chatfed none", group, creator, "/chatfed", federationsModule.chatFed},
		{"myfeds none", pm, member, "/myfeds", federationsModule.myFeds},
		{"promote none", pm, owner, "/fedpromote", federationsModule.fedPromote},
		{"promote self", pm, owner, "/fedpromote " + strconv.FormatInt(ownerID, 10), federationsModule.fedPromote},
		{"demoteme invalid", pm, member, "/feddemoteme nope", federationsModule.fedDemoteMe},
		{"reason missing", pm, owner, "/fedreason", federationsModule.fedReason},
		{"reason junk", pm, owner, "/fedreason maybe", federationsModule.fedReason},
		{"fban missing user", pm, owner, "/fban", federationsModule.fban},
		{"fban self", pm, owner, "/fban " + strconv.FormatInt(ownerID, 10), federationsModule.fban},
		{"unfban missing", pm, owner, "/unfban 1", federationsModule.unfban},
		{"sub self", pm, owner, "/subfed " + fed.FedID, federationsModule.subFed},
		{"sub invalid", pm, owner, "/subfed nope", federationsModule.subFed},
		{"unsub missing", pm, owner, "/unsubfed 550e8400-e29b-41d4-a716-446655440000", federationsModule.unsubFed},
		{"setfedlog private", pm, owner, "/setfedlog", federationsModule.setFedLog},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, tc.chat, tc.from, tc.text)
			if err := tc.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s: %v", tc.name, err)
			}
		})
	}

	noop := encodeCallbackData(fedCallbackNamespace, map[string]string{"a": "noop"})
	if err := federationsModule.fedCallback(bot, newModuleCallbackContext(bot, pm, owner, noop)); err != ext.EndGroups {
		t.Fatalf("callback noop: %v", err)
	}
	stale := encodeCallbackData(fedCallbackNamespace, map[string]string{"a": "nope"})
	if err := federationsModule.fedCallback(bot, newModuleCallbackContext(bot, pm, owner, stale)); err != ext.EndGroups {
		t.Fatalf("callback stale: %v", err)
	}
}
