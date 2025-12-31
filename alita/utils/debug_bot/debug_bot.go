package debug_bot

import (
	"encoding/json"
	"fmt"
)

// PrettyPrintStruct formats and prints a struct as indented JSON to stdout.
// Returns the formatted JSON string for further use or logging.
func PrettyPrintStruct(v any) string {
	prettyStruct, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		errMsg := fmt.Sprintf("[DebugBot] Failed to marshal struct: %v", err)
		fmt.Println(errMsg)
		return errMsg
	}
	jsonStruct := string(prettyStruct)
	fmt.Printf("%s\n\n", jsonStruct)
	return jsonStruct
}
