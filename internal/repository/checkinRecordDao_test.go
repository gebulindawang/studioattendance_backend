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
	if len(slots) != 5 {
		t.Fatalf("got %d slots, want 5", len(slots))
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
	if tuesday := slots[1]; tuesday.Section != 1 || tuesday.SlotStart.Hour() != 8 || tuesday.SlotEnd.Hour() != 11 {
		t.Fatalf("Tuesday should use one record for both consecutive blocks: %+v", tuesday)
	}
}

func TestConsecutiveBlocksSplitAtCoursesAndMissingBlocks(t *testing.T) {
	day := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	blocks := []AttendanceBlock{{Section: 3}, {Section: 4}, {Section: 5}, {Section: 7}, {Section: 8}}
	courses := []model.Schedule{{Weekday: 5, StartWeek: 1, EndWeek: 1, StartSection: 8, EndSection: 8}}

	full := dailyAttendancePeriods(courses, blocks, day, 2)
	if len(full) != 2 || full[0].First.Section != 3 || full[0].Last.Section != 5 ||
		full[1].First.Section != 7 || full[1].Last.Section != 8 {
		t.Fatalf("expected runs 3–5 and 7–8 when the course is inactive, got %+v", full)
	}
	split := dailyAttendancePeriods(courses, blocks, day, 1)
	if len(split) != 3 || split[0].First.Section != 3 || split[0].Last.Section != 3 ||
		split[1].First.Section != 5 || split[1].Last.Section != 5 {
		t.Fatalf("expected the class in block 4 to split the first run, got %+v", split)
	}
}

func TestContinuousBlocksSplitBetweenMorningAfternoonAndEvening(t *testing.T) {
	day := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	blocks := []AttendanceBlock{{Section: 1}, {Section: 2}, {Section: 3}, {Section: 4}, {Section: 5}, {Section: 6}, {Section: 7}}
	periods := dailyAttendancePeriods(nil, blocks, day, 1)
	if len(periods) != 3 || periods[0].First.Section != 1 || periods[0].Last.Section != 2 ||
		periods[1].First.Section != 3 || periods[1].Last.Section != 5 ||
		periods[2].First.Section != 6 || periods[2].Last.Section != 7 {
		t.Fatalf("expected morning 1–2, afternoon 3–5, evening 6–7, got %+v", periods)
	}

	start, end, signed := day.Add(8*time.Hour), day.Add(21*time.Hour), day.Add(7*time.Hour+57*time.Minute)
	records, generated := weekCheckinRecords(7, []model.CheckinRecord{{
		ID: 1, UserID: 7, CheckinDate: day, Section: 1, SlotStart: &start, SlotEnd: &end,
		Status: model.CheckinNormal, CheckinTime: &signed,
	}}, periods)
	if generated != 1 || len(records) != 1 || records[0].LastSection != 2 {
		t.Fatalf("an old all-day record should only cover the morning under the new rule, got %+v", records)
	}
}

func TestWeekCheckinRecordsReuseExistingRows(t *testing.T) {
	day := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	start := day.Add(13 * time.Hour)
	later := start.Add(2 * time.Hour)
	periods := []attendancePeriod{{Day: day, First: AttendanceBlock{Section: 3}, Last: AttendanceBlock{Section: 4}}}
	stored := []model.CheckinRecord{
		{ID: 1, UserID: 7, CheckinDate: day, Section: 3, SlotStart: &start},
		{ID: 2, UserID: 7, CheckinDate: day, Section: 4, SlotStart: &later},
	}
	records, generated := weekCheckinRecords(7, stored, periods)
	if generated != 1 || len(records) != 1 || records[0].ID != 1 ||
		records[0].FirstSection != 3 || records[0].LastSection != 4 {
		t.Fatalf("expected one record for both blocks, got %+v", records)
	}

	stored[1].Status = model.CheckinNormal
	stored[1].CheckinTime = &later
	records, generated = weekCheckinRecords(7, stored, periods)
	if generated != 1 || len(records) != 1 || records[0].ID != 2 || records[0].FirstSection != 3 ||
		records[0].LastSection != 4 || records[0].Status != model.CheckinNormal {
		t.Fatalf("expected the previously signed block to cover the run, got %+v", records)
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
