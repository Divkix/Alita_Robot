// Package settings declares chat settings shared by commands, /settings and the
// panel. A Declaration owns the default, allowed values, preconditions, required
// right, non-creating read and write (including any side effect). Adapters parse
// and render; they enforce Right using their own permission responders, validate
// the whole batch before writing, and never call a declared setting's repository
// setter. Bot action paths also use Read so invalid stored values use Default.
//
// This package may import repositories and config, never modules, panel, helpers
// or chat_status. Module-owned effects attach here at startup; a missing required
// attachment must refuse a write before storing anything. Read checks allowed
// values, not write preconditions, and never repairs or creates a database row.
// Switch declarations also supply the input-free on/off button cycle. Adapters
// map neutral rights independently; panel batches require their union, never a
// strongest-right approximation.
package settings

import "fmt"

type Type string

const (
	Switch   Type = "switch"
	Number   Type = "number"
	Duration Type = "duration" // integer seconds
)

// Right describes policy without coupling it to a surface's reply machinery.
type Right uint8

const (
	Admin         Right = iota
	AdminBot            // caller and bot must both be administrators
	ChangeInfo          // caller is an admin with can_change_info
	ChangeInfoBot       // ChangeInfo, with the bot an administrator
)

type Declaration struct {
	Key          string
	Type         Type
	Default      any
	Right        Right
	read         func(int64) any
	allowed      func(any) error
	precondition func(int64, any) error
	write        func(int64, any) error
}

// State is the effective value and whether it differs from the declaration's
// default. Values are scalar bools, ints or strings, never slices or maps.
type State struct {
	Value   any
	Changed bool
}

var declarations = []*Declaration{
	RaidTime, RaidActionTime, AutoAntiRaidThreshold,
	Welcome, Goodbye, CleanWelcome, CleanGoodbye, CleanService, AutoApprove,
}

// Lookup returns the declaration for a wire key such as antiraid.raid_time.
func Lookup(key string) (*Declaration, bool) {
	for _, declaration := range declarations {
		if declaration.Key == key {
			return declaration, true
		}
	}
	return nil, false
}

// ToggleValues lists the input-free values a /settings button can cycle through.
func (d *Declaration) ToggleValues() []string {
	if d.Type == Switch {
		return switchValues
	}
	return nil
}

var switchValues = []string{"off", "on"}

func (d *Declaration) Validate(value any) error {
	switch d.Type {
	case Switch:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case Number, Duration:
		if _, ok := value.(int); !ok {
			return fmt.Errorf("must be an integer")
		}
	default:
		return fmt.Errorf("unknown setting type %q", d.Type)
	}
	if d.allowed != nil {
		return d.allowed(value)
	}
	return nil
}

func (d *Declaration) Read(chatID int64) State {
	value := d.effective(d.read(chatID))
	return State{Value: value, Changed: value != d.Default}
}

func (d *Declaration) effective(value any) any {
	if value == nil || d.Validate(value) != nil {
		return d.Default
	}
	return value
}

// Write validates again so no adapter can bypass the setting's rules. Rights
// are enforced by the adapter, before the first write in a batch.
func (d *Declaration) Write(chatID int64, value any) error {
	if err := d.Validate(value); err != nil {
		return err
	}
	if d.precondition != nil {
		if err := d.precondition(chatID, value); err != nil {
			return err
		}
	}
	return d.write(chatID, value)
}
