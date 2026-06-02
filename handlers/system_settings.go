package handlers

import (
	"LabPanel/config"
	"LabPanel/models"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetSystemSettings(c *gin.Context) {
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载配置失败"})
		return
	}

	c.JSON(http.StatusOK, models.SystemSettingsResponse{
		Title:    cfg.AppTitle,
		Username: cfg.AdminUsername,
	})
}

func UpdateSystemTitle(c *gin.Context) {
	var req models.SystemTitleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统标题不能为空"})
		return
	}

	if err := config.SetDotEnvValues([]config.EnvChange{
		{Key: "APP_TITLE", Value: title},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存系统标题失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "系统标题已更新", "title": title})
}

func UpdateAdminAccount(c *gin.Context) {
	var req models.AdminAccountUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "管理员账号不能为空"})
		return
	}

	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载配置失败"})
		return
	}
	if !config.VerifyPassword(cfg.AdminHashedPassword, req.CurrentPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前密码错误"})
		return
	}

	changes := []config.EnvChange{
		{Key: "ADMIN_USERNAME", Value: username},
		{Key: "ADMIN_PASSWORD", Delete: true},
	}

	newPassword := strings.TrimSpace(req.NewPassword)
	if newPassword != "" {
		hashedPassword, err := config.HashPassword(newPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成密码哈希失败"})
			return
		}
		changes = append(changes, config.EnvChange{Key: "ADMIN_HASHED_PASSWORD", Value: hashedPassword})
	}

	if err := config.SetDotEnvValues(changes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存管理员账号失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "管理员账号已更新", "username": username})
}
