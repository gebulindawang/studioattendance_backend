package repository

import (
	"StudioAttendance_backend/internal/model"
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const checkinEarly = 20 * time.Minute
const checkinLate = 5 * time.Minute

var (
	ErrCheckinUnavailable = errors.New("当前没有可签到的节次")
	ErrCheckinExempted    = errors.New("该节次有课，已豁免")
	ErrCheckinTooEarly    = errors.New("未到签到时间")
	ErrCheckinClosed      = errors.New("签到已结束")
	ErrCheckinDuplicate   = errors.New("该节次已有考勤记录")
)

type AttendanceSection struct {
	Section    int       `json:"section"`
	StartTime  time.Time `json:"start_time"`
	OpensAt    time.Time `json:"opens_at"`
	ClosesAt   time.Time `json:"closes_at"`
	CourseName string    `json:"course_name,omitempty"`
	Status     int8      `json:"status,omitempty"`
}

type TodayAttendance struct {
	Now      time.Time           `json:"now"`
	Date     string              `json:"date"`
	Sections []AttendanceSection `json:"sections"`
}

type CheckinEntry struct {
	model.CheckinRecord
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

func ListCheckinRecord() ([]CheckinEntry, error) {
	records := make([]CheckinEntry, 0)
	err := DB.Model(&model.CheckinRecord{}).
		Select("checkin_records.*, COALESCE(users.username, '') AS username, COALESCE(users.nickname, '已删除成员') AS nickname").
		Joins("LEFT JOIN users ON users.id = checkin_records.user_id").
		Order("checkin_records.checkin_date DESC, checkin_records.section, checkin_records.id").
		Scan(&records).Error
	return records, err
}

func attendanceWeek(semester *model.Semester, date time.Time) int {
	if semester == nil {
		return 0
	}
	day := date.Format(time.DateOnly)
	if day < semester.StartDate.Format(time.DateOnly) || day > semester.EndDate.Format(time.DateOnly) {
		return 0
	}
	monday := func(d time.Time) time.Time {
		// 在 UTC 日历上比较自然日，避免时区偏移参与周次计算。
		y, m, day := d.Date()
		d = time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
		return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	}
	week := int(monday(date).Sub(monday(semester.StartDate))/(7*24*time.Hour)) + 1
	if week < 1 || week > semester.TotalWeeks {
		return 0
	}
	return week
}

func configuredAttendanceSections(blocks []model.SectionTime, day time.Time) []AttendanceSection {
	sections := make([]AttendanceSection, 0, len(blocks)*2)
	for _, block := range blocks {
		number, numberErr := strconv.Atoi(block.SectionNumber)
		clock, clockErr := time.Parse(time.TimeOnly, block.StartTime)
		if numberErr != nil || number < 1 || number > 24 || clockErr != nil {
			continue
		}
		y, m, d := day.Date()
		start := time.Date(y, m, d, clock.Hour(), clock.Minute(), clock.Second(), 0, day.Location())
		for _, section := range []int{number*2 - 1, number * 2} {
			sections = append(sections, AttendanceSection{
				Section: section, StartTime: start,
				OpensAt: start.Add(-checkinEarly), ClosesAt: start.Add(checkinLate),
			})
		}
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].Section < sections[j].Section })
	return sections
}

func markCourseExemptions(sections []AttendanceSection, schedules []model.Schedule, day time.Time, week int) {
	weekday := int8((int(day.Weekday())+6)%7 + 1)
	for _, course := range schedules {
		if course.Weekday != weekday || week < course.StartWeek || week > course.EndWeek ||
			(course.WeekType == model.WeekTypeOdd && week%2 == 0) ||
			(course.WeekType == model.WeekTypeEven && week%2 != 0) {
			continue
		}
		for i := range sections {
			if sections[i].Section >= course.StartSection && sections[i].Section <= course.EndSection {
				sections[i].Status = model.CheckinExempted
				sections[i].CourseName = course.CourseName
			}
		}
	}
}

func userAttendanceSections(db *gorm.DB, userID int, now time.Time) ([]AttendanceSection, error) {
	config, err := loadSemesterConfig(db)
	sections := make([]AttendanceSection, 0)
	if err != nil {
		return nil, err
	}
	week := attendanceWeek(config.Semester, now)
	if week == 0 {
		return sections, nil
	}
	var schedules []model.Schedule
	if err := db.Where("user_id = ?", userID).Order("id").Find(&schedules).Error; err != nil {
		return nil, err
	}
	if len(schedules) == 0 {
		return sections, nil
	}
	sections = configuredAttendanceSections(config.SectionTime, now)
	markCourseExemptions(sections, schedules, now, week)
	return sections, nil
}

