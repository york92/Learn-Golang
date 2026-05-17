package handler

import (
	"strconv"

	"edu-platform/middleware"
	"edu-platform/service"
	"edu-platform/utils"

	"github.com/gin-gonic/gin"
)

// CourseHandler 课程 HTTP 处理器
type CourseHandler struct {
	svc *service.CourseService
}

// NewCourseHandler 构造函数
func NewCourseHandler(svc *service.CourseService) *CourseHandler {
	return &CourseHandler{svc: svc}
}

// List godoc
// GET /api/v1/courses?page=1&page_size=10
func (h *CourseHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	courses, total, err := h.svc.ListCourses(page, pageSize)
	if err != nil {
		utils.InternalError(c, "failed to fetch courses")
		return
	}

	utils.Success(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"courses":   courses,
	})
}

// Get godoc
// GET /api/v1/courses/:id
func (h *CourseHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid course id")
		return
	}

	course, err := h.svc.GetCourse(uint(id))
	if err != nil {
		utils.NotFound(c, "course not found")
		return
	}

	utils.Success(c, course)
}

// Create godoc
// POST /api/v1/courses  （需要 teacher 角色）
func (h *CourseHandler) Create(c *gin.Context) {
	teacherID, ok := middleware.GetCurrentUserID(c)
	if !ok {
		utils.Unauthorized(c, "not authenticated")
		return
	}

	var input service.CreateCourseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	course, err := h.svc.CreateCourse(teacherID, &input)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}

	utils.Success(c, course)
}

// Publish godoc
// PATCH /api/v1/courses/:id/publish  （需要 teacher 角色）
func (h *CourseHandler) Publish(c *gin.Context) {
	teacherID, ok := middleware.GetCurrentUserID(c)
	if !ok {
		utils.Unauthorized(c, "not authenticated")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid course id")
		return
	}

	if err := h.svc.PublishCourse(uint(id), teacherID); err != nil {
		utils.Fail(c, 403, err.Error())
		return
	}

	utils.Success(c, gin.H{"message": "course published"})
}

// MyCoursesAsTeacher godoc
// GET /api/v1/teachers/me/courses
func (h *CourseHandler) MyCoursesAsTeacher(c *gin.Context) {
	teacherID, ok := middleware.GetCurrentUserID(c)
	if !ok {
		utils.Unauthorized(c, "not authenticated")
		return
	}

	courses, err := h.svc.GetTeacherCourses(teacherID)
	if err != nil {
		utils.InternalError(c, "failed to fetch courses")
		return
	}

	utils.Success(c, courses)
}
