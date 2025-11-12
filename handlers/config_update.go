package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UpdateBaseConfig(c *gin.Context) {
	var req models.FrpConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	frpService := service.NewFrpConfigService()
	// 获取现有配置以保留代理列表
	existingConfig, err := frpService.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新基础配置，保留代理列表
	existingConfig.ServerAddr = req.ServerAddr
	existingConfig.ServerPort = req.ServerPort
	existingConfig.Auth = req.Auth

	if err := frpService.SaveConfig(existingConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "基础配置更新成功"})
}

