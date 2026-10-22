//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/bans"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// banRecordsOf returns the ban_records rows of chatID, oldest first.
func banRecordsOf(t *testing.T, chatID int64) []models.BanRecord {
	t.Helper()
	var rows []models.BanRecord
	if err := db.DB.Where("chat_id = ?", chatID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load ban records of chat %d: %v", chatID, err)
	}
	return rows
}

// singleBanRecord fails unless chatID has exactly one ban record and returns it.
func singleBanRecord(t *testing.T, chatID int64) models.BanRecord {
	t.Helper()
	rows := banRecordsOf(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("chat %d has %d ban records, want exactly 1", chatID, len(rows))
	}
	return rows[0]
}

// seedBanRecord stores a ban record as recordBan would.
func seedBanRecord(t *testing.T, r models.BanRecord) {
	t.Helper()
	if _, err := bans.Record(r); err != nil {
		t.Fatalf("Record(%+v) error = %v", r, err)
	}
}

// Every ban command records the ban it made, with the admin, the reason and,
// for /tban, its end.
func TestBanCommandsRecordTheBan(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		reason string
		timed  bool
		run    func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "ban", text: "/ban spam", reason: "spam", run: bansModule.ban},
		{name: "tban", text: "/tban 1h spam", reason: "spam", timed: true, run: bansModule.tBan},
		{name: "dban", text: "/dban spam", reason: "spam", run: bansModule.dBan},
		{name: "sban", text: "/sban", run: bansModule.sBan},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			start := time.Now()
			ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, tt.text)
			if err := tt.run(f.bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s() error = %v, want EndGroups", tt.name, err)
			}

			got := singleBanRecord(t, f.chat.Id)
			if got.UserId != f.member.Id || got.BannedBy != f.admin.Id || got.Source != models.BanSourceManual {
				t.Fatalf("record = %+v, want user %d banned by %d from manual", got, f.member.Id, f.admin.Id)
			}
			if got.Reason != tt.reason {
				t.Fatalf("record reason = %q, want %q", got.Reason, tt.reason)
			}
			if !got.IsOpen(time.Now()) {
				t.Fatalf("record = %+v, want an open ban", got)
			}
			switch {
			case !tt.timed && got.Until != nil:
				t.Fatalf("record until = %v, want a permanent ban", got.Until)
			case tt.timed && (got.Until == nil || got.Until.Before(start.Add(59*time.Minute)) || got.Until.After(time.Now().Add(61*time.Minute))):
				t.Fatalf("record until = %v, want about an hour from now", got.Until)
			}
		})
	}
}

func TestBanMeRecordsTheBanAsTheUsersOwn(t *testing.T) {
	f := newAdminChatFixture(t)
	if err := bansModule.banme(f.bot, newModuleMessageContext(f.bot, f.chat, f.member, "/banme")); err != ext.EndGroups {
		t.Fatalf("banme() error = %v, want EndGroups", err)
	}
	got := singleBanRecord(t, f.chat.Id)
	if got.UserId != f.member.Id || got.BannedBy != f.member.Id || got.Source != models.BanSourceManual {
		t.Fatalf("record = %+v, want the member's own ban", got)
	}
}

func TestRestrictBanButtonRecordsTheBan(t *testing.T) {
	f := newAdminChatFixture(t)
	data := encodeCallbackData("restrict", map[string]string{"a": "ban", "u": fmt.Sprint(f.member.Id)})
	if err := bansModule.restrictButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data)); err != ext.EndGroups {
		t.Fatalf("restrictButtonHandler(ban) error = %v, want EndGroups", err)
	}
	got := singleBanRecord(t, f.chat.Id)
	if got.UserId != f.member.Id || got.BannedBy != f.admin.Id || got.Source != models.BanSourceManual {
		t.Fatalf("record = %+v, want the pressing admin's ban", got)
	}
}

// The record follows the ban: a ban Telegram refused records nothing, and a
// kick is no ban.
func TestOnlyBansTelegramAcceptedAreRecorded(t *testing.T) {
	t.Run("refused ban", func(t *testing.T) {
		f := newAdminChatFixture(t)
		requestErr := errors.New("Forbidden: bot is not an administrator")
		f.client.errors["banChatMember"] = requestErr
		ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/ban spam")
		if err := bansModule.ban(f.bot, ctx); !errors.Is(err, requestErr) {
			t.Fatalf("ban() error = %v, want the request error", err)
		}
		if rows := banRecordsOf(t, f.chat.Id); len(rows) != 0 {
			t.Fatalf("refused ban recorded %d rows, want none", len(rows))
		}
	})
	t.Run("kick", func(t *testing.T) {
		f := newAdminChatFixture(t)
		ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/kick spam")
		if err := bansModule.kick(f.bot, ctx); err != ext.EndGroups {
			t.Fatalf("kick() error = %v, want EndGroups", err)
		}
		if rows := banRecordsOf(t, f.chat.Id); len(rows) != 0 {
			t.Fatalf("kick recorded %d ban rows, want none", len(rows))
		}
	})
}

// A ban whose record cannot be written still bans and answers as usual.
func TestBanRecordWriteFailureKeepsTheBanReply(t *testing.T) {
	f := newAdminChatFixture(t)
	if err := db.DB.Migrator().DropTable(&models.BanRecord{}); err != nil {
		t.Fatalf("drop ban_records: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.BanRecord{}); err != nil {
			t.Errorf("restore ban_records: %v", err)
		}
	})
	ctx := newMuteReplyContext(f.bot, f.chat, f.admin, f.member, "/ban spam")

	if err := bansModule.ban(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban() error = %v, want EndGroups despite the record write failure", err)
	}
	assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, f.member.Id))
	reply := f.client.onlyCall(t, "sendMessage")
	if reply.Params["reply_markup"] == nil {
		t.Fatal("ban reply lost its unban button after the record write failed")
	}
}

