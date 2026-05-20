//go:build testtools

package cache

import "testing"

func TestInitTestMarshalRestoresPreviousMarshaler(t *testing.T) {
	withMemoryMarshaler(t)

	const key = "alita:test:init-marshal"
	if err := GetMarshal().Set(Context, key, "before"); err != nil {
		t.Fatalf("Set(before) error = %v", err)
	}

	restore := InitTestMarshal()
	t.Cleanup(restore)

	if GetMarshal() == nil {
		t.Fatal("GetMarshal() = nil after InitTestMarshal")
	}
	if err := GetMarshal().Set(Context, key, "after"); err != nil {
		t.Fatalf("Set(after) error = %v", err)
	}

	restore()

	var got string
	if _, err := GetMarshal().Get(Context, key, &got); err != nil {
		t.Fatalf("Get(before) error = %v", err)
	}
	if got != "before" {
		t.Fatalf("Get() after restore = %q, want before", got)
	}
}
