package repository

import (
	"StudioAttendance_backend/internal/model"

	"gorm.io/gorm"
)

func FindScheduleImageByID(userId int) ([]string, error) {
	var ScheduleImages []model.ScheduleImage
	result := DB.Where("user_id = ?", userId).Find(&ScheduleImages)
	if result.Error != nil {
		return nil, result.Error
	}
	var URLS []string
	for _, scheduleImage := range ScheduleImages {
		URLS = append(URLS, scheduleImage.ImagesUrl)
	}
	return URLS, nil
}

// 查询最近一次课表的URL
func FindLatestScheduleImage(userID int) (string, error) {
	var scheduleImage model.ScheduleImage
	result := DB.Where("user_id = ?", userID).Order("id DESC").First(&scheduleImage).Limit(1)
	if result.Error != nil {
		return "", result.Error
	}
	return scheduleImage.ImagesUrl, nil
}

// 动态查询课表
func ListSchedules(userID *int) ([]model.Schedule, error) {
	var schedules []model.Schedule
	query := DB.Model(&model.Schedule{})
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	result := query.Find(&schedules)
	return schedules, result.Error
}

func CreateSchedules(schedules []model.Schedule) error {
	result := DB.Create(&schedules)
	return result.Error
}

// ReplaceUserSchedules 用给定课程整体替换某用户的课表(先删后插,事务保证原子性)。
// 归属 user_id 与自增 id 均由服务端重置,不信任请求体里的值。
func ReplaceUserSchedules(userID int, schedules []model.Schedule) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.Schedule{}).Error; err != nil {
			return err
		}
		for i := range schedules {
			schedules[i].ID = 0
			schedules[i].UserID = userID
		}
		return tx.Create(&schedules).Error
	})
}

func UpdateSchedules(schedule model.Schedule) error {
	return DB.Save(&schedule).Error
}

func CreateScheduleImage(url string, userID int) error {
	var ScheduleImage model.ScheduleImage = model.ScheduleImage{UserID: userID, ImagesUrl: url}
	result := DB.Create(&ScheduleImage)
	return result.Error
}

func FindImagesAmountByUserId(userId int) (int, error) {
	var count int64 
	err := DB.Model(&model.ScheduleImage{}).
	Where("user_id = ?", userId ).
	Count(&count)
	return int(count),err.Error
}

func DeleteScheduleByUserId(userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userId).Delete(&model.Schedule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userId).Delete(&model.ScheduleImage{}).Error; err != nil {
			return err
		}
		return nil
	})
}
