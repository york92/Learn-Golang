package handler

import (
	"net/http"

	"edu-platform/middleware"
	"edu-platform/service"
	"edu-platform/utils"

	"github.com/gin-gonic/gin"
)

// UserHandler 用户 HTTP 处理器
type UserHandler struct {
	svc *service.UserService
}

// NewUserHandler 构造函数
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Register godoc
// POST /api/v1/auth/register
func (h *UserHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	resp, err := h.svc.Register(&input)
	if err != nil {
		utils.Fail(c, http.StatusConflict, err.Error())
		return
	}

	utils.Success(c, resp)
}

// Login godoc
// POST /api/v1/auth/login
func (h *UserHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	resp, err := h.svc.Login(&input)
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}

	utils.Success(c, resp)
}

// GetProfile godoc
// GET /api/v1/users/me  （需要 JWT）
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, ok := middleware.GetCurrentUserID(c)
	if !ok {
		utils.Unauthorized(c, "not authenticated")
		return
	}

	user, err := h.svc.GetProfile(userID)
	if err != nil {
		utils.NotFound(c, "user not found")
		return
	}

	utils.Success(c, user)
}
