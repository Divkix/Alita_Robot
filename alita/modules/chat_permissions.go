package modules

import "github.com/PaulSonOfLars/gotgbot/v2"

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
	CanManageTopics:       false,
	CanSendPolls:          false,
	CanSendOtherMessages:  false,
}

// defaultUnmutePermissions represents a safe fallback when chat defaults are unavailable.
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
		CanManageTopics:       false,
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
	}
}

// resolveUnmutePermissions uses chat default permissions when available.
func resolveUnmutePermissions(chatInfo *gotgbot.ChatFullInfo) gotgbot.ChatPermissions {
	if chatInfo != nil && chatInfo.Permissions != nil {
		return *chatInfo.Permissions
	}
	return defaultUnmutePermissions()
}
