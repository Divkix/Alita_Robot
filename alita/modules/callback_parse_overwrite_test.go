package modules

import "testing"

func TestParseOverwriteCallbackData(t *testing.T) {
	tests := []struct {
		namespace string
		action    string
		token     string
	}{
		{namespace: "filters_overwrite", action: "yes", token: "f0.token"},
		{namespace: "notes.overwrite", action: "no", token: "tok._-42"},
	}
	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			data := encodeCallbackData(tt.namespace, map[string]string{
				"a": tt.action,
				"t": tt.token,
			})
			action, token, ok := parseOverwriteCallbackData(data, tt.namespace)
			if !ok || action != tt.action || token != tt.token {
				t.Fatalf("parseOverwriteCallbackData() = %q, %q, %v, want %q, %q, true", action, token, ok, tt.action, tt.token)
			}
		})
	}
}

func TestParseOverwriteCallbackDataRejectsLegacy(t *testing.T) {
	tests := []struct {
		namespace string
		data      string
	}{
		{namespace: "notes.overwrite", data: "notes.overwrite.yes.123_note.key_with.parts"},
		{namespace: "filters_overwrite", data: "filters_overwrite.foo.bar_baz"},
		{namespace: "filters_overwrite", data: "filters_overwrite.cancel"},
	}
	for _, tt := range tests {
		if _, _, ok := parseOverwriteCallbackData(tt.data, tt.namespace); ok {
			t.Fatalf("parseOverwriteCallbackData(%q) accepted legacy data %q", tt.namespace, tt.data)
		}
	}
}
