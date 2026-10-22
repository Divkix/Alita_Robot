//go:build testtools

package panel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

func newWarningsFixture(t *testing.T) (panelFixture, int64) {
	t.Helper()
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	seedPanelChat(t, chatID, "Warnings Chat", fixtureNow)
	fixture.client.admins[chatID] = adminListJSON(panelTestUserID, 999)
	return fixture, chatID
}

// seedChatWarns writes a warns row and its entries the way the repository
// stores them: entry reasons in the reasons column, position for position.
func seedChatWarns(t *testing.T, chatID, userID int64, expiry int, entries []models.WarnEntry) {
	t.Helper()
	// Postgres has user and chat foreign keys on both tables; the chat comes
	// from seedPanelChat and the rest of the panel never needs a user row.
	if err := user.EnsureUserInDb(userID, "", ""); err != nil {
		t.Fatalf("seed warned user: %v", err)
	}
	settings := models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute", WarnExpiry: expiry}
	if err := db.DB.Create(&settings).Error; err != nil {
		t.Fatalf("seed warn settings: %v", err)
	}
	reasons := make(models.StringArray, 0, len(entries))
	for _, entry := range entries {
		reasons = append(reasons, entry.Reason)
	}
	row := models.Warns{UserId: userID, ChatId: chatID, NumWarns: len(entries), Reasons: reasons}
	if err := db.DB.Create(&row).Error; err != nil {
		t.Fatalf("seed warns row: %v", err)
	}
	for i := range entries {
		entries[i].ID = 0
		entries[i].UserId = userID
		entries[i].ChatId = chatID
		if err := db.DB.Create(&entries[i]).Error; err != nil {
			t.Fatalf("seed warn entry: %v", err)
		}
	}
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnEntry{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{})
	})
}

