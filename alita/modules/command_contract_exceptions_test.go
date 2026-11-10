//go:build testtools

package modules

// One key per observed command name and failed clause. Do not exempt a whole
// command: another clause must still catch its regressions. The sweep reports
// the exact key to add for a failure, and fails when an entry stops failing.
// #1181-#1189 remove their entries as the remaining legacy handlers migrate.
var commandContractExceptions = map[string]string{
	"fbanstat/alias-member": "#1188: legacy shared handler checks fedstat, not its fbanstat alias row",
}
