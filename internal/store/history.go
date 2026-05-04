package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type HistoryEntry struct {
	Alias        string    `json:"alias"`
	LastUsedAt   time.Time `json:"last_used_at"`
	ConnectCount int       `json:"connect_count"`
}

type History map[string]HistoryEntry

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jump", "history.json"), nil
}

func Load() History {
	path, err := Path()
	if err != nil {
		return History{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return History{}
	}
	var h History
	if json.Unmarshal(data, &h) != nil {
		return History{}
	}
	return h
}

func RenameHistory(oldAlias, newAlias string) error {
	h := Load()
	entry, ok := h[oldAlias]
	if !ok {
		return nil
	}
	entry.Alias = newAlias
	h[newAlias] = entry
	delete(h, oldAlias)

	path, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func Record(alias string) error {
	h := Load()
	entry := h[alias]
	entry.Alias = alias
	entry.LastUsedAt = time.Now()
	entry.ConnectCount++
	h[alias] = entry

	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
