package main

import (
	"LabPanel/config"
	"LabPanel/handlers"
	"LabPanel/middleware"
	"LabPanel/service"
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "hash-password" {
		hashedPassword, err := config.HashPassword(os.Args[2])
		if err != nil {
			log.Fatalf("生成密码哈希失败: %v", err)
		}
		fmt.Println(hashedPassword)
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "migrate-config" {
		if _, err := config.Load(); err != nil {
			log.Fatalf("迁移配置失败: %v", err)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	// Reserve the panel port before restoring forwards so a saved rule cannot
	// claim the management endpoint and prevent access to the panel.
	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("监听面板端口失败: %v", err)
	}
	defer listener.Close()
	if err := service.InitPortForwardService(cfg.PortForwardsPath); err != nil {
		log.Printf("初始化端口转发失败: %v", err)
	} else {
		defer service.GetPortForwardService().Close()
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
			auth.GET("/system-settings", handlers.GetSystemSettings)
			auth.PUT("/system-settings/title", handlers.UpdateSystemTitle)
			auth.PUT("/system-settings/account", handlers.UpdateAdminAccount)

			// 代理管理
			auth.GET("/proxies", handlers.GetProxyList)
			auth.POST("/proxies", handlers.AddProxy)
			auth.PUT("/proxies", handlers.UpdateProxy)
			auth.DELETE("/proxies", handlers.DeleteProxy)

			// 宿主机端口转发（独立于 FRP）
			auth.GET("/port-forwards", handlers.GetPortForwards)
			auth.POST("/port-forwards", handlers.CreatePortForward)
			auth.PUT("/port-forwards/:id", handlers.UpdatePortForward)
			auth.DELETE("/port-forwards/:id", handlers.DeletePortForward)

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

			// Docker管理
			auth.GET("/docker/list", handlers.GetDockerList)
			auth.POST("/docker/start", handlers.StartDockerContainer)
			auth.POST("/docker/stop", handlers.StopDockerContainer)
			auth.POST("/docker/restart", handlers.RestartDockerContainer)
			auth.DELETE("/docker/delete", handlers.DeleteDockerContainer)
			auth.DELETE("/docker/images", handlers.DeleteDockerImage)

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
		serveIndexHTML(c)
	})

	log.Printf("服务器启动在端口 %s", cfg.Port)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Handler: r, ReadHeaderTimeout: 10 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("关闭 HTTP 服务失败: %v", err)
		}
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("服务器运行失败: %v", err)
	}
	stop()
	<-shutdownDone
}

func serveIndexHTML(c *gin.Context) {
	data, err := os.ReadFile("./frontend/dist/index.html")
	if err != nil {
		c.File("./frontend/dist/index.html")
		return
	}

	appTitle := "LabPanel 管理面板"
	if cfg, err := config.Load(); err == nil {
		appTitle = cfg.AppTitle
	}
	title := html.EscapeString(appTitle)
	content := strings.ReplaceAll(string(data), "%APP_TITLE%", title)
	content = strings.Replace(content, "<title>LabPanel 管理面板</title>", "<title>"+title+"</title>", 1)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(content))
}
