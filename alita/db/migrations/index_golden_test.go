package migrations

import "testing"

func TestParseGoldenIndexesRoundTrip(t *testing.T) {
	const data = `chats|uk_chats_chat_id|CREATE UNIQUE INDEX uk_chats_chat_id ON public.chats USING btree (chat_id)
users|idx_users_user_name|CREATE INDEX idx_users_user_name ON public.users USING btree (username)
`

	indexes, err := parseGoldenIndexes(data)
	if err != nil {
		t.Fatalf("parseGoldenIndexes() error = %v", err)
	}
	if len(indexes) != 2 {
		t.Fatalf("parsed %d indexes, want 2", len(indexes))
	}
	if indexes[0].Table != "chats" || indexes[0].Name != "uk_chats_chat_id" {
		t.Fatalf("first index = %+v", indexes[0])
	}

	if got := formatGoldenIndexes(indexes); got != data {
		t.Fatalf("formatGoldenIndexes() =\n%s\nwant\n%s", got, data)
	}

	// Rewriting must sort, whichever order the catalog returned.
	unsorted := []goldenIndex{indexes[1], indexes[0]}
	if got := formatGoldenIndexes(unsorted); got != data {
		t.Fatalf("formatGoldenIndexes(unsorted) =\n%s\nwant\n%s", got, data)
	}
}

func TestParseGoldenIndexesRejectsMalformedRows(t *testing.T) {
	rows := map[string]string{
		"two fields":     "users|idx_users_user_name\n",
		"empty table":    "|idx_users_user_name|CREATE INDEX idx ON public.users USING btree (username)\n",
		"empty def":      "users|idx_users_user_name|\n",
		"duplicate pair": "users|idx|CREATE INDEX idx ON public.users USING btree (username)\nusers|idx|CREATE INDEX idx ON public.users USING btree (username)\n",
	}
	for name, data := range rows {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGoldenIndexes(data); err == nil {
				t.Fatalf("parseGoldenIndexes(%q) succeeded, want an error", data)
			}
		})
	}
}

func TestEmbeddedGoldenIndexesParse(t *testing.T) {
	if _, err := parseGoldenIndexes(expectedIndexesGolden); err != nil {
		t.Fatalf("embedded %s does not parse: %v", expectedIndexesFile, err)
	}
}

// A dropped index, a redefined index and a reintroduced index must all be
// reported; matching golden and catalog entries must produce no diff.
func TestDiffIndexesDetectsDrift(t *testing.T) {
	const (
		userNameDef   = "CREATE INDEX idx_users_user_name ON public.users USING btree (username)"
		approvedDef   = "CREATE INDEX idx_approved_users_chat_id ON public.approved_users USING btree (chat_id)"
		captchaDef    = "CREATE INDEX idx_captcha_attempts_chat_id ON public.captcha_attempts USING btree (chat_id)"
		captchaNewDef = "CREATE INDEX idx_captcha_attempts_chat_id ON public.captcha_attempts USING btree (chat_id, user_id)"
		warnsDef      = "CREATE INDEX idx_warns_users_chat_id ON public.warns_users USING btree (chat_id)"
	)

	expected := []goldenIndex{
		{Table: "users", Name: "idx_users_user_name", Def: userNameDef},
		{Table: "approved_users", Name: "idx_approved_users_chat_id", Def: approvedDef},
		{Table: "captcha_attempts", Name: "idx_captcha_attempts_chat_id", Def: captchaDef},
	}
	actual := []goldenIndex{
		{Table: "users", Name: "idx_users_user_name", Def: userNameDef},
		{Table: "captcha_attempts", Name: "idx_captcha_attempts_chat_id", Def: captchaNewDef},
		{Table: "warns_users", Name: "idx_warns_users_chat_id", Def: warnsDef},
	}

	diffs := diffIndexes(expected, actual)
	if len(diffs) != 3 {
		t.Fatalf("diffIndexes() returned %d diffs, want 3: %v", len(diffs), diffs)
	}
	if diffs[0].Kind != indexMissing || diffs[0].Name != "idx_approved_users_chat_id" {
		t.Errorf("first diff = %+v, want missing idx_approved_users_chat_id", diffs[0])
	}
	if diffs[1].Kind != indexChanged || diffs[1].Name != "idx_captcha_attempts_chat_id" {
		t.Errorf("second diff = %+v, want changed idx_captcha_attempts_chat_id", diffs[1])
	}
	if diffs[2].Kind != indexUnexpected || diffs[2].Name != "idx_warns_users_chat_id" {
		t.Errorf("third diff = %+v, want unexpected idx_warns_users_chat_id", diffs[2])
	}

	if diffs := diffIndexes(expected, expected); len(diffs) != 0 {
		t.Errorf("diffIndexes(expected, expected) = %v, want no diffs", diffs)
	}
}
