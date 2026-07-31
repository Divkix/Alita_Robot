package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestDisableSettingsDoesNotMapChatLevelDeleteSetting(t *testing.T) {
	modelType := reflect.TypeOf(DisableSettings{})
	for i := range modelType.NumField() {
		field := modelType.Field(i)
		if strings.Contains(field.Tag.Get("gorm"), "column:delete_commands") {
			t.Fatalf("DisableSettings.%s maps delete_commands; that column belongs to DisableChatSettings", field.Name)
		}
	}
}
