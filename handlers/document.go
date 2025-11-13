package handlers

import (
	"LabPanel/models"
	"LabPanel/service"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetDocumentList(c *gin.Context) {
	docService := service.NewDocumentService()
	documents, err := docService.ListDocuments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.DocumentListResponse{
		Documents: documents,
	})
}

func GetDocument(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文档ID不能为空"})
		return
	}

	docService := service.NewDocumentService()
	document, err := docService.GetDocument(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, document)
}

func CreateDocument(c *gin.Context) {
	var req models.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	docService := service.NewDocumentService()
	document, err := docService.CreateDocument(req.Title, req.Content, req.Order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, document)
}

func UpdateDocument(c *gin.Context) {
	var req models.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	docService := service.NewDocumentService()
	if err := docService.UpdateDocument(req.ID, req.Title, req.Content, req.Order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文档更新成功"})
}

func DeleteDocument(c *gin.Context) {
	var req models.DeleteDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	docService := service.NewDocumentService()
	if err := docService.DeleteDocument(req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "文档删除成功"})
}

