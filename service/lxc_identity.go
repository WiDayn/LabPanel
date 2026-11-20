package service

import (
	"crypto/rand"
	"fmt"
	"time"
)

const lxcUUIDKey = "volatile.uuid"

func newLxcUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:]), nil
}

func setLxcUUID(name, uuid string, restore bool) error {
	args := []string{"config", "set", name, lxcUUIDKey + "=" + uuid}
	if restore {
		args = append(args, "volatile.uuid.generation="+uuid)
	} else {
		args = append(args, "--project=default")
	}
	if _, err := lxcStorageCommand(15*time.Second, args...); err != nil {
		return fmt.Errorf("写入容器 UUID 失败: %w", err)
	}
	return nil
}
