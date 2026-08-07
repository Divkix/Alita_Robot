package modules

import (
	"strings"

	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	log "github.com/sirupsen/logrus"
)

func encodeCallbackData(namespace string, fields map[string]string) string {
	data, err := callbackcodec.Encode(namespace, fields)
	if err != nil {
		log.WithFields(log.Fields{
			"namespace": namespace,
			"error":     err,
		}).Warn("[CallbackCodec] Failed to encode callback data - button will be dead")
		return ""
	}
	return data
}

// encodeCallbackDataChecked returns error when encoding fails so callers can abort the send.
//nolint:unused // kept for future callers that need to handle 64-byte overflow explicitly
func encodeCallbackDataChecked(namespace string, fields map[string]string) (string, error) {
	data, err := callbackcodec.Encode(namespace, fields)
	if err != nil {
		log.WithFields(log.Fields{
			"namespace": namespace,
			"error":     err,
		}).Warn("[CallbackCodec] Failed to encode callback data")
		return "", err
	}
	return data, nil
}

func mustCallbackData(namespace string, fields map[string]string) (string, bool) {
	data, err := callbackcodec.Encode(namespace, fields)
	if err != nil {
		log.WithFields(log.Fields{
			"namespace": namespace,
			"error":     err,
		}).Warn("[CallbackCodec] Failed to encode callback data")
		return "", false
	}
	return data, true
}

func decodeCallbackData(data string, expectedNamespaces ...string) (*callbackcodec.Decoded, bool) {
	decoded, err := callbackcodec.Decode(data)
	if err != nil {
		return nil, false
	}
	if len(expectedNamespaces) == 0 {
		return decoded, true
	}
	for _, expected := range expectedNamespaces {
		if strings.EqualFold(decoded.Namespace, expected) {
			return decoded, true
		}
	}
	return nil, false
}
