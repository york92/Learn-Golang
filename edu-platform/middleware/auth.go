package middleware

import (
	"strings"

	"edu-platform/config"
	"edu-platform/utils"

	"github.com/gin-gonic/gin"
)

const claimsKey = "claims"

// JWTAuth JWT 鉴权中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			utils.Unauthorized(c, "missing or invalid authorization header")
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := utils.ParseToken(tokenStr, config.AppConfig.JWT.Secret)
		if err != nil {
			utils.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		// 将 claims 存入上下文，后续 handler 可直接取用
		c.Set(claimsKey, claims)
		c.Next()
	}
}

// RequireRole 角色鉴权中间件（需在 JWTAuth 之后使用）
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(claimsKey)
		if !exists {
			utils.Unauthorized(c, "not authenticated")
			c.Abort()
			return
		}

		userClaims, ok := claims.(*utils.Claims)
		if !ok {
			utils.Unauthorized(c, "invalid claims")
			c.Abort()
			return
		}

		for _, role := range roles {
			if userClaims.Role == role {
				c.Next()
				return
			}
		}

		utils.Forbidden(c, "insufficient permissions")
		c.Abort()
	}
}

// GetCurrentUserID 从上下文中获取当前用户 ID（供 handler 调用）
func GetCurrentUserID(c *gin.Context) (uint, bool) {
	claims, exists := c.Get(claimsKey)
	if !exists {
		return 0, false
	}
	userClaims, ok := claims.(*utils.Claims)
	if !ok {
		return 0, false
	}
	return userClaims.UserID, true
}
