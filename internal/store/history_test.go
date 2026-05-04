package store

import (
	"sync"
	"testing"
	"time"
)

func TestLoad_EmptyOnMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	h := Load()
	if len(h) != 0 {
		t.Errorf("expected empty history, got %d entries", len(h))
	}
}

func TestRecord_CreatesEntry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Record("myapp-prod"); err != nil {
		t.Fatal(err)
	}
	h := Load()
	e, ok := h["myapp-prod"]
	if !ok {
		t.Fatal("expected history entry")
	}
	if e.ConnectCount != 1 {
		t.Errorf("expected count 1, got %d", e.ConnectCount)
	}
	if e.Alias != "myapp-prod" {
		t.Errorf("expected alias myapp-prod, got %s", e.Alias)
	}
}

func TestRecord_IncrementsCount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < 3; i++ {
		if err := Record("myapp-prod"); err != nil {
			t.Fatal(err)
		}
	}
	if Load()["myapp-prod"].ConnectCount != 3 {
		t.Errorf("expected count 3, got %d", Load()["myapp-prod"].ConnectCount)
	}
}

func TestRecord_UpdatesLastUsedAt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	before := time.Now().Add(-time.Second)
	if err := Record("myapp-prod"); err != nil {
		t.Fatal(err)
	}
	if Load()["myapp-prod"].LastUsedAt.Before(before) {
		t.Error("LastUsedAt should be recent")
	}
}

func TestRecord_MultipleAliases(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, alias := range []string{"a", "b", "c"} {
		if err := Record(alias); err != nil {
			t.Fatal(err)
		}
	}
	h := Load()
	if len(h) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(h))
	}
}

func TestRenameHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Record("old-alias"); err != nil {
		t.Fatal(err)
	}
	if err := RenameHistory("old-alias", "new-alias"); err != nil {
		t.Fatal(err)
	}
	h := Load()
	if _, ok := h["old-alias"]; ok {
		t.Error("old alias should be removed")
	}
	e, ok := h["new-alias"]
	if !ok {
		t.Fatal("new alias should exist")
	}
	if e.Alias != "new-alias" {
		t.Errorf("alias field should be updated, got %s", e.Alias)
	}
}

func TestRenameHistory_PreservesCount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < 5; i++ {
		if err := Record("old-alias"); err != nil {
			t.Fatal(err)
		}
	}
	if err := RenameHistory("old-alias", "new-alias"); err != nil {
		t.Fatal(err)
	}
	if Load()["new-alias"].ConnectCount != 5 {
		t.Errorf("connect count should be preserved, got %d", Load()["new-alias"].ConnectCount)
	}
}

func TestRenameHistory_NonExistent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := RenameHistory("nonexistent", "new"); err != nil {
		t.Error("renaming nonexistent alias should not error")
	}
}

func TestRecord_Concurrent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const n = 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Record("myapp-prod")
		}()
	}
	wg.Wait()
	count := Load()["myapp-prod"].ConnectCount
	if count != n {
		t.Errorf("concurrent records: expected count %d, got %d", n, count)
	}
}
