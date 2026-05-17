package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Fail 返回业务错误响应
func Fail(c *gin.Context, httpCode int, message string) {
	c.JSON(httpCode, Response{
		Code:    httpCode,
		Message: message,
	})
}

// BadRequest 400
func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, message)
}

// Unauthorized 401
func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, message)
}

// Forbidden 403
func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, message)
}

// NotFound 404
func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, message)
}

// InternalError 500
func InternalError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, message)
}
