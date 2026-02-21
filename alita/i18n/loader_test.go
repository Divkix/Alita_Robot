package i18n

import (
	"testing"
)

func TestExtractLangCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		{"yml extension", "en.yml", "en"},
		{"yaml extension", "fr.yaml", "fr"},
		{"no extension", "en", "en"},
		{"empty string", "", ""},
		// filepath.Ext returns ".bak" so TrimSuffix("en.yml", ".bak") → "en.yml",
		// then TrimSuffix("en.yml", ".yml") → "en", TrimSuffix("en", ".yaml") → "en"
		{"double extension bak", "en.yml.bak", "en"},
		{"hindi yml", "hi.yml", "hi"},
		{"spanish yaml", "es.yaml", "es"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractLangCode(tc.fileName)
			if got != tc.want {
				t.Errorf("extractLangCode(%q) = %q, want %q", tc.fileName, got, tc.want)
			}
		})
	}
}

func TestIsYAMLFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{"yml extension", "test.yml", true},
		{"yaml extension", "test.yaml", true},
		{"json extension", "test.json", false},
		{"empty string", "", false},
		{"only yml extension", ".yml", true},
		{"only yaml extension", ".yaml", true},
		{"uppercase YML is case insensitive", "TEST.YML", true},
		{"uppercase YAML is case insensitive", "TEST.YAML", true},
		{"txt extension", "test.txt", false},
		{"no extension", "testfile", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isYAMLFile(tc.fileName)
			if got != tc.want {
				t.Errorf("isYAMLFile(%q) = %v, want %v", tc.fileName, got, tc.want)
			}
		})
	}
}

func TestValidateYAMLStructure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{
			name:    "valid map",
			content: []byte("key: value\nanother: 123\n"),
			wantErr: false,
		},
		{
			name:    "valid nested map",
			content: []byte("greetings:\n  hello: Hello World\n  bye: Goodbye\n"),
			wantErr: false,
		},
		{
			name:    "array root is error",
			content: []byte("- item1\n- item2\n"),
			wantErr: true,
		},
		{
			name:    "invalid yaml is error",
			content: []byte("key: [unclosed bracket"),
			wantErr: true,
		},
		{
			name:    "empty content is error",
			content: []byte(""),
			wantErr: true,
		},
		{
			name:    "nil content is error",
			content: nil,
			wantErr: true,
		},
		{
			name:    "scalar root is error",
			content: []byte("just a string"),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateYAMLStructure(tc.content)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateYAMLStructure() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestCompileViper(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{
			name:    "valid yaml returns non-nil viper",
			content: []byte("key: value\nnested:\n  inner: hello\n"),
			wantErr: false,
		},
		{
			name:    "valid empty map",
			content: []byte("{}\n"),
			wantErr: false,
		},
		{
			name:    "invalid yaml returns error",
			content: []byte("key: [unclosed"),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			vi, err := compileViper(tc.content)
			if tc.wantErr {
				if err == nil {
					t.Errorf("compileViper() expected error, got nil")
				}
				if vi != nil {
					t.Errorf("compileViper() expected nil viper on error, got non-nil")
				}
			} else {
				if err != nil {
					t.Errorf("compileViper() unexpected error: %v", err)
				}
				if vi == nil {
					t.Errorf("compileViper() expected non-nil viper, got nil")
				}
			}
		})
	}
}

func TestCompileViper_ReadsValues(t *testing.T) {
	t.Parallel()
	content := []byte("greeting: hello\nnumber: 42\n")
	vi, err := compileViper(content)
	if err != nil {
		t.Fatalf("compileViper() unexpected error: %v", err)
	}
	if vi.GetString("greeting") != "hello" {
		t.Errorf("viper GetString(greeting) = %q, want %q", vi.GetString("greeting"), "hello")
	}
	if vi.GetInt("number") != 42 {
		t.Errorf("viper GetInt(number) = %d, want %d", vi.GetInt("number"), 42)
	}
}
