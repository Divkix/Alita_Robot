package callbackcodec

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	// Version identifies the current callback payload format.
	Version = "v1"
	// MaxCallbackDataLen matches Telegram's callback_data limit.
	MaxCallbackDataLen = 64
)

var (
	// ErrInvalidFormat indicates callback data is not in codec format.
	ErrInvalidFormat = errors.New("invalid callback format")
	// ErrUnsupportedVersion indicates the callback version is not supported.
	ErrUnsupportedVersion = errors.New("unsupported callback version")
	// ErrInvalidNamespace indicates callback namespace is invalid.
	ErrInvalidNamespace = errors.New("invalid callback namespace")
	// ErrDataTooLong indicates encoded callback data exceeds Telegram limits.
	ErrDataTooLong = errors.New("callback data exceeds max length")
)

// Decoded represents a decoded callback payload.
type Decoded struct {
	Namespace string
	Fields    map[string]string
}

// Encode serializes callback payload fields into a compact versioned format.
// Format: <namespace>|v1|<query-encoded fields>
func Encode(namespace string, fields map[string]string) (string, error) {
	if namespace == "" || strings.Contains(namespace, "|") {
		return "", ErrInvalidNamespace
	}

	values := url.Values{}
	for k, v := range fields {
		if k == "" {
			continue
		}
		values.Set(k, v)
	}

	payload := values.Encode()
	if payload == "" {
		payload = "_"
	}

	data := fmt.Sprintf("%s|%s|%s", namespace, Version, payload)
	if len(data) > MaxCallbackDataLen {
		return "", fmt.Errorf("%w: %d > %d", ErrDataTooLong, len(data), MaxCallbackDataLen)
	}
	return data, nil
}

// EncodeOrFallback returns encoded data, or fallback when encoding fails.
func EncodeOrFallback(namespace string, fields map[string]string, fallback string) string {
	data, err := Encode(namespace, fields)
	if err != nil {
		return fallback
	}
	return data
}

// Decode parses callback data in codec format.
func Decode(data string) (*Decoded, error) {
	parts := strings.SplitN(data, "|", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidFormat
	}

	namespace := parts[0]
	version := parts[1]
	rawPayload := parts[2]

	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	if version != Version {
		return nil, ErrUnsupportedVersion
	}

	fields := make(map[string]string)
	if rawPayload != "_" && rawPayload != "" {
		values, err := url.ParseQuery(rawPayload)
		if err != nil {
			return nil, ErrInvalidFormat
		}
		for k, v := range values {
			if len(v) == 0 {
				fields[k] = ""
				continue
			}
			fields[k] = v[0]
		}
	}

	return &Decoded{
		Namespace: namespace,
		Fields:    fields,
	}, nil
}

// Field returns a decoded field value and whether it exists.
func (d *Decoded) Field(key string) (string, bool) {
	if d == nil {
		return "", false
	}
	v, ok := d.Fields[key]
	return v, ok
}
