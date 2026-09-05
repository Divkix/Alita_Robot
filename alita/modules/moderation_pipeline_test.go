//go:build testtools

package modules

import (
	"testing"
)

func TestMigratedDescriptorsHaveChecks(t *testing.T) {
	initBanDescs()
	initMuteDescs()
	initWarnDescs()
	initPurgeDescs()
	descs := map[string]int{
		"ban": len(banDesc.RequiredChecks), "sban": len(sbanDesc.RequiredChecks),
		"mute": len(muteDesc.RequiredChecks), "warn": len(warnDesc.RequiredChecks),
		"del": len(delDesc.RequiredChecks), "purge": len(purgeDesc.RequiredChecks),
		"resetwarns": len(resetWarnsDesc.RequiredChecks), "warns": len(warnsStatusDesc.RequiredChecks),
	}
	for name, n := range descs {
		if n == 0 {
			t.Fatalf("%s descriptor has no checks", name)
		}
	}
	if len(warnsStatusDesc.RequiredChecks) != 1 {
		t.Fatalf("warns status checks = %d, want 1 (disabled only)", len(warnsStatusDesc.RequiredChecks))
	}
}
