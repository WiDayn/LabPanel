package handlers

import (
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetHostInfo(c *gin.Context) {
	hostService := service.NewHostService()
	hostInfo, err := hostService.GetHostInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取宿主机 IP 失败"})
		return
	}

	c.JSON(http.StatusOK, hostInfo)
}
