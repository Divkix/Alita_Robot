package federations

import (
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
	if _, err := CreateFederation(ownerID, "other"); err == nil {
		t.Fatal("second federation for same owner should fail")
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
