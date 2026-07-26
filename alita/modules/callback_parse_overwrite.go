package modules

import (
	"crypto/rand"
	"encoding/hex"
)

func newOverwriteToken() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func parseOverwriteCallbackData(data, namespace string) (action, token string, ok bool) {
	decoded, ok := decodeCallbackData(data, namespace)
	if !ok {
		return "", "", false
	}

	action, _ = decoded.Field("a")
	token, _ = decoded.Field("t")
	return action, token, action != ""
}
