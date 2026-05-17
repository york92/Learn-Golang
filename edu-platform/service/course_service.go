package service

import (
	"errors"

	"edu-platform/model"
	"edu-platform/repository"
)

// CourseService 课程业务逻辑层
type CourseService struct {
	courseRepo *repository.CourseRepo
}

// NewCourseService 构造函数
func NewCourseService(courseRepo *repository.CourseRepo) *CourseService {
	return &CourseService{courseRepo: courseRepo}
}

// CreateCourseInput 创建课程参数
type CreateCourseInput struct {
	Title       string  `json:"title"       binding:"required,max=256"`
	Description string  `json:"description"`
	CoverURL    string  `json:"cover_url"`
	Price       float64 `json:"price"`
	Level       string  `json:"level"`
}

// CreateCourse 创建课程（仅教师可调用，权限在 handler 层验证）
func (s *CourseService) CreateCourse(teacherID uint, input *CreateCourseInput) (*model.Course, error) {
	level := model.CourseLevel(input.Level)
	if level != model.CourseLevelBeginner &&
		level != model.CourseLevelIntermediate &&
		level != model.CourseLevelAdvanced {
		level = model.CourseLevelBeginner
	}

	course := &model.Course{
		Title:       input.Title,
		Description: input.Description,
		CoverURL:    input.CoverURL,
		Price:       input.Price,
		Level:       level,
		Status:      model.CourseStatusDraft,
		TeacherID:   teacherID,
	}

	if err := s.courseRepo.Create(course); err != nil {
		return nil, errors.New("failed to create course")
	}
	return course, nil
}

// GetCourse 获取课程详情（含章节）
func (s *CourseService) GetCourse(id uint) (*model.Course, error) {
	return s.courseRepo.FindByIDWithChapters(id)
}

// ListCourses 分页获取已发布课程
func (s *CourseService) ListCourses(page, pageSize int) ([]model.Course, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	return s.courseRepo.List(page, pageSize)
}

// PublishCourse 发布课程（只有课程所属教师可操作）
func (s *CourseService) PublishCourse(courseID, teacherID uint) error {
	course, err := s.courseRepo.FindByID(courseID)
	if err != nil {
		return errors.New("course not found")
	}
	if course.TeacherID != teacherID {
		return errors.New("no permission to publish this course")
	}
	course.Status = model.CourseStatusPublished
	return s.courseRepo.Update(course)
}

// GetTeacherCourses 获取教师的所有课程
func (s *CourseService) GetTeacherCourses(teacherID uint) ([]model.Course, error) {
	return s.courseRepo.FindByTeacherID(teacherID)
}
