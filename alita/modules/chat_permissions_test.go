package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestDefaultUnmutePermissions(t *testing.T) {
	t.Parallel()

	perms := defaultUnmutePermissions()

	if !perms.CanSendMessages {
		t.Errorf("CanSendMessages = false, want true")
	}
	if !perms.CanSendPhotos {
		t.Errorf("CanSendPhotos = false, want true")
	}
	if !perms.CanSendVideos {
		t.Errorf("CanSendVideos = false, want true")
	}
	if !perms.CanSendAudios {
		t.Errorf("CanSendAudios = false, want true")
	}
	if !perms.CanSendDocuments {
		t.Errorf("CanSendDocuments = false, want true")
	}
	if !perms.CanSendVideoNotes {
		t.Errorf("CanSendVideoNotes = false, want true")
	}
	if !perms.CanSendVoiceNotes {
		t.Errorf("CanSendVoiceNotes = false, want true")
	}
	if !perms.CanAddWebPagePreviews {
		t.Errorf("CanAddWebPagePreviews = false, want true")
	}
	if perms.CanChangeInfo {
		t.Errorf("CanChangeInfo = true, want false")
	}
	if !perms.CanInviteUsers {
		t.Errorf("CanInviteUsers = false, want true")
	}
	if perms.CanPinMessages {
		t.Errorf("CanPinMessages = true, want false")
	}
	if perms.CanManageTopics {
		t.Errorf("CanManageTopics = true, want false")
	}
	if !perms.CanSendPolls {
		t.Errorf("CanSendPolls = false, want true")
	}
	if !perms.CanSendOtherMessages {
		t.Errorf("CanSendOtherMessages = false, want true")
	}
}

func TestResolveUnmutePermissions(t *testing.T) {
	t.Parallel()

	defaults := defaultUnmutePermissions()

	tests := []struct {
		name     string
		input    *gotgbot.ChatFullInfo
		wantPerm gotgbot.ChatPermissions
	}{
		{
			name:     "nil input uses defaults",
			input:    nil,
			wantPerm: defaults,
		},
		{
			name:     "ChatFullInfo with nil Permissions uses defaults",
			input:    &gotgbot.ChatFullInfo{Permissions: nil},
			wantPerm: defaults,
		},
		{
			name: "ChatFullInfo with Permissions uses provided permissions",
			input: &gotgbot.ChatFullInfo{
				Permissions: &gotgbot.ChatPermissions{
					CanSendMessages: false,
				},
			},
			wantPerm: gotgbot.ChatPermissions{
				CanSendMessages: false,
			},
		},
		{
			name: "ChatFullInfo with CanPinMessages true uses provided permissions",
			input: &gotgbot.ChatFullInfo{
				Permissions: &gotgbot.ChatPermissions{
					CanSendMessages: true,
					CanPinMessages:  true,
				},
			},
			wantPerm: gotgbot.ChatPermissions{
				CanSendMessages: true,
				CanPinMessages:  true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveUnmutePermissions(tc.input)

			if got.CanSendMessages != tc.wantPerm.CanSendMessages {
				t.Errorf("CanSendMessages = %v, want %v", got.CanSendMessages, tc.wantPerm.CanSendMessages)
			}
			if got.CanSendPhotos != tc.wantPerm.CanSendPhotos {
				t.Errorf("CanSendPhotos = %v, want %v", got.CanSendPhotos, tc.wantPerm.CanSendPhotos)
			}
			if got.CanSendVideos != tc.wantPerm.CanSendVideos {
				t.Errorf("CanSendVideos = %v, want %v", got.CanSendVideos, tc.wantPerm.CanSendVideos)
			}
			if got.CanSendAudios != tc.wantPerm.CanSendAudios {
				t.Errorf("CanSendAudios = %v, want %v", got.CanSendAudios, tc.wantPerm.CanSendAudios)
			}
			if got.CanSendDocuments != tc.wantPerm.CanSendDocuments {
				t.Errorf("CanSendDocuments = %v, want %v", got.CanSendDocuments, tc.wantPerm.CanSendDocuments)
			}
			if got.CanSendVideoNotes != tc.wantPerm.CanSendVideoNotes {
				t.Errorf("CanSendVideoNotes = %v, want %v", got.CanSendVideoNotes, tc.wantPerm.CanSendVideoNotes)
			}
			if got.CanSendVoiceNotes != tc.wantPerm.CanSendVoiceNotes {
				t.Errorf("CanSendVoiceNotes = %v, want %v", got.CanSendVoiceNotes, tc.wantPerm.CanSendVoiceNotes)
			}
			if got.CanAddWebPagePreviews != tc.wantPerm.CanAddWebPagePreviews {
				t.Errorf("CanAddWebPagePreviews = %v, want %v", got.CanAddWebPagePreviews, tc.wantPerm.CanAddWebPagePreviews)
			}
			if got.CanChangeInfo != tc.wantPerm.CanChangeInfo {
				t.Errorf("CanChangeInfo = %v, want %v", got.CanChangeInfo, tc.wantPerm.CanChangeInfo)
			}
			if got.CanInviteUsers != tc.wantPerm.CanInviteUsers {
				t.Errorf("CanInviteUsers = %v, want %v", got.CanInviteUsers, tc.wantPerm.CanInviteUsers)
			}
			if got.CanPinMessages != tc.wantPerm.CanPinMessages {
				t.Errorf("CanPinMessages = %v, want %v", got.CanPinMessages, tc.wantPerm.CanPinMessages)
			}
			if got.CanManageTopics != tc.wantPerm.CanManageTopics {
				t.Errorf("CanManageTopics = %v, want %v", got.CanManageTopics, tc.wantPerm.CanManageTopics)
			}
			if got.CanSendPolls != tc.wantPerm.CanSendPolls {
				t.Errorf("CanSendPolls = %v, want %v", got.CanSendPolls, tc.wantPerm.CanSendPolls)
			}
			if got.CanSendOtherMessages != tc.wantPerm.CanSendOtherMessages {
				t.Errorf("CanSendOtherMessages = %v, want %v", got.CanSendOtherMessages, tc.wantPerm.CanSendOtherMessages)
			}
		})
	}
}
