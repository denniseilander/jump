package store

import (
	"testing"
)

func TestLoadConfig_DefaultsOnMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := LoadConfig()
	if cfg.ConnectTimeout != 0 {
		t.Errorf("expected zero timeout, got %d", cfg.ConnectTimeout)
	}
	if cfg.DefaultUser != "" {
		t.Errorf("expected empty user, got %q", cfg.DefaultUser)
	}
}

func TestSaveAndLoadConfig_Roundtrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := Config{
		DefaultIdentity: "~/.ssh/id_ed25519",
		DefaultUser:     "deploy",
		DefaultPort:     "2222",
		ConnectTimeout:  30,
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got := LoadConfig()
	if got.DefaultIdentity != cfg.DefaultIdentity {
		t.Errorf("identity: got %q", got.DefaultIdentity)
	}
	if got.DefaultUser != cfg.DefaultUser {
		t.Errorf("user: got %q", got.DefaultUser)
	}
	if got.DefaultPort != cfg.DefaultPort {
		t.Errorf("port: got %q", got.DefaultPort)
	}
	if got.ConnectTimeout != cfg.ConnectTimeout {
		t.Errorf("timeout: got %d", got.ConnectTimeout)
	}
}

func TestSaveConfig_Overwrites(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveConfig(Config{DefaultUser: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(Config{DefaultUser: "second"}); err != nil {
		t.Fatal(err)
	}
	if LoadConfig().DefaultUser != "second" {
		t.Error("second save should overwrite first")
	}
}

func TestDefaultConnectTimeout(t *testing.T) {
	if DefaultConnectTimeout <= 0 {
		t.Errorf("DefaultConnectTimeout should be positive, got %d", DefaultConnectTimeout)
	}
}
