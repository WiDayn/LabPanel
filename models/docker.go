package models

type DockerContainer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	State     string `json:"state"`
	Status    string `json:"status"`
	Ports     string `json:"ports"`
	CreatedAt string `json:"createdAt"`
	Size      string `json:"size"`
}

type DockerImage struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	CreatedAt  string `json:"createdAt"`
}

type DockerListResponse struct {
	Ready      bool              `json:"ready"`
	Message    string            `json:"message"`
	Check      ComponentCheck    `json:"check"`
	Containers []DockerContainer `json:"containers"`
	Images     []DockerImage     `json:"images"`
}

type DockerNameRequest struct {
	Name string `json:"name" binding:"required"`
}

type DockerImageRequest struct {
	ID string `json:"id" binding:"required"`
}
