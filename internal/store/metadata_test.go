package store

import (
	"testing"
)

func TestLoadIndex_EmptyOnMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	idx := LoadIndex()
	if idx.Hosts == nil {
		t.Error("Hosts map should not be nil")
	}
	if len(idx.Hosts) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx.Hosts))
	}
}

func TestSaveAndLoadIndex_Roundtrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	idx := Index{
		Version: 1,
		Hosts: map[string]MetadataEntry{
			"myapp-prod": {
				Application: "myapp",
				Environment: "prod",
				Tags:        []string{"web", "production"},
				Description: "Production server",
			},
		},
	}
	if err := SaveIndex(idx); err != nil {
		t.Fatal(err)
	}
	loaded := LoadIndex()
	e, ok := loaded.Hosts["myapp-prod"]
	if !ok {
		t.Fatal("expected entry for myapp-prod")
	}
	if e.Application != "myapp" {
		t.Errorf("application: got %q", e.Application)
	}
	if e.Environment != "prod" {
		t.Errorf("environment: got %q", e.Environment)
	}
	if len(e.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(e.Tags))
	}
	if e.Description != "Production server" {
		t.Errorf("description: got %q", e.Description)
	}
}

func TestSetAndGetMeta(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetMeta("myapp-prod", func(e *MetadataEntry) {
		e.Description = "My production host"
		e.Tags = []string{"web", "prod"}
	}); err != nil {
		t.Fatal(err)
	}
	entry := GetMeta("myapp-prod")
	if entry.Description != "My production host" {
		t.Errorf("description: got %q", entry.Description)
	}
	if len(entry.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(entry.Tags))
	}
}

func TestSetMeta_Update(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetMeta("myapp-prod", func(e *MetadataEntry) {
		e.Description = "first"
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetMeta("myapp-prod", func(e *MetadataEntry) {
		e.Description = "second"
	}); err != nil {
		t.Fatal(err)
	}
	if GetMeta("myapp-prod").Description != "second" {
		t.Error("second SetMeta should overwrite first")
	}
}

func TestGetMeta_NotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	entry := GetMeta("nonexistent")
	if entry.Description != "" || len(entry.Tags) != 0 {
		t.Error("missing entry should return zero MetadataEntry")
	}
}

func TestSaveIndex_MultipleHosts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	idx := Index{
		Version: 1,
		Hosts: map[string]MetadataEntry{
			"host-a": {Application: "app-a"},
			"host-b": {Application: "app-b"},
			"host-c": {Application: "app-c"},
		},
	}
	if err := SaveIndex(idx); err != nil {
		t.Fatal(err)
	}
	loaded := LoadIndex()
	if len(loaded.Hosts) != 3 {
		t.Errorf("expected 3 hosts, got %d", len(loaded.Hosts))
	}
}
