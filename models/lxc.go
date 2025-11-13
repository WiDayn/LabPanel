package models

type LxcContainer struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	IPV4     string `json:"ipv4"`
	IPV6     string `json:"ipv6"`
	Type     string `json:"type"`
	Arch     string `json:"arch"`
	Profiles string `json:"profiles"`
}

type LxcListResponse struct {
	Containers []LxcContainer `json:"containers"`
}

type CreateLxcRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type DeleteLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

type RestartLxcRequest struct {
	Name string `json:"name" binding:"required"`
}

type ChangePasswordLxcRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

