package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetProxyList(c *gin.Context) {
	frpService := service.NewFrpConfigService()
	frpConfig, err := frpService.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ProxyListResponse{
		Config:  models.FrpConfig{ServerAddr: frpConfig.ServerAddr, ServerPort: frpConfig.ServerPort},
		Proxies: frpConfig.Proxies,
	})
}

func GetFRPTargets(c *gin.Context) {
	targets, err := service.ListFRPTargets()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"targets": targets})
}

func ConfirmProxyBinding(c *gin.Context) {
	var req struct {
		Name    string              `json:"name" binding:"required"`
		Binding models.ProxyBinding `json:"binding"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择规则和容器网卡"})
		return
	}
	if err := service.NewFrpConfigService().ConfirmBinding(req.Name, req.Binding); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func AddProxy(c *gin.Context) {
	var req models.AddProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	frpService := service.NewFrpConfigService()
	if err := frpService.AddProxy(req.Proxy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "代理添加成功"})
}

func UpdateProxy(c *gin.Context) {
	var req models.UpdateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	frpService := service.NewFrpConfigService()
	if req.Index == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "索引字段不能为空"})
		return
	}
	if err := frpService.UpdateProxy(*req.Index, req.Name, req.Proxy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "代理更新成功"})
}

func DeleteProxy(c *gin.Context) {
	var req models.DeleteProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果 JSON 绑定失败，尝试从查询参数获取
		index := c.Query("index")
		if index == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
			return
		}
		var err error
		if req.Index, err = strconv.Atoi(index); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "索引格式错误"})
			return
		}
	}

	frpService := service.NewFrpConfigService()
	if err := frpService.DeleteProxy(req.Index, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "代理删除成功"})
}
