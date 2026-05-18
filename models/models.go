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

type ConfigResponse struct {
	Content string `json:"content"`
}

type ConfigUpdateRequest struct {
	Content string `json:"content" binding:"required"`
}

type ServiceStatusResponse struct {
	Status       string `json:"status"`
	Active       bool   `json:"active"`
	StatusDetail string `json:"statusDetail"`
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