func GetTodayAttendance(userID int, now time.Time) (TodayAttendance, error) {
	result := TodayAttendance{Date: now.Format(time.DateOnly), Now: now}
	err := DB.Transaction(func(tx *gorm.DB) error {
		sections, err := userAttendanceSections(tx, userID, now)
		if err != nil {
			return err
		}
		result.Sections = sections
		if len(sections) == 0 {
			return nil
		}

		var records []model.CheckinRecord
		if err := tx.Where("user_id = ? AND checkin_date = ?", userID, result.Date).Find(&records).Error; err != nil {
			return err
		}
		statusBySection := make(map[int]int8, len(records))
		for _, record := range records {
			statusBySection[record.Section] = record.Status
		}
		for i := range sections {
			if status := statusBySection[sections[i].Section]; status != 0 {
				sections[i].Status = status
			}
		}
		return nil
	})
	return result, err
}

func checkinStatus(section AttendanceSection, now time.Time) (int8, error) {
	if section.Status == model.CheckinExempted {
		return 0, ErrCheckinExempted
	}
	switch {
	case now.Before(section.OpensAt):
		return 0, ErrCheckinTooEarly
	case !now.After(section.StartTime):
		return model.CheckinNormal, nil
	case !now.After(section.ClosesAt):
		return model.CheckinLate, nil
	default:
		return 0, ErrCheckinClosed
	}
}

func CheckIn(userID, section int, now time.Time) (model.CheckinRecord, error) {
	var record model.CheckinRecord
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Select("id", "status", "role").Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, userID).Error; err != nil {
			return err
		}
		if user.Role != model.RoleMember || user.Status != model.StatusActive {
			return ErrCheckinUnavailable
		}

		sections, err := userAttendanceSections(tx, userID, now)
		if err != nil {
			return err
		}
		for _, current := range sections {
			if current.Section != section {
				continue
			}
			status, err := checkinStatus(current, now)
			if err != nil {
				return err
			}
			y, m, d := now.Date()
			record = model.CheckinRecord{
				UserID: uint(userID), Section: section,
				CheckinDate: time.Date(y, m, d, 0, 0, 0, 0, now.Location()),
				CheckinTime: &now, Status: status,
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrCheckinDuplicate
			}
			return nil
		}
		return ErrCheckinUnavailable
	})
	return record, err
}

// SettleAttendance fills missed or exempted slots after the sign-in window closes.
// The unique record key preserves any earlier sign-in/leave and makes retries safe.
func SettleAttendance(ctx context.Context, now time.Time) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		config, err := loadSemesterConfig(tx)
		if err != nil {
			return err
		}
		week := attendanceWeek(config.Semester, now)
		if week == 0 {
			return nil
		}
		var closed []AttendanceSection
		for _, section := range configuredAttendanceSections(config.SectionTime, now) {
			if now.After(section.ClosesAt) {
				closed = append(closed, section)
			}
		}
		if len(closed) == 0 {
			return nil
		}
		var schedules []model.Schedule
		err = tx.Model(&model.Schedule{}).Select("schedules.*").
			Joins("JOIN users ON users.id = schedules.user_id").
			Where("users.role = ? AND users.status = ? AND users.deleted_at IS NULL", model.RoleMember, model.StatusActive).
			Order("schedules.id").Find(&schedules).Error
		if err != nil {
			return err
		}
		var existing []model.CheckinRecord
		if err := tx.Select("user_id", "section").
			Where("checkin_date = ?", now.Format(time.DateOnly)).Find(&existing).Error; err != nil {
			return err
		}
		type recordKey struct{ user, section int }
		recorded := make(map[recordKey]bool, len(existing))
		for _, record := range existing {
			recorded[recordKey{int(record.UserID), record.Section}] = true
		}
		schedulesByUser := make(map[int][]model.Schedule)
		for _, schedule := range schedules {
			schedulesByUser[schedule.UserID] = append(schedulesByUser[schedule.UserID], schedule)
		}
		userIDs := make([]int, 0, len(schedulesByUser))
		for userID := range schedulesByUser {
			userIDs = append(userIDs, userID)
		}
		sort.Ints(userIDs)
		y, m, d := now.Date()
		day := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		var records []model.CheckinRecord
		for _, userID := range userIDs {
			sections := append([]AttendanceSection(nil), closed...)
			markCourseExemptions(sections, schedulesByUser[userID], day, week)
			for _, section := range sections {
				if recorded[recordKey{userID, section.Section}] {
					continue
				}
				status := section.Status
				if status == 0 {
					status = model.CheckinAbsent
				}
				records = append(records, model.CheckinRecord{
					UserID: uint(userID), CheckinDate: day, Section: section.Section, Status: status,
				})
			}
		}
		if len(records) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&records, 250).Error
	})
}
