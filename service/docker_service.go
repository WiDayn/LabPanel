package service

import (
	"LabPanel/models"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type DockerService struct{}

type dockerContainerLine struct {
	ID        string `json:"ID"`
	Names     string `json:"Names"`
	Image     string `json:"Image"`
	State     string `json:"State"`
	Status    string `json:"Status"`
	Ports     string `json:"Ports"`
	CreatedAt string `json:"CreatedAt"`
	Size      string `json:"Size"`
}

type dockerImageLine struct {
	ID         string `json:"ID"`
	Repository string `json:"Repository"`
	Tag        string `json:"Tag"`
	Size       string `json:"Size"`
	CreatedAt  string `json:"CreatedAt"`
}

func NewDockerService() *DockerService {
	return &DockerService{}
}

func (s *DockerService) Check() models.ComponentCheck {
	return NewEnvironmentCheckService().CheckDocker()
}

func (s *DockerService) List() (models.DockerListResponse, error) {
	check := s.Check()
	response := models.DockerListResponse{
		Ready:   check.Ready,
		Message: check.Message,
		Check:   check,
	}
	if !check.Ready {
		return response, nil
	}

	containers, err := s.ListContainers()
	if err != nil {
		return response, err
	}
	images, err := s.ListImages()
	if err != nil {
		return response, err
	}
	response.Containers = containers
	response.Images = images
	return response, nil
}

func (s *DockerService) ListContainers() ([]models.DockerContainer, error) {
	output, err := exec.Command("docker", "ps", "-a", "--format", "{{json .}}").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("读取 Docker 容器失败: %v, 输出: %s", err, string(output))
	}

	var rawContainers []dockerContainerLine
	if err := parseDockerJSONLines(output, &rawContainers); err != nil {
		return nil, fmt.Errorf("解析 Docker 容器列表失败: %v", err)
	}

	containers := make([]models.DockerContainer, 0, len(rawContainers))
	for _, raw := range rawContainers {
		containers = append(containers, models.DockerContainer{
			ID:        raw.ID,
			Name:      raw.Names,
			Image:     raw.Image,
			State:     normalizeDockerState(raw.State),
			Status:    raw.Status,
			Ports:     raw.Ports,
			CreatedAt: raw.CreatedAt,
			Size:      raw.Size,
		})
	}
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].Name < containers[j].Name
	})
	return containers, nil
}

func (s *DockerService) ListImages() ([]models.DockerImage, error) {
	output, err := exec.Command("docker", "images", "--format", "{{json .}}").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("读取 Docker 镜像失败: %v, 输出: %s", err, string(output))
	}

	var rawImages []dockerImageLine
	if err := parseDockerJSONLines(output, &rawImages); err != nil {
		return nil, fmt.Errorf("解析 Docker 镜像列表失败: %v", err)
	}

	images := make([]models.DockerImage, 0, len(rawImages))
	for _, raw := range rawImages {
		images = append(images, models.DockerImage{
			ID:         raw.ID,
			Repository: raw.Repository,
			Tag:        raw.Tag,
			Size:       raw.Size,
			CreatedAt:  raw.CreatedAt,
		})
	}
	sort.Slice(images, func(i, j int) bool {
		left := images[i].Repository + ":" + images[i].Tag
		right := images[j].Repository + ":" + images[j].Tag
		return left < right
	})
	return images, nil
}

func (s *DockerService) StartContainer(name string) error {
	return s.runContainerCommand("启动 Docker 容器失败", "start", name)
}

func (s *DockerService) StopContainer(name string) error {
	return s.runContainerCommand("停止 Docker 容器失败", "stop", name)
}

func (s *DockerService) RestartContainer(name string) error {
	return s.runContainerCommand("重启 Docker 容器失败", "restart", name)
}

func (s *DockerService) DeleteContainer(name string) error {
	return s.runContainerCommand("删除 Docker 容器失败", "rm", "-f", name)
}

func (s *DockerService) DeleteImage(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("镜像 ID 不能为空")
	}
	output, err := exec.Command("docker", "rmi", id).CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除 Docker 镜像失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (s *DockerService) runContainerCommand(message string, args ...string) error {
	if len(args) == 0 || strings.TrimSpace(args[len(args)-1]) == "" {
		return fmt.Errorf("容器名称不能为空")
	}
	output, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v, 输出: %s", message, err, string(output))
	}
	return nil
}

func parseDockerJSONLines[T any](output []byte, result *[]T) error {
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return err
		}
		*result = append(*result, item)
	}
	return nil
}

func normalizeDockerState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return "Running"
	case "exited":
		return "Exited"
	case "created":
		return "Created"
	case "paused":
		return "Paused"
	case "restarting":
		return "Restarting"
	case "removing":
		return "Removing"
	case "dead":
		return "Dead"
	default:
		if strings.TrimSpace(state) == "" {
			return "Unknown"
		}
		return state
	}
}
