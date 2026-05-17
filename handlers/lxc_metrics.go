package handlers

import (
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetLxcMetrics(c *gin.Context) {
	name := c.Query("name")
	rangeKey := c.DefaultQuery("range", "1h")

	metrics := service.GetLxcMetricsService().Response(name, rangeKey)
	c.JSON(http.StatusOK, metrics)
}
