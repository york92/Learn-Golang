package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// getArticlesHandler 获取文章列表
func getArticlesHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	category := r.URL.Query().Get("category")
	tag := r.URL.Query().Get("tag")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 10
	offset := (page - 1) * limit

	query := `
		SELECT DISTINCT a.id, a.title, a.content, a.author_id, u.username, 
		       a.category, a.cover_image, a.views, a.likes, a.status, 
		       a.created_at, a.updated_at
		FROM articles a
		JOIN users u ON a.author_id = u.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
		WHERE a.status = 'published'
	`

	args := []interface{}{}

	if category != "" {
		query += " AND a.category = ?"
		args = append(args, category)
	}

	if tag != "" {
		query += " AND t.name = ?"
		args = append(args, tag)
	}

	query += " ORDER BY a.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "查询失败",
		})
		return
	}
	defer rows.Close()

	articles := []Article{}
	for rows.Next() {
		var article Article
		err := rows.Scan(
			&article.ID, &article.Title, &article.Content, &article.AuthorID,
			&article.AuthorName, &article.Category, &article.CoverImage,
			&article.Views, &article.Likes, &article.Status,
			&article.CreatedAt, &article.UpdatedAt,
		)
		if err != nil {
			continue
		}

		// 获取文章标签
		article.Tags = getArticleTags(article.ID)

		// 截断内容为摘要
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

// getArticleHandler 获取单篇文章
func getArticleHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var article Article
	err := db.QueryRow(`
		SELECT a.id, a.title, a.content, a.author_id, u.username, 
		       a.category, a.cover_image, a.views, a.likes, a.status, 
		       a.created_at, a.updated_at
		FROM articles a
		JOIN users u ON a.author_id = u.id
		WHERE a.id = ?
	`, id).Scan(
		&article.ID, &article.Title, &article.Content, &article.AuthorID,
		&article.AuthorName, &article.Category, &article.CoverImage,
		&article.Views, &article.Likes, &article.Status,
		&article.CreatedAt, &article.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "文章不存在",
		})
		return
	} else if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "查询失败",
		})
		return
	}

	// 获取标签
	article.Tags = getArticleTags(article.ID)

	// 增加浏览量
	db.Exec("UPDATE articles SET views = views + 1 WHERE id = ?", id)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    article,
	})
}

// createArticleHandler 创建文章
func createArticleHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	if user.Role != "admin" && user.Role != "author" {
		respondJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "没有权限创建文章",
		})
		return
	}

	var req ArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	if req.Title == "" || req.Content == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "标题和内容不能为空",
		})
		return
	}

	result, err := db.Exec(`
		INSERT INTO articles (title, content, author_id, category, cover_image, status) 
		VALUES (?, ?, ?, ?, ?, ?)
	`, req.Title, req.Content, user.ID, req.Category, req.CoverImage, "published")

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "创建失败",
		})
		return
	}

	articleID, _ := result.LastInsertId()

	// 添加标签
	for _, tagName := range req.Tags {
		addTagToArticle(int(articleID), tagName)
	}

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "创建成功",
		Data: map[string]interface{}{
			"article_id": articleID,
		},
	})
}

// updateArticleHandler 更新文章
func updateArticleHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// 检查权限
	var authorID int
	err := db.QueryRow("SELECT author_id FROM articles WHERE id = ?", id).Scan(&authorID)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "文章不存在",
		})
		return
	}

	if user.Role != "admin" && user.ID != authorID {
		respondJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "没有权限修改此文章",
		})
		return
	}

	var req ArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "无效的请求数据",
		})
		return
	}

	_, err = db.Exec(`
		UPDATE articles 
		SET title = ?, content = ?, category = ?, cover_image = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`, req.Title, req.Content, req.Category, req.CoverImage, id)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "更新失败",
		})
		return
	}

	// 更新标签
	db.Exec("DELETE FROM article_tags WHERE article_id = ?", id)
	for _, tagName := range req.Tags {
		addTagToArticle(id, tagName)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "更新成功",
	})
}

// deleteArticleHandler 删除文章
func deleteArticleHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	user := getUserFromContext(r)
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// 检查权限
	var authorID int
	err := db.QueryRow("SELECT author_id FROM articles WHERE id = ?", id).Scan(&authorID)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "文章不存在",
		})
		return
	}

	if user.Role != "admin" && user.ID != authorID {
		respondJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "没有权限删除此文章",
		})
		return
	}

	_, err = db.Exec("DELETE FROM articles WHERE id = ?", id)
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

// likeArticleHandler 点赞文章
func likeArticleHandler(w http.ResponseWriter, r *http.Request) {
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
		WHERE user_id = ? AND target_type = 'article' AND target_id = ?
	`, user.ID, id).Scan(&exists)

	if exists > 0 {
		// 取消点赞
		db.Exec(`
			DELETE FROM likes 
			WHERE user_id = ? AND target_type = 'article' AND target_id = ?
		`, user.ID, id)
		db.Exec("UPDATE articles SET likes = likes - 1 WHERE id = ?", id)
	} else {
		// 添加点赞
		db.Exec(`
			INSERT INTO likes (user_id, target_type, target_id) 
			VALUES (?, 'article', ?)
		`, user.ID, id)
		db.Exec("UPDATE articles SET likes = likes + 1 WHERE id = ?", id)
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "操作成功",
	})
}

// 辅助函数：获取文章标签
func getArticleTags(articleID int) []string {
	rows, err := db.Query(`
		SELECT t.name FROM tags t
		JOIN article_tags at ON t.id = at.tag_id
		WHERE at.article_id = ?
	`, articleID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var tag string
		rows.Scan(&tag)
		tags = append(tags, tag)
	}
	return tags
}

// 辅助函数：为文章添加标签
func addTagToArticle(articleID int, tagName string) {
	if tagName == "" {
		return
	}

	var tagID int
	err := db.QueryRow("SELECT id FROM tags WHERE name = ?", tagName).Scan(&tagID)
	if err == sql.ErrNoRows {
		result, _ := db.Exec("INSERT INTO tags (name) VALUES (?)", tagName)
		id, _ := result.LastInsertId()
		tagID = int(id)
	}

	db.Exec(`
		INSERT OR IGNORE INTO article_tags (article_id, tag_id) 
		VALUES (?, ?)
	`, articleID, tagID)
}
