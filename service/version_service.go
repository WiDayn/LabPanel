package service

import (
	"LabPanel/models"
	"fmt"
	"os/exec"
	"strings"
)

var (
	AppVersion    = "1.0.0"
	AppCommit     = "unknown"
	AppCommitDate = "unknown"
)

func GetAppVersion() models.AppVersionResponse {
	version := firstNonEmpty(AppVersion, "1.0.0")
	commit := firstNonEmpty(AppCommit, gitOutput("rev-parse", "--short=12", "HEAD"), "unknown")
	commitDate := firstNonEmpty(AppCommitDate, gitOutput("show", "-s", "--format=%cs", "HEAD"), "unknown")

	return models.AppVersionResponse{
		Version:    version,
		Commit:     commit,
		CommitDate: commitDate,
		Display:    fmt.Sprintf("%s-%s-%s", version, commit, commitDate),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "unknown" {
			return value
		}
	}
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[len(values)-1])
}

func gitOutput(args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
