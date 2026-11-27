package repository

import (
	"StudioAttendance_backend/internal/model"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrMakeupUnavailable = errors.New("只有已截止且未签到的工作室时段可以申请补签")
	ErrMakeupPending     = errors.New("这条补签申请正在审核")
	ErrMakeupReviewed    = errors.New("这条申请已经审核，请刷新列表")
)

type MemberAttendanceEntry struct {
	model.CheckinRecord
	MakeupRequest    *model.CheckinMakeupRequest `json:"makeup_request,omitempty"`
	CanRequestMakeup bool                        `json:"can_request_makeup"`
}

type MemberAttendance struct {
	Month     string                  `json:"month"`
	WeekState string                  `json:"week_state"`
	Now       time.Time               `json:"now"`
	Next      *WeekAttendanceRecord   `json:"next,omitempty"`
	Records   []MemberAttendanceEntry `json:"records"`
}

type MakeupEntry struct {
	model.CheckinMakeupRequest
	Nickname     string     `json:"nickname"`
	CheckinDate  time.Time  `json:"checkin_date"`
	Section      int        `json:"section"`
	SlotStart    *time.Time `json:"slot_start,omitempty"`
	SlotEnd      *time.Time `json:"slot_end,omitempty"`
	ReviewerName string     `json:"reviewer_name"`
}

func makeupEligible(record model.CheckinRecord, now time.Time) bool {
	return record.RecordKind == model.CheckinKindStudio && record.Status == model.CheckinPending &&
		record.CheckinTime == nil && record.SlotStart != nil && !now.Before(*record.SlotStart)
}

func GetMemberAttendance(ctx context.Context, userID int, month, now time.Time) (MemberAttendance, error) {
	result := MemberAttendance{Month: month.Format("2006-01"), Now: now, Records: []MemberAttendanceEntry{}}
	week, err := GetWeekAttendance(userID, now)
	if err != nil {
		return result, err
	}
	result.WeekState = week.State
	visibleThisWeek := make(map[uint]bool, len(week.Records))
	for _, record := range week.Records {
		visibleThisWeek[record.ID] = true
		if record.Status == model.CheckinPending && record.SlotStart != nil && now.Before(*record.SlotStart) &&
			(result.Next == nil || record.SlotStart.Before(*result.Next.SlotStart)) {
			next := record
			result.Next = &next
		}
	}

	err = DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var records []model.CheckinRecord
		if err := tx.Where("user_id = ? AND checkin_date >= ? AND checkin_date < ?", userID,
			month.Format(time.DateOnly), month.AddDate(0, 1, 0).Format(time.DateOnly)).
			Order("checkin_date DESC, section, id").Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(records))
		for _, record := range records {
			ids = append(ids, record.ID)
		}
		var requests []model.CheckinMakeupRequest
		if err := tx.Where("record_id IN ?", ids).Order("id").Find(&requests).Error; err != nil {
			return err
		}
		latest := make(map[uint]*model.CheckinMakeupRequest, len(requests))
		for i := range requests {
			latest[requests[i].RecordID] = &requests[i]
		}
		start := monday(now)
		end := start.AddDate(0, 0, 7)
		for _, record := range records {
			request := latest[record.ID]
			notInCurrentWeekPlan := record.RecordKind == model.CheckinKindStudio && record.Status == model.CheckinPending &&
				!record.CheckinDate.Before(start) && record.CheckinDate.Before(end) && !visibleThisWeek[record.ID]
			if notInCurrentWeekPlan && request == nil {
				continue
			}
			entry := MemberAttendanceEntry{CheckinRecord: record, MakeupRequest: request}
			if makeupEligible(record, now) {
				entry.Status = model.CheckinClosed
				entry.CanRequestMakeup = !notInCurrentWeekPlan && (request == nil || request.Status == model.MakeupRejected)
			}
			result.Records = append(result.Records, entry)
		}
		return nil
	})
	return result, attendanceDBError(err)
}

