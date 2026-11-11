package repository

import "StudioAttendance_backend/internal/model"

func ListCheckinRecord() ([]model.CheckinRecord,error){
	var records []model.CheckinRecord
	result := DB.Find(&records)
	if result.Error != nil{
		return  nil , result.Error
	}
	return  records,nil
}