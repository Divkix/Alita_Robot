package models

// ChatDailyStat is one chat's activity counters for one UTC day.
//
// Day is a YYYY-MM-DD string computed in Go (stats.Day), never by the
// database, so PostgreSQL and the SQLite tests agree on the row key. The
// counters are only ever added to: a replica flush never writes a value it
// computed from zero.
type ChatDailyStat struct {
	ChatID   int64  `gorm:"column:chat_id;primaryKey" json:"chat_id"`
	Day      string `gorm:"column:day;primaryKey;size:10" json:"day"`
	Messages int64  `gorm:"column:messages;not null;default:0" json:"messages"`
	Joins    int64  `gorm:"column:joins;not null;default:0" json:"joins"`
	Leaves   int64  `gorm:"column:leaves;not null;default:0" json:"leaves"`
}

func (ChatDailyStat) TableName() string {
	return "chat_daily_stats"
}
