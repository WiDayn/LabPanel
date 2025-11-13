package models

type Document struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Order   int    `json:"order"`
}

type DocumentListResponse struct {
	Documents []Document `json:"documents"`
}

type CreateDocumentRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content"`
	Order   int    `json:"order"`
}

type UpdateDocumentRequest struct {
	ID      string `json:"id" binding:"required"`
	Title   string `json:"title" binding:"required"`
	Content string `json:"content"`
	Order   int    `json:"order"`
}

type DeleteDocumentRequest struct {
	ID string `json:"id" binding:"required"`
}

