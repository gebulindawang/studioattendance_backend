package model

import (
	"time"

	"gorm.io/gorm"
)

// 用户角色
const (
	RoleMember int8 = 0 // 成员
	RoleAdmin  int8 = 1 // 管理员
)

// 用户状态
const (
	StatusDisabled int8 = 0 // 禁用
	StatusActive   int8 = 1 // 在职
)

// User 对应表 users
type User struct {
	ID        int            `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"size:64;not null" json:"username"`
	Password  string         `gorm:"size:100;not null" json:"password"` // json:"-":密码哈希永远不返回给前端
	Nickname  string         `gorm:"size:64;not null" json:"nickname"`
	Image     string         `json:"image"`
	Role      int8           `json:"role"`
	Status    int8           `json:"status"`
	CreateBy  *int           `json:"create_by"` // 可空:NULL 用指针表示
	UpdateBy  *int           `json:"update_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // gorm.DeletedAt 启用软删除,查询自动过滤已删记录
}

type UserPO struct {
	ID        int            `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"size:64;not null" json:"username"`
	Password  string         `gorm:"size:100;not null" json:"-"`
	Nickname  string         `gorm:"size:64;not null" json:"nickname"`
	Image     string         `json:"image"`
	Role      int8           `json:"role"`
	Status    int8           `json:"status"`
	CreateBy  *int           `json:"create_by"` // 可空:NULL 用指针表示
	UpdateBy  *int           `json:"update_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // gorm.DeletedAt 启用软删除,查询自动过滤已删记录
}
