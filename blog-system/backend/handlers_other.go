package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// getCategoriesHandler 获取分类列表
func getCategoriesHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	rows, err := db.Query(`
		SELECT DISTINCT category FROM articles 
		WHERE category IS NOT NULL AND category != ''
		ORDER BY category
	`)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "查询失败",
		})
		return
	}
	defer rows.Close()

	categories := []string{}
	for rows.Next() {
		var category string
		rows.Scan(&category)
		categories = append(categories, category)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    categories,
	})
}

// getTagsHandler 获取标签列表
func getTagsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	rows, err := db.Query("SELECT id, name FROM tags ORDER BY name")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "查询失败",
		})
		return
	}
	defer rows.Close()

	tags := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var name string
		rows.Scan(&id, &name)
		tags = append(tags, map[string]interface{}{
			"id":   id,
			"name": name,
		})
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    tags,
	})
}

// searchHandler 搜索
func searchHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	keyword := r.URL.Query().Get("q")
	if keyword == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "搜索关键词不能为空",
		})
		return
	}

	searchPattern := "%" + keyword + "%"

	rows, err := db.Query(`
		SELECT DISTINCT a.id, a.title, a.content, a.author_id, u.username, 
		       a.category, a.cover_image, a.views, a.likes, a.status, 
		       a.created_at, a.updated_at
		FROM articles a
		JOIN users u ON a.author_id = u.id
		WHERE a.status = 'published' 
		  AND (a.title LIKE ? OR a.content LIKE ? OR u.username LIKE ?)
		ORDER BY a.created_at DESC
		LIMIT 20
	`, searchPattern, searchPattern, searchPattern)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "搜索失败",
		})
		return
	}
	defer rows.Close()

	articles := []Article{}
	for rows.Next() {
		var article Article
		rows.Scan(
			&article.ID, &article.Title, &article.Content, &article.AuthorID,
			&article.AuthorName, &article.Category, &article.CoverImage,
			&article.Views, &article.Likes, &article.Status,
			&article.CreatedAt, &article.UpdatedAt,
		)

		// 获取标签
		article.Tags = getArticleTags(article.ID)

		// 截断内容
		if len(article.Content) > 200 {
			article.Content = article.Content[:200] + "..."
		}

		articles = append(articles, article)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    articles,
	})
}

// getNotificationsHandler 获取通知列表
func getNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)

	rows, err := db.Query(`
		SELECT id, user_id, type, content, related_id, is_read, created_at
		FROM notifications
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 50
	`, user.ID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "查询失败",
		})
		return
	}
	defer rows.Close()

	notifications := []Notification{}
	for rows.Next() {
		var notif Notification
		var isRead int
		rows.Scan(
			&notif.ID, &notif.UserID, &notif.Type, &notif.Content,
			&notif.RelatedID, &isRead, &notif.CreatedAt,
		)
		notif.IsRead = isRead == 1
		notifications = append(notifications, notif)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    notifications,
	})
}

// markNotificationReadHandler 标记通知已读
func markNotificationReadHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	_, err := db.Exec(`
		UPDATE notifications SET is_read = 1 
		WHERE id = ? AND user_id = ?
	`, id, user.ID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "操作失败",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "标记成功",
	})
}

// getUsersHandler 获取用户列表（管理员）
func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	rows, err := db.Query(`
		SELECT id, username, email, role, avatar, created_at
		FROM users
		ORDER BY created_at DESC
	`)

	if err != nil {
		log.Printf("Query users error: %v", err)
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "查询失败",
		})
		return
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var user User
		var avatar sql.NullString
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &avatar, &user.CreatedAt)
		if err != nil {
			log.Printf("Scan user error: %v", err)
			continue
		}
		if avatar.Valid {
			user.Avatar = avatar.String
		}
		users = append(users, user)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    users,
	})
}

// updateUserRoleHandler 更新用户角色（管理员）
func updateUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var req struct {
		Role string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	if req.Role != "admin" && req.Role != "author" && req.Role != "reader" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的角色",
		})
		return
	}

	_, err := db.Exec("UPDATE users SET role = ? WHERE id = ?", req.Role, id)
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

// getPendingCommentsHandler 获取待审核评论（管理员）
func getPendingCommentsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	rows, err := db.Query(`
		SELECT c.id, c.article_id, c.user_id, u.username, u.avatar, 
		       c.parent_id, c.content, c.likes, c.status, c.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.status = 'pending'
		ORDER BY c.created_at DESC
	`)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "查询失败",
		})
		return
	}
	defer rows.Close()

	comments := []Comment{}
	for rows.Next() {
		var comment Comment
		rows.Scan(
			&comment.ID, &comment.ArticleID, &comment.UserID, &comment.Username,
			&comment.UserAvatar, &comment.ParentID, &comment.Content,
			&comment.Likes, &comment.Status, &comment.CreatedAt,
		)
		comments = append(comments, comment)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    comments,
	})
}

// approveCommentHandler 审核评论（管理员）
func approveCommentHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var req struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	if req.Status != "approved" && req.Status != "rejected" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的状态",
		})
		return
	}

	_, err := db.Exec("UPDATE comments SET status = ? WHERE id = ?", req.Status, id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "操作失败",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "操作成功",
	})
}

// createNotification 创建通知（辅助函数）
func createNotification(userID int, notifType, content string, relatedID int) {
	db.Exec(`
		INSERT INTO notifications (user_id, type, content, related_id) 
		VALUES (?, ?, ?, ?)
	`, userID, notifType, content, relatedID)
}
