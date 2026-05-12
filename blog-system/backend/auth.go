package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userContextKey contextKey = "user"

// hashPassword 加密密码
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// checkPassword 验证密码
func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// generateToken 生成随机token
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// createSession 创建会话
func createSession(userID int) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(24 * time.Hour * 7) // 7天过期

	_, err = db.Exec(`
		INSERT INTO sessions (user_id, token, expires_at) 
		VALUES (?, ?, ?)
	`, userID, token, expiresAt)

	return token, err
}

// validateSession 验证会话
func validateSession(token string) (*User, error) {
	var user User
	var avatar sql.NullString
	var expiresAt time.Time

	err := db.QueryRow(`
		SELECT u.id, u.username, u.email, u.role, u.avatar, u.created_at, u.updated_at, s.expires_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ?
	`, token).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &avatar,
		&user.CreatedAt, &user.UpdatedAt, &expiresAt)

	if err != nil {
		log.Printf("Session validation error for token %s: %v", token[:10]+"...", err)
		return nil, err
	}

	if avatar.Valid {
		user.Avatar = avatar.String
	}

	if time.Now().After(expiresAt) {
		db.Exec("DELETE FROM sessions WHERE token = ?", token)
		log.Printf("Session expired for user %d", user.ID)
		return nil, sql.ErrNoRows
	}

	return &user, nil
}

// authMiddleware 认证中间件
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondJSON(w, http.StatusUnauthorized, Response{
				Success: false,
				Message: "未授权访问",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		user, err := validateSession(token)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, Response{
				Success: false,
				Message: "会话无效或已过期",
			})
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

// adminMiddleware 管理员权限中间件
func adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(userContextKey).(*User)
		if user.Role != "admin" {
			respondJSON(w, http.StatusForbidden, Response{
				Success: false,
				Message: "需要管理员权限",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getUserFromContext 从上下文获取用户
func getUserFromContext(r *http.Request) *User {
	user, ok := r.Context().Value(userContextKey).(*User)
	if !ok {
		return nil
	}
	return user
}

// respondJSON 返回JSON响应
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// enableCORS 启用CORS
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}
