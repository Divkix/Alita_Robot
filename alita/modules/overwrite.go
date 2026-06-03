package modules

import (
	"sync"

	"github.com/divkix/Alita_Robot/alita/db"
)

// notesOverwriteMap is a package-level concurrent-safe map for note overwrites.
// This is separate from moduleStruct to avoid copylocks issues with value receivers.
var notesOverwriteMap sync.Map

// overwriteBase holds common fields for temporary state storage during command flows.
type overwriteBase struct {
	ChatID   int64
	ItemName string // filterWord or noteWord
	Text     string
	FileID   string
	Buttons  []db.Button
	DataType int
}

// struct for filters module
type overwriteFilter struct {
	overwriteBase
}

// struct for notes module
type overwriteNote struct {
	overwriteBase
	pvtOnly     bool
	grpOnly     bool
	adminOnly   bool
	webPrev     bool
	isProtected bool
	noNotif     bool
}
