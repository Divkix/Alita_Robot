//go:build testtools

package federations

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func cleanupFederation(t *testing.T, fedID string) {
	t.Helper()
	if err := db.DB.Where("fed_id = ?", fedID).Delete(&models.Federation{}).Error; err != nil {
		t.Fatalf("cleanup federation error: %v", err)
	}
}

func TestBanAndUnbanUser(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	userID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "Ban Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	if err := BanUser(fed.FedID, userID, "spam", ownerID); err != nil {
		t.Fatalf("BanUser() error = %v", err)
	}

	banned, reason, err := IsBanned(fed.FedID, userID)
	if err != nil {
		t.Fatalf("IsBanned() error = %v", err)
	}
	if !banned {
		t.Fatal("IsBanned() = false, want true")
	}
	if reason != "spam" {
		t.Fatalf("reason = %q, want spam", reason)
	}

	if err := UnbanUser(fed.FedID, userID); err != nil {
		t.Fatalf("UnbanUser() error = %v", err)
	}

	banned, _, err = IsBanned(fed.FedID, userID)
	if err != nil {
		t.Fatalf("IsBanned() error = %v", err)
	}
	if banned {
		t.Fatal("IsBanned() = true after UnbanUser, want false")
	}
}

func TestGetUserFederationBans(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	userID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "User Bans Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	if err := BanUser(fed.FedID, userID, "abuse", ownerID); err != nil {
		t.Fatalf("BanUser() error = %v", err)
	}

	bans, err := GetUserFederationBans(userID)
	if err != nil {
		t.Fatalf("GetUserFederationBans() error = %v", err)
	}
	if len(bans) != 1 {
		t.Fatalf("len(bans) = %d, want 1", len(bans))
	}
	if bans[0].FedID != fed.FedID {
		t.Fatalf("FedID = %q, want %q", bans[0].FedID, fed.FedID)
	}
}

func TestIsBannedInFederationOrSubs(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	userID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "Primary")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	subOwnerID := time.Now().UnixNano() + 2
	subFed, err := CreateFederation(subOwnerID, "Subscribed")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, subFed.FedID) })

	if err := SubscribeFederation(fed.FedID, subFed.FedID); err != nil {
		t.Fatalf("SubscribeFederation() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("federation_id = ?", fed.ID).Delete(&models.FederationSubscription{}).Error
	})

	if err := BanUser(subFed.FedID, userID, "sub ban", subOwnerID); err != nil {
		t.Fatalf("BanUser() error = %v", err)
	}

	banned, info, err := IsBannedInFederationOrSubs(fed.FedID, userID)
	if err != nil {
		t.Fatalf("IsBannedInFederationOrSubs() error = %v", err)
	}
	if !banned {
		t.Fatal("IsBannedInFederationOrSubs() = false, want true")
	}
	if info == nil || info.FedID != subFed.FedID {
		t.Fatalf("ban info FedID = %q, want %q", info.FedID, subFed.FedID)
	}
}

func TestCreateFederation(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	name := "Test Federation"

	fed, err := CreateFederation(ownerID, name)
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	if fed == nil {
		t.Fatal("CreateFederation() returned nil federation")
	}

	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	if fed.Name != name {
		t.Fatalf("Name = %q, want %q", fed.Name, name)
	}
	if fed.OwnerID != ownerID {
		t.Fatalf("OwnerID = %d, want %d", fed.OwnerID, ownerID)
	}
	if fed.FedID == "" {
		t.Fatal("FedID is empty")
	}
}

func TestCreateFederationDuplicateOwner(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	fed, err := CreateFederation(ownerID, "First Fed")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	_, err = CreateFederation(ownerID, "Second Fed")
	if err != ErrAlreadyFederationOwner {
		t.Fatalf("expected ErrAlreadyFederationOwner, got %v", err)
	}
}

func TestRenameFederation(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	fed, err := CreateFederation(ownerID, "Old Name")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	newName := "New Name"
	if err := RenameFederation(fed.FedID, newName); err != nil {
		t.Fatalf("RenameFederation() error = %v", err)
	}

	updated, err := GetFederationByID(fed.FedID)
	if err != nil {
		t.Fatalf("GetFederationByID() error = %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("Name = %q, want %q", updated.Name, newName)
	}
}

func TestDeleteFederation(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	fed, err := CreateFederation(ownerID, "Delete Me")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}

	if err := DeleteFederation(fed.FedID); err != nil {
		t.Fatalf("DeleteFederation() error = %v", err)
	}

	_, err = GetFederationByID(fed.FedID)
	if err != ErrFederationNotFound {
		t.Fatalf("expected ErrFederationNotFound, got %v", err)
	}
}

