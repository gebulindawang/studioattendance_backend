package model

import "time"

// 周类型
const (
	WeekTypeEvery int8 = 0 // 每周
	WeekTypeOdd   int8 = 1 // 单周
	WeekTypeEven  int8 = 2 // 双周
)

// Schedule 对应表 schedules(OCR 导入的课表)
// 判断"今天该不该上这门课": weekday 匹配 && start_week <= 当前周 <= end_week && week_type 匹配
type Schedule struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	UserID       int       `json:"user_id"`
	CourseName   string    `json:"course_name"`
	Location     string    `json:"location"`
	Weekday      int8      `json:"weekday"` // 1周一 ~ 7周日
	StartSection int       `json:"start_section"`
	EndSection   int       `json:"end_section"`
	StartWeek    int       `json:"start_week"`
	EndWeek      int       `json:"end_week"`
	WeekType     int8      `json:"week_type"` // 0每周 1单周 2双周
	Confidence   float64   `json:"confidence,omitempty" gorm:"-"` // LLM 识别置信度 0~1,不落库;来自数据库的记录没有该字段
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ScheduleImage struct {
	ID     int
	UserID int
	ImagesUrl    string
}

func (s *Schedule) TableName() string {
	return "schedules"
}

func (si *ScheduleImage) TableName() string {
	return "schedules_image"
}
