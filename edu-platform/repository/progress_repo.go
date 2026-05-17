package repository

import (
	"edu-platform/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProgressRepo 学习进度数据访问层
type ProgressRepo struct {
	db *gorm.DB
}

// NewProgressRepo 构造函数
func NewProgressRepo(db *gorm.DB) *ProgressRepo {
	return &ProgressRepo{db: db}
}

// Upsert 创建或更新进度（同一学生同一章节只保留一条记录）
func (r *ProgressRepo) Upsert(p *model.Progress) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "student_id"}, {Name: "chapter_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "watched_secs", "is_finished", "finished_at"}),
	}).Create(p).Error
}

// FindByStudentAndCourse 查询学生在某课程下所有章节的进度
func (r *ProgressRepo) FindByStudentAndCourse(studentID, courseID uint) ([]model.Progress, error) {
	var list []model.Progress
	err := r.db.Where("student_id = ? AND course_id = ?", studentID, courseID).
		Preload("Chapter").
		Find(&list).Error
	return list, err
}

// FindByStudentAndChapter 查询单个章节进度
func (r *ProgressRepo) FindByStudentAndChapter(studentID, chapterID uint) (*model.Progress, error) {
	var p model.Progress
	err := r.db.Where("student_id = ? AND chapter_id = ?", studentID, chapterID).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CountFinishedChapters 统计学生在某课程已完成章节数
func (r *ProgressRepo) CountFinishedChapters(studentID, courseID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Progress{}).
		Where("student_id = ? AND course_id = ? AND is_finished = ?", studentID, courseID, true).
		Count(&count).Error
	return count, err
}