func TestGetFederationByOwner(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	fed, err := CreateFederation(ownerID, "Owner Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	found, err := GetFederationByOwner(ownerID)
	if err != nil {
		t.Fatalf("GetFederationByOwner() error = %v", err)
	}
	if found.FedID != fed.FedID {
		t.Fatalf("FedID = %q, want %q", found.FedID, fed.FedID)
	}
}

func TestJoinAndLeaveChat(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	chatID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "Chat Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	if err := JoinChat(fed.FedID, chatID, ownerID); err != nil {
		t.Fatalf("JoinChat() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.FederationChat{}).Error
	})

	joined, err := GetChatFederation(chatID)
	if err != nil {
		t.Fatalf("GetChatFederation() error = %v", err)
	}
	if joined.FedID != fed.FedID {
		t.Fatalf("FedID = %q, want %q", joined.FedID, fed.FedID)
	}

	if err := LeaveChat(chatID); err != nil {
		t.Fatalf("LeaveChat() error = %v", err)
	}

	_, err = GetChatFederation(chatID)
	if err != ErrFederationNotFound {
		t.Fatalf("expected ErrFederationNotFound, got %v", err)
	}
}

func TestJoinChatAlreadyInFederation(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	chatID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "First Fed")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	fed2, err := CreateFederation(ownerID+1, "Second Fed")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed2.FedID) })

	if err := JoinChat(fed.FedID, chatID, ownerID); err != nil {
		t.Fatalf("JoinChat() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.FederationChat{}).Error
	})

	if err := JoinChat(fed2.FedID, chatID, ownerID); err != ErrChatAlreadyInFederation {
		t.Fatalf("expected ErrChatAlreadyInFederation, got %v", err)
	}
}

func TestAdminManagement(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	adminID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "Admin Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	if err := AddAdmin(fed.FedID, adminID, ownerID); err != nil {
		t.Fatalf("AddAdmin() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("federation_id = ?", fed.ID).Delete(&models.FederationAdmin{}).Error
	})

	isAdmin, err := IsAdmin(fed.FedID, adminID)
	if err != nil {
		t.Fatalf("IsAdmin() error = %v", err)
	}
	if !isAdmin {
		t.Fatal("IsAdmin() = false, want true")
	}

	admins, err := ListAdmins(fed.FedID)
	if err != nil {
		t.Fatalf("ListAdmins() error = %v", err)
	}
	if len(admins) != 1 {
		t.Fatalf("len(admins) = %d, want 1", len(admins))
	}

	if err := RemoveAdmin(fed.FedID, adminID); err != nil {
		t.Fatalf("RemoveAdmin() error = %v", err)
	}

	isAdmin, err = IsAdmin(fed.FedID, adminID)
	if err != nil {
		t.Fatalf("IsAdmin() error = %v", err)
	}
	if isAdmin {
		t.Fatal("IsAdmin() = true after RemoveAdmin, want false")
	}
}

func TestIsAdminOwner(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	fed, err := CreateFederation(ownerID, "Owner Admin Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	isAdmin, err := IsAdmin(fed.FedID, ownerID)
	if err != nil {
		t.Fatalf("IsAdmin() error = %v", err)
	}
	if !isAdmin {
		t.Fatal("IsAdmin() = false for owner, want true")
	}
}

func TestListFederationChats(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	chatID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "Chats Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	if err := JoinChat(fed.FedID, chatID, ownerID); err != nil {
		t.Fatalf("JoinChat() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.FederationChat{}).Error
	})

	chats, err := ListFederationChats(fed.FedID)
	if err != nil {
		t.Fatalf("ListFederationChats() error = %v", err)
	}
	if len(chats) != 1 || chats[0] != chatID {
		t.Fatalf("chats = %v, want [%d]", chats, chatID)
	}
}

func TestSetQuiet(t *testing.T) {
	skipIfNoDb(t)

	ownerID := time.Now().UnixNano()
	chatID := time.Now().UnixNano() + 1
	fed, err := CreateFederation(ownerID, "Quiet Test")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { cleanupFederation(t, fed.FedID) })

	if err := JoinChat(fed.FedID, chatID, ownerID); err != nil {
		t.Fatalf("JoinChat() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.FederationChat{}).Error
	})

	if err := SetQuiet(chatID, true); err != nil {
		t.Fatalf("SetQuiet() error = %v", err)
	}

	quiet, err := IsQuiet(chatID)
	if err != nil {
		t.Fatalf("IsQuiet() error = %v", err)
	}
	if !quiet {
		t.Fatal("IsQuiet() = false, want true")
	}
}
