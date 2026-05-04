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

// withLock acquires an exclusive O_EXCL lock file before calling fn.
// Cross-platform: O_EXCL create is atomic on POSIX and NTFS.
// Falls through without lock if a stale lock lingers past 500ms.
func withLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			defer os.Remove(lockPath)
			f.Close()
			return fn()
		}
		if !os.IsExist(err) {
			// unexpected error — proceed without lock
			return fn()
		}
		if time.Now().After(deadline) {
			// stale lock — remove and proceed
			os.Remove(lockPath)
			return fn()
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func RenameHistory(oldAlias, newAlias string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return withLock(path, func() error {
		h := Load()
		entry, ok := h[oldAlias]
		if !ok {
			return nil
		}
		entry.Alias = newAlias
		h[newAlias] = entry
		delete(h, oldAlias)
		data, err := json.MarshalIndent(h, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0600)
	})
}

func Record(alias string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return withLock(path, func() error {
		h := Load()
		entry := h[alias]
		entry.Alias = alias
		entry.LastUsedAt = time.Now()
		entry.ConnectCount++
		h[alias] = entry
		data, err := json.MarshalIndent(h, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0600)
	})
}