func warningsPage(t *testing.T, fixture panelFixture, chatID int64) warningsResponse {
	t.Helper()
	recorder := panelRequest(t, fixture.handler, http.MethodGet, listTarget(chatID, "warnings"), readFixture(t, "initdata_main.txt"), "", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET warnings status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var body warningsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode warnings response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func removeWarning(t *testing.T, fixture panelFixture, chatID, userID int64) warningRemoveResponse {
	t.Helper()
	body := mustJSON(t, warningRemoveRequest{UserID: userID})
	recorder := panelRequest(t, fixture.handler, http.MethodDelete, listTarget(chatID, "warnings"), readFixture(t, "initdata_main.txt"), body, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("DELETE warning status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	var response warningRemoveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode remove response %q: %v", recorder.Body.String(), err)
	}
	return response
}

// warnStateString renders the observable database state of one user's warns,
// IDs excluded so two identical fixtures compare equal.
func warnStateString(t *testing.T, chatID, userID int64) string {
	t.Helper()
	var row models.Warns
	if err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).First(&row).Error; err != nil {
		t.Fatalf("load warns row: %v", err)
	}
	var entries []models.WarnEntry
	if err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Order("id").Find(&entries).Error; err != nil {
		t.Fatalf("load warn entries: %v", err)
	}
	parts := []string{fmt.Sprintf("num=%d reasons=%v", row.NumWarns, []string(row.Reasons))}
	for _, entry := range entries {
		stamp := "nil"
		if entry.WarnedAt != nil {
			stamp = entry.WarnedAt.UTC().Format(time.RFC3339Nano)
		}
		parts = append(parts, fmt.Sprintf("%s/%d/%t/%t/%s", entry.Reason, entry.WarnedBy, entry.Anonymous, entry.WarnedAt != nil, stamp))
	}
	return strings.Join(parts, "|")
}

func TestWarningsLabelsExpiredEntriesAndUndatedNeverExpire(t *testing.T) {
	fixture, chatID := newWarningsFixture(t)

	now := time.Now().UTC()
	old := now.Add(-2 * time.Hour)
	fresh := now.Add(-time.Minute)
	entries := []models.WarnEntry{
		{Reason: "old", WarnedBy: 77, WarnedAt: &old},
		{Reason: "new", WarnedBy: 78, WarnedAt: &fresh},
		{Reason: "legacy", WarnedAt: nil},
		{Reason: "bot warn", WarnedBy: models.WarnedByBot, WarnedAt: &fresh},
		{Reason: "anonymous warn", WarnedBy: 77, Anonymous: true, WarnedAt: &fresh},
	}
	seedChatWarns(t, chatID, panelTestUserID, 3600, entries)

	page := warningsPage(t, fixture, chatID)
	if page.ReadOnly {
		t.Fatal("read_only = true for an admin, want false")
	}
	if len(page.Users) != 1 || page.Users[0].UserID != panelTestUserID {
		t.Fatalf("users = %+v, want only user %d", page.Users, panelTestUserID)
	}
	user := page.Users[0]
	if len(user.Entries) != len(entries) {
		t.Fatalf("entries = %d, want %d", len(user.Entries), len(entries))
	}
	if !user.Entries[0].Expired {
		t.Fatal("old entry is not labelled expired")
	}
	for i := 1; i < len(user.Entries); i++ {
		if user.Entries[i].Expired {
			t.Fatalf("entry %d (%q) is labelled expired, want active", i, user.Entries[i].Reason)
		}
	}
	if user.Entries[2].WarnedAt != nil {
		t.Fatalf("undated entry warned_at = %v, want nil", user.Entries[2].WarnedAt)
	}
	if !user.Entries[3].Automatic || user.Entries[3].WarnedBy != models.WarnedByBot {
		t.Fatalf("bot entry = %+v, want automatic with warned_by %d", user.Entries[3], models.WarnedByBot)
	}
	if !user.Entries[4].Anonymous || user.Entries[4].WarnedBy != 0 {
		t.Fatalf("anonymous entry = %+v, want anonymous with hidden warned_by", user.Entries[4])
	}
	if user.Active != 4 {
		t.Fatalf("active = %d, want 4 (the expired entry does not count)", user.Active)
	}
}

func TestPanelRemoveWarningMatchesCommandRemove(t *testing.T) {
	fixture, panelChatID := newWarningsFixture(t)
	commandChatID := uniquePanelChatID()
	seedPanelChat(t, commandChatID, "Command Warnings Chat", fixtureNow)
	fixture.client.admins[commandChatID] = adminListJSON(panelTestUserID, 999)

	stamp := time.Now().UTC()
	entries := []models.WarnEntry{
		{Reason: "first", WarnedBy: 77, WarnedAt: &stamp},
		{Reason: "second", WarnedBy: 78, WarnedAt: &stamp},
	}
	seedChatWarns(t, panelChatID, panelTestUserID, 3600, entries)
	seedChatWarns(t, commandChatID, panelTestUserID, 3600, entries)

	// Prime both caches so the write's invalidation is observable.
	if count, _ := warns.GetWarns(panelTestUserID, panelChatID); count != 2 {
		t.Fatalf("panel chat seeded count = %d, want 2", count)
	}
	if count, _ := warns.GetWarns(panelTestUserID, commandChatID); count != 2 {
		t.Fatalf("command chat seeded count = %d, want 2", count)
	}

	result := removeWarning(t, fixture, panelChatID, panelTestUserID)
	if !result.Removed || result.Active != 1 {
		t.Fatalf("panel remove = %+v, want removed with 1 active", result)
	}
	removed, err := warns.RemoveWarn(panelTestUserID, commandChatID)
	if err != nil {
		t.Fatalf("RemoveWarn() error = %v", err)
	}
	if !removed {
		t.Fatal("command RemoveWarn() removed nothing")
	}

	if got, want := warnStateString(t, panelChatID, panelTestUserID), warnStateString(t, commandChatID, panelTestUserID); got != want {
		t.Fatalf("panel state %q != command state %q", got, want)
	}
	if count, _ := warns.GetWarns(panelTestUserID, panelChatID); count != 1 {
		t.Fatalf("panel chat count after remove = %d, want 1", count)
	}
	if count, _ := warns.GetWarns(panelTestUserID, commandChatID); count != 1 {
		t.Fatalf("command chat count after remove = %d, want 1", count)
	}
}

func TestNonAdminCannotRemoveWarning(t *testing.T) {
	fixture := newPanelFixture(t)
	chatID := uniquePanelChatID()
	seedPanelChat(t, chatID, "Read-only Warnings Chat", fixtureNow)
	fixture.client.admins[chatID] = adminListJSON(999)

	stamp := time.Now().UTC()
	seedChatWarns(t, chatID, panelTestUserID, 0, []models.WarnEntry{
		{Reason: "kept", WarnedBy: 77, WarnedAt: &stamp},
	})

	body := mustJSON(t, warningRemoveRequest{UserID: panelTestUserID})
	recorder := panelRequest(t, fixture.handler, http.MethodDelete, listTarget(chatID, "warnings"), readFixture(t, "initdata_main.txt"), body, "application/json")
	requireAPIError(t, recorder, http.StatusForbidden, "not_admin")

	if count, _ := warns.GetWarns(panelTestUserID, chatID); count != 1 {
		t.Fatalf("warns after refused remove = %d, want 1", count)
	}
	if page := warningsPage(t, fixture, chatID); !page.ReadOnly {
		t.Fatal("read_only = false for a non-admin, want true")
	}
}

func TestWarningsRejectsMissingUserID(t *testing.T) {
	fixture, chatID := newWarningsFixture(t)

	recorder := panelRequest(t, fixture.handler, http.MethodDelete, listTarget(chatID, "warnings"), readFixture(t, "initdata_main.txt"), `{}`, "application/json")
	requireAPIError(t, recorder, http.StatusBadRequest, "invalid_user_id")
}
