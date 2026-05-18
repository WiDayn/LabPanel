package service

import (
	"LabPanel/config"
	"LabPanel/models"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	if s.cleanupStoreLocked(&store, true) {
		if err := s.saveLocked(store); err != nil {
			return models.LxcGroupsResponse{}, err
		}
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
	s.cleanupStoreLocked(&store, true)
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
	s.cleanupStoreLocked(&store, false)
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
	if normalizeGroupStore(&store) {
		_ = s.saveLocked(store)
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

func (s *LxcGroupService) cleanupStoreLocked(store *lxcGroupStore, pruneStaleContainers bool) bool {
	changed := normalizeGroupStore(store)
	if pruneStaleContainers {
		if containerNames, err := currentLxcContainerNameSet(); err == nil {
			changed = pruneStaleContainerGroups(store, containerNames) || changed
		}
	}
	changed = pruneUnusedGroups(store) || changed
	return changed
}

func normalizeGroupStore(store *lxcGroupStore) bool {
	if store.Groups == nil {
		store.Groups = []models.LxcGroup{}
	}
	if store.ContainerGroups == nil {
		store.ContainerGroups = map[string][]string{}
	}

	changed := false
	seenNames := map[string]struct{}{}
	validIDs := map[string]struct{}{}
	groups := make([]models.LxcGroup, 0, len(store.Groups))
	for _, group := range store.Groups {
		originalID := group.ID
		originalName := group.Name
		originalColor := group.Color
		group.ID = strings.TrimSpace(group.ID)
		group.Name = strings.TrimSpace(group.Name)
		group.Color = strings.TrimSpace(group.Color)
		if group.ID == "" || group.Name == "" {
			changed = true
			continue
		}
		nameKey := strings.ToLower(group.Name)
		if _, ok := seenNames[nameKey]; ok {
			changed = true
			continue
		}
		seenNames[nameKey] = struct{}{}
		if group.Color == "" {
			group.Color = lxcGroupColors[len(groups)%len(lxcGroupColors)]
		}
		if group.ID != originalID || group.Name != originalName || group.Color != originalColor {
			changed = true
		}
		validIDs[group.ID] = struct{}{}
		groups = append(groups, group)
	}
	if len(groups) != len(store.Groups) {
		changed = true
	}
	store.Groups = groups

	containerGroups := make(map[string][]string, len(store.ContainerGroups))
	for containerName, groupIDs := range store.ContainerGroups {
		trimmedName := strings.TrimSpace(containerName)
		if trimmedName == "" {
			changed = true
			continue
		}
		normalizedIDs, idsChanged := normalizeStoredGroupIDs(groupIDs, validIDs)
		if idsChanged || trimmedName != containerName {
			changed = true
		}
		if len(normalizedIDs) == 0 {
			changed = true
			continue
		}
		if existing, ok := containerGroups[trimmedName]; ok {
			containerGroups[trimmedName] = mergeGroupIDs(existing, normalizedIDs)
			changed = true
			continue
		}
		containerGroups[trimmedName] = normalizedIDs
	}
	if len(containerGroups) != len(store.ContainerGroups) {
		changed = true
	}
	store.ContainerGroups = containerGroups
	return changed
}

func normalizeStoredGroupIDs(groupIDs []string, validIDs map[string]struct{}) ([]string, bool) {
	changed := false
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		trimmedID := strings.TrimSpace(groupID)
		if trimmedID == "" {
			changed = true
			continue
		}
		if _, ok := validIDs[trimmedID]; !ok {
			changed = true
			continue
		}
		if _, ok := seen[trimmedID]; ok {
			changed = true
			continue
		}
		if trimmedID != groupID {
			changed = true
		}
		seen[trimmedID] = struct{}{}
		normalized = append(normalized, trimmedID)
	}
	if len(normalized) != len(groupIDs) {
		changed = true
	}
	return normalized, changed
}

func mergeGroupIDs(left, right []string) []string {
	seen := map[string]struct{}{}
	merged := make([]string, 0, len(left)+len(right))
	for _, groupID := range append(append([]string{}, left...), right...) {
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		merged = append(merged, groupID)
	}
	return merged
}

func pruneStaleContainerGroups(store *lxcGroupStore, containerNames map[string]struct{}) bool {
	changed := false
	for containerName := range store.ContainerGroups {
		if _, ok := containerNames[containerName]; !ok {
			delete(store.ContainerGroups, containerName)
			changed = true
		}
	}
	return changed
}

func pruneUnusedGroups(store *lxcGroupStore) bool {
	used := map[string]struct{}{}
	for _, groupIDs := range store.ContainerGroups {
		for _, groupID := range groupIDs {
			used[groupID] = struct{}{}
		}
	}

	changed := false
	groups := make([]models.LxcGroup, 0, len(store.Groups))
	for _, group := range store.Groups {
		if _, ok := used[group.ID]; !ok {
			changed = true
			continue
		}
		groups = append(groups, group)
	}
	if len(groups) != len(store.Groups) {
		changed = true
	}
	store.Groups = groups
	return changed
}

func currentLxcContainerNameSet() (map[string]struct{}, error) {
	output, err := exec.Command("lxc", "list", "--format", "json").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("执行 lxc list 失败: %v, 输出: %s", err, string(output))
	}
	jsonOutput, err := extractJSONArray(output)
	if err != nil {
		return nil, err
	}
	var rawContainers []map[string]interface{}
	if err := json.Unmarshal(jsonOutput, &rawContainers); err != nil {
		return nil, err
	}
	containerNames := make(map[string]struct{}, len(rawContainers))
	for _, raw := range rawContainers {
		name := strings.TrimSpace(getString(raw, "name"))
		if name != "" {
			containerNames[name] = struct{}{}
		}
	}
	return containerNames, nil
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
