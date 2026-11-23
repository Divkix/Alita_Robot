package modules

import (
	"github.com/divkix/Alita_Robot/alita/utils/opstate"
)

func newOverwriteToken() (string, error) {
	return opstate.NewToken()
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
