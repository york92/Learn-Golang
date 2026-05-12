package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

func main() {
	// 初始化数据库
	initDB()
	defer db.Close()

	// 创建路由
	r := mux.NewRouter()

	// API路由
	api := r.PathPrefix("/api").Subrouter()

	// 用户相关路由
	api.HandleFunc("/register", registerHandler).Methods("POST")
	api.HandleFunc("/login", loginHandler).Methods("POST")
	api.HandleFunc("/logout", authMiddleware(logoutHandler)).Methods("POST")
	api.HandleFunc("/profile", authMiddleware(getProfileHandler)).Methods("GET")
	api.HandleFunc("/profile", authMiddleware(updateProfileHandler)).Methods("PUT")

	// 文章相关路由
	api.HandleFunc("/articles", getArticlesHandler).Methods("GET")
	api.HandleFunc("/articles", authMiddleware(createArticleHandler)).Methods("POST")
	api.HandleFunc("/articles/{id}", getArticleHandler).Methods("GET")
	api.HandleFunc("/articles/{id}", authMiddleware(updateArticleHandler)).Methods("PUT")
	api.HandleFunc("/articles/{id}", authMiddleware(deleteArticleHandler)).Methods("DELETE")
	api.HandleFunc("/articles/{id}/like", authMiddleware(likeArticleHandler)).Methods("POST")

	// 评论相关路由
	api.HandleFunc("/articles/{id}/comments", getCommentsHandler).Methods("GET")
	api.HandleFunc("/articles/{id}/comments", authMiddleware(createCommentHandler)).Methods("POST")
	api.HandleFunc("/comments/{id}", authMiddleware(updateCommentHandler)).Methods("PUT")
	api.HandleFunc("/comments/{id}", authMiddleware(deleteCommentHandler)).Methods("DELETE")
	api.HandleFunc("/comments/{id}/like", authMiddleware(likeCommentHandler)).Methods("POST")

	// 分类和标签路由
	api.HandleFunc("/categories", getCategoriesHandler).Methods("GET")
	api.HandleFunc("/tags", getTagsHandler).Methods("GET")

	// 搜索路由
	api.HandleFunc("/search", searchHandler).Methods("GET")

	// 通知路由
	api.HandleFunc("/notifications", authMiddleware(getNotificationsHandler)).Methods("GET")
	api.HandleFunc("/notifications/{id}/read", authMiddleware(markNotificationReadHandler)).Methods("PUT")

	// 管理员路由
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(func(next http.Handler) http.Handler {
		return authMiddleware(func(w http.ResponseWriter, r *http.Request) {
			adminMiddleware(next).ServeHTTP(w, r)
		})
	})
	admin.HandleFunc("/users", getUsersHandler).Methods("GET")
	admin.HandleFunc("/users/{id}", updateUserRoleHandler).Methods("PUT")
	admin.HandleFunc("/comments/pending", getPendingCommentsHandler).Methods("GET")
	admin.HandleFunc("/comments/{id}/approve", approveCommentHandler).Methods("PUT")

	// 静态文件和SPA路由
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API 请求已经被上面的路由处理了
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// 处理静态资源
		if path == "/styles.css" {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			http.ServeFile(w, r, "../frontend/styles.css")
			return
		}

		if path == "/app.js" {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			http.ServeFile(w, r, "../frontend/app.js")
			return
		}

		// 所有其他请求返回 index.html (SPA)
		http.ServeFile(w, r, "../frontend/index.html")
	})

	// 启动服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	log.Printf("Visit: http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
