package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type MetadataEntry struct {
	Application string   `json:"application,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
}

type Index struct {
	Version int                      `json:"version"`
	Hosts   map[string]MetadataEntry `json:"hosts"`
}

func metadataPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jump", "index.json"), nil
}

func LoadIndex() Index {
	path, err := metadataPath()
	if err != nil {
		return Index{Version: 1, Hosts: map[string]MetadataEntry{}}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Index{Version: 1, Hosts: map[string]MetadataEntry{}}
	}
	var idx Index
	if json.Unmarshal(data, &idx) != nil {
		return Index{Version: 1, Hosts: map[string]MetadataEntry{}}
	}
	if idx.Hosts == nil {
		idx.Hosts = map[string]MetadataEntry{}
	}
	return idx
}

func SaveIndex(idx Index) error {
	path, err := metadataPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func GetMeta(alias string) MetadataEntry {
	return LoadIndex().Hosts[alias]
}

func SetMeta(alias string, update func(e *MetadataEntry)) error {
	idx := LoadIndex()
	entry := idx.Hosts[alias]
	update(&entry)
	idx.Hosts[alias] = entry
	return SaveIndex(idx)
}
