package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const dotEnvPath = ".env"

type EnvChange struct {
	Key    string
	Value  string
	Delete bool
}

func SetDotEnvValues(changes []EnvChange) error {
	if len(changes) == 0 {
		return nil
	}

	lines := []string{}
	if data, err := os.ReadFile(dotEnvPath); err == nil {
		lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	pending := make(map[string]EnvChange, len(changes))
	for _, change := range changes {
		pending[change.Key] = change
	}

	updated := make([]string, 0, len(lines)+len(changes))
	for _, line := range lines {
		key := parseDotEnvKey(line)
		change, ok := pending[key]
		if !ok {
			updated = append(updated, line)
			continue
		}
		delete(pending, key)
		if change.Delete {
			continue
		}
		updated = append(updated, formatDotEnvLine(change.Key, change.Value))
	}

	for _, change := range changes {
		if _, ok := pending[change.Key]; !ok {
			continue
		}
		if !change.Delete {
			updated = append(updated, formatDotEnvLine(change.Key, change.Value))
		}
		delete(pending, change.Key)
	}

	content := strings.Join(updated, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(dotEnvPath, []byte(content), 0600); err != nil {
		return err
	}

	for _, change := range changes {
		if change.Delete {
			_ = os.Unsetenv(change.Key)
			continue
		}
		_ = os.Setenv(change.Key, change.Value)
	}

	return nil
}

func migrateDotEnvPassword() error {
	values, err := godotenv.Read(dotEnvPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	password, hasPassword := values["ADMIN_PASSWORD"]
	if !hasPassword {
		return nil
	}

	changes := []EnvChange{{Key: "ADMIN_PASSWORD", Delete: true}}
	if _, hasHash := values["ADMIN_HASHED_PASSWORD"]; !hasHash {
		hashedPassword, err := HashPassword(password)
		if err != nil {
			return err
		}
		changes = append(changes, EnvChange{Key: "ADMIN_HASHED_PASSWORD", Value: hashedPassword})
	}

	return SetDotEnvValues(changes)
}

func parseDotEnvKey(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "export ")
	key, _, ok := strings.Cut(trimmed, "=")
	if !ok {
		return ""
	}
	key = strings.TrimSpace(key)
	for _, r := range key {
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return key
}

func formatDotEnvLine(key, value string) string {
	line, err := godotenv.Marshal(map[string]string{key: value})
	if err != nil {
		return key + "=" + value
	}
	return strings.TrimSpace(line)
}
