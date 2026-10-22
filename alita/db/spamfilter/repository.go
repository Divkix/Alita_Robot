// Package spamfilter stores the texts admins labelled spam or ham and each
// chat's choice to match new messages against them. Normalising and comparing
// texts lives in alita/utils/spamtext; acting on a match lives in the
// SpamFilter module.
package spamfilter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/spamtext"
)

const (
	// MaxSamplesPerLabel caps a chat's spam samples, and its ham samples
	// apart; a new one past it drops the chat's oldest of that label.
	MaxSamplesPerLabel = 200
	// Retention is how long a sample is kept after it was labelled.
	Retention = 90 * 24 * time.Hour
	// DryRunPeriod is how long a chat that turned the filter on only logs
	// matches before it deletes them.
	DryRunPeriod = 7 * 24 * time.Hour
	// MinThreshold and MaxThreshold bound the similarity a chat may require.
	MinThreshold = 0.5
	MaxThreshold = 1.0

	// MaxSharedSamples caps the shared corpus, newest texts first, so a
	// message is compared with at most this many shared texts.
	MaxSharedSamples = 300
	// sharedCandidates bounds the texts flagged in two or more chats that the
	// shared corpus considers, and sharedRowsPerText the chats compared for
	// each.
	sharedCandidates  = 1000
	sharedRowsPerText = 20

	settingsCacheModule = "spam_filter_settings"
	samplesCacheModule  = "spam_samples"
	sharedCacheModule   = "spam_shared_corpus"
	// sharedCacheTTL is how long the shared corpus is served before it is
	// computed again; a write anywhere also clears it.
	sharedCacheTTL = 10 * time.Minute
)

var (
	// ErrInvalidMode is returned for a mode other than off, dryrun or on.
	ErrInvalidMode = errors.New("spamfilter: invalid mode")
	// ErrInvalidThreshold is returned for a threshold outside [0.5, 1].
	ErrInvalidThreshold = errors.New("spamfilter: invalid threshold")
	// ErrTooShort is returned for a text too short to learn from.
	ErrTooShort = errors.New("spamfilter: text too short to store")
)

// ValidMode reports whether mode is one SetMode accepts.
func ValidMode(mode string) bool {
	switch mode {
	case models.SpamFilterOff, models.SpamFilterDryRun, models.SpamFilterOn:
		return true
	}
	return false
}

// ValidThreshold reports whether threshold is one SetThreshold accepts.
func ValidThreshold(threshold float64) bool {
	return threshold >= MinThreshold && threshold <= MaxThreshold
}

