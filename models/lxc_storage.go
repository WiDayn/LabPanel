package models

import "time"

type LxcStoragePool struct {
	Name           string  `json:"name"`
	Driver         string  `json:"driver"`
	Source         string  `json:"source"`
	AvailableBytes *uint64 `json:"availableBytes"`
	TotalBytes     *uint64 `json:"totalBytes"`
}

type LxcStorageMount struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Path     string `json:"path"`
	Pool     string `json:"pool,omitempty"`
	Readonly bool   `json:"readonly"`
	Editable bool   `json:"editable"`
}

type LxcStorageMove struct {
	Name       string    `json:"name"`
	SourcePool string    `json:"sourcePool"`
	TargetPool string    `json:"targetPool"`
	Status     string    `json:"status"`
	Stage      string    `json:"stage"`
	Message    string    `json:"message"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
