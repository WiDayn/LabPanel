package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetAppConfig(c *gin.Context) {
	appConfigService := service.NewAppConfigService()
	appConfig, err := appConfigService.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, appConfig)
}

func UpdateAppConfig(c *gin.Context) {
	var req models.UpdateAppConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	appConfigService := service.NewAppConfigService()
	if err := appConfigService.UpdateConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "应用配置更新成功"})
}
