package modules

import (
	"strings"

	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	log "github.com/sirupsen/logrus"
)

func encodeCallbackData(namespace string, fields map[string]string, fallback string) string {
	data, err := callbackcodec.Encode(namespace, fields)
	if err != nil {
		log.WithFields(log.Fields{
			"namespace": namespace,
			"error":     err,
		}).Warn("[CallbackCodec] Failed to encode callback data; using fallback")
		return fallback
	}
	return data
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
