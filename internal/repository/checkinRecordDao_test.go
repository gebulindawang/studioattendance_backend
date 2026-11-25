package repository

import (
	"StudioAttendance_backend/internal/model"
	"errors"
	"testing"
	"time"
)

func TestWeekSlotsSkipAnyConflictingSmallSection(t *testing.T) {
	wednesday := time.Date(2026, 11, 25, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	start := monday(wednesday)
	if date := start.Format(time.DateOnly); date != "2026-11-23" {
		t.Fatalf("week starts on %s, want Monday 2026-11-23", date)
	}
	config := model.SemesterConfig{
		Semester: &model.Semester{
			StartDate:  start.AddDate(0, 0, -14),
			EndDate:    start.AddDate(0, 0, 4),
			TotalWeeks: 3,
		},
		SectionTime: []model.SectionTime{
			{SectionNumber: "2", StartTime: "10:00:00", EndTime: "11:40:00"},
			{SectionNumber: "1", StartTime: "08:00:00", EndTime: "09:40:00"},
		},
	}
	courses := []model.Schedule{
		{Weekday: 1, StartWeek: 1, EndWeek: 3, WeekType: model.WeekTypeOdd, StartSection: 2, EndSection: 2},
		{Weekday: 2, StartWeek: 1, EndWeek: 3, WeekType: model.WeekTypeEven, StartSection: 3, EndSection: 4},
	}
	slots := weekSlots(7, courses, config, attendanceBlocks(config.SectionTime), start)
	if len(slots) != 9 {
		t.Fatalf("got %d slots, want 9", len(slots))
	}
	for _, slot := range slots {
		if slot.CheckinDate.After(config.Semester.EndDate) ||
			(slot.CheckinDate.Equal(start) && slot.Section == 1) ||
			slot.RecordKind != model.CheckinKindStudio || slot.Status != model.CheckinPending {
			t.Fatalf("unexpected slot: %+v", slot)
		}
	}
	if first := slots[0]; first.Section != 2 || first.SlotStart.Hour() != 10 || first.SlotEnd.Minute() != 40 {
		t.Fatalf("unexpected first block: %+v", first)
	}
}

func TestFiveMinuteCheckinWindow(t *testing.T) {
	start := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	open := start.Add(-5 * time.Minute)
	cases := []struct {
		name string
		now  time.Time
		want error
	}{
		{"too early", open.Add(-time.Nanosecond), ErrCheckinTooEarly},
		{"opens", open, nil},
		{"before start", start.Add(-time.Nanosecond), nil},
		{"at start", start, ErrCheckinClosed},
		{"after start", start.Add(time.Second), ErrCheckinClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkinWindow(start, tc.now); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}

	slot := model.CheckinRecord{SlotStart: &start, Status: model.CheckinPending}
	if !canRefreshSlot(slot, open.Add(-time.Second)) || canRefreshSlot(slot, open) {
		t.Fatal("only an unopened pending slot can be adjusted")
	}
	slot.Status = model.CheckinNormal
	if canRefreshSlot(slot, open.Add(-time.Second)) {
		t.Fatal("a signed slot must be preserved")
	}
}
