//go:build testtools

package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/probation"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
)

func probationSettings(t *testing.T, chatID int64) *models.ProbationSettings {
	t.Helper()
	settings, err := probation.GetSettings(context.Background(), chatID)
	if err != nil {
		t.Fatalf("probation.GetSettings: %v", err)
	}
	return settings
}

// Probation is off with its documented defaults, and reading it creates no row.
func TestSettingsProbationDefaults(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)

	body := decodeSettingsResponse(t, panelRequest(t, fixture.handler, http.MethodGet, settingsPath(chatID),
		readFixture(t, "initdata_main.txt"), "", ""))
	if fieldBool(t, body, "antispam.probation_enabled") || fieldInt(t, body, "antispam.probation_time") != 86400 ||
		fieldInt(t, body, "antispam.probation_messages") != 5 || fieldInt(t, body, "antispam.probation_min_length") != 20 ||
		!fieldBool(t, body, "antispam.probation_block_links") || !fieldBool(t, body, "antispam.probation_block_forwards") ||
		fieldBool(t, body, "antispam.probation_block_media") || fieldBool(t, body, "antispam.probation_notice") {
		t.Fatalf("probation defaults = %+v", body.Modules[5])
	}
	if count := countChatRows(t, &models.ProbationSettings{}, chatID); count != 0 {
		t.Fatalf("GET created %d probation rows", count)
	}
}

// /probation checks can_restrict_members, so its panel fields need the
// caller's and the bot's restrict right; the other antispam fields still need
// admin status only.
func TestSettingsProbationNeedsRestrict(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")
	fixture.client.rights[settingsFixtureUserID] = panelFakeRights{canChangeInfo: true}

	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"antispam.probation_enabled":true}}`, "application/json")
	requireAPIError(t, recorder, http.StatusForbidden, "missing_permission")
	if count := countChatRows(t, &models.ProbationSettings{}, chatID); count != 0 {
		t.Fatalf("refused probation write created %d rows", count)
	}

	recorder = panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"antispam.spamdb_action":"log"}}`, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("spamdb write without restrict: status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if row, err := spamdbsettings.GetSpamDBSettingsContext(context.Background(), chatID); err != nil || row.Action != "log" {
		t.Fatalf("spamdb action = %+v, %v; want log", row, err)
	}

	fixture.client.rights[settingsFixtureUserID] = panelFakeRights{canRestrictMembers: true}
	fixture.client.botRights.canRestrictMembers = false
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"antispam.probation_enabled":true}}`, "application/json")
	requireAPIError(t, recorder, http.StatusConflict, "bot_missing_permission")

	fixture.client.botRights.canRestrictMembers = true
	recorder = panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"antispam.probation_enabled":true,"antispam.probation_time":7200,"antispam.probation_messages":3,
		"antispam.probation_min_length":10,"antispam.probation_block_media":true,"antispam.probation_notice":true}}`, "application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	got := probationSettings(t, chatID)
	if !got.Enabled || got.DurationSeconds != 7200 || got.MessageCount != 3 || got.MinLength != 10 ||
		!got.BlockLinks || !got.BlockForwards || !got.BlockMedia || !got.Notice {
		t.Fatalf("probation after patch = %+v", got)
	}
}

func TestSettingsProbationValidation(t *testing.T) {
	fixture, chatID := newSettingsFixture(t)
	raw := readFixture(t, "initdata_main.txt")

	for name, body := range map[string]string{
		"time too short":    `{"changes":{"antispam.probation_time":3599}}`,
		"time too long":     `{"changes":{"antispam.probation_time":2592001}}`,
		"no messages":       `{"changes":{"antispam.probation_messages":0}}`,
		"min length high":   `{"changes":{"antispam.probation_min_length":501}}`,
		"nothing blocked":   `{"changes":{"antispam.probation_block_links":false,"antispam.probation_block_forwards":false}}`,
		"valid beside none": `{"changes":{"antispam.probation_enabled":true,"antispam.probation_messages":101}}`,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw, body, "application/json")
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", recorder.Code, recorder.Body.String())
			}
			var response settingsPatchResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(response.Errors) == 0 {
				t.Fatalf("errors empty, want field errors (body %s)", recorder.Body.String())
			}
			if count := countChatRows(t, &models.ProbationSettings{}, chatID); count != 0 {
				t.Fatalf("rejected patch created %d probation rows", count)
			}
		})
	}

	// Swapping links for media in one batch keeps one kind blocked.
	recorder := panelRequest(t, fixture.handler, http.MethodPatch, settingsPath(chatID), raw,
		`{"changes":{"antispam.probation_block_links":false,"antispam.probation_block_forwards":false,"antispam.probation_block_media":true}}`,
		"application/json")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}
	if got := probationSettings(t, chatID); got.BlockLinks || got.BlockForwards || !got.BlockMedia {
		t.Fatalf("blocked after swap = %+v, want media only", got)
	}
}
