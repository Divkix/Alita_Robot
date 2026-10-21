//go:build testtools

package modules

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// A quoted argument that is no group's title stays the command's own.
func TestFilterInPMOnConnectionKeepsQuotedKeyword(t *testing.T) {
	f := oneOffFixture(t)
	setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: f.chat.Title})
	f.connectAdmin(t)
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, `/filter "hello world" hi there`)

	_ = withPMGroupArg("filter", helpers.PMGroupArgOwn, filtersModule.addFilter)(f.bot, ctx)

	if !filters.DoesFilterExists(f.chat.Id, "hello world") {
		t.Fatalf("filter \"hello world\" not saved in the connected group; sent:\n%s", sentText(f.client))
	}
}
