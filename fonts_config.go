package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func defaultFontsConfigPath(appRoot string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = appRoot
	}
	return filepath.Join(homeDir, ".masik", "fonts.json")
}

func loadFontsConfig(path string) (appearanceSettings, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return appearanceSettings{}, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := defaultAppearanceSettings()
		if err := saveFontsConfig(path, cfg); err != nil {
			return appearanceSettings{}, err
		}
		return cfg, nil
	} else if err != nil {
		return appearanceSettings{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return appearanceSettings{}, err
	}

	var cfg appearanceSettings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return appearanceSettings{}, err
	}
	return normalizeAppearanceSettings(cfg), nil
}

func saveFontsConfig(path string, cfg appearanceSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	cfg = normalizeAppearanceSettings(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
