package repository

import (
	"StudioAttendance_backend/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Register(user model.User) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Role = model.RoleMember
	user.Status = model.StatusActive
	user.Password = string(passwordHash)
	return DB.Create(&user).Error
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

func UpdateMemberStatus(id int, status int8, adminID int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.User{}).Where("id = ? AND role = ?", id, model.RoleMember).
			Updates(map[string]any{"status": status, "update_by": adminID})
		if result.Error != nil || result.RowsAffected > 0 {
			return result.Error
		}
		var member model.User
		return tx.Where("id = ? AND role = ?", id, model.RoleMember).Take(&member).Error
	})
}

func ListUsers() ([]model.UserPO, error) {
	users := make([]model.UserPO, 0)
	// 成员管理页需要看到全部账号(含 role=0 的成员和禁用的),不按角色过滤
	result := DB.Omit("password").Order("id").Find(&users)
	return users, result.Error
}
