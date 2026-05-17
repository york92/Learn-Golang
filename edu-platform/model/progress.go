package model

import (
	"time"

	"gorm.io/gorm"
)

// ProgressStatus 学习状态
type ProgressStatus string

const (
	ProgressStatusNotStarted ProgressStatus = "not_started"
	ProgressStatusInProgress ProgressStatus = "in_progress"
	ProgressStatusCompleted  ProgressStatus = "completed"
)

// Progress 学生学习进度表
// 每条记录代表：某学生对某课程的某章节的学习状态
// 唯一约束：(student_id, chapter_id)
type Progress struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime"           json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"           json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                    json:"-"`

	Status      ProgressStatus `gorm:"type:varchar(16);default:'not_started'" json:"status"`
	WatchedSecs int            `gorm:"default:0"                              json:"watched_secs"` // 视频已观看秒数
	IsFinished  bool           `gorm:"default:false;index"                    json:"is_finished"`
	FinishedAt  *time.Time     `gorm:"default:null"                           json:"finished_at"`

	// 外键：学生
	StudentID uint `gorm:"not null;index"                                                        json:"student_id"`
	// Student   User `gorm:"foreignKey:StudentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"    json:"-"`
	Student User `gorm:"foreignKey:StudentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	// 外键：课程（冗余，方便按课程统计整体进度）
	CourseID uint   `gorm:"not null;index"                                                        json:"course_id"`
	Course   Course `gorm:"foreignKey:CourseID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"     json:"-"`

	// 外键：章节
	// ChapterID uint `gorm:"not null;index"                                                        json:"chapter_id"`
	ChapterID uint    `gorm:"not null;uniqueIndex:idx_student_chapter" json:"chapter_id"`
	Chapter   Chapter `gorm:"foreignKey:ChapterID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"   json:"-"`
}

// TableName 指定表名为 progress（GORM 默认会生成 progresses）
func (Progress) TableName() string {
	return "progress"
}