// Hash is the content_hash of a normalised text.
func Hash(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func settingsKey(chatID int64) string { return cache.CacheKey(settingsCacheModule, chatID) }
func samplesKey(chatID int64) string  { return cache.CacheKey(samplesCacheModule, chatID) }
func sharedKey() string               { return cache.CacheKey(sharedCacheModule) }

// invalidateSamples clears the chat's samples and the shared corpus, which
// any chat's spam rows feed.
func invalidateSamples(chatID int64) {
	cache.DeleteCache(samplesKey(chatID))
	cache.DeleteCache(sharedKey())
}

//
// Settings
//

// GetSettings returns the chat's settings. A missing row reads as off at the
// default threshold.
func GetSettings(ctx context.Context, chatID int64) (*models.SpamFilterSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, settingsKey(chatID), cache.CacheTTLSpamFilter,
		func(ctx context.Context) (*models.SpamFilterSettings, error) {
			settings := &models.SpamFilterSettings{}
			err := db.GetRecordContext(ctx, settings, models.SpamFilterSettings{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &models.SpamFilterSettings{
					ChatID:    chatID,
					Mode:      models.SpamFilterOff,
					Threshold: models.DefaultSpamFilterThreshold,
				}, nil
			}
			if err != nil {
				return nil, err
			}
			return settings, nil
		})
}

// Acting reports whether the settings delete a match at now: the filter is
// on and its dry run, counted from EnabledAt, is over.
func Acting(settings *models.SpamFilterSettings, now time.Time) bool {
	if settings == nil || settings.Mode != models.SpamFilterOn || settings.EnabledAt == nil {
		return false
	}
	return !now.Before(DryRunEnds(settings))
}

// DryRunEnds is when a chat's dry run is over, or the zero time when the
// filter is off.
func DryRunEnds(settings *models.SpamFilterSettings) time.Time {
	if settings == nil || settings.EnabledAt == nil {
		return time.Time{}
	}
	return settings.EnabledAt.Add(DryRunPeriod)
}

// SetMode switches the chat's filter. Off clears EnabledAt; dryrun or on
// sets it to now only when the chat was off, so moving from dryrun to on
// keeps the dry run already done.
func SetMode(chatID int64, mode string) error {
	if !ValidMode(mode) {
		return ErrInvalidMode
	}
	now := time.Now().UTC()
	row := map[string]any{"mode": mode, "enabled_at": nil}
	enabledAt := clause.Set{{Column: clause.Column{Name: "enabled_at"}, Value: nil}}
	if mode != models.SpamFilterOff {
		row["enabled_at"] = now
		enabledAt = clause.Set{{
			Column: clause.Column{Name: "enabled_at"},
			Value:  gorm.Expr("COALESCE(spam_filter_settings.enabled_at, ?)", now),
		}}
	}
	updates := append(clause.AssignmentColumns([]string{"mode", "updated_at"}), enabledAt...)
	return upsertSettings(chatID, row, updates)
}

// SetThreshold sets the similarity a match needs, keeping the mode.
func SetThreshold(chatID int64, threshold float64) error {
	if !ValidThreshold(threshold) {
		return ErrInvalidThreshold
	}
	return upsertSettings(chatID, map[string]any{"threshold": threshold},
		clause.AssignmentColumns([]string{"threshold", "updated_at"}))
}

// upsertSettings inserts row with the defaults it lacks, or applies updates to
// the existing row. row is a map because GORM swaps a struct's zero value for
// the column default on insert.
func upsertSettings(chatID int64, row map[string]any, updates clause.Set) error {
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return fmt.Errorf("spam filter settings: %w", err)
	}
	now := time.Now().UTC()
	insert := map[string]any{
		"chat_id":    chatID,
		"mode":       models.SpamFilterOff,
		"threshold":  models.DefaultSpamFilterThreshold,
		"enabled_at": nil,
		"created_at": now,
		"updated_at": now,
	}
	for column, value := range row {
		insert[column] = value
	}
	err := db.DB.Model(&models.SpamFilterSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: updates,
	}).Create(insert).Error
	if err != nil {
		log.Errorf("[Database] spam filter settings: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(settingsKey(chatID))
	return nil
}

//
// Samples
//

// NewSample is a text to label. AdminIDs are the chat's human admins when it
// was labelled; OwnerID and AddedBy join them in the stored set.
type NewSample struct {
	ChatID   int64
	Label    string
	Text     string
	SenderID int64
	AddedBy  int64
	OwnerID  int64
	AdminIDs []int64
}

// Add stores s, replacing the chat's earlier label for the same text, and
// drops the chat's oldest samples of that label past MaxSamplesPerLabel. A
// text shorter than spamtext.MinRunes once normalised is not stored:
// ErrTooShort.
func Add(s NewSample) (*models.SpamSample, error) {
	if s.Label != models.SpamLabelSpam && s.Label != models.SpamLabelHam {
		return nil, fmt.Errorf("spamfilter: invalid label %q", s.Label)
	}
	normalized := spamtext.Normalize(s.Text)
	if spamtext.RuneCount(normalized) < spamtext.MinRunes {
		return nil, ErrTooShort
	}
	if err := chats.EnsureChatInDb(s.ChatID, ""); err != nil {
		return nil, fmt.Errorf("add spam sample: %w", err)
	}

	sample := &models.SpamSample{
		ChatID:      s.ChatID,
		Label:       s.Label,
		Text:        truncateRunes(s.Text, spamtext.MaxRunes),
		Normalized:  normalized,
		ContentHash: Hash(normalized),
		SenderID:    s.SenderID,
		AddedBy:     s.AddedBy,
		OwnerID:     s.OwnerID,
		AdminIDs:    adminSet(s.AdminIDs, s.OwnerID, s.AddedBy),
		CreatedAt:   time.Now().UTC(),
	}
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "chat_id"}, {Name: "content_hash"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"label", "text", "normalized", "sender_id", "added_by", "owner_id", "admin_ids", "created_at",
			}),
		}).Create(sample).Error; err != nil {
			return err
		}
		// The upsert's returned id is not reliable across drivers on the
		// update path: read the row back.
		if err := tx.Where("chat_id = ? AND content_hash = ?", sample.ChatID, sample.ContentHash).
			Take(sample).Error; err != nil {
			return err
		}
		return pruneLabel(tx, s.ChatID, s.Label)
	})
	if err != nil {
		return nil, fmt.Errorf("add spam sample: %w", err)
	}
	invalidateSamples(s.ChatID)
	return sample, nil
}

