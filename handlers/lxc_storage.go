package handlers

import (
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetLxcStorage(c *gin.Context) {
	pools, err := service.ListLxcStoragePools()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	mounts, err := service.LxcStorageMountpoints()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"pools": pools, "mounts": []string{}, "mountError": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pools": pools, "mounts": mounts})
}

func CreateLxcStoragePool(c *gin.Context) {
	var request struct {
		Name  string `json:"name" binding:"required"`
		Mount string `json:"mount" binding:"required"`
	}
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写存储池名称和挂载点"})
		return
	}
	if err := service.CreateLxcStoragePool(request.Name, request.Mount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusCreated)
}

func GetLxcStorageMounts(c *gin.Context) {
	mounts, err := service.LxcStorageMounts(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mounts": mounts})
}

func GetLxcStorageMoves(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"moves": service.ListLxcStorageMoves()})
}

func StartLxcStorageMove(c *gin.Context) {
	var request struct {
		Name string `json:"name" binding:"required"`
		Pool string `json:"pool" binding:"required"`
	}
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择容器和目标存储池"})
		return
	}
	move, err := service.StartLxcStorageMove(request.Name, request.Pool)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"move": move})
}
