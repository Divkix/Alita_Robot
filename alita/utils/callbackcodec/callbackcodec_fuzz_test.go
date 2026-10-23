package callbackcodec

import (
	"strings"
	"testing"
)

// FuzzDecode checks that Decode never panics, and that whatever it accepts
// has a non-empty namespace without the separator and readable fields.
func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		"",
		"|",
		"||",
		"bans|v1|_",
		"bans|v1|",
		"bans|v1|a=1&b=2",
		"bans|v2|a=1",
		"|v1|a=1",
		"ns|v1|a=%ZZ",
		"ns|v1|a=1;b=2",
		"ns|v1|%F0%9F%98%80=%F0%9F%98%80",
		"😀|v1|k=😀|x",
		"ns|v1|a&a=&=b&&",
		"\xff|v1|\xfe=\xfd",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		decoded, err := Decode(data)
		if err != nil {
			if decoded != nil {
				t.Fatalf("Decode(%q) returned %+v with error %v", data, decoded, err)
			}
			if v, ok := decoded.Field(data); ok || v != "" {
				t.Fatalf("Field on a failed decode = %q, %v; want \"\", false", v, ok)
			}
			return
		}
		if decoded.Namespace == "" || strings.Contains(decoded.Namespace, "|") {
			t.Fatalf("Decode(%q) accepted namespace %q", data, decoded.Namespace)
		}
		for k, want := range decoded.Fields {
			got, ok := decoded.Field(k)
			if !ok || got != want {
				t.Fatalf("Decode(%q).Field(%q) = %q, %v; want %q, true", data, k, got, ok, want)
			}
		}
	})
}

// FuzzEncodeRoundTrip checks that every payload Encode accepts fits
// Telegram's callback_data limit and decodes back to the same namespace and
// fields, and that EncodeOrFallback falls back exactly when Encode fails.
func FuzzEncodeRoundTrip(f *testing.F) {
	type seed struct {
		ns                     string
		k1, v1, k2, v2, k3, v3 string
		pairs                  uint8
		fallback               string
	}
	for _, s := range []seed{
		{"bans", "a", "1", "", "", "", "", 1, "fb"},
		{"bans", "u", "123456789", "c", "-1001234567890", "x", "y", 3, ""},
		{"ns", "", "", "", "", "", "", 0, "fallback"},
		{"ns", "", "dropped", "k", "", "", "", 2, ""},
		{"", "a", "1", "", "", "", "", 1, "empty-ns"},
		{"a|b", "a", "1", "", "", "", "", 1, "pipe-ns"},
		{"😀", "🎉", "😀👍🏽", "k", "\U0001F600", "", "", 2, "emoji"},
		{"ns", "a&b=c", "%2F+ |;", "_", "", "a", "a", 3, ""},
		{"ns", "same", "1", "same", "2", "", "", 2, ""},
		{"\xff", "\xfe", "\xfd\xc3", "", "", "", "", 1, "invalid-utf8"},
		{"ns", "k", strings.Repeat("v", 55), "", "", "", "", 1, "at-limit"},
		{"ns", "k", strings.Repeat("v", 56), "", "", "", "", 1, "over-limit"},
	} {
		f.Add(s.ns, s.k1, s.v1, s.k2, s.v2, s.k3, s.v3, s.pairs, s.fallback)
	}

	f.Fuzz(func(t *testing.T, ns, k1, v1, k2, v2, k3, v3 string, pairs uint8, fallback string) {
		kv := [][2]string{{k1, v1}, {k2, v2}, {k3, v3}}[:int(pairs%4)]
		fields := make(map[string]string, len(kv))
		for _, p := range kv {
			fields[p[0]] = p[1]
		}

		data, err := Encode(ns, fields)

		got := EncodeOrFallback(ns, fields, fallback)
		if err != nil && got != fallback {
			t.Fatalf("EncodeOrFallback = %q after Encode error %v; want fallback %q", got, err, fallback)
		}
		if err == nil && got != data {
			t.Fatalf("EncodeOrFallback = %q; want Encode result %q", got, data)
		}
		if err != nil {
			return
		}

		if len(data) > MaxCallbackDataLen {
			t.Fatalf("Encode returned %d bytes (%q); limit is %d", len(data), data, MaxCallbackDataLen)
		}

		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(Encode(%q, %q)) = %q failed: %v", ns, fields, data, err)
		}
		if decoded.Namespace != ns {
			t.Fatalf("namespace round trip: got %q, want %q (data %q)", decoded.Namespace, ns, data)
		}

		want := 0
		for k, v := range fields {
			if k == "" {
				continue // Encode drops empty keys.
			}
			want++
			gotV, ok := decoded.Field(k)
			if !ok || gotV != v {
				t.Fatalf("field %q round trip: got %q, %v; want %q (data %q)", k, gotV, ok, v, data)
			}
		}
		if len(decoded.Fields) != want {
			t.Fatalf("decoded %d fields %q; want %d from %q (data %q)", len(decoded.Fields), decoded.Fields, want, fields, data)
		}
	})
}
