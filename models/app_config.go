package models

type AppConfigResponse struct {
	LxcImage     string `json:"lxcImage"`
	LxcBackupDir string `json:"lxcBackupDir"`
}

type UpdateAppConfigRequest struct {
	LxcImage     string `json:"lxcImage" binding:"required"`
	LxcBackupDir string `json:"lxcBackupDir" binding:"required"`
}
