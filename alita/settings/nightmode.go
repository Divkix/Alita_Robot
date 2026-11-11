package settings

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/nightmode"
)

var nightModeLockTypes []string

// AttachNightModeLockTypes registers the locks owner's supported types at startup.
// The owner excludes join-only locks. Nil or an empty list leaves writes unbound.
func AttachNightModeLockTypes(types []string) {
	nightModeLockTypes = slices.Clone(types)
	slices.Sort(nightModeLockTypes)
	nightModeLockTypes = slices.Compact(nightModeLockTypes)
}

// NightModeInputError preserves the offending input for localized adapters.
// RefusalKey still identifies the existing locale key through Unwrap.
type NightModeInputError struct {
	Refusal  Refusal
	Input    string
	LockType string
}

func (e *NightModeInputError) Error() string { return e.Refusal.Error() }
func (e *NightModeInputError) Unwrap() error { return e.Refusal }

var NightModeEnabled, NightModeStart, NightModeEnd, NightModeLockTypes = nightModeDeclarations()

func nightModeAttachment() error {
	if len(nightModeLockTypes) == 0 {
		return fmt.Errorf("night mode lock types are not attached")
	}
	return nil
}

func parseNightLockTypes(value string) ([]string, error) {
	if err := nightModeAttachment(); err != nil {
		return nil, err
	}
	types := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	if len(types) == 0 {
		return nil, Refusal("nightmode_no_types")
	}
	for i, field := range types {
		lockType := strings.ToLower(field)
		if !slices.Contains(nightModeLockTypes, lockType) {
			return nil, &NightModeInputError{Refusal: Refusal("nightmode_invalid_type"), LockType: field}
		}
		types[i] = lockType
	}
	slices.Sort(types)
	return slices.Compact(types), nil
}

func nightClock(minute int) string {
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

func nightModeDeclarations() (*Declaration, *Declaration, *Declaration, *Declaration) {
	enabled := &Declaration{Key: "nightmode.enabled", Type: Switch, Default: false, Right: AdminBot}
	start := &Declaration{Key: "nightmode.start", Type: Text, Default: "00:00", Right: AdminBot}
	end := &Declaration{Key: "nightmode.end", Type: Text, Default: "00:00", Right: AdminBot}
	types := &Declaration{Key: "nightmode.lock_types", Type: Text, Default: "", Right: AdminBot}
	clockAllowed := func(value any) error {
		if _, ok := nightmode.ParseClock(value.(string)); !ok {
			return &NightModeInputError{Refusal: Refusal("nightmode_invalid_time"), Input: value.(string)}
		}
		return nil
	}
	start.allowed, end.allowed = clockAllowed, clockAllowed
	types.allowed = func(value any) error {
		_, err := parseNightLockTypes(value.(string))
		return err
	}
	read := func(chatID int64) (*models.NightModeSettings, error) {
		return readNightMode(chatID, enabled, start, end, types)
	}
	enabled.read = func(chatID int64) any {
		row, err := read(chatID)
		if err != nil {
			return nil
		}
		return row.Enabled
	}
	start.read = func(chatID int64) any {
		row, err := read(chatID)
		if err != nil {
			return nil
		}
		return nightClock(row.StartMinute)
	}
	end.read = func(chatID int64) any {
		row, err := read(chatID)
		if err != nil {
			return nil
		}
		return nightClock(row.EndMinute)
	}
	types.read = func(chatID int64) any {
		row, err := read(chatID)
		if err != nil {
			return nil
		}
		return row.LockTypes
	}
	resolve := func(input WriteContext, changes map[string]any) (*models.NightModeSettings, []string, error) {
		row, err := read(input.ChatID)
		if err != nil {
			return nil, nil, err
		}
		if value, changed := changes[enabled.Key]; changed {
			row.Enabled = value.(bool)
		}
		if !row.Enabled {
			for _, field := range []*Declaration{start, end, types} {
				if _, changed := changes[field.Key]; changed {
					return nil, nil, fmt.Errorf("night mode must be enabled to change its window or lock types")
				}
			}
			return row, nil, nil
		}
		if value, changed := changes[start.Key]; changed {
			row.StartMinute, _ = nightmode.ParseClock(value.(string))
		}
		if value, changed := changes[end.Key]; changed {
			row.EndMinute, _ = nightmode.ParseClock(value.(string))
		}
		if row.StartMinute == row.EndMinute {
			return nil, nil, Refusal("nightmode_same_times")
		}
		lockTypes := row.LockTypes
		if value, changed := changes[types.Key]; changed {
			lockTypes = value.(string)
		}
		selected, err := parseNightLockTypes(lockTypes)
		return row, selected, err
	}
	group := &sharedWrite{
		fields: []*Declaration{enabled, start, end, types},
		validate: func(input WriteContext, changes map[string]any) error {
			_, _, err := resolve(input, changes)
			return err
		},
		write: func(input WriteContext, changes map[string]any) error {
			row, selected, err := resolve(input, changes)
			if err != nil {
				return err
			}
			if !row.Enabled {
				return nightmode.Disable(input.ChatID)
			}
			return nightmode.Enable(input.ChatID, row.StartMinute, row.EndMinute, selected)
		},
	}
	for _, field := range group.fields {
		field.shared = group
		field.attachment = nightModeAttachment
	}
	return enabled, start, end, types
}

// ReadNightMode returns a normalized copy without repairing cached or stored data.
// Invalid fields use declaration defaults; an invalid window or types disables
// the effective configuration, so corrupt data cannot activate moderation.
func ReadNightMode(chatID int64) (*models.NightModeSettings, error) {
	return readNightMode(chatID, NightModeEnabled, NightModeStart, NightModeEnd, NightModeLockTypes)
}

func readNightMode(chatID int64, enabled, start, end, types *Declaration) (*models.NightModeSettings, error) {
	stored, err := nightmode.Get(chatID)
	if err != nil {
		return nil, err
	}
	row := *stored
	if row.StartMinute < 0 || row.StartMinute >= nightmode.MinutesPerDay {
		row.StartMinute, _ = nightmode.ParseClock(start.Default.(string))
		row.Enabled = enabled.Default.(bool)
	}
	if row.EndMinute < 0 || row.EndMinute >= nightmode.MinutesPerDay {
		row.EndMinute, _ = nightmode.ParseClock(end.Default.(string))
		row.Enabled = enabled.Default.(bool)
	}
	selected, err := parseNightLockTypes(row.LockTypes)
	if err != nil {
		row.LockTypes = types.Default.(string)
		row.Enabled = enabled.Default.(bool)
	} else {
		row.LockTypes = strings.Join(selected, ",")
	}
	if row.StartMinute == row.EndMinute || row.LockTypes == "" {
		row.Enabled = enabled.Default.(bool)
	}
	return &row, nil
}
