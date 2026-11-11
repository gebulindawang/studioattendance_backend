package repository

import (
	"StudioAttendance_backend/internal/model"

	"golang.org/x/crypto/bcrypt"
)

func Register(user model.User,userID int)error {
	secret_password,err := bcrypt.GenerateFromPassword([]byte(user.Password),bcrypt.DefaultCost)
	if err  != nil{
		return  err
	}
	user.CreateBy = &userID
	user.UpdateBy = &userID
	user.Password = string(secret_password)
	DB.Create(&user)
	return nil
}

func FindUserByUsername(username string) (model.User, error) {
	var select_user model.User
	result := DB.Where("username = ?", username).First(&select_user)
	if result.Error != nil {
		return model.User{}, result.Error
	}
	return select_user, result.Error
}

func FindUserByID(id int) (model.User, error) {
	var select_user model.User
	result := DB.Where("id = ?", id).First(&select_user)
	if result.Error != nil {
		return model.User{}, result.Error
	}
	return select_user, result.Error
}

func UpdateUser(user model.User) (int, error) {
	result := DB.Where("id = ?", user.ID).Save(&user)
	return int(result.RowsAffected), result.Error
}

func ListUsers() ([]model.User, error) {
	var users []model.User
	result := DB.Find(&users)
	return users, result.Error
}
