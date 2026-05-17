package service

import (
	"time"

	"edu-platform/model"
	"edu-platform/repository"
)

// ProgressService 学习进度业务逻辑层
type ProgressService struct {
	progressRepo *repository.ProgressRepo
	courseRepo   *repository.CourseRepo
}

// NewProgressService 构造函数
func NewProgressService(pr *repository.ProgressRepo, cr *repository.CourseRepo) *ProgressService {
	return &ProgressService{progressRepo: pr, courseRepo: cr}
}

// UpdateProgressInput 更新进度参数
type UpdateProgressInput struct {
	ChapterID   uint `json:"chapter_id"   binding:"required"`
	CourseID    uint `json:"course_id"    binding:"required"`
	WatchedSecs int  `json:"watched_secs"`
	IsFinished  bool `json:"is_finished"`
}

// UpdateProgress 更新学生章节学习进度
func (s *ProgressService) UpdateProgress(studentID uint, input *UpdateProgressInput) (*model.Progress, error) {
	status := model.ProgressStatusInProgress
	var finishedAt *time.Time

	if input.IsFinished {
		status = model.ProgressStatusCompleted
		now := time.Now()
		finishedAt = &now
	}

	p := &model.Progress{
		StudentID:   studentID,
		CourseID:    input.CourseID,
		ChapterID:   input.ChapterID,
		WatchedSecs: input.WatchedSecs,
		IsFinished:  input.IsFinished,
		Status:      status,
		FinishedAt:  finishedAt,
	}

	if err := s.progressRepo.Upsert(p); err != nil {
		return nil, err
	}
	return p, nil
}

// CourseProgressSummary 课程整体进度汇总
type CourseProgressSummary struct {
	CourseID         uint             `json:"course_id"`
	TotalChapters    int              `json:"total_chapters"`
	FinishedChapters int64            `json:"finished_chapters"`
	Percentage       float64          `json:"percentage"`
	Details          []model.Progress `json:"details"`
}

// GetCourseProgress 获取学生在某课程的整体学习进度
func (s *ProgressService) GetCourseProgress(studentID, courseID uint) (*CourseProgressSummary, error) {
	course, err := s.courseRepo.FindByID(courseID)
	if err != nil {
		return nil, err
	}

	details, err := s.progressRepo.FindByStudentAndCourse(studentID, courseID)
	if err != nil {
		return nil, err
	}

	finished, err := s.progressRepo.CountFinishedChapters(studentID, courseID)
	if err != nil {
		return nil, err
	}

	var percentage float64
	if course.ChapterCount > 0 {
		percentage = float64(finished) / float64(course.ChapterCount) * 100
	}

	return &CourseProgressSummary{
		CourseID:         courseID,
		TotalChapters:    course.ChapterCount,
		FinishedChapters: finished,
		Percentage:       percentage,
		Details:          details,
	}, nil
}
