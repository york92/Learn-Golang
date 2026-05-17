package handler

import (
	"strconv"

	"edu-platform/middleware"
	"edu-platform/service"
	"edu-platform/utils"

	"github.com/gin-gonic/gin"
)

// ProgressHandler 学习进度 HTTP 处理器
type ProgressHandler struct {
	svc *service.ProgressService
}

// NewProgressHandler 构造函数
func NewProgressHandler(svc *service.ProgressService) *ProgressHandler {
	return &ProgressHandler{svc: svc}
}

// UpdateProgress godoc
// POST /api/v1/progress  （需要 student 角色）
func (h *ProgressHandler) UpdateProgress(c *gin.Context) {
	studentID, ok := middleware.GetCurrentUserID(c)
	if !ok {
		utils.Unauthorized(c, "not authenticated")
		return
	}

	var input service.UpdateProgressInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	p, err := h.svc.UpdateProgress(studentID, &input)
	if err != nil {
		utils.InternalError(c, "failed to update progress")
		return
	}

	utils.Success(c, p)
}

// GetCourseProgress godoc
// GET /api/v1/progress/courses/:course_id
func (h *ProgressHandler) GetCourseProgress(c *gin.Context) {
	studentID, ok := middleware.GetCurrentUserID(c)
	if !ok {
		utils.Unauthorized(c, "not authenticated")
		return
	}

	courseID, err := strconv.ParseUint(c.Param("course_id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid course id")
		return
	}

	summary, err := h.svc.GetCourseProgress(studentID, uint(courseID))
	if err != nil {
		utils.InternalError(c, "failed to fetch progress")
		return
	}

	utils.Success(c, summary)
}
