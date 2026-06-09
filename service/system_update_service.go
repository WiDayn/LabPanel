package service

import (
	"LabPanel/models"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	updateSourceGitHub     = "github"
	updateSourceGhProxyCom = "gh-proxy"
	updateSourceGhProxyOrg = "gh-proxy-org"
	labPanelRepoURL        = "https://github.com/WiDayn/LabPanel.git"
	ghProxyComPrefix       = "https://gh-proxy.com/"
	ghProxyOrgPrefix       = "https://gh-proxy.org/"
)

func CheckSystemUpdate(source string) (models.SystemUpdateCheckResponse, error) {
	source, remoteURL, err := normalizeUpdateSource(source)
	if err != nil {
		return models.SystemUpdateCheckResponse{}, err
	}

	current := GetAppVersion()
	branch := firstNonEmpty(gitOutput("rev-parse", "--abbrev-ref", "HEAD"), "main")
	localCommit := firstNonEmpty(gitOutput("rev-parse", "HEAD"), current.Commit)
	remoteCommit, err := remoteHeadCommit(remoteURL, branch)
	if err != nil {
		return models.SystemUpdateCheckResponse{}, err
	}

	hasUpdate := localCommit != "" && remoteCommit != "" && !strings.HasPrefix(localCommit, remoteCommit) && !strings.HasPrefix(remoteCommit, localCommit)
	message := "当前已是最新版本"
	if hasUpdate {
		message = "检测到可用更新"
	}

	return models.SystemUpdateCheckResponse{
		Source:            source,
		Current:           current,
		Branch:            branch,
		RemoteCommit:      remoteCommit,
		RemoteCommitShort: shortCommit(remoteCommit),
		HasUpdate:         hasUpdate,
		Message:           message,
	}, nil
}

func ProbeSystemUpdateSource(source string) (models.SystemUpdateProbeResponse, error) {
	source, remoteURL, err := normalizeUpdateSource(source)
	if err != nil {
		return models.SystemUpdateProbeResponse{}, err
	}

	branch := firstNonEmpty(gitOutput("rev-parse", "--abbrev-ref", "HEAD"), "main")
	startedAt := time.Now()
	remoteCommit, err := remoteHeadCommitWithTimeout(remoteURL, branch, 10*time.Second)
	latencyMs := time.Since(startedAt).Milliseconds()
	response := models.SystemUpdateProbeResponse{
		Source:            source,
		Reachable:         err == nil,
		LatencyMs:         latencyMs,
		RemoteCommit:      remoteCommit,
		RemoteCommitShort: shortCommit(remoteCommit),
	}

	if err != nil {
		response.Message = err.Error()
		return response, nil
	}

	response.Message = fmt.Sprintf("连通，耗时 %d ms", latencyMs)
	return response, nil
}

func StartSystemUpdate(source string) (models.SystemUpdateApplyResponse, error) {
	source, _, err := normalizeUpdateSource(source)
	if err != nil {
		return models.SystemUpdateApplyResponse{}, err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return models.SystemUpdateApplyResponse{}, err
	}

	unitName := fmt.Sprintf("lab-panel-update-%d", time.Now().Unix())
	args := []string{
		"--unit", unitName,
		"--collect",
		"--property", "WorkingDirectory=" + workDir,
		"--setenv", "GIT_TRANSPORT=https",
		"--setenv", "GITHUB_ROUTE=" + updateRouteName(source),
		"--setenv", "DEFAULT_GITHUB_PROXY=" + updateProxyPrefix(source),
		filepath.Join(workDir, "update.sh"),
	}

	cmd := exec.Command("systemd-run", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return models.SystemUpdateApplyResponse{}, fmt.Errorf("启动更新任务失败: %v, 输出: %s", err, strings.TrimSpace(string(output)))
	}

	return models.SystemUpdateApplyResponse{
		Source:  source,
		Command: "systemctl status " + unitName,
		Message: "更新任务已启动",
	}, nil
}

func normalizeUpdateSource(source string) (string, string, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	switch source {
	case "", updateSourceGitHub:
		return updateSourceGitHub, labPanelRepoURL, nil
	case updateSourceGhProxyCom, "gh-proxy.com", "proxy":
		return updateSourceGhProxyCom, ghProxyComPrefix + labPanelRepoURL, nil
	case updateSourceGhProxyOrg, "gh-proxy.org":
		return updateSourceGhProxyOrg, ghProxyOrgPrefix + labPanelRepoURL, nil
	default:
		return "", "", fmt.Errorf("不支持的更新来源: %s", source)
	}
}

func updateRouteName(source string) string {
	if source == updateSourceGitHub {
		return "github"
	}
	return "gh-proxy"
}

func updateProxyPrefix(source string) string {
	if source == updateSourceGhProxyOrg {
		return ghProxyOrgPrefix
	}
	return ghProxyComPrefix
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func remoteHeadCommit(remoteURL, branch string) (string, error) {
	return remoteHeadCommitWithTimeout(remoteURL, branch, 20*time.Second)
}

func remoteHeadCommitWithTimeout(remoteURL, branch string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", remoteURL, branch)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("检查更新超时")
	}
	if err != nil {
		return "", fmt.Errorf("检查远端版本失败: %v, 输出: %s", err, strings.TrimSpace(string(output)))
	}

	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "", fmt.Errorf("远端分支不存在: %s", branch)
	}
	return fields[0], nil
}
