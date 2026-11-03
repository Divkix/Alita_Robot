package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMergeCoverage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inputs  []string
		want    string
		wantErr string
	}{
		{
			name: "overlapping and distinct blocks",
			inputs: []string{
				"mode: atomic\nz.go:1.1,2.1 3 0\na.go:1.1,1.2 2 2\nonly first.go:3.1,3.2 1 0\n",
				"mode: atomic\nz.go:1.1,2.1 3 4\na.go:01.01,01.02 2 5\nonly-second.go:4.1,5.1 4 3\n",
			},
			want: "mode: atomic\na.go:1.1,1.2 2 7\nonly first.go:3.1,3.2 1 0\nonly-second.go:4.1,5.1 4 3\nz.go:1.1,2.1 3 4\n",
		},
		{
			name:    "mode mismatch",
			inputs:  []string{"mode: atomic\na.go:1.1,1.2 1 0\n", "mode: set\na.go:1.1,1.2 1 1\n"},
			wantErr: "expected mode: atomic",
		},
		{
			name:    "statement mismatch",
			inputs:  []string{"mode: atomic\na.go:1.1,1.2 1 0\n", "mode: atomic\na.go:1.1,1.2 2 1\n"},
			wantErr: "statement count mismatch",
		},
		{
			name:    "count overflow",
			inputs:  []string{"mode: atomic\na.go:1.1,1.2 1 9223372036854775807\na.go:1.1,1.2 1 1\n"},
			wantErr: "coverage count overflow",
		},
		{
			name:    "invalid number",
			inputs:  []string{"mode: atomic\na.go:1.1,1.2 1 9223372036854775808\n"},
			wantErr: "invalid coverage number",
		},
		{
			name:    "invalid block",
			inputs:  []string{"mode: atomic\na.go:1.1,1.2 1 -1\n"},
			wantErr: "invalid coverage block",
		},
		{
			name:    "invalid coordinates",
			inputs:  []string{"mode: atomic\na.go:2.1,1.2 1 0\n"},
			wantErr: "invalid coverage coordinates",
		},
		{name: "missing mode", inputs: []string{""}, wantErr: "missing coverage mode"},
		{name: "no blocks", inputs: []string{"mode: atomic\n"}, wantErr: "no coverage blocks"},
		{name: "no inputs", wantErr: "usage:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var paths []string
			for i, input := range tc.inputs {
				path := filepath.Join(dir, strconv.Itoa(i)+".out")
				if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
			}
			var output bytes.Buffer
			err := mergeCoverage(paths, &output)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got error %v, want %q", err, tc.wantErr)
				}
				if output.Len() != 0 {
					t.Fatal("wrote a partial profile for invalid input")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tc.want {
				t.Fatalf("got profile %q, want %q", got, tc.want)
			}
		})
	}
	var output bytes.Buffer
	if err := mergeCoverage([]string{filepath.Join(t.TempDir(), "missing.out")}, &output); !os.IsNotExist(err) {
		t.Fatalf("got error %v, want missing file", err)
	}
}
