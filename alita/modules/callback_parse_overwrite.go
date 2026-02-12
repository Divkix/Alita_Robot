package modules

import "strings"

func parseFilterOverwriteCallbackData(data string) (action, token, legacyFilterWord string, ok bool) {
	if decoded, decodedOK := decodeCallbackData(data, "filters_overwrite"); decodedOK {
		action, _ = decoded.Field("a")
		token, _ = decoded.Field("t")
		return action, token, "", action != ""
	}

	const legacyPrefix = "filters_overwrite."
	if !strings.HasPrefix(data, legacyPrefix) {
		return "", "", "", false
	}

	legacyPayload := strings.TrimPrefix(data, legacyPrefix)
	if legacyPayload == "" {
		return "", "", "", false
	}
	if legacyPayload == "cancel" {
		return "cancel", "", "", true
	}
	return "yes", "", legacyPayload, true
}

func parseNoteOverwriteCallbackData(data string) (action, token, legacyMapKey string, ok bool) {
	if decoded, decodedOK := decodeCallbackData(data, "notes.overwrite"); decodedOK {
		action, _ = decoded.Field("a")
		token, _ = decoded.Field("t")
		return action, token, "", action != ""
	}

	const legacyPrefix = "notes.overwrite."
	if !strings.HasPrefix(data, legacyPrefix) {
		return "", "", "", false
	}

	legacyPayload := strings.TrimPrefix(data, legacyPrefix)
	parts := strings.SplitN(legacyPayload, ".", 2)
	if len(parts) != 2 {
		return "", "", "", false
	}

	action = parts[0]
	legacyMapKey = parts[1]
	return action, "", legacyMapKey, action != "" && legacyMapKey != ""
}
