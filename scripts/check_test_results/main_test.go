package main

import (
	"io"
	"strings"
	"testing"
)

func TestCheckResultsRejectsFailuresAndUnexpectedSkips(t *testing.T) {
	for _, input := range []string{
		`{"Action":"skip","Package":"repo/approvals","Test":"TestPersistApproval"}`,
		`{"Action":"fail","Package":"repo/approvals"}`,
		`{"Action":"build-fail","ImportPath":"repo/approvals"}`,
		``,
	} {
		if err := checkResults(strings.NewReader(input), io.Discard); err == nil {
			t.Errorf("accepted unsuccessful test stream %q", input)
		}
	}
	if err := checkResults(strings.NewReader(`{"Action":"pass","Package":"repo/approvals"}`), io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestCheckResultsAllowsPostgresOnlySkipsOutsidePostgres(t *testing.T) {
	skips := []string{
		`{"Action":"skip","Package":"github.com/divkix/Alita_Robot/alita/db/chats","Test":"TestUpdateChat"}`,
		`{"Action":"skip","Package":"github.com/divkix/Alita_Robot/alita/db/backup","Test":"TestWarnsImportAndClearWaitForTheChatLock"}`,
		`{"Action":"skip","Package":"github.com/divkix/Alita_Robot/alita/db/warns","Test":"TestResetsWaitForTheChatLock"}`,
	}
	pass := `{"Action":"pass","Package":"repo/approvals"}`
	for _, skip := range skips {
		t.Setenv("ALITA_TEST_DATABASE", "")
		if err := checkResults(strings.NewReader(skip+"\n"+pass), io.Discard); err != nil {
			t.Errorf("rejected Postgres-only skip without Postgres %q: %v", skip, err)
		}
		t.Setenv("ALITA_TEST_DATABASE", "true")
		if err := checkResults(strings.NewReader(skip+"\n"+pass), io.Discard); err == nil {
			t.Errorf("accepted skip %q on Postgres", skip)
		}
	}
	t.Setenv("ALITA_TEST_DATABASE", "")
	other := `{"Action":"skip","Package":"github.com/divkix/Alita_Robot/alita/db/warns","Test":"TestOther"}`
	if err := checkResults(strings.NewReader(other+"\n"+pass), io.Discard); err == nil {
		t.Error("accepted an unlisted skip")
	}
}
