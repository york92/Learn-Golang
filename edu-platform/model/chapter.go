package model

import (
	"time"

	"gorm.io/gorm"
)

// ChapterType 章节内容类型
type ChapterType string

const (
	ChapterTypeVideo   ChapterType = "video"
	ChapterTypeArticle ChapterType = "article"
	ChapterTypeQuiz    ChapterType = "quiz"
)

// Chapter 章节表（隶属于某门课程）
type Chapter struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime"           json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"           json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                    json:"-"`

	Title       string      `gorm:"type:varchar(256);not null"       json:"title"`
	Description string      `gorm:"type:text"                        json:"description"`
	Type        ChapterType `gorm:"type:varchar(16);default:'video'" json:"type"`

	// 视频章节字段
	VideoURL string `gorm:"type:varchar(512)" json:"video_url,omitempty"`
	Duration int    `gorm:"default:0"         json:"duration"` // 秒

	// 图文章节字段（Markdown）
	Content string `gorm:"type:text" json:"content,omitempty"`

	// 是否允许试看
	IsFree bool `gorm:"default:false" json:"is_free"`

	// 在课程中的排序（升序）
	SortOrder int `gorm:"default:0;index" json:"sort_order"`

	// 外键：所属课程
	CourseID uint   `gorm:"not null;index"                                                        json:"course_id"`
	Course   Course `gorm:"foreignKey:CourseID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"      json:"-"`
}
