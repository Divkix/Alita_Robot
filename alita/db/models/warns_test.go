package models

import (
	"testing"
	"time"
)

func issued(reason string, by int64) WarnEntry {
	at := time.Unix(1_700_000_000+by, 0).UTC()
	return WarnEntry{UserId: 5, ChatId: -9, Reason: reason, WarnedBy: by, WarnedAt: &at}
}

func checkEntries(t *testing.T, got, want []WarnEntry) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("entries = %+v, want %+v", got, want)
	}
	for i := range want {
		g, w := got[i], want[i]
		sameTime := (g.WarnedAt == nil) == (w.WarnedAt == nil) && (g.WarnedAt == nil || g.WarnedAt.Equal(*w.WarnedAt))
		if g.Reason != w.Reason || g.WarnedBy != w.WarnedBy || g.Anonymous != w.Anonymous || !sameTime ||
			g.UserId != w.UserId || g.ChatId != w.ChatId {
			t.Fatalf("entry %d = %+v, want %+v", i, g, w)
		}
	}
}

func legacy(reason string) WarnEntry {
	return WarnEntry{UserId: 5, ChatId: -9, Reason: reason}
}

func TestReconcileWarnEntries(t *testing.T) {
	a, b, d := issued("a", 1), issued("b", 2), issued("d", 4)

	tests := []struct {
		name     string
		warns    Warns
		stored   []WarnEntry
		want     []WarnEntry
		wantKept int
	}{
		{
			name:     "in step",
			warns:    Warns{NumWarns: 2, Reasons: StringArray{"a", "b"}},
			stored:   []WarnEntry{a, b},
			want:     []WarnEntry{a, b},
			wantKept: 2,
		},
		{
			// New a, b; old /rmwarn (num 1, [a]); old /warn c (num 2, [a c]).
			name:     "old release popped then appended",
			warns:    Warns{NumWarns: 2, Reasons: StringArray{"a", "c"}},
			stored:   []WarnEntry{a, b},
			want:     []WarnEntry{a, legacy("c")},
			wantKept: 1,
		},
		{
			// The case above after a new /warn d, which appends its entry to the
			// reconciled list.
			name:     "new warn after the repair",
			warns:    Warns{NumWarns: 3, Reasons: StringArray{"a", "c", "d"}},
			stored:   []WarnEntry{a, legacy("c"), d},
			want:     []WarnEntry{a, legacy("c"), d},
			wantKept: 3,
		},
		{
			// Old /resetwarns deletes warns_users alone, then old /warn spam.
			name:     "more entries than warns",
			warns:    Warns{NumWarns: 1, Reasons: StringArray{"spam"}},
			stored:   []WarnEntry{a, b, issued("c", 3)},
			want:     []WarnEntry{legacy("spam")},
			wantKept: 0,
		},
		{
			// Only a leading run belongs to counted warns: past the first
			// mismatch every stored entry was popped by an older release.
			name:     "match after a mismatch is stale",
			warns:    Warns{NumWarns: 3, Reasons: StringArray{"a", "x", "c"}},
			stored:   []WarnEntry{a, b, issued("c", 3)},
			want:     []WarnEntry{a, legacy("x"), legacy("c")},
			wantKept: 1,
		},
		{
			name:     "old release appended only",
			warns:    Warns{NumWarns: 2, Reasons: StringArray{"a", "b"}},
			stored:   []WarnEntry{a},
			want:     []WarnEntry{a, legacy("b")},
			wantKept: 1,
		},
		{
			name:     "restored by an older release",
			warns:    Warns{NumWarns: 2, Reasons: StringArray{"old one", "old two"}},
			want:     []WarnEntry{legacy("old one"), legacy("old two")},
			wantKept: 0,
		},
		{
			name:     "no warns",
			warns:    Warns{NumWarns: 0, Reasons: StringArray{"a"}},
			stored:   []WarnEntry{a},
			want:     nil,
			wantKept: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.warns.UserId, tt.warns.ChatId = 5, -9
			got, kept := ReconcileWarnEntries(&tt.warns, tt.stored)
			checkEntries(t, got, tt.want)
			if kept != tt.wantKept {
				t.Fatalf("kept = %d, want %d", kept, tt.wantKept)
			}
		})
	}
}
