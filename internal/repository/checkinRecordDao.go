package repository

import (
	"StudioAttendance_backend/internal/model"
	"errors"
	"fmt"
	"time"
)

func ListCheckinRecord() ([]model.CheckinRecord, error) {
	var records []model.CheckinRecord
	result := DB.Find(&records)
	if result.Error != nil {
		return nil, result.Error
	}
	return records, nil
}

func CheckIn(userId int, section int, week int) error {
	now := time.Now()

	// section_time.start_time 是 TIME 列(只有时刻没有日期),
	// 扫成 string 解析时分秒,再拼上今天的日期才是完整的上课时间
	var startStr string
	err := DB.Table("section_time").
		Where("section_number = ?", section).
		Select("start_time").
		Take(&startStr).Error
	if err != nil {
		return fmt.Errorf("第 %d 节课不存在", section)
	}
	t, err := time.Parse("15:04:05", startStr)
	if err != nil {
		return fmt.Errorf("解析上课时间失败: %w", err)
	}
	classTime := time.Date(now.Year(), now.Month(), now.Day(),
		t.Hour(), t.Minute(), t.Second(), 0, now.Location())

	// 同一用户同一课节当天只能签一次
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int64
	err = DB.Model(&model.CheckinRecord{}).
		Where("user_id = ? AND section = ? AND checkin_date >= ? AND checkin_date < ?",
			userId, section, todayStart, todayStart.AddDate(0, 0, 1)).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该课节已签到，请勿重复签到")
	}

	record := model.CheckinRecord{
		UserID:      uint(userId),
		Section:     section,
		CheckinTime: &now,
		CheckinDate: now,
	}

	elapsed := now.Sub(classTime)
	switch {
	case elapsed < -20 * time.Minute:
		return errors.New("未到签到时间")
	case elapsed <= 0:
		record.Status = model.CheckinNormal
	case elapsed <= 5 * time.Minute:
		record.Status = model.CheckinLate
	default:
		record.Status = model.CheckinAbsent
	}

	return DB.Create(&record).Error
}

