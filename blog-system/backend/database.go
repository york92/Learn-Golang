package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "blog.db")
	if err != nil {
		log.Fatal(err)
	}

	// 创建用户表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			role TEXT DEFAULT 'reader',
			avatar TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建文章表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			author_id INTEGER NOT NULL,
			category TEXT,
			cover_image TEXT,
			views INTEGER DEFAULT 0,
			likes INTEGER DEFAULT 0,
			status TEXT DEFAULT 'published',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (author_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建标签表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建文章标签关联表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS article_tags (
			article_id INTEGER NOT NULL,
			tag_id INTEGER NOT NULL,
			PRIMARY KEY (article_id, tag_id),
			FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
			FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建评论表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			parent_id INTEGER,
			content TEXT NOT NULL,
			likes INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建点赞表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS likes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			target_type TEXT NOT NULL,
			target_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, target_type, target_id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建通知表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			related_id INTEGER,
			is_read INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 创建会话表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 插入默认管理员账户（如果不存在）
	insertDefaultAdmin()

	log.Println("Database initialized successfully")
}

func insertDefaultAdmin() {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		log.Printf("Error checking admin: %v", err)
		return
	}

	if count == 0 {
		hashedPassword, _ := hashPassword("admin123")
		_, err = db.Exec(`
			INSERT INTO users (username, email, password, role) 
			VALUES (?, ?, ?, ?)
		`, "admin", "admin@blog.com", hashedPassword, "admin")
		if err != nil {
			log.Printf("Error creating admin: %v", err)
		} else {
			log.Println("Default admin created: username=admin, password=admin123")
		}
	}
}
