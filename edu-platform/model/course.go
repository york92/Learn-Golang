package model

import (
	"time"

	"gorm.io/gorm"
)

// CourseStatus 课程状态
type CourseStatus string

const (
	CourseStatusDraft     CourseStatus = "draft"
	CourseStatusPublished CourseStatus = "published"
	CourseStatusArchived  CourseStatus = "archived"
)

// CourseLevel 课程难度
type CourseLevel string

const (
	CourseLevelBeginner     CourseLevel = "beginner"
	CourseLevelIntermediate CourseLevel = "intermediate"
	CourseLevelAdvanced     CourseLevel = "advanced"
)

// Course 课程表
type Course struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime"           json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"           json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                    json:"-"`

	Title       string       `gorm:"type:varchar(256);not null;index"     json:"title"`
	Description string       `gorm:"type:text"                            json:"description"`
	CoverURL    string       `gorm:"type:varchar(512)"                    json:"cover_url"`
	Price       float64      `gorm:"type:decimal(10,2);default:0"         json:"price"`
	Status      CourseStatus `gorm:"type:varchar(16);default:'draft'"     json:"status"`
	Level       CourseLevel  `gorm:"type:varchar(16);default:'beginner'"  json:"level"`

	// 冗余统计字段，提升列表查询性能
	EnrollCount  int `gorm:"default:0" json:"enroll_count"`
	ChapterCount int `gorm:"default:0" json:"chapter_count"`

	// 外键：发布该课程的教师
	TeacherID uint `gorm:"not null;index"                                                        json:"teacher_id"`
	Teacher   User `gorm:"foreignKey:TeacherID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"    json:"teacher,omitempty"`

	// 关联章节（课程删除时级联删除章节）
	Chapters []Chapter `gorm:"foreignKey:CourseID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"chapters,omitempty"`
}
