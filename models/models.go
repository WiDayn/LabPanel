package models

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type PublicConfigResponse struct {
	Title string `json:"title"`
}

type AppVersionResponse struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	CommitDate string `json:"commitDate"`
	Display    string `json:"display"`
}

type SystemSettingsResponse struct {
	Title    string `json:"title"`
	Username string `json:"username"`
}

type SystemTitleUpdateRequest struct {
	Title string `json:"title" binding:"required"`
}

type AdminAccountUpdateRequest struct {
	Username        string `json:"username" binding:"required"`
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword"`
}

type SystemUpdateRequest struct {
	Source string `json:"source"`
}

type SystemUpdateCheckResponse struct {
	Source            string             `json:"source"`
	Current           AppVersionResponse `json:"current"`
	Branch            string             `json:"branch"`
	RemoteCommit      string             `json:"remoteCommit"`
	RemoteCommitShort string             `json:"remoteCommitShort"`
	HasUpdate         bool               `json:"hasUpdate"`
	Message           string             `json:"message"`
}

type SystemUpdateApplyResponse struct {
	Source  string `json:"source"`
	Command string `json:"command"`
	Message string `json:"message"`
}

type ConfigResponse struct {
	Content string `json:"content"`
}

type ConfigUpdateRequest struct {
	Content string `json:"content" binding:"required"`
}

type ServiceStatusResponse struct {
	Status        string `json:"status"`
	Active        bool   `json:"active"`
	StatusDetail  string `json:"statusDetail"`
	DetailCommand string `json:"detailCommand"`
}

type InstallCommand struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

type InstallGuide struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Commands    []InstallCommand `json:"commands"`
}

type ComponentCheck struct {
	Name         string         `json:"name"`
	Installed    bool           `json:"installed"`
	Ready        bool           `json:"ready"`
	Message      string         `json:"message"`
	MissingItems []string       `json:"missingItems"`
	Guides       []InstallGuide `json:"guides"`
}

type EnvironmentCheckResponse struct {
	FRP ComponentCheck `json:"frp"`
	LXC ComponentCheck `json:"lxc"`
}
