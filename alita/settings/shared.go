package settings

// sharedWrite owns a relationship and its single persistence operation. Every
// field in the group points at the same instance. Validation overlays requested
// values on a normalized read; adapters never resolve dependent fields themselves.
// write revalidates and stores the same resolved snapshot; it must not re-read
// dependent fields between validating their relationship and persisting them.
// A nil write keeps per-field persistence/outcomes while sharing batch validation;
// adapters must write enabled switches first before disabling another member.
type sharedWrite struct {
	fields   []*Declaration
	validate func(WriteContext, map[string]any) error
	write    func(WriteContext, map[string]any) error
}

// WriteKey identifies fields that must share one write and one outcome in a
// batch. Adapters cache the outcome, including nil, once per key per request.
func (d *Declaration) WriteKey() string {
	if d.shared != nil && d.shared.write != nil {
		return d.shared.fields[0].Key
	}
	return d.Key
}

// ValidateChanges checks declared fields and their relationships before any
// write. Undeclared keys remain the adapter's responsibility. A relationship
// failure is reported against every requested field in that shared write.
func ValidateChanges(input WriteContext, changes map[string]any) map[string]error {
	failures := make(map[string]error)
	checked := make(map[*sharedWrite]bool)
	for key, value := range changes {
		if d, ok := Lookup(key); ok {
			if err := d.validateValueWrite(input, value); err != nil {
				failures[key] = err
			}
		}
	}
	for key := range changes {
		d, ok := Lookup(key)
		if !ok || d.shared == nil || checked[d.shared] {
			continue
		}
		checked[d.shared] = true
		invalid := false
		for _, field := range d.shared.fields {
			invalid = invalid || failures[field.Key] != nil
		}
		if invalid {
			continue
		}
		if err := d.shared.validate(input, changes); err != nil {
			for _, field := range d.shared.fields {
				if _, changed := changes[field.Key]; changed {
					failures[field.Key] = err
				}
			}
		}
	}
	return failures
}

// WriteChanges writes this declaration or its shared group, using only the
// group's requested values. Call once per WriteKey after validating the whole
// request; every requested field in that group receives this same outcome.
func (d *Declaration) WriteChanges(input WriteContext, changes map[string]any) error {
	if d.shared == nil || d.shared.write == nil {
		return d.Write(input, changes[d.Key])
	}
	for _, field := range d.shared.fields {
		if value, changed := changes[field.Key]; changed {
			if err := field.validateValueWrite(input, value); err != nil {
				return err
			}
		}
	}
	return d.shared.write(input, changes)
}
