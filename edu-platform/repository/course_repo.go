package repository

import (
	"edu-platform/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CourseRepo 课程数据访问层
type CourseRepo struct {
	db *gorm.DB
}

// NewCourseRepo 构造函数
func NewCourseRepo(db *gorm.DB) *CourseRepo {
	return &CourseRepo{db: db}
}

// Create 创建课程
func (r *CourseRepo) Create(course *model.Course) error {
	return r.db.Create(course).Error
}

// FindByID 按 ID 查询课程（预加载教师信息）
func (r *CourseRepo) FindByID(id uint) (*model.Course, error) {
	var course model.Course
	if err := r.db.Preload("Teacher").First(&course, id).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

// FindByIDWithChapters 查询课程及其章节
func (r *CourseRepo) FindByIDWithChapters(id uint) (*model.Course, error) {
	var course model.Course
	if err := r.db.
		Preload("Teacher").
		Preload("Chapters", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		First(&course, id).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

// List 分页查询已发布课程列表
func (r *CourseRepo) List(page, pageSize int) ([]model.Course, int64, error) {
	var courses []model.Course
	var total int64

	query := r.db.Model(&model.Course{}).
		Where("status = ?", model.CourseStatusPublished)

	query.Count(&total)

	err := query.
		Preload("Teacher").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&courses).Error

	return courses, total, err
}

// FindByTeacherID 查询某教师的所有课程
func (r *CourseRepo) FindByTeacherID(teacherID uint) ([]model.Course, error) {
	var courses []model.Course
	err := r.db.Where("teacher_id = ?", teacherID).
		Order("created_at DESC").
		Find(&courses).Error
	return courses, err
}

// Update 更新课程
func (r *CourseRepo) Update(course *model.Course) error {
	return r.db.Save(course).Error
}

// Delete 软删除课程
func (r *CourseRepo) Delete(id uint) error {
	return r.db.Delete(&model.Course{}, id).Error
}

// IncrementEnrollCount 报名人数 +1
func (r *CourseRepo) IncrementEnrollCount(id uint) error {
	return r.db.Model(&model.Course{}).Where("id = ?", id).
		// UpdateColumn("enroll_count", gorm.Expr("enroll_count + 1")).Error
		UpdateColumn("enroll_count", clause.Expr{SQL: "enroll_count + 1"}).Error
}