// pruneLabel deletes the chat's samples of label past the newest
// MaxSamplesPerLabel.
func pruneLabel(tx *gorm.DB, chatID int64, label string) error {
	keep := tx.Model(&models.SpamSample{}).Select("id").
		Where("chat_id = ? AND label = ?", chatID, label).
		Order("created_at DESC, id DESC").Limit(MaxSamplesPerLabel)
	return tx.Where("chat_id = ? AND label = ? AND id NOT IN (?)", chatID, label, keep).
		Delete(&models.SpamSample{}).Error
}

// adminSet renders the chat's admins, owner and labeller as the sorted,
// comma-separated admin_ids column.
func adminSet(admins []int64, extra ...int64) string {
	ids := make([]int64, 0, len(admins)+len(extra))
	for _, id := range append(slices.Clone(admins), extra...) {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func parseAdminSet(column string) map[int64]struct{} {
	set := make(map[int64]struct{})
	for part := range strings.SplitSeq(column, ",") {
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			set[id] = struct{}{}
		}
	}
	return set
}

// List returns the chat's samples of both labels inside retention, newest
// first. The admin columns are left out: only the shared corpus reads them.
func List(ctx context.Context, chatID int64) ([]models.SpamSample, error) {
	return cache.GetFromCacheOrLoad(ctx, samplesKey(chatID), cache.CacheTTLSpamFilter,
		func(ctx context.Context) ([]models.SpamSample, error) {
			var rows []models.SpamSample
			err := db.DB.WithContext(ctx).
				Select("id", "chat_id", "label", "text", "normalized", "sender_id", "added_by", "created_at").
				Where("chat_id = ? AND created_at >= ?", chatID, time.Now().UTC().Add(-Retention)).
				Order("created_at DESC, id DESC").
				Limit(2 * MaxSamplesPerLabel).
				Find(&rows).Error
			if err != nil {
				return nil, fmt.Errorf("list spam samples: %w", err)
			}
			return rows, nil
		})
}

// Get returns one sample, or gorm.ErrRecordNotFound.
func Get(id int64) (*models.SpamSample, error) {
	sample := &models.SpamSample{}
	if err := db.DB.Where("id = ?", id).Take(sample).Error; err != nil {
		return nil, err
	}
	return sample, nil
}

// Delete removes the chat's sample id, of either label, and reports whether
// there was one. Another chat's sample is never removed.
func Delete(chatID, id int64) (bool, error) {
	result := db.DB.Where("chat_id = ? AND id = ?", chatID, id).Delete(&models.SpamSample{})
	if result.Error != nil {
		return false, fmt.Errorf("delete spam sample: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	invalidateSamples(chatID)
	return true, nil
}

// MarkHam answers an admin's "not spam" on a match: the chat's sample
// sampleID, when it is the chat's own, is removed, and text is stored as the
// chat's ham. A shared sample from another chat is left alone; the chat's ham
// keeps it from matching there. A text too short to store still removes the
// sample.
func MarkHam(chatID, sampleID int64, text string, actor int64) error {
	if _, err := Delete(chatID, sampleID); err != nil {
		return err
	}
	_, err := Add(NewSample{ChatID: chatID, Label: models.SpamLabelHam, Text: text, AddedBy: actor})
	if errors.Is(err, ErrTooShort) {
		return nil
	}
	return err
}

// RelabelSenderAsHam turns the chat's spam samples taken from senderID's
// messages into ham, for an unban of a user /spam banned, and returns how many
// it turned.
func RelabelSenderAsHam(chatID, senderID, actor int64) (int64, error) {
	var turned int64
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.SpamSample{}).
			Where("chat_id = ? AND sender_id = ? AND label = ?", chatID, senderID, models.SpamLabelSpam).
			Updates(map[string]any{
				"label":      models.SpamLabelHam,
				"added_by":   actor,
				"created_at": time.Now().UTC(),
			})
		if result.Error != nil {
			return result.Error
		}
		turned = result.RowsAffected
		if turned == 0 {
			return nil
		}
		return pruneLabel(tx, chatID, models.SpamLabelHam)
	})
	if err != nil {
		return 0, fmt.Errorf("relabel spam samples as ham: %w", err)
	}
	if turned > 0 {
		invalidateSamples(chatID)
	}
	return turned, nil
}

