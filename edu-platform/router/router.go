package router

import (
	"edu-platform/handler"
	"edu-platform/middleware"
	"edu-platform/repository"
	"edu-platform/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register 注册所有路由
func Register(r *gin.Engine, db *gorm.DB) {
	// ── 全局中间件 ──────────────────────────────────────────────
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	// ── 依赖注入：Repository → Service → Handler ────────────────
	userRepo     := repository.NewUserRepo(db)
	courseRepo   := repository.NewCourseRepo(db)
	progressRepo := repository.NewProgressRepo(db)

	userSvc     := service.NewUserService(userRepo)
	courseSvc   := service.NewCourseService(courseRepo)
	progressSvc := service.NewProgressService(progressRepo, courseRepo)

	userH     := handler.NewUserHandler(userSvc)
	courseH   := handler.NewCourseHandler(courseSvc)
	progressH := handler.NewProgressHandler(progressSvc)

	// ── API 路由组 ───────────────────────────────────────────────
	api := r.Group("/api/v1")

	// 公开接口（无需鉴权）
	auth := api.Group("/auth")
	{
		auth.POST("/register", userH.Register) // 注册
		auth.POST("/login", userH.Login)       // 登录
	}

	// 课程列表/详情（公开）
	api.GET("/courses", courseH.List)        // 获取课程列表
	api.GET("/courses/:id", courseH.Get)     // 获取课程详情

	// 需要登录的接口
	authed := api.Group("")
	authed.Use(middleware.JWTAuth())
	{
		// 用户
		authed.GET("/users/me", userH.GetProfile)

		// 教师专属
		teacher := authed.Group("")
		teacher.Use(middleware.RequireRole("teacher", "admin"))
		{
			teacher.POST("/courses", courseH.Create)                    // 创建课程
			teacher.PATCH("/courses/:id/publish", courseH.Publish)      // 发布课程
			teacher.GET("/teachers/me/courses", courseH.MyCoursesAsTeacher) // 我的课程
		}

		// 学生专属
		student := authed.Group("")
		student.Use(middleware.RequireRole("student", "admin"))
		{
			student.POST("/progress", progressH.UpdateProgress)                           // 更新学习进度
			student.GET("/progress/courses/:course_id", progressH.GetCourseProgress)      // 查询课程进度
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
