package main

import (
	"path/filepath"
	"testing"
)

func TestResolveOutputPath(t *testing.T) {
	t.Parallel()

	projectRoot := filepath.Join(string(filepath.Separator), "repo", "alita")
	absoluteOutput := filepath.Join(string(filepath.Separator), "tmp", "alita-docs")

	tests := []struct {
		name       string
		outputPath string
		want       string
	}{
		{
			name:       "relative output is resolved under project root",
			outputPath: filepath.Join("docs", "src", "content", "docs"),
			want:       filepath.Join(projectRoot, "docs", "src", "content", "docs"),
		},
		{
			name:       "absolute output is preserved",
			outputPath: absoluteOutput,
			want:       absoluteOutput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveOutputPath(projectRoot, tt.outputPath)
			if got != tt.want {
				t.Fatalf("resolveOutputPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
