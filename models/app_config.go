package models

type AppConfigResponse struct {
	LxcImage string `json:"lxcImage"`
}

type UpdateAppConfigRequest struct {
	LxcImage string `json:"lxcImage" binding:"required"`
}
