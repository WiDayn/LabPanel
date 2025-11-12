package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetLxcList(c *gin.Context) {
	lxcService := service.NewLxcService()
	containers, err := lxcService.ListContainers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"containers": containers,
	})
}

func CreateLxcContainer(c *gin.Context) {
	var req models.CreateLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.CreateContainer(req.Name, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器创建成功"})
}

func DeleteLxcContainer(c *gin.Context) {
	var req models.DeleteLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.DeleteContainer(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器删除成功"})
}

func RestartLxcContainer(c *gin.Context) {
	var req models.RestartLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.RestartContainer(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器重启成功"})
}

