package service

import (
	"os"
	"path/filepath"
	"testing"

	"LabPanel/models"
)

func TestUpdateConfigRefreshesCurrentProcessEnv(t *testing.T) {
	t.Setenv("LXC_IMAGE", "ubuntu:22.04")
	t.Setenv("LXC_BACKUP_DIR", "./backups")

	service := &AppConfigService{
		envPath: filepath.Join(t.TempDir(), ".env"),
	}

	err := service.UpdateConfig(models.UpdateAppConfigRequest{
		LxcImage:     "ubuntu:24.04",
		LxcBackupDir: "/data/labpanel/backups",
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if got := os.Getenv("LXC_IMAGE"); got != "ubuntu:24.04" {
		t.Fatalf("expected LXC_IMAGE to be refreshed, got %q", got)
	}
	if got := os.Getenv("LXC_BACKUP_DIR"); got != "/data/labpanel/backups" {
		t.Fatalf("expected LXC_BACKUP_DIR to be refreshed, got %q", got)
	}
}
