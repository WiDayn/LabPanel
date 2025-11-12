package models

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type ConfigResponse struct {
	Content string `json:"content"`
}

type ConfigUpdateRequest struct {
	Content string `json:"content" binding:"required"`
}

type ServiceStatusResponse struct {
	Status      string `json:"status"`
	Active      bool   `json:"active"`
	StatusDetail string `json:"statusDetail"`
}

