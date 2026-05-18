package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RestartService(c *gin.Context) {
	systemctlService := service.NewSystemctlService()
	if err := systemctlService.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "服务重启成功"})
}

func GetServiceStatus(c *gin.Context) {
	systemctlService := service.NewSystemctlService()
	isActive, statusDetail, detailCommand, err := systemctlService.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status := "inactive"
	if isActive {
		status = "active"
	}

	c.JSON(http.StatusOK, models.ServiceStatusResponse{
		Status:        status,
		Active:        isActive,
		StatusDetail:  statusDetail,
		DetailCommand: detailCommand,
	})
}
