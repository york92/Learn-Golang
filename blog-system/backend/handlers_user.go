package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

// registerHandler 用户注册
func registerHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	// 验证输入
	if req.Username == "" || req.Email == "" || req.Password == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "用户名、邮箱和密码不能为空",
		})
		return
	}

	if len(req.Password) < 6 {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "密码长度不能少于6位",
		})
		return
	}

	// 检查用户名是否已存在
	var exists int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ? OR email = ?",
		req.Username, req.Email).Scan(&exists)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "数据库错误",
		})
		return
	}

	if exists > 0 {
		respondJSON(w, http.StatusConflict, Response{
			Success: false,
			Message: "用户名或邮箱已存在",
		})
		return
	}

	// 加密密码
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "密码加密失败",
		})
		return
	}

	// 插入用户
	result, err := db.Exec(`
		INSERT INTO users (username, email, password, role) 
		VALUES (?, ?, ?, ?)
	`, req.Username, req.Email, hashedPassword, "author")

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "创建用户失败",
		})
		return
	}

	userID, _ := result.LastInsertId()

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "注册成功",
		Data: map[string]interface{}{
			"user_id": userID,
		},
	})
}

// loginHandler 用户登录
// loginHandler 用户登录
func loginHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	// 查询用户
	var user User
	var hashedPassword string
	var avatar sql.NullString // 使用 NullString 处理可能为 NULL 的字段

	err := db.QueryRow(`
		SELECT id, username, email, password, role, avatar, created_at, updated_at 
		FROM users 
		WHERE username = ? OR email = ?
	`, req.Username, req.Username).Scan(
		&user.ID, &user.Username, &user.Email, &hashedPassword,
		&user.Role, &avatar, &user.CreatedAt, &user.UpdatedAt,
	)

	// 处理 avatar 可能为 NULL 的情况
	if avatar.Valid {
		user.Avatar = avatar.String
	} else {
		user.Avatar = ""
	}

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Message: "用户名或密码错误",
		})
		return
	} else if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "数据库错误",
		})
		return
	}

	// 验证密码
	if !checkPassword(req.Password, hashedPassword) {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Message: "用户名或密码错误",
		})
		return
	}

	// 创建会话
	token, err := createSession(user.ID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "创建会话失败",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "登录成功",
		Data: map[string]interface{}{
			"token": token,
			"user":  user,
		},
	})
}

// logoutHandler 用户登出
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	db.Exec("DELETE FROM sessions WHERE token = ?", token)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "登出成功",
	})
}

// getProfileHandler 获取用户资料
func getProfileHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)

	// 重新查询以获取完整信息
	var avatar sql.NullString
	err := db.QueryRow(`
		SELECT avatar FROM users WHERE id = ?
	`, user.ID).Scan(&avatar)

	if err == nil {
		if avatar.Valid {
			user.Avatar = avatar.String
		} else {
			user.Avatar = ""
		}
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    user,
	})
}

// updateProfileHandler 更新用户资料
func updateProfileHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)

	var req struct {
		Email  string `json:"email"`
		Avatar string `json:"avatar"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	_, err := db.Exec(`
		UPDATE users 
		SET email = ?, avatar = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`, req.Email, req.Avatar, user.ID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "更新失败",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "更新成功",
	})
}
