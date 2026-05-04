package search

import (
	"testing"

	"github.com/denniseilander/jump/internal/sshconfig"
)

func h(alias, hostname, user string, meta map[string]string) sshconfig.Host {
	return sshconfig.Host{Alias: alias, HostName: hostname, User: user, Meta: meta}
}

func TestFind_EmptyQuery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myapp-prod", "prod.example.com", "deploy", nil),
		h("myapp-dev", "dev.example.com", "deploy", nil),
	}
	results := Find(hosts, "")
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
}

func TestFind_ExactAlias(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myapp-prod", "prod.example.com", "deploy", nil),
		h("other-prod", "other.example.com", "deploy", nil),
	}
	results := Find(hosts, "myapp-prod")
	if len(results) == 0 {
		t.Fatal("expected match")
	}
	if results[0].Host.Alias != "myapp-prod" {
		t.Errorf("expected myapp-prod, got %s", results[0].Host.Alias)
	}
	if results[0].Score != 100 {
		t.Errorf("exact alias score should be 100, got %d", results[0].Score)
	}
}

func TestFind_PrefixAlias(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myapp-prod", "prod.example.com", "deploy", nil),
		h("other-prod", "other.example.com", "deploy", nil),
	}
	results := Find(hosts, "myapp")
	if len(results) == 0 || results[0].Host.Alias != "myapp-prod" {
		t.Error("prefix should match myapp-prod")
	}
}

func TestFind_NoMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	results := Find([]sshconfig.Host{h("myapp-prod", "prod.example.com", "deploy", nil)}, "zzznomatch")
	if len(results) != 0 {
		t.Fatalf("expected 0, got %d", len(results))
	}
}

func TestFind_MultiToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("shop-web-prod", "prod.shop.com", "deploy", map[string]string{"app": "shop", "env": "prod"}),
		h("shop-web-acc", "acc.shop.com", "deploy", map[string]string{"app": "shop", "env": "acc"}),
		h("api-prod", "prod.api.com", "deploy", map[string]string{"app": "api", "env": "prod"}),
	}
	results := Find(hosts, "shop prod")
	if len(results) == 0 {
		t.Fatal("expected matches")
	}
	if results[0].Host.Alias != "shop-web-prod" {
		t.Errorf("expected shop-web-prod first, got %s", results[0].Host.Alias)
	}
}

func TestFind_SynonymProduction(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myapp-prod", "prod.example.com", "deploy", map[string]string{"env": "prod"}),
		h("myapp-acc", "acc.example.com", "deploy", map[string]string{"env": "acc"}),
	}
	results := Find(hosts, "production")
	found := false
	for _, r := range results {
		if r.Host.Alias == "myapp-prod" {
			found = true
		}
	}
	if !found {
		t.Error("'production' should match env=prod via synonym")
	}
}

func TestFind_SynonymAcceptance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myapp-acc", "acc.example.com", "deploy", map[string]string{"env": "acc"}),
	}
	results := Find(hosts, "staging")
	if len(results) == 0 {
		t.Error("'staging' should match env=acc via synonym")
	}
}

func TestFind_SynonymDev(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myapp-dev", "dev.example.com", "deploy", map[string]string{"env": "dev"}),
	}
	results := Find(hosts, "development")
	if len(results) == 0 {
		t.Error("'development' should match env=dev via synonym")
	}
}

func TestFind_MetadataApp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("h1", "prod.example.com", "deploy", map[string]string{"app": "webshop"}),
		h("h2", "other.example.com", "deploy", nil),
	}
	results := Find(hosts, "webshop")
	if len(results) == 0 || results[0].Host.Alias != "h1" {
		t.Error("expected match on app metadata")
	}
}

func TestFind_MetadataTag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("h1", "prod.example.com", "deploy", map[string]string{"tags": "gateway,production"}),
	}
	results := Find(hosts, "gateway")
	if len(results) == 0 {
		t.Error("expected match on tag")
	}
}

func TestFind_MetadataClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("h1", "prod.example.com", "deploy", map[string]string{"client": "acme"}),
	}
	results := Find(hosts, "acme")
	if len(results) == 0 {
		t.Error("expected match on client metadata")
	}
}

func TestFind_HostnameMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myhost", "unique-server.example.com", "deploy", nil),
	}
	results := Find(hosts, "unique-server")
	if len(results) == 0 {
		t.Error("expected match on hostname")
	}
}

func TestFind_UserMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myhost", "example.com", "specialuser", nil),
	}
	results := Find(hosts, "specialuser")
	if len(results) == 0 {
		t.Error("expected match on user")
	}
}

func TestFind_SortedByScore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("unrelated", "other.example.com", "user", nil),
		h("myapp-prod", "prod.example.com", "deploy", nil),
	}
	results := Find(hosts, "myapp-prod")
	if len(results) < 1 || results[0].Host.Alias != "myapp-prod" {
		t.Error("exact match should rank first")
	}
}

func TestFind_AllReturnedOnEmptyQuery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := make([]sshconfig.Host, 5)
	for i := range hosts {
		hosts[i] = h("host-"+string(rune('a'+i)), "example.com", "user", nil)
	}
	results := Find(hosts, "")
	if len(results) != 5 {
		t.Fatalf("empty query should return all hosts, got %d", len(results))
	}
}

func TestFindWithReasons_Returns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hosts := []sshconfig.Host{
		h("myapp-prod", "prod.example.com", "deploy", map[string]string{"env": "prod"}),
	}
	results := FindWithReasons(hosts, "myapp")
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if len(results[0].Reasons) == 0 {
		t.Error("expected match reasons")
	}
}

func TestFindWithReasons_EmptyQuery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	results := FindWithReasons([]sshconfig.Host{h("myapp", "h", "u", nil)}, "")
	if len(results) != 0 {
		t.Error("empty query should return nil")
	}
}

func TestExpandTokens_Synonym(t *testing.T) {
	groups := expandTokens([]string{"production"})
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	found := false
	for _, v := range groups[0] {
		if v == "prod" {
			found = true
		}
	}
	if !found {
		t.Error("'production' should expand to include 'prod'")
	}
}

func TestExpandTokens_NoSynonym(t *testing.T) {
	groups := expandTokens([]string{"myapp"})
	if len(groups) != 1 || len(groups[0]) != 1 || groups[0][0] != "myapp" {
		t.Errorf("unknown token should pass through unchanged, got %v", groups)
	}
}

func TestExpandTokens_Multiple(t *testing.T) {
	groups := expandTokens([]string{"myapp", "prod"})
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}
