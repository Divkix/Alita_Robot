package modules

func parseFilterOverwriteCallbackData(data string) (action, token string, ok bool) {
	if decoded, decodedOK := decodeCallbackData(data, "filters_overwrite"); decodedOK {
		action, _ = decoded.Field("a")
		token, _ = decoded.Field("t")
		return action, token, action != ""
	}

	return "", "", false
}

func parseNoteOverwriteCallbackData(data string) (action, token string, ok bool) {
	if decoded, decodedOK := decodeCallbackData(data, "notes.overwrite"); decodedOK {
		action, _ = decoded.Field("a")
		token, _ = decoded.Field("t")
		return action, token, action != ""
	}

	return "", "", false
}
