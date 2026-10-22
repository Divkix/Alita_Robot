package models

// ChatDailyStat is one chat's activity counters for one UTC day.
//
// Day is a YYYY-MM-DD string computed in Go (stats.Day), never by the
// database, so PostgreSQL and the SQLite tests agree on the row key. The
// counters are only ever added to: a replica flush never writes a value it
// computed from zero.
type ChatDailyStat struct {
	ChatID   int64  `gorm:"column:chat_id;primaryKey" json:"chat_id"`
	Day      string `gorm:"column:day;primaryKey;size:10;index:idx_chat_daily_stats_day" json:"day"`
	Messages int64  `gorm:"column:messages;not null;default:0" json:"messages"`
	Joins    int64  `gorm:"column:joins;not null;default:0" json:"joins"`
	Leaves   int64  `gorm:"column:leaves;not null;default:0" json:"leaves"`
}

func (ChatDailyStat) TableName() string {
	return "chat_daily_stats"
}

// ChatHourlyStat is one chat's message count for one UTC hour of one UTC day.
//
// Day and Hour are computed in Go (alita/utils/stats), like ChatDailyStat's
// day, and the counter is only ever added to. A reader that shows the chat's
// local time converts (Day, Hour) itself; nothing here knows a timezone.
type ChatHourlyStat struct {
	ChatID   int64  `gorm:"column:chat_id;primaryKey" json:"chat_id"`
	Day      string `gorm:"column:day;primaryKey;size:10;index:idx_chat_hourly_stats_day" json:"day"`
	Hour     int16  `gorm:"column:hour;primaryKey;autoIncrement:false;check:chk_chat_hourly_stats_hour,hour >= 0 AND hour <= 23" json:"hour"`
	Messages int64  `gorm:"column:messages;not null;default:0" json:"messages"`
}

func (ChatHourlyStat) TableName() string {
	return "chat_hourly_stats"
}

// ChatMemberDailyStat is one user's message count in one chat for one UTC
// day. Only real users are counted (no channels, no anonymous admins), and the
// counter is only ever added to.
type ChatMemberDailyStat struct {
	ChatID   int64  `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Day      string `gorm:"column:day;primaryKey;size:10;index:idx_chat_member_daily_stats_day" json:"day"`
	UserID   int64  `gorm:"column:user_id;primaryKey;autoIncrement:false" json:"user_id"`
	Messages int64  `gorm:"column:messages;not null;default:0" json:"messages"`
}

func (ChatMemberDailyStat) TableName() string {
	return "chat_member_daily_stats"
}

// ChatMemberCount is one chat's Telegram member count on one UTC day. The row
// doubles as the day's claim: Count is nil from the claim until the count is
// written, and stays nil when Telegram refused permanently.
type ChatMemberCount struct {
	ChatID int64  `gorm:"column:chat_id;primaryKey;autoIncrement:false" json:"chat_id"`
	Day    string `gorm:"column:day;primaryKey;size:10;index:idx_chat_member_counts_day" json:"day"`
	Count  *int64 `gorm:"column:count" json:"count"`
}

func (ChatMemberCount) TableName() string {
	return "chat_member_counts"
}