func TestUnbanPathsCloseTheBanRecord(t *testing.T) {
	for name, unban := range map[string]func(f adminChatFixture) error{
		"command": func(f adminChatFixture) error {
			return bansModule.unban(f.bot, newModuleMessageContext(f.bot, f.chat, f.admin, fmt.Sprintf("/unban %d", f.member.Id)))
		},
		"button": func(f adminChatFixture) error {
			data := encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(f.member.Id)})
			return bansModule.unrestrictButtonHandler(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data))
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: f.member.Id, BannedBy: 7, Source: models.BanSourceAntiflood})

			if err := unban(f); err != ext.EndGroups {
				t.Fatalf("unban %s error = %v, want EndGroups", name, err)
			}
			got := singleBanRecord(t, f.chat.Id)
			if got.LiftedAt == nil || got.LiftedBy == nil || *got.LiftedBy != f.admin.Id {
				t.Fatalf("record = %+v, want it lifted by %d", got, f.admin.Id)
			}
		})
	}
}

func TestUnbanAllClosesTheBanRecordsOfTheUsersItUnbanned(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	const listed, unlisted = 43, 44
	for _, userID := range []int64{listed, unlisted} {
		seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: userID, Source: models.BanSourceManual, BannedBy: f.admin.Id})
	}
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", f.chat.Id).
		Update("users", models.Int64Array{listed}).Error; err != nil {
		t.Fatalf("seed users of %d: %v", f.chat.Id, err)
	}
	dbcache.DeleteCache(dbcache.CacheKey("chat_users", f.chat.Id))

	data := encodeCallbackData("unbanall", map[string]string{"a": "yes"})
	if err := bansModule.unbanAllCallback(f.bot, newModuleCallbackContext(f.bot, f.chat, f.admin, data)); err != ext.EndGroups {
		t.Fatalf("unbanAllCallback(yes) error = %v, want EndGroups", err)
	}

	for _, r := range banRecordsOf(t, f.chat.Id) {
		lifted := r.LiftedAt != nil && r.LiftedBy != nil && *r.LiftedBy == f.admin.Id
		if lifted != (r.UserId == listed) {
			t.Fatalf("record of %d = %+v; want only user %d's lifted, by %d", r.UserId, r, listed, f.admin.Id)
		}
	}
}

// A warn limit the bot's own warns reached is recorded as the bot's ban.
func TestWarnLimitBanRecordsTheBan(t *testing.T) {
	f := newAdminChatFixture(t)
	tr := i18n.MustNewTranslator("en")
	settings := &models.WarnSettings{WarnMode: "ban", WarnLimit: 3}
	if _, punished, _, err := punishAtWarnLimit(f.bot, &f.chat, tr, settings, f.member, 3, 0, "flooding"); err != nil || !punished {
		t.Fatalf("punishAtWarnLimit() = %v, %v; want the member banned", punished, err)
	}
	got := singleBanRecord(t, f.chat.Id)
	if got.UserId != f.member.Id || got.BannedBy != 0 || got.Source != models.BanSourceWarn || got.Reason != "flooding" {
		t.Fatalf("record = %+v, want the bot's warn ban for flooding", got)
	}
}

func TestBanRecordRetentionDeletesOnlyOldEndedBans(t *testing.T) {
	f := newAdminChatFixture(t)
	old := time.Now().Add(-banRecordRetention - 24*time.Hour)
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: 1, Source: models.BanSourceManual, CreatedAt: old})
	seedBanRecord(t, models.BanRecord{ChatId: f.chat.Id, UserId: 2, Source: models.BanSourceManual, CreatedAt: old, Until: &old})

	sweepExpiredBanRecords(time.Now())

	if got := singleBanRecord(t, f.chat.Id); got.UserId != 1 {
		t.Fatalf("kept the record of %d, want the open permanent ban of 1", got.UserId)
	}
}

// A federation ban records the ban in each chat of the federation with the
// federation as its detail, and the unfban closes those records.
func TestFedBanWalkRecordsAndUnfbanCloses(t *testing.T) {
	bot := newModuleTestBot(newModuleBotClient())
	owner := uniquePositiveUserID()
	fed, err := federations.CreateFederation(owner, "Record Fed")
	if err != nil {
		t.Fatalf("CreateFederation() error = %v", err)
	}
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })
	target := uniquePositiveUserID()
	chat := seedFedChat(t, fed.FedID, target)
	t.Cleanup(func() { _ = db.DB.Where("chat_id = ?", chat.Id).Delete(&models.BanRecord{}).Error })
	if _, _, err := federations.Fban(fed.FedID, target, owner, "spam"); err != nil {
		t.Fatalf("Fban() error = %v", err)
	}

	applyActiveFban(bot, fed.FedID, target, owner)

	got := singleBanRecord(t, chat.Id)
	if got.UserId != target || got.BannedBy != owner || got.Source != models.BanSourceFed || got.SourceDetail != fed.FedID {
		t.Fatalf("record = %+v, want the fed ban of %d by %d from %s", got, target, owner, fed.FedID)
	}
	if err := federations.Unfban(fed.FedID, target); err != nil {
		t.Fatalf("Unfban() error = %v", err)
	}

	applyActiveUnfban(bot, fed.FedID, target, owner)

	if got := singleBanRecord(t, chat.Id); got.LiftedAt == nil || got.LiftedBy == nil || *got.LiftedBy != owner {
		t.Fatalf("record = %+v, want it lifted by %d", got, owner)
	}
}
