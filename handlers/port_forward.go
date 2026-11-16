package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func portForwardService(c *gin.Context) *service.PortForwardService {
	s := service.GetPortForwardService()
	if s == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "端口转发服务未初始化，请检查服务日志和端口转发配置文件"})
	}
	return s
}

func GetPortForwards(c *gin.Context) {
	if s := portForwardService(c); s != nil {
		c.JSON(http.StatusOK, gin.H{"forwards": s.List()})
	}
}

func CreatePortForward(c *gin.Context) {
	savePortForward(c, "")
}

func UpdatePortForward(c *gin.Context) {
	savePortForward(c, c.Param("id"))
}

func savePortForward(c *gin.Context, id string) {
	s := portForwardService(c)
	if s == nil {
		return
	}
	var rule models.PortForward
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	forward, err := s.Save(id, rule)
	if err != nil {
		portForwardError(c, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"forward": forward})
}

func DeletePortForward(c *gin.Context) {
	s := portForwardService(c)
	if s == nil {
		return
	}
	if err := s.Delete(c.Param("id")); err != nil {
		portForwardError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "端口转发已删除"})
}

func portForwardError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrPortForwardInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrPortForwardConflict):
		status = http.StatusConflict
	case errors.Is(err, service.ErrPortForwardNotFound):
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
