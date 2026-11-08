package repository

import (
	"StudioAttendance_backend/internal/model"
)

func Register(user model.User) {
	DB.Create(&user)
}

func FindUserByUsername(username string) (model.User,error){
	var select_user model.User
	result := DB.Where("username = ?",username).Find(&select_user)
	if result.Error != nil{
		return model.User{},result.Error
	}
	return select_user,result.Error
}

func UpdateUser(user model.User) (int,error){
	result := DB.Where("id = ?",user.ID).Save(&user)
	return int(result.RowsAffected),result.Error
}