func validateMakeupRecord(tx *gorm.DB, record model.CheckinRecord, now time.Time) error {
	if !makeupEligible(record, now) {
		return ErrMakeupUnavailable
	}
	// Historical records keep the eligibility they had when generated.
	if record.CheckinDate.Before(monday(now)) {
		return nil
	}
	period, err := checkinPeriod(tx, record)
	if err != nil {
		return err
	}
	return checkPeriodIncomplete(tx, record, period)
}

func SubmitCheckinMakeup(ctx context.Context, userID int, id uint, reason string, now time.Time) (model.CheckinMakeupRequest, error) {
	result := model.CheckinMakeupRequest{RecordID: id, Reason: reason, Status: model.MakeupPending}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		member, err := lockScheduleOwner(tx, userID)
		if err != nil {
			return err
		}
		if member.Role != model.RoleMember || member.Status != model.StatusActive {
			return gorm.ErrRecordNotFound
		}
		var record model.CheckinRecord
		if err := tx.Where("user_id = ?", userID).First(&record, id).Error; err != nil {
			return err
		}
		if err := validateMakeupRecord(tx, record, now); err != nil {
			return err
		}
		var pending int64
		if err := tx.Model(&model.CheckinMakeupRequest{}).Where("record_id = ? AND status = ?", id, model.MakeupPending).
			Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrMakeupPending
		}
		return tx.Create(&result).Error
	})
	return result, attendanceDBError(err)
}

func ListCheckinMakeups(ctx context.Context) ([]MakeupEntry, error) {
	entries := make([]MakeupEntry, 0)
	err := DB.WithContext(ctx).Model(&model.CheckinMakeupRequest{}).
		Select("checkin_makeup_requests.*, r.checkin_date, r.section, r.slot_start, r.slot_end, COALESCE(request_member.nickname, '') AS nickname, COALESCE(request_reviewer.nickname, '') AS reviewer_name").
		Joins("JOIN checkin_records r ON r.id = checkin_makeup_requests.record_id").
		Joins("LEFT JOIN users request_member ON request_member.id = r.user_id").
		Joins("LEFT JOIN users request_reviewer ON request_reviewer.id = checkin_makeup_requests.reviewer_id").
		Order("checkin_makeup_requests.id DESC").Scan(&entries).Error
	return entries, attendanceDBError(err)
}

func ReviewCheckinMakeup(ctx context.Context, reviewerID int, id uint, decision, note string, now time.Time) error {
	var target struct {
		RecordID uint
		UserID   int
	}
	query := DB.WithContext(ctx).Model(&model.CheckinMakeupRequest{}).
		Select("checkin_makeup_requests.record_id, r.user_id").
		Joins("JOIN checkin_records r ON r.id = checkin_makeup_requests.record_id").
		Where("checkin_makeup_requests.id = ?", id).Take(&target)
	if query.Error != nil {
		return attendanceDBError(query.Error)
	}

	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Use the same member lock as check-in, course confirmation, and weekly generation.
		if _, err := lockScheduleOwner(tx, target.UserID); err != nil {
			return err
		}
		var request model.CheckinMakeupRequest
		if err := tx.First(&request, id).Error; err != nil {
			return err
		}
		if request.Status != model.MakeupPending {
			return ErrMakeupReviewed
		}
		if decision == model.MakeupApproved {
			var record model.CheckinRecord
			if err := tx.First(&record, target.RecordID).Error; err != nil {
				return err
			}
			if err := validateMakeupRecord(tx, record, now); err != nil {
				return err
			}
			result := tx.Model(&record).Where("status = ? AND checkin_time IS NULL", model.CheckinPending).
				Update("status", model.CheckinMadeUp)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrMakeupUnavailable
			}
		}
		result := tx.Model(&request).Where("status = ?", model.MakeupPending).Updates(map[string]any{
			"status": decision, "reviewer_id": reviewerID, "review_note": note, "reviewed_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrMakeupReviewed
		}
		return nil
	})
	return attendanceDBError(err)
}
