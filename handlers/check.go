package handlers

import (
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetEnvironmentCheck(c *gin.Context) {
	checkService := service.NewEnvironmentCheckService()
	c.JSON(http.StatusOK, checkService.Check())
}
