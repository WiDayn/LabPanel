package service

import (
	"LabPanel/models"
	"testing"
	"time"
)

func TestCleanupStoreKeepsUnusedGroups(t *testing.T) {
	group := models.LxcGroup{
		ID:        "grp_unused",
		Name:      "unused",
		Color:     "blue",
		CreatedAt: time.Now(),
	}
	store := lxcGroupStore{
		Groups:          []models.LxcGroup{group},
		ContainerGroups: map[string][]string{},
	}

	changed := (&LxcGroupService{}).cleanupStoreLocked(&store, false)
	if changed {
		t.Fatal("expected clean store to remain unchanged")
	}
	if len(store.Groups) != 1 || store.Groups[0].ID != group.ID {
		t.Fatalf("expected unused group to be retained, got %#v", store.Groups)
	}
}
