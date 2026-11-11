package repository

import (
	"StudioAttendance_backend/internal/model"
)

func FindScheduleImageByID(id int) (string, error) {
	var ScheduleImage model.ScheduleImage
	result := DB.Where("id = ?", id).Find(&ScheduleImage)
	if result.Error != nil {
		return "", result.Error
	}
	return ScheduleImage.ImagesUrl, nil
}

func FindLatestScheduleImage(userID int) (string, error) {
	var scheduleImage model.ScheduleImage
	result := DB.Where("user_id = ?", userID).Order("id DESC").First(&scheduleImage)
	if result.Error != nil {
		return "", result.Error
	}
	return scheduleImage.ImagesUrl, nil
}

func CreateScheduleImage(url string, userID int) error {
	var ScheduleImage model.ScheduleImage = model.ScheduleImage{UserID: userID, ImagesUrl: url}
	result := DB.Create(&ScheduleImage)
	return result.Error
}
