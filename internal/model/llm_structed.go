package model
// 大模型统一返回类型json
type Course struct{
	CourseName   string    `json:"course_name"`
	Location     string    `json:"location"`
	Weekday      int8      `json:"weekday"` // 1周一 ~ 7周日
	StartSection int       `json:"start_section"`
	EndSection   int       `json:"end_section"`
	StartWeek    int       `json:"start_week"`
	EndWeek      int	   `json:"end_week"`
	WeekType     string    `json:"week_type"` // 全周/单周/双周
}