package callbackcodec

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	Version = "v1"
	// MaxCallbackDataLen matches Telegram's callback_data limit.
	MaxCallbackDataLen = 64
)

var (
	ErrInvalidFormat      = errors.New("invalid callback format")
	ErrUnsupportedVersion = errors.New("unsupported callback version")
	ErrInvalidNamespace   = errors.New("invalid callback namespace")
	ErrDataTooLong        = errors.New("callback data exceeds max length")
)

type Decoded struct {
	Namespace string
	Fields    map[string]string
}

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

func EncodeOrFallback(namespace string, fields map[string]string, fallback string) string {
	data, err := Encode(namespace, fields)
	if err != nil {
		return fallback
	}
	return data
}

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

func (d *Decoded) Field(key string) (string, bool) {
	if d == nil {
		return "", false
	}
	v, ok := d.Fields[key]
	return v, ok
}
