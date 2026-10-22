//go:build testtools

package modules

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// checkName names a check by its function: a closure by the function that
// builds it, so helpers.CheckDisabled("x") names as helpers.CheckDisabled.
func checkName(check helpers.CheckFunc) string {
	return runtime.FuncForPC(reflect.ValueOf(check).Pointer()).Name()
}

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
	// Disabled, then the group from PM; no admin check, since any member may
	// run /warns.
	var got []string
	for _, check := range warnsStatusDesc.RequiredChecks {
		got = append(got, checkName(check))
	}
	want := []string{checkName(helpers.CheckDisabled("warns")), checkName(warnsChatCheck)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warns status checks = %v, want %v", got, want)
	}
}
