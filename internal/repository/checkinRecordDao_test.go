package repository

import (
	"StudioAttendance_backend/internal/model"
	"errors"
	"testing"
	"time"
)

func TestAttendanceScheduleUsesSmallSectionsAndWeekParity(t *testing.T) {
	day := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	semester := &model.Semester{StartDate: day.AddDate(0, 0, -14), EndDate: day.AddDate(0, 0, 90), TotalWeeks: 15}
	week := attendanceWeek(semester, day)
	if week != 3 {
		t.Fatalf("week = %d, want 3", week)
	}
	if got := attendanceWeek(semester, semester.StartDate.AddDate(0, 0, -1)); got != 0 {
		t.Fatalf("week before the semester = %d, want 0", got)
	}

	sections := configuredAttendanceSections([]model.SectionTime{
		{SectionNumber: "3", StartTime: "10:00:00"},
		{SectionNumber: "2", StartTime: "08:00:00"},
	}, day)
	if len(sections) != 4 || sections[0].Section != 3 || sections[1].Section != 4 ||
		!sections[0].StartTime.Equal(day.Add(8*time.Hour)) || !sections[1].StartTime.Equal(sections[0].StartTime) {
		t.Fatalf("unexpected mapping of blocks to small sections: %+v", sections)
	}
	markCourseExemptions(sections, []model.Schedule{
		{Weekday: 1, StartWeek: 1, EndWeek: 5, WeekType: model.WeekTypeOdd, StartSection: 3, EndSection: 4},
		{Weekday: 1, StartWeek: 1, EndWeek: 5, WeekType: model.WeekTypeEven, StartSection: 5, EndSection: 5},
		{Weekday: 2, StartWeek: 1, EndWeek: 5, WeekType: model.WeekTypeEvery, StartSection: 6, EndSection: 6},
	}, day, week)
	for i, section := range sections {
		var want int8
		if i < 2 {
			want = model.CheckinExempted
		}
		if section.Status != want {
			t.Errorf("section %d has status %d, want %d", section.Section, section.Status, want)
		}
	}
}

func TestAttendanceWindowBoundaries(t *testing.T) {
	day := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	section := configuredAttendanceSections([]model.SectionTime{{SectionNumber: "1", StartTime: "08:00:00"}}, day)[0]
	cases := []struct {
		name   string
		at     time.Time
		status int8
		err    error
	}{
		{"before window", section.OpensAt.Add(-time.Nanosecond), 0, ErrCheckinTooEarly},
		{"opens", section.OpensAt, model.CheckinNormal, nil},
		{"starts", section.StartTime, model.CheckinNormal, nil},
		{"late", section.StartTime.Add(time.Nanosecond), model.CheckinLate, nil},
		{"closes", section.ClosesAt, model.CheckinLate, nil},
		{"after window", section.ClosesAt.Add(time.Nanosecond), 0, ErrCheckinClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, err := checkinStatus(section, tc.at)
			if status != tc.status || !errors.Is(err, tc.err) {
				t.Fatalf("got status %d and error %v, want status %d and error %v", status, err, tc.status, tc.err)
			}
		})
	}
	section.Status = model.CheckinExempted
	if _, err := checkinStatus(section, section.StartTime); !errors.Is(err, ErrCheckinExempted) {
		t.Fatalf("check-in with a class produced %v, want %v", err, ErrCheckinExempted)
	}
}
