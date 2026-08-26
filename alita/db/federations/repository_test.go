package federations

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestFederationLifecycle(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
	ownerID := time.Now().UnixNano()
	t.Cleanup(func() {
		if fed := GetFedByOwner(ownerID); fed != nil {
			_ = DeleteFederation(fed.FedID)
		}
		_ = db.DB.Where("user_id = ?", ownerID).Delete(&models.User{}).Error
	})

	fed, err := CreateFederation(ownerID, "  Test Fed  ")
	if err != nil {
		t.Fatalf("CreateFederation: %v", err)
	}
	if fed.Name != "Test Fed" {
		t.Fatalf("name = %q, want trimmed", fed.Name)
	}
	if GetFed(fed.FedID) == nil {
		t.Fatal("GetFed returned nil")
	}
	if got := GetFedByOwner(ownerID); got == nil || got.FedID != fed.FedID {
		t.Fatal("GetFedByOwner mismatch")
	}
	if _, err := CreateFederation(ownerID, "other"); !errors.Is(err, ErrAlreadyOwnsFed) {
		t.Fatalf("second federation for same owner should fail: %v", err)
	}

	renamed, err := RenameFederation(ownerID, "Renamed")
	if err != nil || renamed.Name != "Renamed" {
		t.Fatalf("RenameFederation: %v name=%q", err, renamed.Name)
	}

	chatID := -time.Now().UnixNano()
	if err := chats.EnsureChatInDb(chatID, "fed chat"); err != nil {
		t.Fatalf("EnsureChatInDb: %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})
	if err := JoinFed(chatID, "fed chat", fed.FedID); err != nil {
		t.Fatalf("JoinFed: %v", err)
	}
	if GetChatFed(chatID) == nil {
		t.Fatal("GetChatFed nil after join")
	}
	if err := SetQuietFed(chatID, true); err != nil {
		t.Fatalf("SetQuietFed: %v", err)
	}
	if !GetChatFed(chatID).Quiet {
		t.Fatal("quiet not persisted")
	}

	other := time.Now().UnixNano() + 1
	if err := PromoteFedAdmin(fed.FedID, other); err != nil {
		t.Fatalf("PromoteFedAdmin: %v", err)
	}
	if !IsFedAdmin(fed.FedID, other) {
		t.Fatal("promoted user is not admin")
	}

	ban, created, err := Fban(fed.FedID, other+5, ownerID, "spam")
	if err != nil || !created || ban.Reason != "spam" {
		t.Fatalf("Fban: created=%v err=%v ban=%+v", created, err, ban)
	}
	found, source := FindBanInFedTree(fed.FedID, other+5)
	if found == nil || source != fed.FedID {
		t.Fatalf("FindBanInFedTree = %+v %s", found, source)
	}
	if err := Unfban(fed.FedID, other+5); err != nil {
		t.Fatalf("Unfban: %v", err)
	}
	if GetFedBan(fed.FedID, other+5) != nil {
		t.Fatal("ban still present after unfban")
	}

	if err := LeaveFed(chatID); err != nil {
		t.Fatalf("LeaveFed: %v", err)
	}
	if GetChatFed(chatID) != nil {
		t.Fatal("membership remained after leave")
	}
	if err := DeleteFederation(fed.FedID); err != nil {
		t.Fatalf("DeleteFederation: %v", err)
	}
	if GetFed(fed.FedID) != nil {
		t.Fatal("federation survived delete")
	}
}

func TestFederationSubscriptions(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
	ownerA := time.Now().UnixNano()
	ownerB := ownerA + 1
	a, err := CreateFederation(ownerA, "A")
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := CreateFederation(ownerB, "B")
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	t.Cleanup(func() {
		_ = DeleteFederation(a.FedID)
		_ = DeleteFederation(b.FedID)
	})
	if err := SubscribeFed(a.FedID, b.FedID); err != nil {
		t.Fatalf("SubscribeFed: %v", err)
	}
	if err := SubscribeFed(a.FedID, a.FedID); err == nil {
		t.Fatal("self-subscribe should fail")
	}
	if _, _, err := Fban(b.FedID, 4242, ownerB, "from B"); err != nil {
		t.Fatalf("Fban B: %v", err)
	}
	found, source := FindBanInFedTree(a.FedID, 4242)
	if found == nil || source != b.FedID {
		t.Fatalf("expected subscribed ban from B, got %+v %s", found, source)
	}
	if err := UnsubscribeFed(a.FedID, b.FedID); err != nil {
		t.Fatalf("UnsubscribeFed: %v", err)
	}
	found, _ = FindBanInFedTree(a.FedID, 4242)
	if found != nil {
		t.Fatal("ban still visible after unsubscribe")
	}
}

func TestFederationAdminFlagsAndImport(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
	ownerID := time.Now().UnixNano() + 50
	adminID := ownerID + 1
	memberID := ownerID + 2
	fed, err := CreateFederation(ownerID, "Admin Flags Fed")
	if err != nil {
		t.Fatalf("CreateFederation: %v", err)
	}
	t.Cleanup(func() { _ = DeleteFederation(fed.FedID) })

	if err := PromoteFedAdmin(fed.FedID, ownerID); err == nil {
		t.Fatal("promoting the owner should fail")
	}
	if err := PromoteFedAdmin(fed.FedID, adminID); err != nil {
		t.Fatalf("PromoteFedAdmin: %v", err)
	}
	if err := PromoteFedAdmin(fed.FedID, adminID); err == nil {
		t.Fatal("second promote should fail")
	}
	listed := ListFedsForAdmin(adminID)
	if len(listed) != 1 || listed[0].FedID != fed.FedID {
		t.Fatalf("ListFedsForAdmin = %+v", listed)
	}
	owned := ListFedsForAdmin(ownerID)
	if len(owned) != 1 || owned[0].FedID != fed.FedID {
		t.Fatalf("owner ListFedsForAdmin = %+v", owned)
	}

	if err := SetRequireReason(fed.FedID, true); err != nil {
		t.Fatalf("SetRequireReason: %v", err)
	}
	if err := SetNotifyOwner(fed.FedID, false); err != nil {
		t.Fatalf("SetNotifyOwner: %v", err)
	}
	if err := SetFedLogChat(fed.FedID, -100123); err != nil {
		t.Fatalf("SetFedLogChat: %v", err)
	}
	updated := GetFed(fed.FedID)
	if updated == nil || !updated.RequireReason || updated.NotifyOwner || updated.LogChatID != -100123 {
		t.Fatalf("updated flags = %+v", updated)
	}
	if err := SetFedLogChat(fed.FedID, 0); err != nil {
		t.Fatalf("clear SetFedLogChat: %v", err)
	}
	if GetFed(fed.FedID).LogChatID != 0 {
		t.Fatal("log chat id not cleared")
	}

	written, err := ImportBans(fed.FedID, []models.FederationBan{
		{UserID: memberID, Reason: "import"},
		{UserID: 0, Reason: "skip"},
	})
	if err != nil || written != 1 {
		t.Fatalf("ImportBans written=%d err=%v", written, err)
	}
	if CountFedBans(fed.FedID) != 1 {
		t.Fatalf("CountFedBans = %d", CountFedBans(fed.FedID))
	}
	bans, err := ListFedBans(fed.FedID)
	if err != nil || len(bans) != 1 || bans[0].UserID != memberID {
		t.Fatalf("ListFedBans = %+v err=%v", bans, err)
	}
	userBans, err := ListUserFedBans(memberID)
	if err != nil || len(userBans) != 1 {
		t.Fatalf("ListUserFedBans = %+v err=%v", userBans, err)
	}

	if err := DemoteFedAdmin(fed.FedID, adminID); err != nil {
		t.Fatalf("DemoteFedAdmin: %v", err)
	}
	if IsFedAdmin(fed.FedID, adminID) {
		t.Fatal("demoted admin still has access")
	}
	if err := DemoteFedAdmin(fed.FedID, adminID); err == nil {
		t.Fatal("demoting a missing admin should fail")
	}
}

func TestFederationChatMembershipHelpers(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
	ownerID := time.Now().UnixNano() + 90
	memberID := ownerID + 3
	fed, err := CreateFederation(ownerID, "Chat Helpers Fed")
	if err != nil {
		t.Fatalf("CreateFederation: %v", err)
	}
	t.Cleanup(func() { _ = DeleteFederation(fed.FedID) })

	chatID := -time.Now().UnixNano()
	if err := chats.EnsureChatInDb(chatID, "member chat"); err != nil {
		t.Fatalf("EnsureChatInDb: %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatID).
		Update("users", models.Int64Array{memberID}).Error; err != nil {
		t.Fatalf("set users: %v", err)
	}
	if err := JoinFed(chatID, "member chat", fed.FedID); err != nil {
		t.Fatalf("JoinFed: %v", err)
	}
	if CountFedChats(fed.FedID) != 1 {
		t.Fatalf("CountFedChats = %d", CountFedChats(fed.FedID))
	}
	ids, err := ListFedChatIDs(fed.FedID)
	if err != nil || len(ids) != 1 || ids[0] != chatID {
		t.Fatalf("ListFedChatIDs = %v err=%v", ids, err)
	}
	if found := ChatsContainingUser(memberID, ids); len(found) != 1 || found[0] != chatID {
		t.Fatalf("ChatsContainingUser = %v", found)
	}
	if found := ChatsContainingUser(memberID+99, ids); len(found) != 0 {
		t.Fatalf("unexpected membership %v", found)
	}
	if ChatsContainingUser(memberID, nil) != nil {
		t.Fatal("empty chat list should return nil")
	}

	otherOwner := ownerID + 10
	other, err := CreateFederation(otherOwner, "Other Fed")
	if err != nil {
		t.Fatalf("CreateFederation other: %v", err)
	}
	t.Cleanup(func() { _ = DeleteFederation(other.FedID) })
	if err := JoinFed(chatID, "member chat", other.FedID); !errors.Is(err, ErrAlreadyJoined) {
		t.Fatalf("joining a second federation should fail: %v", err)
	}
	if err := JoinFed(chatID, "member chat", fed.FedID); err != nil {
		t.Fatalf("idempotent JoinFed: %v", err)
	}

	longName := strings.Repeat("n", 80)
	renamed, err := RenameFederation(ownerID, longName)
	if err != nil || len(renamed.Name) != 64 {
		t.Fatalf("long rename name=%q err=%v", renamed.Name, err)
	}
}

func TestGetChatFedAndBanNegativeCache(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
	chatID := -time.Now().UnixNano()
	if GetChatFed(chatID) != nil {
		t.Fatal("missing chat membership should be nil")
	}
	if GetChatFed(chatID) != nil {
		t.Fatal("negative-cached chat membership should stay nil")
	}

	ownerID := time.Now().UnixNano()
	fed, err := CreateFederation(ownerID, "cache fed")
	if err != nil {
		t.Fatalf("CreateFederation: %v", err)
	}
	t.Cleanup(func() {
		_ = DeleteFederation(fed.FedID)
		_ = db.DB.Where("user_id = ?", ownerID).Delete(&models.User{}).Error
	})
	missingUser := ownerID + 99
	if GetFedBan(fed.FedID, missingUser) != nil {
		t.Fatal("missing ban should be nil")
	}
	if GetFedBan(fed.FedID, missingUser) != nil {
		t.Fatal("negative-cached ban should stay nil")
	}
	if found, _ := FindBanInFedTree(fed.FedID, missingUser); found != nil {
		t.Fatal("FindBanInFedTree should miss after negative cache")
	}
}

func TestDeleteFederationInvalidatesBanAndSubscriberCaches(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
	ownerA := time.Now().UnixNano()
	ownerB := ownerA + 1
	a, err := CreateFederation(ownerA, "delete-src")
	if err != nil {
		t.Fatalf("CreateFederation A: %v", err)
	}
	b, err := CreateFederation(ownerB, "delete-sub")
	if err != nil {
		t.Fatalf("CreateFederation B: %v", err)
	}
	t.Cleanup(func() {
		_ = DeleteFederation(a.FedID)
		_ = DeleteFederation(b.FedID)
		_ = db.DB.Where("user_id IN ?", []int64{ownerA, ownerB}).Delete(&models.User{}).Error
	})

	bannedUser := ownerA + 50
	if _, _, err := Fban(a.FedID, bannedUser, ownerA, "cached"); err != nil {
		t.Fatalf("Fban: %v", err)
	}
	if GetFedBan(a.FedID, bannedUser) == nil {
		t.Fatal("expected ban to be readable before delete")
	}
	if err := SubscribeFed(b.FedID, a.FedID); err != nil {
		t.Fatalf("SubscribeFed: %v", err)
	}
	subs := ListSubscribedFedIDs(b.FedID)
	if len(subs) != 1 || subs[0] != a.FedID {
		t.Fatalf("ListSubscribedFedIDs = %v, want [%s]", subs, a.FedID)
	}

	if err := DeleteFederation(a.FedID); err != nil {
		t.Fatalf("DeleteFederation: %v", err)
	}
	if GetFedBan(a.FedID, bannedUser) != nil {
		t.Fatal("cached ban survived DeleteFederation")
	}
	if found, _ := FindBanInFedTree(b.FedID, bannedUser); found != nil {
		t.Fatal("subscriber still sees deleted federation ban")
	}
	if got := ListSubscribedFedIDs(b.FedID); len(got) != 0 {
		t.Fatalf("subscriber fed_subs cache = %v, want empty after target delete", got)
	}
}
