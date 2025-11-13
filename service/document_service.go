package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type DocumentService struct {
	cfg *config.Config
}

func NewDocumentService() *DocumentService {
	cfg, _ := config.Load()
	service := &DocumentService{cfg: cfg}
	// 确保目录存在
	os.MkdirAll(cfg.DocsPath, 0755)
	os.MkdirAll(cfg.UploadPath, 0755)
	return service
}

// getIndexPath 获取索引文件路径
func (s *DocumentService) getIndexPath() string {
	return filepath.Join(s.cfg.DocsPath, "index.json")
}

// getDocPath 获取文档文件路径
func (s *DocumentService) getDocPath(id string) string {
	return filepath.Join(s.cfg.DocsPath, id+".md")
}

// loadIndex 加载文档索引
func (s *DocumentService) loadIndex() ([]models.Document, error) {
	indexPath := s.getIndexPath()
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Document{}, nil
		}
		return nil, fmt.Errorf("读取索引文件失败: %v", err)
	}

	var documents []models.Document
	if err := json.Unmarshal(data, &documents); err != nil {
		return nil, fmt.Errorf("解析索引文件失败: %v", err)
	}

	return documents, nil
}

// saveIndex 保存文档索引
func (s *DocumentService) saveIndex(documents []models.Document) error {
	indexPath := s.getIndexPath()
	data, err := json.MarshalIndent(documents, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化索引失败: %v", err)
	}

	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("写入索引文件失败: %v", err)
	}

	return nil
}

// ListDocuments 获取文档列表
func (s *DocumentService) ListDocuments() ([]models.Document, error) {
	documents, err := s.loadIndex()
	if err != nil {
		return nil, err
	}

	// 按Order排序
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Order < documents[j].Order
	})

	return documents, nil
}

// GetDocument 获取文档详情
func (s *DocumentService) GetDocument(id string) (*models.Document, error) {
	documents, err := s.loadIndex()
	if err != nil {
		return nil, err
	}

	for _, doc := range documents {
		if doc.ID == id {
			// 读取文档内容
			docPath := s.getDocPath(id)
			content, err := os.ReadFile(docPath)
			if err != nil {
				if os.IsNotExist(err) {
					doc.Content = ""
				} else {
					return nil, fmt.Errorf("读取文档内容失败: %v", err)
				}
			} else {
				doc.Content = string(content)
			}
			return &doc, nil
		}
	}

	return nil, fmt.Errorf("文档不存在")
}

// CreateDocument 创建文档
func (s *DocumentService) CreateDocument(title, content string, order int) (*models.Document, error) {
	documents, err := s.loadIndex()
	if err != nil {
		return nil, err
	}

	// 生成ID（使用标题的简化版本）
	id := strings.ToLower(strings.ReplaceAll(title, " ", "_"))
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "\\", "_")
	
	// 确保ID唯一
	originalID := id
	counter := 1
	for {
		exists := false
		for _, doc := range documents {
			if doc.ID == id {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		id = fmt.Sprintf("%s_%d", originalID, counter)
		counter++
	}

	// 保存文档内容
	docPath := s.getDocPath(id)
	if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("保存文档内容失败: %v", err)
	}

	// 添加到索引
	doc := models.Document{
		ID:      id,
		Title:   title,
		Content: content,
		Order:   order,
	}
	documents = append(documents, doc)

	if err := s.saveIndex(documents); err != nil {
		// 如果保存索引失败，删除已创建的文档文件
		os.Remove(docPath)
		return nil, err
	}

	return &doc, nil
}

// UpdateDocument 更新文档
func (s *DocumentService) UpdateDocument(id, title, content string, order int) error {
	documents, err := s.loadIndex()
	if err != nil {
		return err
	}

	found := false
	for i := range documents {
		if documents[i].ID == id {
			documents[i].Title = title
			documents[i].Content = content
			documents[i].Order = order
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("文档不存在")
	}

	// 保存文档内容
	docPath := s.getDocPath(id)
	if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("保存文档内容失败: %v", err)
	}

	// 保存索引
	return s.saveIndex(documents)
}

// DeleteDocument 删除文档
func (s *DocumentService) DeleteDocument(id string) error {
	documents, err := s.loadIndex()
	if err != nil {
		return err
	}

	found := false
	newDocuments := []models.Document{}
	for _, doc := range documents {
		if doc.ID == id {
			found = true
		} else {
			newDocuments = append(newDocuments, doc)
		}
	}

	if !found {
		return fmt.Errorf("文档不存在")
	}

	// 删除文档文件
	docPath := s.getDocPath(id)
	os.Remove(docPath)

	// 保存索引
	return s.saveIndex(newDocuments)
}

