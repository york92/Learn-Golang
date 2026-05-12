package main

import (
	"time"
)

// User 用户模型
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Article 文章模型
type Article struct {
	ID         int       `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	AuthorID   int       `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Category   string    `json:"category"`
	CoverImage string    `json:"cover_image"`
	Tags       []string  `json:"tags"`
	Views      int       `json:"views"`
	Likes      int       `json:"likes"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Comment 评论模型
type Comment struct {
	ID         int       `json:"id"`
	ArticleID  int       `json:"article_id"`
	UserID     int       `json:"user_id"`
	Username   string    `json:"username"`
	UserAvatar string    `json:"user_avatar"`
	ParentID   *int      `json:"parent_id"`
	Content    string    `json:"content"`
	Likes      int       `json:"likes"`
	Status     string    `json:"status"`
	Replies    []Comment `json:"replies,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Notification 通知模型
type Notification struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	RelatedID int       `json:"related_id"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// Session 会话模型
type Session struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ArticleRequest 文章请求
type ArticleRequest struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Category   string   `json:"category"`
	CoverImage string   `json:"cover_image"`
	Tags       []string `json:"tags"`
}

// CommentRequest 评论请求
type CommentRequest struct {
	Content  string `json:"content"`
	ParentID *int   `json:"parent_id"`
}

// Response 统一响应结构
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
