// Package settings declares chat settings shared by commands, /settings and the
// panel. A Declaration owns the default, allowed values, preconditions, required
// right, non-creating read and write (including any side effect). Adapters parse
// and render; they enforce Right using their own permission responders, validate
// the whole batch before writing, and never call a declared setting's repository
// setter. Bot action paths also use ReadContext so invalid stored values use Default.
//
// This package may import repositories and config, never modules, panel, helpers
// or chat_status. Module-owned effects attach here at startup; a missing required
// attachment must refuse a write before storing anything. Reads check allowed
// values, not write preconditions, and never repair or create a database row.
// Switch declarations also supply the input-free on/off button cycle. Adapters
// map neutral rights independently; panel batches require their union, never a
// strongest-right approximation.
// A sharedWrite owns dependent fields' validation and single persistence call;
// adapters validate the whole request, then reuse one outcome per WriteKey.
// Operator preconditions return Refusal locale keys for each adapter to render.
// They participate in ValidateChanges before any batch write. Related independent
// switches validate their combined state, then enabled switches write first.
package settings

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// Refusal identifies an existing locale key without coupling declarations to a
// renderer. Commands translate it in their language; the panel uses plain English.
type Refusal string

func (r Refusal) Error() string { return string(r) }

func RefusalKey(err error) string {
	var refusal Refusal
	if errors.As(err, &refusal) {
		return string(refusal)
	}
	return ""
}

type Type string

const (
	Switch   Type = "switch"
	Number   Type = "number"
	Duration Type = "duration" // integer seconds
	Choice   Type = "choice"
	Text     Type = "text"
)

// Right describes policy without coupling it to a surface's reply machinery.
type Right uint8

const (
	Admin           Right = iota
	AdminBot              // caller and bot must both be administrators
	ChangeInfo            // caller is an admin with can_change_info
	ChangeInfoBot         // ChangeInfo, with the bot an administrator
	Restrict              // caller and bot administrators with can_restrict_members
	RestrictUser          // caller can_restrict_members; bot must be an administrator
	OwnerBot              // only the owner may write; bot must be an administrator
	FederationAdmin       // caller is an admin of a chat that belongs to a federation
)

type Declaration struct {
	Key     string
	Type    Type
	Default any
	Right   Right
	Options []string
	// FailureKey optionally supplies a locale key for command/button write failures.
	FailureKey   func(any) string
	toggleValues []string
	shared       *sharedWrite
	unchanged    func(any) bool
	attachment   func() error
	read         func(context.Context, int64) any
	allowed      func(any) error
	precondition func(WriteContext, any) error
	write        func(WriteContext, any) error
}

// WriteContext carries the request context, chat title needed by captcha, and
// bot used by attached effects. Adapters pass their update or request context.
type WriteContext struct {
	Context context.Context
	ChatID  int64
	Title   string
	Bot     *gotgbot.Bot
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
	CaptchaEnabled, CaptchaJoinRequests, CaptchaMode, CaptchaTimeout, CaptchaAction, CaptchaMaxAttempts,
	FloodLimit, FloodDelete, WarnLimit, WarnExpiry,
	FloodMode, FloodActionTime, WarnMode, WarnActionTime,
	ProbationEnabled, ProbationTime, ProbationMessages, ProbationMinLength,
	ProbationBlockLinks, ProbationBlockForwards, ProbationBlockMedia, ProbationNotice,
	AISpamEnabled, SpamDBEnabled, SpamDBAction, SpamDBCAS, SpamDBLols, SpamDBShared,
	ReportsEnabled, ReportMuteThreshold, DigestEnabled, DigestDestination,
	Timezone, Language, PrivateRules,
	NightModeEnabled, NightModeStart, NightModeEnd, NightModeLockTypes,
	PrivateNotes, AntiChannelPin, CleanLinked, AllowConnect, AnonAdmin,
	DeleteDisabledCommands, QuietFed, EphemeralReplies,
}

// All iterates every declared setting in registration order.
func All() iter.Seq[*Declaration] {
	return slices.Values(declarations)
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
	if d.toggleValues != nil {
		return d.toggleValues
	}
	if d.Type == Switch {
		return switchValues
	}
	if d.Type == Choice {
		return d.Options
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
	case Choice:
		text, ok := value.(string)
		if !ok || !slices.Contains(d.Options, text) {
			return fmt.Errorf("must be one of: %s", strings.Join(d.Options, ", "))
		}
	case Text:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be text")
		}
	default:
		return fmt.Errorf("unknown setting type %q", d.Type)
	}
	if d.allowed != nil {
		return d.allowed(value)
	}
	return nil
}

func (d *Declaration) ReadContext(ctx context.Context, chatID int64) State {
	value := d.effective(d.read(ctx, chatID))
	changed := value != d.Default
	if d.unchanged != nil {
		changed = !d.unchanged(value)
	}
	return State{Value: value, Changed: changed}
}

func (d *Declaration) effective(value any) any {
	if value == nil || d.Validate(value) != nil {
		return d.Default
	}
	return value
}

func storedValue[T any](load func(context.Context, int64) (*T, error), value func(*T) any) func(context.Context, int64) any {
	return func(ctx context.Context, chatID int64) any {
		row, err := load(ctx, chatID)
		if err != nil || row == nil {
			return nil
		}
		return value(row)
	}
}

// CheckAttachments fails startup when a module has not attached a required
// effect or allowed-values provider. Owners attach in init before serving writes.
func CheckAttachments() error {
	for _, declaration := range declarations {
		if declaration.attachment != nil {
			if err := declaration.attachment(); err != nil {
				return fmt.Errorf("%s: %w", declaration.Key, err)
			}
		}
	}
	return nil
}

// ValidateWrite includes write-only preconditions, so adapters can validate a
// whole batch before storing any field. Read checks only allowed values.
func (d *Declaration) ValidateWrite(input WriteContext, value any) error {
	if err := d.validateValueWrite(input, value); err != nil {
		return err
	}
	if d.shared != nil {
		return d.shared.validate(input, map[string]any{d.Key: value})
	}
	return nil
}

func (d *Declaration) validateValueWrite(input WriteContext, value any) error {
	if err := d.Validate(value); err != nil {
		return err
	}
	if d.attachment != nil {
		if err := d.attachment(); err != nil {
			return err
		}
	}
	if d.precondition != nil {
		return d.precondition(input, value)
	}
	return nil
}

// Write validates again so no adapter bypasses the rules or attached effect.
// Rights are enforced by the adapter before the first write in a batch.
func (d *Declaration) Write(input WriteContext, value any) error {
	if d.shared != nil && d.shared.write != nil {
		return d.WriteChanges(input, map[string]any{d.Key: value})
	}
	if err := d.ValidateWrite(input, value); err != nil {
		return err
	}
	return d.write(input, value)
}
