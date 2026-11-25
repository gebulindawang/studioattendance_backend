package repository

import (
	"StudioAttendance_backend/internal/model"
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const checkinLead = 5 * time.Minute

var (
	ErrCheckinUnavailable    = errors.New("找不到可签到的记录")
	ErrCheckinTooEarly       = errors.New("开始前 5 分钟才开放签到")
	ErrCheckinClosed         = errors.New("签到窗口已结束")
	ErrCheckinCourseConflict = errors.New("课表未确认或该大节有课，请刷新签到表")
	ErrAttendanceConfig      = errors.New("本周没有可用的学期或大节配置")
	ErrAttendanceSchema      = errors.New("工作室签到表结构尚未升级")
)

type AttendanceBlock struct {
	Section int    `json:"section"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

type WeekAttendance struct {
	Now         time.Time             `json:"now"`
	WeekStart   string                `json:"week_start"`
	Days        []string              `json:"days"`
	Blocks      []AttendanceBlock     `json:"blocks"`
	Records     []model.CheckinRecord `json:"records"`
	State       string                `json:"state"`
	UploadCount int                   `json:"upload_count"`
	MaxUploads  int                   `json:"max_uploads"`
}

type MemberGeneration struct {
	UserID int    `json:"user_id"`
	State  string `json:"state"`
	Slots  int    `json:"slots"`
}

type WeekChanges struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
}

type WeekGeneration struct {
	WeekStart string `json:"week_start"`
	WeekChanges
	Members []MemberGeneration `json:"members"`
}

type CheckinEntry struct {
	model.CheckinRecord
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

func ListCheckinRecord(now time.Time) ([]CheckinEntry, error) {
	records := make([]CheckinEntry, 0)
	err := DB.Model(&model.CheckinRecord{}).
		Select("checkin_records.*, COALESCE(users.username, '') AS username, COALESCE(users.nickname, '已删除成员') AS nickname").
		Joins("LEFT JOIN users ON users.id = checkin_records.user_id").
		Order("checkin_records.checkin_date DESC, checkin_records.section, checkin_records.id").
		Scan(&records).Error
	for i := range records {
		record := &records[i]
		if record.RecordKind == model.CheckinKindStudio && record.Status == model.CheckinPending &&
			record.SlotStart != nil && !now.Before(*record.SlotStart) {
			record.Status = model.CheckinClosed
		}
	}
	return records, err
}

func monday(date time.Time) time.Time {
	y, m, day := date.Date()
	d := time.Date(y, m, day, 0, 0, 0, 0, date.Location())
	return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
}

func attendanceWeek(semester *model.Semester, date time.Time) int {
	if semester == nil {
		return 0
	}
	day := date.Format(time.DateOnly)
	if day < semester.StartDate.Format(time.DateOnly) || day > semester.EndDate.Format(time.DateOnly) {
		return 0
	}
	// Compare civil Mondays in UTC so daylight-saving changes do not affect the week number.
	toCalendarMonday := func(d time.Time) time.Time {
		y, m, day := d.Date()
		return monday(time.Date(y, m, day, 0, 0, 0, 0, time.UTC))
	}
	week := int(toCalendarMonday(date).Sub(toCalendarMonday(semester.StartDate))/(7*24*time.Hour)) + 1
	if week < 1 || week > semester.TotalWeeks {
		return 0
	}
	return week
}

func attendanceBlocks(settings []model.SectionTime) []AttendanceBlock {
	blocks := make([]AttendanceBlock, 0, len(settings))
	seen := make(map[int]bool)
	for _, setting := range settings {
		number, numberErr := strconv.Atoi(setting.SectionNumber)
		start, startErr := time.Parse(time.TimeOnly, setting.StartTime)
		end, endErr := time.Parse(time.TimeOnly, setting.EndTime)
		if numberErr != nil || startErr != nil || endErr != nil ||
			number < 1 || number > 24 || seen[number] || !end.After(start) {
			continue
		}
		seen[number] = true
		blocks = append(blocks, AttendanceBlock{Section: number, Start: setting.StartTime, End: setting.EndTime})
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Section < blocks[j].Section })
	return blocks
}

func hasCourseInBlock(courses []model.Schedule, day time.Time, week, block int) bool {
	weekday := int8((int(day.Weekday())+6)%7 + 1)
	for _, course := range courses {
		if course.Weekday != weekday || week < course.StartWeek || week > course.EndWeek ||
			(course.WeekType == model.WeekTypeOdd && week%2 == 0) ||
			(course.WeekType == model.WeekTypeEven && week%2 != 0) {
			continue
		}
		if course.StartSection <= block*2 && course.EndSection >= block*2-1 {
			return true
		}
	}
	return false
}

func weekSlots(userID int, courses []model.Schedule, config model.SemesterConfig, blocks []AttendanceBlock, startWeek time.Time) []model.CheckinRecord {
	slots := make([]model.CheckinRecord, 0)
	for i := range 7 {
		day := startWeek.AddDate(0, 0, i)
		week := attendanceWeek(config.Semester, day)
		if week == 0 {
			continue
		}
		for _, block := range blocks {
			if hasCourseInBlock(courses, day, week, block.Section) {
				continue
			}
			date := day.Format(time.DateOnly) + " "
			start, _ := time.ParseInLocation(time.DateTime, date+block.Start, day.Location())
			end, _ := time.ParseInLocation(time.DateTime, date+block.End, day.Location())
			slots = append(slots, model.CheckinRecord{
				UserID: uint(userID), CheckinDate: day, Section: block.Section,
				Status: model.CheckinPending, RecordKind: model.CheckinKindStudio, SlotStart: &start, SlotEnd: &end,
			})
		}
	}
	return slots
}

func checkinWindow(start, now time.Time) error {
	switch {
	case now.Before(start.Add(-checkinLead)):
		return ErrCheckinTooEarly
	case !now.Before(start):
		return ErrCheckinClosed
	default:
		return nil
	}
}

type slotKey struct {
	UserID  uint
	Date    string
	Section int
}

func attendanceSlotKey(record model.CheckinRecord) slotKey {
	return slotKey{record.UserID, record.CheckinDate.Format(time.DateOnly), record.Section}
}

func studioWeekRecords(tx *gorm.DB, ids []int, weekStart time.Time, records *[]model.CheckinRecord) error {
	err := tx.Where("record_kind = ? AND user_id IN ? AND checkin_date >= ? AND checkin_date < ?", model.CheckinKindStudio,
		ids, weekStart.Format(time.DateOnly), weekStart.AddDate(0, 0, 7).Format(time.DateOnly)).
		Order("checkin_date, section").Find(records).Error
	return attendanceDBError(err)
}

func attendanceDBError(err error) error {
	var dbErr *mysql.MySQLError
	if errors.As(err, &dbErr) && dbErr.Number == 1054 &&
		(strings.Contains(dbErr.Message, "record_kind") || strings.Contains(dbErr.Message, "slot_start") || strings.Contains(dbErr.Message, "slot_end")) {
		return errors.Join(ErrAttendanceSchema, err)
	}
	return err
}

func hasTermDates(semester *model.Semester, weekStart time.Time) bool {
	for i := range 7 {
		if attendanceWeek(semester, weekStart.AddDate(0, 0, i)) != 0 {
			return true
		}
	}
	return false
}

func GetWeekAttendance(userID int, now time.Time) (WeekAttendance, error) {
	start := monday(now)
	result := WeekAttendance{
		Now: now, WeekStart: start.Format(time.DateOnly),
		Days: make([]string, 0, 7), Records: []model.CheckinRecord{}, MaxUploads: model.MaxScheduleUploads,
	}
	for i := range 7 {
		result.Days = append(result.Days, start.AddDate(0, 0, i).Format(time.DateOnly))
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		config, err := loadSemesterConfig(tx)
		if err != nil {
			return err
		}
		result.Blocks = attendanceBlocks(config.SectionTime)
		result.UploadCount, err = countScheduleImages(tx, userID)
		if err != nil {
			return err
		}
		var courses []model.Schedule
		if err := tx.Where("user_id = ?", userID).Find(&courses).Error; err != nil {
			return err
		}
		var stored []model.CheckinRecord
		if err := studioWeekRecords(tx, []int{userID}, start, &stored); err != nil {
			return err
		}

		if len(courses) == 0 {
			result.State = "not_uploaded"
			if result.UploadCount > 0 {
				result.State = "waiting_review"
			}
		} else if len(result.Blocks) == 0 || !hasTermDates(config.Semester, start) {
			result.State = "no_configuration"
		} else if len(weekSlots(userID, courses, config, result.Blocks, start)) == 0 {
			result.State = "no_slots"
		} else {
			result.State = "not_generated"
		}

		for _, record := range stored {
			week := attendanceWeek(config.Semester, record.CheckinDate)
			eligible := len(courses) > 0 && week > 0 && record.SlotStart != nil &&
				!hasCourseInBlock(courses, record.CheckinDate, week, record.Section)
			if record.Status == model.CheckinNormal || eligible {
				if record.SlotStart != nil {
					opensAt := record.SlotStart.Add(-checkinLead)
					record.OpensAt = &opensAt
				}
				result.Records = append(result.Records, record)
			}
			if eligible && result.State == "not_generated" {
				result.State = "ready"
			}
		}
		return nil
	})
	return result, err
}

func canRefreshSlot(record model.CheckinRecord, now time.Time) bool {
	return record.Status == model.CheckinPending && record.CheckinTime == nil &&
		record.SlotStart != nil && now.Before(record.SlotStart.Add(-checkinLead))
}

// Callers hold the selected members' row locks while applying changes.
func reconcileWeekSlots(tx *gorm.DB, ids []int, desired []model.CheckinRecord, start, now time.Time) (WeekChanges, error) {
	var result WeekChanges
	if len(ids) == 0 {
		return result, nil
	}
	var stored []model.CheckinRecord
	if err := studioWeekRecords(tx, ids, start, &stored); err != nil {
		return result, err
	}
	existing := make(map[slotKey]model.CheckinRecord, len(stored))
	for _, record := range stored {
		existing[attendanceSlotKey(record)] = record
	}

	var added []model.CheckinRecord
	for _, slot := range desired {
		key := attendanceSlotKey(slot)
		previous, found := existing[key]
		if !found {
			added = append(added, slot)
			continue
		}
		delete(existing, key)
		if canRefreshSlot(previous, now) && now.Before(slot.SlotStart.Add(-checkinLead)) &&
			(!previous.SlotStart.Equal(*slot.SlotStart) || previous.SlotEnd == nil || !previous.SlotEnd.Equal(*slot.SlotEnd)) {
			update := tx.Model(&previous).Updates(map[string]any{"slot_start": slot.SlotStart, "slot_end": slot.SlotEnd})
			if update.Error != nil {
				return result, attendanceDBError(update.Error)
			}
			result.Updated += int(update.RowsAffected)
		}
	}
	var removed []uint
	for _, record := range existing {
		if canRefreshSlot(record, now) {
			removed = append(removed, record.ID)
		}
	}
	if len(removed) > 0 {
		deletion := tx.Delete(&model.CheckinRecord{}, removed)
		if deletion.Error != nil {
			return result, deletion.Error
		}
		result.Removed = int(deletion.RowsAffected)
	}
	if len(added) > 0 {
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&added, 250)
		if insert.Error != nil {
			return result, attendanceDBError(insert.Error)
		}
		result.Added = int(insert.RowsAffected)
	}
	return result, nil
}

func GenerateWeekAttendance(ctx context.Context, ids []int, now time.Time) (WeekGeneration, error) {
	ordered := slices.Clone(ids)
	slices.Sort(ordered)
	ordered = slices.Compact(ordered)
	start := monday(now)
	result := WeekGeneration{WeekStart: start.Format(time.DateOnly), Members: make([]MemberGeneration, 0, len(ordered))}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var users []model.User
		if err := tx.Select("id").Where("id IN ? AND role = ? AND status = ?", ordered, model.RoleMember, model.StatusActive).
			Order("id").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&users).Error; err != nil {
			return err
		}
		active := make(map[int]bool, len(users))
		activeIDs := make([]int, 0, len(users))
		for _, user := range users {
			active[user.ID] = true
			activeIDs = append(activeIDs, user.ID)
		}
		config, err := loadSemesterConfig(tx)
		if err != nil {
			return err
		}
		blocks := attendanceBlocks(config.SectionTime)
		if len(blocks) == 0 || !hasTermDates(config.Semester, start) {
			return ErrAttendanceConfig
		}

		var schedules []model.Schedule
		if err := tx.Where("user_id IN ?", activeIDs).Find(&schedules).Error; err != nil {
			return err
		}
		coursesByUser := make(map[int][]model.Schedule)
		for _, course := range schedules {
			coursesByUser[course.UserID] = append(coursesByUser[course.UserID], course)
		}
		var desired []model.CheckinRecord
		for _, id := range ordered {
			member := MemberGeneration{UserID: id, State: "unavailable"}
			courses := coursesByUser[id]
			switch {
			case !active[id]:
			case len(courses) == 0:
				count, err := countScheduleImages(tx, id)
				if err != nil {
					return err
				}
				member.State = "not_uploaded"
				if count > 0 {
					member.State = "waiting_review"
				}
			default:
				slots := weekSlots(id, courses, config, blocks, start)
				member.State, member.Slots = "ready", len(slots)
				if len(slots) == 0 {
					member.State = "no_slots"
				}
				desired = append(desired, slots...)
			}
			result.Members = append(result.Members, member)
		}
		result.WeekChanges, err = reconcileWeekSlots(tx, activeIDs, desired, start, now)
		return err
	})
	return result, attendanceDBError(err)
}

func CheckIn(userID int, id uint, now time.Time) (model.CheckinRecord, error) {
	var record model.CheckinRecord
	err := DB.Transaction(func(tx *gorm.DB) error {
		var member model.User
		err := tx.Select("id").Where("role = ? AND status = ?", model.RoleMember, model.StatusActive).
			Clauses(clause.Locking{Strength: "UPDATE"}).First(&member, userID).Error
		if err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND record_kind = ?", userID, model.CheckinKindStudio).First(&record, id).Error; err != nil {
			return err
		}
		if record.Status == model.CheckinNormal && record.CheckinTime != nil {
			return nil
		}
		if record.Status != model.CheckinPending || record.SlotStart == nil {
			return ErrCheckinUnavailable
		}
		if err := checkinWindow(*record.SlotStart, now); err != nil {
			return err
		}
		config, err := loadSemesterConfig(tx)
		if err != nil {
			return err
		}
		var courses []model.Schedule
		if err := tx.Where("user_id = ?", userID).Find(&courses).Error; err != nil {
			return err
		}
		week := attendanceWeek(config.Semester, record.CheckinDate)
		if len(courses) == 0 || week == 0 || hasCourseInBlock(courses, record.CheckinDate, week, record.Section) {
			return ErrCheckinCourseConflict
		}
		checkedAt := now.Truncate(time.Second)
		result := tx.Model(&record).
			Where("user_id = ? AND status = ? AND checkin_time IS NULL", userID, model.CheckinPending).
			Updates(map[string]any{"status": model.CheckinNormal, "checkin_time": checkedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrCheckinUnavailable
		}
		record.Status, record.CheckinTime = model.CheckinNormal, &checkedAt
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, ErrCheckinUnavailable
	}
	return record, attendanceDBError(err)
}
