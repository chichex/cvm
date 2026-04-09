package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultMode           = "default"
	BypassPermissionsMode = "bypassPermissions"
)

func Read(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}

	cfg := map[string]any{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Write(path string, cfg map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func SetPermissionsMode(path, mode string) error {
	cfg, err := Read(path)
	if err != nil {
		return err
	}

	permissions, _ := cfg["permissions"].(map[string]any)
	if permissions == nil {
		permissions = map[string]any{}
	}
	permissions["defaultMode"] = mode
	cfg["permissions"] = permissions

	return Write(path, cfg)
}

func GetPermissionsMode(path string) (string, error) {
	cfg, err := Read(path)
	if err != nil {
		return "", err
	}
	permissions, _ := cfg["permissions"].(map[string]any)
	if permissions == nil {
		return "", nil
	}
	mode, _ := permissions["defaultMode"].(string)
	return mode, nil
}
