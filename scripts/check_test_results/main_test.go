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
