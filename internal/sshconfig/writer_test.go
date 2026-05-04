package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupWriter(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".ssh", "config.d"), 0700); err != nil {
		t.Fatal(err)
	}
	// create ~/.ssh/config with Include so ParseDefault finds jump.conf
	sshConfig := filepath.Join(dir, ".ssh", "config")
	if err := os.WriteFile(sshConfig, []byte("Include ~/.ssh/config.d/*.conf\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWriteHost_New(t *testing.T) {
	setupWriter(t)
	h := Host{
		Alias:    "myapp-prod",
		HostName: "prod.example.com",
		User:     "deploy",
		Port:     "22",
		Meta:     map[string]string{"app": "myapp", "env": "prod"},
	}
	if err := WriteHost(h); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Alias != "myapp-prod" {
		t.Errorf("alias: got %s", hosts[0].Alias)
	}
	if hosts[0].HostName != "prod.example.com" {
		t.Errorf("hostname: got %s", hosts[0].HostName)
	}
	if hosts[0].Meta["app"] != "myapp" {
		t.Errorf("app meta: got %s", hosts[0].Meta["app"])
	}
}

func TestWriteHost_Update(t *testing.T) {
	setupWriter(t)
	h := Host{Alias: "myapp-prod", HostName: "old.example.com", User: "deploy"}
	if err := WriteHost(h); err != nil {
		t.Fatal(err)
	}
	h.HostName = "new.example.com"
	if err := WriteHost(h); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host after update, got %d", len(hosts))
	}
	if hosts[0].HostName != "new.example.com" {
		t.Errorf("expected updated hostname, got %s", hosts[0].HostName)
	}
}

func TestWriteHost_MultipleHosts(t *testing.T) {
	setupWriter(t)
	for _, alias := range []string{"myapp-prod", "myapp-acc", "myapp-dev"} {
		if err := WriteHost(Host{Alias: alias, HostName: alias + ".example.com", User: "deploy"}); err != nil {
			t.Fatal(err)
		}
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}
}

func TestDeleteHost(t *testing.T) {
	setupWriter(t)
	if err := WriteHost(Host{Alias: "myapp-prod", HostName: "prod.example.com", User: "deploy"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteHost("myapp-prod"); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Fatalf("expected 0 hosts after delete, got %d", len(hosts))
	}
}

func TestDeleteHost_NotFound(t *testing.T) {
	setupWriter(t)
	if err := DeleteHost("nonexistent"); err == nil {
		t.Error("expected error for nonexistent alias")
	}
}

func TestDeleteHost_PreservesOthers(t *testing.T) {
	setupWriter(t)
	for _, alias := range []string{"keep-a", "delete-me", "keep-b"} {
		if err := WriteHost(Host{Alias: alias, HostName: alias + ".example.com", User: "u"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := DeleteHost("delete-me"); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 remaining hosts, got %d", len(hosts))
	}
	for _, h := range hosts {
		if h.Alias == "delete-me" {
			t.Error("deleted host should not appear")
		}
	}
}

func TestRenameAlias(t *testing.T) {
	setupWriter(t)
	if err := WriteHost(Host{Alias: "myapp-prod", HostName: "prod.example.com", User: "deploy"}); err != nil {
		t.Fatal(err)
	}
	if err := RenameAlias("myapp-prod", "myapp-production"); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Alias != "myapp-production" {
		t.Errorf("expected myapp-production, got %s", hosts[0].Alias)
	}
}

func TestRenameAlias_NotFound(t *testing.T) {
	setupWriter(t)
	if err := RenameAlias("nonexistent", "new"); err == nil {
		t.Error("expected error for nonexistent alias")
	}
}

func TestUpdateHostMeta(t *testing.T) {
	setupWriter(t)
	if err := WriteHost(Host{
		Alias:    "myapp-prod",
		HostName: "prod.example.com",
		User:     "deploy",
		Meta:     map[string]string{"app": "myapp", "env": "prod"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateHostMeta("myapp-prod", map[string]string{"description": "Production server"}); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	if hosts[0].Meta["description"] != "Production server" {
		t.Errorf("description: got %q", hosts[0].Meta["description"])
	}
	if hosts[0].Meta["app"] != "myapp" {
		t.Error("existing meta should be preserved")
	}
}

func TestUpdateHostMeta_TagsMerge(t *testing.T) {
	setupWriter(t)
	if err := WriteHost(Host{
		Alias:    "myapp-prod",
		HostName: "prod.example.com",
		User:     "deploy",
		Meta:     map[string]string{"tags": "web"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateHostMeta("myapp-prod", map[string]string{"tags": "production"}); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	tags := hosts[0].Meta["tags"]
	if !strings.Contains(tags, "web") || !strings.Contains(tags, "production") {
		t.Errorf("tags should be merged, got %q", tags)
	}
}

func TestUpdateHostMeta_NotFound(t *testing.T) {
	setupWriter(t)
	if err := UpdateHostMeta("nonexistent", map[string]string{"description": "x"}); err == nil {
		t.Error("expected error for nonexistent alias")
	}
}

func TestUpdateHostMetaByApp(t *testing.T) {
	setupWriter(t)
	for _, alias := range []string{"myapp-prod", "myapp-acc"} {
		if err := WriteHost(Host{
			Alias:    alias,
			HostName: alias + ".example.com",
			User:     "deploy",
			Meta:     map[string]string{"app": "myapp"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	// add a host with different app — should NOT be updated
	if err := WriteHost(Host{
		Alias:    "other-prod",
		HostName: "other.example.com",
		User:     "deploy",
		Meta:     map[string]string{"app": "other"},
	}); err != nil {
		t.Fatal(err)
	}

	n, err := UpdateHostMetaByApp("myapp", map[string]string{"client": "My App"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 updated, got %d", n)
	}

	hosts, err := ParseDefault()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hosts {
		if h.Meta["app"] == "myapp" && h.Meta["client"] != "My App" {
			t.Errorf("expected client=My App on %s", h.Alias)
		}
		if h.Meta["app"] == "other" && h.Meta["client"] != "" {
			t.Errorf("other app should not be updated, got client=%s", h.Meta["client"])
		}
	}
}

func TestUpdateHostMetaByApp_NoMatch(t *testing.T) {
	setupWriter(t)
	n, err := UpdateHostMetaByApp("nonexistent", map[string]string{"client": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 updated, got %d", n)
	}
}

func TestInitConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if err := InitConfig(); err != nil {
		t.Fatal(err)
	}
	managedPath, err := ManagedConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(managedPath); err != nil {
		t.Errorf("jump.conf should exist after init: %v", err)
	}
}

func TestBackupCreated(t *testing.T) {
	setupWriter(t)
	if err := WriteHost(Host{Alias: "myapp-prod", HostName: "prod.example.com", User: "deploy"}); err != nil {
		t.Fatal(err)
	}
	// second write triggers backup of existing file
	if err := WriteHost(Host{Alias: "myapp-acc", HostName: "acc.example.com", User: "deploy"}); err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	backupDir := filepath.Join(home, ".config", "jump", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("backup dir should exist: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one backup file")
	}
}
