package handlers

import (
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetGPUMonitor(c *gin.Context) {
	rangeKey := c.DefaultQuery("range", "1h")
	c.JSON(http.StatusOK, service.GetGPUMonitorService().Response(rangeKey))
}
