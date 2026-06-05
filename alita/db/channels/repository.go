package channels

import (
	"context"
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetChannelSettings retrieves channel settings from cache or database.
// Returns nil if the channel is not found or an error occurs.
func GetChannelSettings(channelId int64) (channelSrc *models.ChannelSettings) {
	// Use optimized cached query instead of SELECT *
	channelSrc, err := GetChannelSettingsCached(channelId)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] GetChannelSettings: %v - %d", err, channelId)
		return nil
	}
	return channelSrc
}

// UpdateChannel updates or creates a channel record with full metadata.
// Stores channel name and username, and invalidates cache after updates.
// Returns error if database operation fails.
func UpdateChannel(channelId int64, channelName, username string) error {
	// Check if channel already exists
	channelSrc := GetChannelSettings(channelId)
	now := time.Now()

	if channelSrc != nil && channelSrc.ChatId != 0 {
		// Channel exists - check if update is needed
		needsUpdate := false
		updates := bson.M{}

		if channelSrc.ChannelName != channelName && channelName != "" {
			updates["channel_name"] = channelName
			needsUpdate = true
		}
		if channelSrc.Username != username && username != "" {
			updates["username"] = username
			needsUpdate = true
		}

		if needsUpdate {
			updates["updated_at"] = now
			collection := db.DB.Collection("channels")
			_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": channelId}, bson.M{"$set": updates})
			if err != nil {
				log.Errorf("[Database] UpdateChannel: failed to update %d: %v", channelId, err)
				return err
			}
			cache.DeleteCache(cache.CacheKey("channel", channelId))
			log.Debugf("[Database] UpdateChannel: updated channel %d", channelId)
		}
		return nil
	}

	// Create new channel with full metadata
	newChannel := &models.ChannelSettings{
		ChatId:      channelId,
		ChannelId:   channelId,
		ChannelName: channelName,
		Username:    username,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err := db.CreateRecord(newChannel)
	if err != nil {
		log.Errorf("[Database] UpdateChannel: failed to create %d (%s): %v", channelId, username, err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("channel", channelId))
	log.Infof("[Database] UpdateChannel: created channel %d (%s)", channelId, channelName)
	return nil
}

// GetChannelIdByUserName finds a channel ID by username.
// Returns 0 if the channel is not found or an error occurs.
func GetChannelIdByUserName(username string) int64 {
	if username == "" {
		return 0
	}

	var channel models.ChannelSettings
	collection := db.DB.Collection("channels")
	err := collection.FindOne(context.Background(), bson.M{"username": username}, options.FindOne().SetProjection(bson.M{"chat_id": 1})).Decode(&channel)

	if err != nil {
		if err != mongo.ErrNoDocuments {
			log.Errorf("[Database] GetChannelIdByUserName: %v - %s", err, username)
		}
		return 0
	}
	return channel.ChatId
}

// GetChannelInfoById retrieves channel information by channel ID.
// Returns username, name, and whether the channel was found.
func GetChannelInfoById(channelId int64) (username, name string, found bool) {
	channel := GetChannelSettings(channelId)
	if channel != nil && channel.ChatId != 0 {
		username = channel.Username
		name = channel.ChannelName
		found = true
	}
	return
}

// LoadChannelStats returns the total count of channel settings records.
func LoadChannelStats() (count int64) {
	collection := db.DB.Collection("channels")
	count, err := collection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		log.Errorf("[Database] loadChannelStats: %v", err)
		return 0
	}
	return
}
