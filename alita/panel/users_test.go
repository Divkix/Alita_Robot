//go:build testtools

package panel

import (
	"net/http"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

var namedUserSeq atomic.Int64

// seedNamedUser stores a user the way the users tracker does, under an ID no
// other test uses.
func seedNamedUser(t *testing.T, name, username string) int64 {
	t.Helper()
	id := 7_000_000_000 + time.Now().UnixMilli()%1_000_000*1000 + namedUserSeq.Add(1)
	if err := user.EnsureUserInDb(id, username, name); err != nil {
		t.Fatalf("seed user %q: %v", name, err)
	}
	t.Cleanup(func() {
		db.DB.Where("user_id = ?", id).Delete(&models.User{})
	})
	return id
}

func key(id int64) string {
	return strconv.FormatInt(id, 10)
}

func TestTimelineNamesStoredActorsAndTargets(t *testing.T) {
	fixture, chatID := newTimelineFixture(t)
	actor := seedNamedUser(t, "Ada", "ada_admin")
	target := seedNamedUser(t, "Spammer", "")
	unknown := int64(6_999_999_901)
	base := fixtureNow.UTC().Add(-time.Hour)
	recordTimelineEvent(t, models.ModerationEvent{ChatId: chatID, ActorId: actor, TargetUserId: target, Action: models.ModerationActionBan, CreatedAt: base})
	recordTimelineEvent(t, models.ModerationEvent{ChatId: chatID, ActorId: actor, TargetUserId: unknown, Action: models.ModerationActionMute, CreatedAt: base.Add(time.Minute)})

	page := timelinePage(t, fixture, chatID, "")
	want := map[string]userName{
		key(actor):  {Name: "Ada", Username: "ada_admin"},
		key(target): {Name: "Spammer"},
	}
	if !reflect.DeepEqual(page.Names, want) {
		t.Fatalf("timeline names = %+v, want %+v (a user the bot never stored is left out)", page.Names, want)
	}
}

func TestWarningsNameWarnedUsersAndIssuers(t *testing.T) {
	fixture, chatID := newWarningsFixture(t)
	issuer := seedNamedUser(t, "Moderator", "mod")
	warned := seedNamedUser(t, "Troll", "")
	at := fixtureNow.UTC()
	seedChatWarns(t, chatID, warned, 0, []models.WarnEntry{
		{Reason: "spam", WarnedBy: issuer, WarnedAt: &at},
		{Reason: "hidden issuer", WarnedBy: issuer, Anonymous: true, WarnedAt: &at},
	})

	page := warningsPage(t, fixture, chatID, "")
	want := map[string]userName{
		key(issuer): {Name: "Moderator", Username: "mod"},
		key(warned): {Name: "Troll"},
	}
	if !reflect.DeepEqual(page.Names, want) {
		t.Fatalf("warnings names = %+v, want %+v", page.Names, want)
	}
}

func TestWarningsDoNotNameAnAnonymousIssuer(t *testing.T) {
	fixture, chatID := newWarningsFixture(t)
	issuer := seedNamedUser(t, "Hidden Admin", "hidden")
	warned := seedNamedUser(t, "Troll", "")
	at := fixtureNow.UTC()
	seedChatWarns(t, chatID, warned, 0, []models.WarnEntry{
		{Reason: "anon", WarnedBy: issuer, Anonymous: true, WarnedAt: &at},
	})

	page := warningsPage(t, fixture, chatID, "")
	if _, found := page.Names[key(issuer)]; found {
		t.Fatalf("warnings names = %+v, must not name the anonymous issuer %d", page.Names, issuer)
	}
	if page.Names[key(warned)].Name != "Troll" {
		t.Fatalf("warnings names = %+v, want the warned user named", page.Names)
	}
}

func TestOverviewNamesRecentModerationForAdminsOnly(t *testing.T) {
	fixture, chatID := newOverviewFixture(t)
	actor := seedNamedUser(t, "Ada", "")
	target := seedNamedUser(t, "Spammer", "")
	seedModerationEvent(t, chatID, actor, target, models.ModerationActionBan, "spam", fixtureNow.Add(-time.Hour))

	admin := decodeOverview(t, panelRequest(t, fixture.handler, http.MethodGet, overviewPath(chatID, ""), readFixture(t, "initdata_main.txt"), "", ""))
	if admin.Names[key(actor)].Name != "Ada" || admin.Names[key(target)].Name != "Spammer" {
		t.Fatalf("admin overview names = %+v, want actor and target named", admin.Names)
	}

	memberFixture, memberChatID := newOverviewFixture(t)
	memberFixture.client.admins[memberChatID] = adminListJSON(999)
	seedModerationEvent(t, memberChatID, actor, target, models.ModerationActionBan, "spam", fixtureNow.Add(-time.Hour))
	member := decodeOverview(t, panelRequest(t, memberFixture.handler, http.MethodGet, overviewPath(memberChatID, ""), readFixture(t, "initdata_main.txt"), "", ""))
	if len(member.Recent) != 1 {
		t.Fatalf("member overview recent = %+v, want the event", member.Recent)
	}
	if member.Names != nil {
		t.Fatalf("member overview names = %+v, want none for a non-admin", member.Names)
	}
}
