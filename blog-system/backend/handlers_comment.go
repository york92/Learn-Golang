package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// getCommentsHandler 获取评论列表
func getCommentsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	vars := mux.Vars(r)
	articleID, _ := strconv.Atoi(vars["id"])

	rows, err := db.Query(`
		SELECT c.id, c.article_id, c.user_id, u.username, u.avatar, 
		       c.parent_id, c.content, c.likes, c.status, c.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.article_id = ? AND c.status = 'approved' AND c.parent_id IS NULL
		ORDER BY c.created_at DESC
	`, articleID)

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
		err := rows.Scan(
			&comment.ID, &comment.ArticleID, &comment.UserID, &comment.Username,
			&comment.UserAvatar, &comment.ParentID, &comment.Content,
			&comment.Likes, &comment.Status, &comment.CreatedAt,
		)
		if err != nil {
			continue
		}

		// 获取回复
		comment.Replies = getCommentReplies(comment.ID)

		comments = append(comments, comment)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    comments,
	})
}

// createCommentHandler 创建评论
func createCommentHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	vars := mux.Vars(r)
	articleID, _ := strconv.Atoi(vars["id"])

	var req CommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	if req.Content == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "评论内容不能为空",
		})
		return
	}

	result, err := db.Exec(`
		INSERT INTO comments (article_id, user_id, parent_id, content, status) 
		VALUES (?, ?, ?, ?, ?)
	`, articleID, user.ID, req.ParentID, req.Content, "approved")

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "创建失败",
		})
		return
	}

	commentID, _ := result.LastInsertId()

	// 创建通知
	var authorID int
	db.QueryRow("SELECT author_id FROM articles WHERE id = ?", articleID).Scan(&authorID)
	if authorID != user.ID {
		createNotification(authorID, "comment", 
			fmt.Sprintf("%s 评论了你的文章", user.Username), int(commentID))
	}

	// 如果是回复，通知被回复的用户
	if req.ParentID != nil {
		var parentUserID int
		db.QueryRow("SELECT user_id FROM comments WHERE id = ?", *req.ParentID).Scan(&parentUserID)
		if parentUserID != user.ID {
			createNotification(parentUserID, "reply",
				fmt.Sprintf("%s 回复了你的评论", user.Username), int(commentID))
		}
	}

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "评论成功",
		Data: map[string]interface{}{
			"comment_id": commentID,
		},
	})
}

// updateCommentHandler 更新评论
func updateCommentHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// 检查权限
	var userID int
	err := db.QueryRow("SELECT user_id FROM comments WHERE id = ?", id).Scan(&userID)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "评论不存在",
		})
		return
	}

	if user.Role != "admin" && user.ID != userID {
		respondJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "没有权限修改此评论",
		})
		return
	}

	var req CommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	_, err = db.Exec(`
		UPDATE comments SET content = ? WHERE id = ?
	`, req.Content, id)

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

// deleteCommentHandler 删除评论
func deleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// 检查权限
	var userID, articleID int
	err := db.QueryRow(`
		SELECT user_id, article_id FROM comments WHERE id = ?
	`, id).Scan(&userID, &articleID)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "评论不存在",
		})
		return
	}

	var authorID int
	db.QueryRow("SELECT author_id FROM articles WHERE id = ?", articleID).Scan(&authorID)

	if user.Role != "admin" && user.ID != userID && user.ID != authorID {
		respondJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "没有权限删除此评论",
		})
		return
	}

	_, err = db.Exec("DELETE FROM comments WHERE id = ?", id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "删除失败",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "删除成功",
	})
}

// likeCommentHandler 点赞评论
func likeCommentHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// 检查是否已点赞
	var exists int
	db.QueryRow(`
		SELECT COUNT(*) FROM likes 
		WHERE user_id = ? AND target_type = 'comment' AND target_id = ?
	`, user.ID, id).Scan(&exists)

	if exists > 0 {
		// 取消点赞
		db.Exec(`
			DELETE FROM likes 
			WHERE user_id = ? AND target_type = 'comment' AND target_id = ?
		`, user.ID, id)
		db.Exec("UPDATE comments SET likes = likes - 1 WHERE id = ?", id)
	} else {
		// 添加点赞
		db.Exec(`
			INSERT INTO likes (user_id, target_type, target_id) 
			VALUES (?, 'comment', ?)
		`, user.ID, id)
		db.Exec("UPDATE comments SET likes = likes + 1 WHERE id = ?", id)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "操作成功",
	})
}

// 辅助函数：获取评论回复
func getCommentReplies(parentID int) []Comment {
	rows, err := db.Query(`
		SELECT c.id, c.article_id, c.user_id, u.username, u.avatar, 
		       c.parent_id, c.content, c.likes, c.status, c.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.parent_id = ? AND c.status = 'approved'
		ORDER BY c.created_at ASC
	`, parentID)

	if err != nil {
		return []Comment{}
	}
	defer rows.Close()

	replies := []Comment{}
	for rows.Next() {
		var reply Comment
		rows.Scan(
			&reply.ID, &reply.ArticleID, &reply.UserID, &reply.Username,
			&reply.UserAvatar, &reply.ParentID, &reply.Content,
			&reply.Likes, &reply.Status, &reply.CreatedAt,
		)
		replies = append(replies, reply)
	}
	return replies
}
