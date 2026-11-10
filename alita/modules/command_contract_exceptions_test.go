//go:build testtools

package modules

// One key per observed command name and failed clause. Do not exempt a whole
// command: another clause must still catch its regressions. The sweep reports
// the exact key to add for a failure, and fails when an entry stops failing.
// #1181-#1189 remove their entries as the remaining legacy handlers migrate.
var commandContractExceptions = map[string]string{
	"addreaction/proof-only":      "#1182: legacy change-info check refuses beside the proof",
	"addreaction/proof-replay":    "#1182: legacy handler is not registered through the pipeline",
	"removereaction/proof-only":   "#1182: legacy change-info check refuses beside the proof",
	"removereaction/proof-replay": "#1182: legacy handler is not registered through the pipeline",
	"resetreactions/proof-only":   "#1182: legacy change-info check refuses beside the proof",
	"resetreactions/proof-replay": "#1182: legacy handler is not registered through the pipeline",
	"addfilter/proof-only":        "#1183: legacy change-info check refuses beside the proof",
	"addfilter/proof-replay":      "#1183: legacy handler is not registered through the pipeline",
	"filter/proof-only":           "#1183: legacy change-info check refuses beside the proof",
	"filter/proof-replay":         "#1183: legacy handler is not registered through the pipeline",
	"removefilter/proof-only":     "#1183: legacy change-info check refuses beside the proof",
	"removefilter/proof-replay":   "#1183: legacy handler is not registered through the pipeline",
	"rmfilter/proof-only":         "#1183: legacy change-info check refuses beside the proof",
	"rmfilter/proof-replay":       "#1183: legacy handler is not registered through the pipeline",
	"stop/proof-only":             "#1183: legacy change-info check refuses beside the proof",
	"stop/proof-replay":           "#1183: legacy handler is not registered through the pipeline",
	"addnote/proof-only":          "#1183: legacy change-info check refuses beside the proof",
	"addnote/proof-replay":        "#1183: legacy handler is not registered through the pipeline",
	"save/proof-only":             "#1183: legacy change-info check refuses beside the proof",
	"save/proof-replay":           "#1183: legacy handler is not registered through the pipeline",
	"clear/proof-only":            "#1183: legacy change-info check refuses beside the proof",
	"clear/proof-replay":          "#1183: legacy handler is not registered through the pipeline",
	"rmnote/proof-only":           "#1183: legacy change-info check refuses beside the proof",
	"rmnote/proof-replay":         "#1183: legacy handler is not registered through the pipeline",
	"fbanstat/alias-member":       "#1188: legacy shared handler checks fedstat, not its fbanstat alias row",
	"saved/alias-member":          "#1183: legacy shared handler checks notes, not its saved alias row",
}
