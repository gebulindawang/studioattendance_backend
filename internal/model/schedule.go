package model

import "time"

// 周类型
const (
	WeekTypeEvery int8 = 0 // 每周
	WeekTypeOdd   int8 = 1 // 单周
	WeekTypeEven  int8 = 2 // 双周
)

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
	WeekType     int8      `json:"week_type"`                     // 0每周 1单周 2双周
	Confidence   float64   `json:"confidence,omitempty" gorm:"-"` // LLM 识别置信度 0~1,不落库;来自数据库的记录没有该字段
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ScheduleImage struct {
	ID        int        `json:"id"`
	UserID    int        `json:"-"`
	ImagesUrl string     `json:"url"`
	CreatedAt *time.Time `json:"uploaded_at"`
	Reviewed  *bool      `json:"-"`
}

const MaxScheduleUploads = 3

type ScheduleSummary struct {
	Key            string     `json:"key"`
	Username       string     `json:"username"`
	Nickname       string     `json:"nickname"`
	Status         string     `json:"status"`
	UploadCount    int        `json:"upload_count"`
	MaxUploads     int        `json:"max_uploads"`
	LastUploadedAt *time.Time `json:"last_uploaded_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at"`
	CourseCount    int        `json:"course_count"`
}

type ScheduleDetail struct {
	ScheduleSummary
	LatestImage string          `json:"latest_image"`
	Uploads     []ScheduleImage `json:"uploads"`
	Courses     []Schedule      `json:"courses"`
}

type SectionTime struct {
	ID            int
	SectionNumber string
	StartTime     string
	EndTime       string
}

type Semester struct {
	ID         int
	StartDate  time.Time
	EndDate    time.Time
	TotalWeeks int
}

func (s *SectionTime) TableName() string {
	return "section_time"
}

func (s *Semester) TableName() string {
	return "semester"
}

type SemesterConfig struct {
	Semester    *Semester     `json:"semester"`
	SectionTime []SectionTime `json:"section_time"`
}

func (s *Schedule) TableName() string {
	return "schedules"
}

func (si *ScheduleImage) TableName() string {
	return "schedules_image"
}
