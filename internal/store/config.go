package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DefaultIdentity      string `json:"default_identity,omitempty"`
	DefaultUser          string `json:"default_user,omitempty"`
	DefaultPort          string `json:"default_port,omitempty"`
	ConnectTimeout       int    `json:"connect_timeout,omitempty"`
}

const DefaultConnectTimeout = 10

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jump", "config.json"), nil
}

func LoadConfig() Config {
	path, err := configPath()
	if err != nil {
		return Config{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	var c Config
	if json.Unmarshal(data, &c) != nil {
		return Config{}
	}
	return c
}

func SaveConfig(c Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func ConfigPath() (string, error) { return configPath() }
