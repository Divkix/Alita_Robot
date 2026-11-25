package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProfileMergesDuplicateBlocks(t *testing.T) {
	profile := "mode: atomic\n" +
		"example.com/mod/a/x.go:1.1,2.2 3 0\n" +
		"example.com/mod/a/x.go:1.1,2.2 3 1\n"

	blocks, err := parseProfile(strings.NewReader(profile))
	if err != nil {
		t.Fatalf("parseProfile() error = %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d, want 1 merged block", len(blocks))
	}
	merged := blocks["example.com/mod/a/x.go:1.1,2.2"]
	if merged == nil {
		t.Fatal("merged block missing")
	}
	if merged.numStmt != 3 || merged.count != 1 {
		t.Fatalf("block = %+v, want numStmt 3 count 1", *merged)
	}
	pkg := aggregate(blocks)["example.com/mod/a"]
	if pkg == nil || pkg.total != 3 || pkg.covered != 3 {
		t.Fatalf("aggregate = %+v, want total 3 covered 3", pkg)
	}
}

func TestParseProfileRejectsInconsistentNumStmt(t *testing.T) {
	profile := "mode: atomic\n" +
		"example.com/mod/a/x.go:1.1,2.2 3 0\n" +
		"example.com/mod/a/x.go:1.1,2.2 4 1\n"

	if _, err := parseProfile(strings.NewReader(profile)); err == nil || !strings.Contains(err.Error(), "inconsistent numStmt") {
		t.Fatalf("parseProfile() error = %v, want inconsistent numStmt", err)
	}
}

func TestCheckFailsWhenGatedPackageMissing(t *testing.T) {
	var output bytes.Buffer
	err := check(strings.NewReader(gatedProfile("alita/utils/helpers")), &output)
	if err == nil || !strings.Contains(err.Error(), "alita/utils/helpers: no coverage data") {
		t.Fatalf("check() error = %v, want missing package failure", err)
	}
	if !strings.Contains(output.String(), "| alita/utils/helpers | n/a | 97.25% | missing |") {
		t.Fatalf("output missing table row:\n%s", output.String())
	}
}

func TestRunFailsBelowFloor(t *testing.T) {
	profile := gatedProfile("") + modulePrefix + "alita/db/extra.go:9.1,10.2 1 0\n"
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte(profile), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"-profile", path}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() exit = %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "alita/db: 50.00% < 70.10%") {
		t.Fatalf("stderr = %q, want alita/db floor failure", stderr.String())
	}
	if !strings.Contains(stdout.String(), "| alita/db | 50.00% | 70.10% | fail |") {
		t.Fatalf("stdout missing failing row:\n%s", stdout.String())
	}
}

// gatedProfile emits a profile where every gated package is fully covered,
// except the one named in skip.
func gatedProfile(skip string) string {
	var profile strings.Builder
	profile.WriteString("mode: atomic\n")
	for _, f := range floors {
		if f.pkg == skip {
			continue
		}
		fmt.Fprintf(&profile, "%s%s/covered.go:1.1,2.2 1 1\n", modulePrefix, f.pkg)
	}
	return profile.String()
}
