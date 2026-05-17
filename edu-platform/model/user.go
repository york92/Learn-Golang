package model

import (
	"time"

	"gorm.io/gorm"
)

// UserRole 用户角色类型
type UserRole string

const (
	RoleStudent UserRole = "student" // 学生
	RoleTeacher UserRole = "teacher" // 教师
	RoleAdmin   UserRole = "admin"   // 管理员
)

// User 用户表（学生与教师共用，通过 Role 区分）
type User struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"              json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime"                        json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"                        json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                 json:"-"`

	// 账号信息
	Username string   `gorm:"type:varchar(64);uniqueIndex;not null"  json:"username"`
	Email    string   `gorm:"type:varchar(128);uniqueIndex;not null" json:"email"`
	Password string   `gorm:"type:varchar(256);not null"             json:"-"` // bcrypt 哈希，不序列化
	Role     UserRole `gorm:"type:varchar(16);not null;default:'student'" json:"role"`

	// 基本资料
	Avatar   string `gorm:"type:varchar(512)" json:"avatar"`
	Nickname string `gorm:"type:varchar(64)"  json:"nickname"`

	// 教师专属字段（学生为空）
	Bio        string `gorm:"type:text"        json:"bio,omitempty"`
	Title      string `gorm:"type:varchar(64)" json:"title,omitempty"`
	IsVerified bool   `gorm:"default:false"    json:"is_verified,omitempty"`

	// 关联
	CoursesAsTeacher []Course   `gorm:"foreignKey:TeacherID" json:"-"`
	Enrollments      []Progress `gorm:"foreignKey:StudentID" json:"-"`
}
