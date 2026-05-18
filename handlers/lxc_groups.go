package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetLxcGroups(c *gin.Context) {
	groups, err := service.NewLxcGroupService().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groups)
}

func CreateLxcGroup(c *gin.Context) {
	var req models.CreateLxcGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	group, err := service.NewLxcGroupService().Create(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.LxcGroupResponse{Group: group})
}

func UpdateLxcContainerGroups(c *gin.Context) {
	var req models.UpdateLxcContainerGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if err := service.NewLxcGroupService().SetContainerGroups(req.ContainerName, req.GroupIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器分组已更新"})
}
