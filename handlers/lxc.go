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

func ChangePasswordLxcContainer(c *gin.Context) {
	var req models.ChangePasswordLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.ChangePassword(req.Name, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

func StartLxcContainer(c *gin.Context) {
	var req models.StartLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.StartContainer(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器启动成功"})
}

func StopLxcContainer(c *gin.Context) {
	var req models.StopLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.StopContainer(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已关机"})
}

func ForceStopLxcContainer(c *gin.Context) {
	var req models.ForceStopLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.ForceStopContainer(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已强制关机"})
}

func BackupLxcContainer(c *gin.Context) {
	var req models.BackupLxcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	backup, err := service.GetLxcBackupManager().StartBackup(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.BackupLxcResponse{
		Message: "备份任务已启动",
		Backup:  *backup,
	})
}

func GetLxcBackupStatus(c *gin.Context) {
	c.JSON(http.StatusOK, models.LxcBackupStatusListResponse{
		Backups: service.GetLxcBackupManager().ListBackups(),
	})
}

func GetLxcContainerConfig(c *gin.Context) {
	var req models.GetLxcConfigRequest
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	config, err := lxcService.GetContainerConfig(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.GetLxcConfigResponse{Config: config})
}

func UpdateLxcContainerConfig(c *gin.Context) {
	var req models.UpdateLxcConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	lxcService := service.NewLxcService()
	if err := lxcService.UpdateContainerConfig(req.Name, req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置更新成功"})
}
