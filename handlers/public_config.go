package handlers

import (
	"LabPanel/config"
	"LabPanel/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetPublicConfig(c *gin.Context) {
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载配置失败"})
		return
	}

	c.JSON(http.StatusOK, models.PublicConfigResponse{
		Title: cfg.AppTitle,
	})
}
