package db

import (
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// EnsureChannelInDb (via UpdateChannel)
// ---------------------------------------------------------------------------

func TestEnsureChannelInDb(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano() % 1_000_000_000_000)
	if channelID > 0 {
		channelID = -channelID
	}
	channelName := fmt.Sprintf("channel-%d", channelID)
	username := fmt.Sprintf("chan_%d", -channelID)

	err := UpdateChannel(channelID, channelName, username)
	if err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("chat_id = ?", channelID).Delete(&ChannelSettings{}) })

	var ch ChannelSettings
	if err := DB.Where("chat_id = ?", channelID).First(&ch).Error; err != nil {
		t.Fatalf("expected channel %d to exist: %v", channelID, err)
	}
	if ch.ChannelName != channelName {
		t.Errorf("channel name = %q, want %q", ch.ChannelName, channelName)
	}
}

// ---------------------------------------------------------------------------
// GetChannelIdByUserName
// ---------------------------------------------------------------------------

func TestGetChannelIdByUserName(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano()%1_000_000_000_000 + 2000)
	if channelID > 0 {
		channelID = -channelID
	}
	username := fmt.Sprintf("chanun_%d", -channelID)

	if err := UpdateChannel(channelID, "test-channel", username); err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("chat_id = ?", channelID).Delete(&ChannelSettings{}) })

	gotID := GetChannelIdByUserName(username)
	if gotID != channelID {
		t.Errorf("GetChannelIdByUserName(%q) = %d, want %d", username, gotID, channelID)
	}
}

func TestGetChannelIdByUserName_NotFound(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	gotID := GetChannelIdByUserName("nonexistent_channel_xyzabc999")
	if gotID != 0 {
		t.Errorf("GetChannelIdByUserName() = %d for non-existent channel, want 0", gotID)
	}
}

func TestGetChannelIdByUserName_Empty(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	gotID := GetChannelIdByUserName("")
	if gotID != 0 {
		t.Errorf("GetChannelIdByUserName(\"\") = %d, want 0", gotID)
	}
}

// ---------------------------------------------------------------------------
// GetChannelInfoById
// ---------------------------------------------------------------------------

func TestGetChannelInfoById(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano()%1_000_000_000_000 + 3000)
	if channelID > 0 {
		channelID = -channelID
	}
	channelName := fmt.Sprintf("info-channel-%d", channelID)
	username := fmt.Sprintf("infochan_%d", -channelID)

	if err := UpdateChannel(channelID, channelName, username); err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("chat_id = ?", channelID).Delete(&ChannelSettings{}) })

	gotUsername, gotName, found := GetChannelInfoById(channelID)
	if !found {
		t.Fatalf("GetChannelInfoById(%d) found=false, want true", channelID)
	}
	if gotUsername != username {
		t.Errorf("username = %q, want %q", gotUsername, username)
	}
	if gotName != channelName {
		t.Errorf("name = %q, want %q", gotName, channelName)
	}
}

func TestGetChannelInfoById_NotFound(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	_, _, found := GetChannelInfoById(-9999999999999)
	if found {
		t.Error("GetChannelInfoById() found=true for non-existent channel, want false")
	}
}

// ---------------------------------------------------------------------------
// UpdateChannel
// ---------------------------------------------------------------------------

func TestUpdateChannel(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano()%1_000_000_000_000 + 4000)
	if channelID > 0 {
		channelID = -channelID
	}
	username := fmt.Sprintf("updatechan_%d", -channelID)

	if err := UpdateChannel(channelID, "original-name", username); err != nil {
		t.Fatalf("initial UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { DB.Where("chat_id = ?", channelID).Delete(&ChannelSettings{}) })

	// Update the channel name
	updatedName := "updated-name"
	if err := UpdateChannel(channelID, updatedName, username); err != nil {
		t.Fatalf("UpdateChannel() update error = %v", err)
	}

	var ch ChannelSettings
	if err := DB.Where("chat_id = ?", channelID).First(&ch).Error; err != nil {
		t.Fatalf("expected channel to exist: %v", err)
	}
	if ch.ChannelName != updatedName {
		t.Errorf("channel name = %q, want %q", ch.ChannelName, updatedName)
	}
}
