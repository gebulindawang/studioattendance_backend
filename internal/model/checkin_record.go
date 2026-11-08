package model

import "time"

// 签到状态
const (
	CheckinNormal   int8 = 1 // 正常
	CheckinLate     int8 = 2 // 迟到
	CheckinAbsent   int8 = 3 // 缺勤
	CheckinLeave    int8 = 4 // 请假
	CheckinExempted int8 = 5 // 有课豁免
)

// CheckinRecord 对应表 checkin_records(核心表)
// 记录只在两个时机产生:用户点击签到(status=1)、每日结算(3/5)
type CheckinRecord struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `json:"user_id"`
	CheckinDate time.Time  `json:"checkin_date"` // 归属日期
	Section     int        `json:"section"`
	Status      int8       `json:"status"`
	CheckinTime *time.Time `json:"checkin_time"` // 实际签到时间;结算生成的记录为 NULL,故用指针
	Remark      string     `json:"remark"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
