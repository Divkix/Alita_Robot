// Package probation stores each chat's new-member probation settings and the
// members on probation. The message count of a member lives in Redis
// (alita/modules/probation.go), never here, so a message writes nothing to the
// database.
package probation

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const (
	settingsKeyPrefix = "probation_settings"
	// MemberKeyPrefix is the cache prefix of a member's row. alita/db/cache
	// keeps it out of the in-process layer: a member's first message often
	// follows the join within a second, on any replica.
	MemberKeyPrefix = "probation_member"
)

// ErrOutOfRange is returned for a setting outside its limits in models.
var ErrOutOfRange = errors.New("probation: value out of range")

func settingsKey(chatID int64) string {
	return cache.CacheKey(settingsKeyPrefix, chatID)
}

func memberKey(chatID, userID int64) string {
	return cache.CacheKey(MemberKeyPrefix, chatID, userID)
}

// GetSettings returns the chat's settings; a missing row reads as
// models.DefaultProbationSettings (probation off).
func GetSettings(ctx context.Context, chatID int64) (*models.ProbationSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, settingsKey(chatID), cache.CacheTTLProbation,
		func(ctx context.Context) (*models.ProbationSettings, error) {
			settings := &models.ProbationSettings{}
			err := db.GetRecordContext(ctx, settings, models.ProbationSettings{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return models.DefaultProbationSettings(chatID), nil
			}
			if err != nil {
				return nil, err
			}
			return settings, nil
		})
}

// SetEnabled turns probation on or off. Turning it off keeps the members on
// probation: they are checked again only if it is turned back on within their
// probation time.
func SetEnabled(chatID int64, enabled bool) error {
	return upsert(chatID, map[string]any{"enabled": enabled})
}

// SetDuration sets how long a member stays on probation after joining.
func SetDuration(chatID int64, d time.Duration) error {
	if d < models.ProbationDurationMin || d > models.ProbationDurationMax {
		return ErrOutOfRange
	}
	return upsert(chatID, map[string]any{"duration_seconds": int(d / time.Second)})
}

// SetMessageCount sets how many counted messages end probation.
func SetMessageCount(chatID int64, n int) error {
	if n < models.ProbationMessagesMin || n > models.ProbationMessagesMax {
		return ErrOutOfRange
	}
	return upsert(chatID, map[string]any{"message_count": n})
}

// SetMinLength sets how many characters a message needs to count.
func SetMinLength(chatID int64, n int) error {
	if n < models.ProbationMinLengthMin || n > models.ProbationMinLengthMax {
		return ErrOutOfRange
	}
	return upsert(chatID, map[string]any{"min_length": n})
}

// SetBlocked sets what a member on probation may not post.
func SetBlocked(chatID int64, links, forwards, media bool) error {
	return upsert(chatID, map[string]any{"block_links": links, "block_forwards": forwards, "block_media": media})
}

// Block kinds, as /probation block and SetBlock take them.
const (
	BlockLinks    = "links"
	BlockForwards = "forwards"
	BlockMedia    = "media"
)

// ErrInvalidBlock is returned for a block kind other than links, forwards or media.
var ErrInvalidBlock = errors.New("probation: invalid block kind")

// SetBlock blocks or allows one kind, keeping the other two.
func SetBlock(chatID int64, kind string, on bool) error {
	switch kind {
	case BlockLinks, BlockForwards, BlockMedia:
		return upsert(chatID, map[string]any{"block_" + kind: on})
	}
	return ErrInvalidBlock
}

// SetNotice turns the one notice per chat a member gets on a deletion on or off.
func SetNotice(chatID int64, on bool) error {
	return upsert(chatID, map[string]any{"notice": on})
}

// upsert inserts row, or on an existing row updates only its columns, so the
// settings never overwrite each other. row is a map because GORM swaps a
// struct's false value for the column default on insert, which would turn
// block_links=false into true.
func upsert(chatID int64, row map[string]any) error {
	columns := make([]string, 0, len(row)+1)
	for column := range row {
		columns = append(columns, column)
	}
	columns = append(columns, "updated_at")
	now := time.Now()
	row["chat_id"] = chatID
	row["created_at"] = now
	row["updated_at"] = now
	err := db.DB.Model(&models.ProbationSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(columns),
	}).Create(row).Error
	if err != nil {
		log.Errorf("[Database] probation settings %v: %v - %d", columns, err, chatID)
		return err
	}
	cache.DeleteCache(settingsKey(chatID))
	return nil
}

// Member is a member's probation as the cache keeps it. Found is false for a
// user without a row, so a member not on probation is cached too.
type Member struct {
	Found    bool
	JoinedAt time.Time
	// Notified is true once the member got the notice for this probation.
	Notified bool
}

// GetMember returns userID's probation in chatID.
func GetMember(ctx context.Context, chatID, userID int64) (Member, error) {
	return cache.GetFromCacheOrLoad(ctx, memberKey(chatID, userID), cache.CacheTTLProbationMember,
		func(ctx context.Context) (Member, error) {
			var row models.ProbationMember
			err := db.DB.WithContext(ctx).Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&row).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return Member{}, nil
			}
			if err != nil {
				return Member{}, fmt.Errorf("get probation member: %w", err)
			}
			return Member{Found: true, JoinedAt: row.JoinedAt, Notified: row.Notified}, nil
		})
}

// Start puts userID on probation in chatID from joinedAt. A member who joins
// again starts again, with the notice not yet sent.
func Start(chatID, userID int64, joinedAt time.Time) error {
	err := db.DB.Model(&models.ProbationMember{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"joined_at", "notified"}),
	}).Create(map[string]any{
		"chat_id": chatID, "user_id": userID, "joined_at": joinedAt.UTC(), "notified": false,
	}).Error
	if err != nil {
		return fmt.Errorf("start probation: %w", err)
	}
	cache.DeleteCache(memberKey(chatID, userID))
	return nil
}

// End takes userID off probation in chatID; a user without a row is fine.
func End(chatID, userID int64) error {
	if err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.ProbationMember{}).Error; err != nil {
		return fmt.Errorf("end probation: %w", err)
	}
	cache.DeleteCache(memberKey(chatID, userID))
	return nil
}

// ClaimNotice marks that userID got the notice in chatID. It returns true
// only to the first caller for a probation, so the notice goes out once
// across replicas.
func ClaimNotice(chatID, userID int64) (bool, error) {
	res := db.DB.Model(&models.ProbationMember{}).
		Where("chat_id = ? AND user_id = ? AND notified = ?", chatID, userID, false).
		Update("notified", true)
	if res.Error != nil {
		return false, fmt.Errorf("claim probation notice: %w", res.Error)
	}
	if res.RowsAffected != 1 {
		return false, nil
	}
	cache.DeleteCache(memberKey(chatID, userID))
	return true, nil
}

// DeleteJoinedBefore removes the members who joined before cutoff and returns
// how many it removed. Their cached rows are left to expire: a cutoff older
// than the longest probation means a cached row has run out of time anyway.
func DeleteJoinedBefore(cutoff time.Time) (int64, error) {
	res := db.DB.Where("joined_at < ?", cutoff.UTC()).Delete(&models.ProbationMember{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete expired probation members: %w", res.Error)
	}
	return res.RowsAffected, nil
}
