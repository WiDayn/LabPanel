package main

import (
	"LabPanel/config"
	"LabPanel/handlers"
	"LabPanel/middleware"
	"LabPanel/service"
	"html"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if err := service.InitMetricsStore(cfg.MetricsDBPath, cfg.MetricsRetentionDays); err != nil {
		log.Fatalf("初始化监控数据存储失败: %v", err)
	}
	service.StartMetricsCleanup()
	service.GetLxcMetricsService().Start()
	service.GetGPUMonitorService().Start()
	service.GetHostMetricsService().Start()

	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 静态文件服务 - 服务 dist 目录下的所有静态资源
	r.Static("/assets", "./frontend/dist/assets")
	r.StaticFile("/favicon.ico", "./frontend/dist/favicon.ico")
	r.StaticFile("/vite.svg", "./frontend/dist/vite.svg")

	// 上传文件静态服务
	r.Static("/api/uploads", cfg.UploadPath)

	// API 路由必须在 NoRoute 之前定义

	// API 路由
	api := r.Group("/api")
	{
		api.GET("/public-config", handlers.GetPublicConfig)
		api.POST("/login", handlers.Login)

		// 需要鉴权的路由
		auth := api.Group("")
		auth.Use(middleware.AuthMiddleware())
		{
			auth.GET("/check", handlers.GetEnvironmentCheck)
			auth.GET("/app-config", handlers.GetAppConfig)
			auth.GET("/host-info", handlers.GetHostInfo)
			auth.GET("/host/metrics", handlers.GetHostMetrics)
			auth.PUT("/app-config", handlers.UpdateAppConfig)
			auth.GET("/config", handlers.GetConfig)
			auth.PUT("/config", handlers.UpdateConfig)
			auth.POST("/restart", handlers.RestartService)
			auth.GET("/status", handlers.GetServiceStatus)
			auth.GET("/gpu/monitor", handlers.GetGPUMonitor)

			// 代理管理
			auth.GET("/proxies", handlers.GetProxyList)
			auth.POST("/proxies", handlers.AddProxy)
			auth.PUT("/proxies", handlers.UpdateProxy)
			auth.DELETE("/proxies", handlers.DeleteProxy)

			// 基础配置更新
			auth.PUT("/config/base", handlers.UpdateBaseConfig)

			// LXC容器管理
			auth.GET("/lxc/list", handlers.GetLxcList)
			auth.GET("/lxc/groups", handlers.GetLxcGroups)
			auth.POST("/lxc/groups", handlers.CreateLxcGroup)
			auth.PUT("/lxc/groups/container", handlers.UpdateLxcContainerGroups)
			auth.POST("/lxc/create", handlers.CreateLxcContainer)
			auth.DELETE("/lxc/delete", handlers.DeleteLxcContainer)
			auth.POST("/lxc/restart", handlers.RestartLxcContainer)
			auth.POST("/lxc/start", handlers.StartLxcContainer)
			auth.POST("/lxc/stop", handlers.StopLxcContainer)
			auth.POST("/lxc/force-stop", handlers.ForceStopLxcContainer)
			auth.POST("/lxc/backup", handlers.BackupLxcContainer)
			auth.GET("/lxc/backup-status", handlers.GetLxcBackupStatus)
			auth.GET("/lxc/restore-status", handlers.GetLxcRestoreStatus)
			auth.GET("/lxc/backup-archives", handlers.GetLxcBackupArchives)
			auth.DELETE("/lxc/backup-archives", handlers.DeleteLxcBackupArchive)
			auth.GET("/lxc/metrics", handlers.GetLxcMetrics)
			auth.GET("/lxc/config/:name", handlers.GetLxcContainerConfig)
			auth.PUT("/lxc/config", handlers.UpdateLxcContainerConfig)
			auth.PUT("/lxc/password", handlers.ChangePasswordLxcContainer)

			// 文档管理
			auth.GET("/documents", handlers.GetDocumentList)
			auth.GET("/documents/:id", handlers.GetDocument)
			auth.POST("/documents", handlers.CreateDocument)
			auth.PUT("/documents", handlers.UpdateDocument)
			auth.DELETE("/documents", handlers.DeleteDocument)

			// 图片上传
			auth.POST("/upload/image", handlers.UploadImage)
		}
	}

	// SPA 路由 - 所有非 API 请求返回 index.html
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 如果是 API 请求，返回 404
		if len(path) >= 4 && path[:4] == "/api" {
			c.JSON(404, gin.H{"error": "API endpoint not found"})
			return
		}
		// 其他请求返回 index.html（用于 Vue Router）
		serveIndexHTML(c, cfg.AppTitle)
	})

	log.Printf("服务器启动在端口 %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func serveIndexHTML(c *gin.Context, appTitle string) {
	data, err := os.ReadFile("./frontend/dist/index.html")
	if err != nil {
		c.File("./frontend/dist/index.html")
		return
	}

	title := html.EscapeString(appTitle)
	content := strings.ReplaceAll(string(data), "%APP_TITLE%", title)
	content = strings.Replace(content, "<title>LabPanel 管理面板</title>", "<title>"+title+"</title>", 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(content))
}