// DeleteExpiredBefore removes samples labelled before cutoff and returns how
// many it removed, clearing the cache of every chat it touched.
func DeleteExpiredBefore(cutoff time.Time) (int64, error) {
	cutoff = cutoff.UTC()
	var chatIDs []int64
	if err := db.DB.Model(&models.SpamSample{}).Distinct("chat_id").
		Where("created_at < ?", cutoff).Pluck("chat_id", &chatIDs).Error; err != nil {
		return 0, fmt.Errorf("list expired spam samples: %w", err)
	}
	if len(chatIDs) == 0 {
		return 0, nil
	}
	result := db.DB.Where("created_at < ?", cutoff).Delete(&models.SpamSample{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete expired spam samples: %w", result.Error)
	}
	for _, chatID := range chatIDs {
		cache.DeleteCache(samplesKey(chatID))
	}
	cache.DeleteCache(sharedKey())
	return result.RowsAffected, nil
}

//
// Shared corpus
//

// SharedSample is a text in the shared corpus: the newest of the rows that
// put it there.
type SharedSample struct {
	ID         int64  `json:"id"`
	ChatID     int64  `json:"chat_id"`
	Normalized string `json:"normalized"`
}

// Shared returns the shared corpus: spam texts inside retention that two
// unrelated chats flagged, newest first, at most MaxSharedSamples. Two chats
// are unrelated when both owners are known and differ and no admin is in both
// admin sets; a single person running several chats therefore never puts a
// text in the corpus alone.
func Shared(ctx context.Context) ([]SharedSample, error) {
	return cache.GetFromCacheOrLoad(ctx, sharedKey(), sharedCacheTTL, loadShared)
}

func loadShared(ctx context.Context) ([]SharedSample, error) {
	cutoff := time.Now().UTC().Add(-Retention)
	var hashes []string
	err := db.DB.WithContext(ctx).Model(&models.SpamSample{}).
		Select("content_hash").
		Where("label = ? AND owner_id <> 0 AND created_at >= ?", models.SpamLabelSpam, cutoff).
		Group("content_hash").
		Having("COUNT(DISTINCT chat_id) >= 2").
		Order("MAX(created_at) DESC").
		Limit(sharedCandidates).
		Pluck("content_hash", &hashes).Error
	if err != nil {
		return nil, fmt.Errorf("list shared spam candidates: %w", err)
	}
	if len(hashes) == 0 {
		return []SharedSample{}, nil
	}

	var rows []models.SpamSample
	err = db.DB.WithContext(ctx).
		Where("label = ? AND owner_id <> 0 AND created_at >= ? AND content_hash IN ?", models.SpamLabelSpam, cutoff, hashes).
		Order("created_at DESC, id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load shared spam candidates: %w", err)
	}

	byHash := make(map[string][]models.SpamSample, len(hashes))
	for _, row := range rows {
		if len(byHash[row.ContentHash]) < sharedRowsPerText {
			byHash[row.ContentHash] = append(byHash[row.ContentHash], row)
		}
	}
	shared := make([]SharedSample, 0, min(len(hashes), MaxSharedSamples))
	for _, hash := range hashes {
		group := byHash[hash]
		if !hasUnrelatedPair(group) {
			continue
		}
		newest := group[0]
		shared = append(shared, SharedSample{ID: newest.ID, ChatID: newest.ChatID, Normalized: newest.Normalized})
		if len(shared) == MaxSharedSamples {
			break
		}
	}
	return shared, nil
}

// hasUnrelatedPair reports whether two rows of the same text come from
// unrelated chats.
func hasUnrelatedPair(rows []models.SpamSample) bool {
	admins := make([]map[int64]struct{}, len(rows))
	for i := range rows {
		admins[i] = parseAdminSet(rows[i].AdminIDs)
	}
	for i := range rows {
		for j := i + 1; j < len(rows); j++ {
			if unrelated(rows[i], rows[j], admins[i], admins[j]) {
				return true
			}
		}
	}
	return false
}

func unrelated(a, b models.SpamSample, adminsA, adminsB map[int64]struct{}) bool {
	if a.ChatID == b.ChatID || a.OwnerID == 0 || b.OwnerID == 0 || a.OwnerID == b.OwnerID {
		return false
	}
	for id := range adminsA {
		if _, shared := adminsB[id]; shared {
			return false
		}
	}
	return true
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}
