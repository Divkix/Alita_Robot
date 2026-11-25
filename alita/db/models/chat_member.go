package models

// ChatMember is one tracked membership edge between a chat and a user.
//
// It replaces the chats.users jsonb array: one row per (chat, user), so a join
// is an insert under the primary key instead of rewriting the whole array, and
// both directions are index scans. The migration owns the
// REFERENCES chats(chat_id) ON DELETE CASCADE; test AutoMigrate builds the same
// columns.
type ChatMember struct {
	ChatID int64 `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	UserID int64 `gorm:"column:user_id;primaryKey;autoIncrement:false" json:"user_id"`
}

func (ChatMember) TableName() string {
	return "chat_members"
}
