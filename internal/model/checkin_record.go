package model

import "time"

const (
	CheckinPending int8 = 0
	CheckinNormal  int8 = 1
	CheckinClosed  int8 = 6 // 仅用于展示已结束且未签到的工作室时段
)

const CheckinKindStudio int8 = 1

// CheckinRecord 保留旧版小节记录，工作室记录按大节保存。
type CheckinRecord struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `json:"user_id"`
	CheckinDate time.Time  `json:"checkin_date"`
	Section     int        `json:"section"`
	Status      int8       `json:"status"`
	CheckinTime *time.Time `json:"checkin_time"`
	Remark      string     `json:"remark"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	RecordKind  int8       `json:"record_kind"`
	SlotStart   *time.Time `json:"slot_start,omitempty"`
	SlotEnd     *time.Time `json:"slot_end,omitempty"`
	OpensAt     *time.Time `gorm:"-" json:"opens_at,omitempty"`
}
