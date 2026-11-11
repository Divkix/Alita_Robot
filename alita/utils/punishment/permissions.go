package punishment

import (
	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// MutedPermissions represents a fully restricted user - all sending capabilities disabled
var MutedPermissions = gotgbot.ChatPermissions{
	CanSendMessages:       false,
	CanSendPhotos:         false,
	CanSendVideos:         false,
	CanSendAudios:         false,
	CanSendDocuments:      false,
	CanSendVideoNotes:     false,
	CanSendVoiceNotes:     false,
	CanAddWebPagePreviews: false,
	CanChangeInfo:         false,
	CanInviteUsers:        false,
	CanPinMessages:        false,
	CanManageTopics:       helpers.Ptr(false),
	CanSendPolls:          false,
	CanSendOtherMessages:  false,
}

// defaultUnmutePermissions is the fallback when Telegram supplies no defaults.
func defaultUnmutePermissions() gotgbot.ChatPermissions {
	return gotgbot.ChatPermissions{
		CanSendMessages:       true,
		CanSendPhotos:         true,
		CanSendVideos:         true,
		CanSendAudios:         true,
		CanSendDocuments:      true,
		CanSendVideoNotes:     true,
		CanSendVoiceNotes:     true,
		CanAddWebPagePreviews: true,
		CanChangeInfo:         false,
		CanInviteUsers:        true,
		CanPinMessages:        false,
		CanManageTopics:       helpers.Ptr(false),
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
	}
}

// unmutePermissions restores the chat defaults verbatim when available.
func unmutePermissions(chatInfo *gotgbot.ChatFullInfo) gotgbot.ChatPermissions {
	if chatInfo != nil && chatInfo.Permissions != nil {
		return *chatInfo.Permissions
	}
	return defaultUnmutePermissions()
}
