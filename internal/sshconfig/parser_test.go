package sshconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFile_Empty(t *testing.T) {
	path := writeConfig(t, "")
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Fatalf("expected 0 hosts, got %d", len(hosts))
	}
}

func TestParseFile_SingleHost(t *testing.T) {
	path := writeConfig(t, `Host myapp-prod
  HostName prod.example.com
  User deploy
  Port 22
  IdentityFile ~/.ssh/id_ed25519
`)
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	h := hosts[0]
	if h.Alias != "myapp-prod" {
		t.Errorf("alias: got %s", h.Alias)
	}
	if h.HostName != "prod.example.com" {
		t.Errorf("hostname: got %s", h.HostName)
	}
	if h.User != "deploy" {
		t.Errorf("user: got %s", h.User)
	}
	if h.Port != "22" {
		t.Errorf("port: got %s", h.Port)
	}
	if h.Identity != "~/.ssh/id_ed25519" {
		t.Errorf("identity: got %s", h.Identity)
	}
}

func TestParseFile_MultipleHosts(t *testing.T) {
	path := writeConfig(t, `Host host-a
  HostName a.example.com

Host host-b
  HostName b.example.com

Host host-c
  HostName c.example.com
`)
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}
}

func TestParseFile_JumpMeta(t *testing.T) {
	path := writeConfig(t, `# jump: app=myapp env=prod tags=web,production description="Production server"
Host myapp-prod
  HostName prod.example.com
  User deploy
`)
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	meta := hosts[0].Meta
	if meta["app"] != "myapp" {
		t.Errorf("app: got %q", meta["app"])
	}
	if meta["env"] != "prod" {
		t.Errorf("env: got %q", meta["env"])
	}
	if meta["tags"] != "web,production" {
		t.Errorf("tags: got %q", meta["tags"])
	}
	if meta["description"] != "Production server" {
		t.Errorf("description: got %q", meta["description"])
	}
}

func TestParseFile_JumpMeta_SingleQuote(t *testing.T) {
	path := writeConfig(t, "# jump: client='My Client'\nHost h\n  HostName h.example.com\n")
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hosts[0].Meta["client"] != "My Client" {
		t.Errorf("single-quoted client: got %q", hosts[0].Meta["client"])
	}
}

func TestParseFile_WildcardFiltered(t *testing.T) {
	path := writeConfig(t, `Host *
  ServerAliveInterval 60

Host myapp-prod
  HostName prod.example.com
`)
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("wildcard should be filtered, got %d hosts", len(hosts))
	}
	if hosts[0].Alias != "myapp-prod" {
		t.Errorf("expected myapp-prod, got %s", hosts[0].Alias)
	}
}

func TestParseFile_Include(t *testing.T) {
	dir := t.TempDir()

	included := filepath.Join(dir, "included.conf")
	if err := os.WriteFile(included, []byte("Host included-host\n  HostName included.example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}

	main := filepath.Join(dir, "config")
	if err := os.WriteFile(main, []byte("Include "+included+"\n\nHost main-host\n  HostName main.example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}

	hosts, err := ParseFile(main)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts (include + main), got %d", len(hosts))
	}
}

func TestParseFile_CircularInclude(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("Include "+path+"\n\nHost myhost\n  HostName h.example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("circular include should not loop, got %d hosts", len(hosts))
	}
}

func TestParseFile_NonExistent(t *testing.T) {
	hosts, err := ParseFile("/nonexistent/path/config")
	if err != nil {
		t.Fatal("nonexistent file should return nil error")
	}
	if hosts != nil {
		t.Fatal("nonexistent file should return nil hosts")
	}
}

func TestParseFile_RawOptions(t *testing.T) {
	path := writeConfig(t, `Host myhost
  HostName h.example.com
  ServerAliveInterval 60
  StrictHostKeyChecking no
`)
	hosts, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].RawOptions["ServerAliveInterval"] != "60" {
		t.Errorf("ServerAliveInterval: got %q", hosts[0].RawOptions["ServerAliveInterval"])
	}
}

func TestParseJumpMeta_NonJumpComment(t *testing.T) {
	meta := parseJumpMeta("# just a regular comment")
	if meta != nil {
		t.Errorf("non-jump comment should return nil, got %v", meta)
	}
}

func TestParseJumpMeta_Empty(t *testing.T) {
	meta := parseJumpMeta("# jump:")
	if len(meta) != 0 {
		t.Errorf("empty jump comment should return empty map, got %v", meta)
	}
}

func TestFormatHost_Full(t *testing.T) {
	host := Host{Alias: "myapp", HostName: "prod.example.com", User: "deploy", Port: "2222"}
	got := FormatHost(host)
	want := "myapp -> deploy@prod.example.com:2222"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatHost_DefaultPort(t *testing.T) {
	host := Host{Alias: "myapp", HostName: "prod.example.com", User: "deploy", Port: "22"}
	got := FormatHost(host)
	want := "myapp -> deploy@prod.example.com"
	if got != want {
		t.Errorf("port 22 should be omitted, got %q", got)
	}
}

func TestFormatHost_NoUser(t *testing.T) {
	host := Host{Alias: "myapp", HostName: "prod.example.com"}
	got := FormatHost(host)
	want := "myapp -> prod.example.com"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatHost_NoHostname(t *testing.T) {
	host := Host{Alias: "myapp", User: "deploy"}
	got := FormatHost(host)
	want := "myapp -> deploy@myapp"
	if got != want {
		t.Errorf("alias used as fallback hostname, got %q", got)
	}
}
