package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	lxcGroupFileMu sync.Mutex
	lxcGroupColors = []string{
		"blue",
		"emerald",
		"amber",
		"violet",
		"rose",
		"cyan",
		"lime",
		"orange",
		"slate",
		"indigo",
	}
)

type LxcGroupService struct {
	path string
}

type lxcGroupStore struct {
	Groups          []models.LxcGroup   `json:"groups"`
	ContainerGroups map[string][]string `json:"containerGroups"`
}

func NewLxcGroupService() *LxcGroupService {
	cfg, _ := config.Load()
	path := "./lxc_groups.json"
	if cfg != nil && strings.TrimSpace(cfg.LxcGroupsPath) != "" {
		path = strings.TrimSpace(cfg.LxcGroupsPath)
	}
	return &LxcGroupService{path: path}
}

func (s *LxcGroupService) List() (models.LxcGroupsResponse, error) {
	lxcGroupFileMu.Lock()
	defer lxcGroupFileMu.Unlock()

	store, err := s.loadLocked()
	if err != nil {
		return models.LxcGroupsResponse{}, err
	}

	return models.LxcGroupsResponse{
		Groups:          cloneGroups(store.Groups),
		ContainerGroups: cloneContainerGroups(store.ContainerGroups),
	}, nil
}

func (s *LxcGroupService) Create(name string) (models.LxcGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.LxcGroup{}, fmt.Errorf("分组名称不能为空")
	}

	lxcGroupFileMu.Lock()
	defer lxcGroupFileMu.Unlock()

	store, err := s.loadLocked()
	if err != nil {
		return models.LxcGroup{}, err
	}
	for _, group := range store.Groups {
		if strings.EqualFold(strings.TrimSpace(group.Name), name) {
			return models.LxcGroup{}, fmt.Errorf("分组名称已存在: %s", name)
		}
	}

	group := models.LxcGroup{
		ID:        newLxcGroupID(),
		Name:      name,
		Color:     lxcGroupColors[len(store.Groups)%len(lxcGroupColors)],
		CreatedAt: time.Now(),
	}
	store.Groups = append(store.Groups, group)
	if err := s.saveLocked(store); err != nil {
		return models.LxcGroup{}, err
	}
	return group, nil
}

func (s *LxcGroupService) SetContainerGroups(containerName string, groupIDs []string) error {
	containerName = strings.TrimSpace(containerName)
	if containerName == "" {
		return fmt.Errorf("容器名称不能为空")
	}

	lxcGroupFileMu.Lock()
	defer lxcGroupFileMu.Unlock()

	store, err := s.loadLocked()
	if err != nil {
		return err
	}

	normalized, err := normalizeGroupIDs(groupIDs, store.Groups)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		delete(store.ContainerGroups, containerName)
	} else {
		store.ContainerGroups[containerName] = normalized
	}
	return s.saveLocked(store)
}

func (s *LxcGroupService) ValidateGroupIDs(groupIDs []string) error {
	lxcGroupFileMu.Lock()
	defer lxcGroupFileMu.Unlock()

	store, err := s.loadLocked()
	if err != nil {
		return err
	}
	_, err = normalizeGroupIDs(groupIDs, store.Groups)
	return err
}

func (s *LxcGroupService) GroupsForContainer(containerName string) []models.LxcGroup {
	groupsByContainer, err := s.GroupsByContainer()
	if err != nil {
		return []models.LxcGroup{}
	}
	return groupsByContainer[strings.TrimSpace(containerName)]
}

func (s *LxcGroupService) GroupsByContainer() (map[string][]models.LxcGroup, error) {
	lxcGroupFileMu.Lock()
	defer lxcGroupFileMu.Unlock()

	store, err := s.loadLocked()
	if err != nil {
		return nil, err
	}

	byID := make(map[string]models.LxcGroup, len(store.Groups))
	for _, group := range store.Groups {
		byID[group.ID] = group
	}

	result := make(map[string][]models.LxcGroup, len(store.ContainerGroups))
	for containerName, groupIDs := range store.ContainerGroups {
		for _, groupID := range groupIDs {
			if group, ok := byID[groupID]; ok {
				result[containerName] = append(result[containerName], group)
			}
		}
		if result[containerName] == nil {
			result[containerName] = []models.LxcGroup{}
		}
	}
	return result, nil
}

func (s *LxcGroupService) loadLocked() (lxcGroupStore, error) {
	store := lxcGroupStore{
		Groups:          []models.LxcGroup{},
		ContainerGroups: map[string][]string{},
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, fmt.Errorf("读取 LXC 分组失败: %v", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, fmt.Errorf("解析 LXC 分组失败: %v", err)
	}
	if store.Groups == nil {
		store.Groups = []models.LxcGroup{}
	}
	if store.ContainerGroups == nil {
		store.ContainerGroups = map[string][]string{}
	}
	return store, nil
}

func (s *LxcGroupService) saveLocked(store lxcGroupStore) error {
	dir := filepath.Dir(s.path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建 LXC 分组目录失败: %v", err)
		}
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 LXC 分组失败: %v", err)
	}
	if err := os.WriteFile(s.path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("保存 LXC 分组失败: %v", err)
	}
	return nil
}

func normalizeGroupIDs(groupIDs []string, groups []models.LxcGroup) ([]string, error) {
	valid := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		valid[group.ID] = struct{}{}
	}

	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			continue
		}
		if _, ok := valid[groupID]; !ok {
			return nil, fmt.Errorf("分组不存在: %s", groupID)
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		normalized = append(normalized, groupID)
	}
	return normalized, nil
}

func cloneGroups(groups []models.LxcGroup) []models.LxcGroup {
	cloned := make([]models.LxcGroup, len(groups))
	copy(cloned, groups)
	return cloned
}

func cloneContainerGroups(containerGroups map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(containerGroups))
	for containerName, groupIDs := range containerGroups {
		cloned[containerName] = append([]string(nil), groupIDs...)
	}
	return cloned
}

func newLxcGroupID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err == nil {
		return "grp_" + hex.EncodeToString(buf)
	}
	return fmt.Sprintf("grp_%d", time.Now().UnixNano())
}
