package config_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/shhac/agent-dd/internal/config"
)

func setup(t *testing.T) {
	t.Helper()
	config.SetConfigDir(t.TempDir())
	config.ClearCache()
}

func TestStoreAndReadOrganization(t *testing.T) {
	setup(t)

	if err := config.StoreOrganization("prod", config.Organization{Site: "datadoghq.com"}); err != nil {
		t.Fatalf("StoreOrganization: %v", err)
	}

	config.ClearCache()
	cfg := config.Read()

	org, ok := cfg.Organizations["prod"]
	if !ok {
		t.Fatal("expected organization 'prod' to exist")
	}
	if org.Site != "datadoghq.com" {
		t.Errorf("Site = %q, want %q", org.Site, "datadoghq.com")
	}
}

func TestStoreAutoDefault(t *testing.T) {
	setup(t)

	if err := config.StoreOrganization("first", config.Organization{Site: "datadoghq.eu"}); err != nil {
		t.Fatalf("StoreOrganization: %v", err)
	}

	cfg := config.Read()
	if cfg.DefaultOrg != "first" {
		t.Errorf("DefaultOrg = %q, want %q", cfg.DefaultOrg, "first")
	}

	if err := config.StoreOrganization("second", config.Organization{Site: "datadoghq.com"}); err != nil {
		t.Fatalf("StoreOrganization: %v", err)
	}

	cfg = config.Read()
	if cfg.DefaultOrg != "first" {
		t.Errorf("DefaultOrg should remain %q after adding second org, got %q", "first", cfg.DefaultOrg)
	}
}

func TestRemoveOrganization(t *testing.T) {
	setup(t)

	config.StoreOrganization("alpha", config.Organization{Site: "datadoghq.com"})
	config.StoreOrganization("beta", config.Organization{Site: "datadoghq.eu"})

	cfg := config.Read()
	if cfg.DefaultOrg != "alpha" {
		t.Fatalf("expected default to be 'alpha', got %q", cfg.DefaultOrg)
	}

	if err := config.RemoveOrganization("alpha"); err != nil {
		t.Fatalf("RemoveOrganization: %v", err)
	}

	config.ClearCache()
	cfg = config.Read()

	if _, ok := cfg.Organizations["alpha"]; ok {
		t.Error("expected 'alpha' to be removed")
	}
	if cfg.DefaultOrg == "" {
		t.Error("expected a new default to be picked after removing the default org")
	}
	if cfg.DefaultOrg == "alpha" {
		t.Error("default should not still be 'alpha' after removal")
	}
}

func TestSetDefault(t *testing.T) {
	setup(t)

	// Setting default to non-existent alias is a no-op.
	if err := config.SetDefault("ghost"); err != nil {
		t.Fatalf("SetDefault(ghost): %v", err)
	}
	cfg := config.Read()
	if cfg.DefaultOrg != "" {
		t.Errorf("DefaultOrg = %q after setting non-existent alias, want empty", cfg.DefaultOrg)
	}

	config.StoreOrganization("one", config.Organization{Site: "datadoghq.com"})
	config.StoreOrganization("two", config.Organization{Site: "datadoghq.eu"})

	if err := config.SetDefault("two"); err != nil {
		t.Fatalf("SetDefault(two): %v", err)
	}

	config.ClearCache()
	cfg = config.Read()
	if cfg.DefaultOrg != "two" {
		t.Errorf("DefaultOrg = %q, want %q", cfg.DefaultOrg, "two")
	}
}

// Concurrent StoreOrganization calls must not lose each other's entries.
//
// The hand-rolled version read the whole config, mutated a copy, and wrote it
// back with os.WriteFile — no lock. Two concurrent `org add` invocations each
// built their write from a snapshot taken before the other landed, so the
// loser's organization was silently erased. This also exercises the
// in-process package-level cache: Read() must not hand back a stale cached
// *Config to a writer racing another StoreOrganization call.
func TestConcurrentStoreOrganizationDoesNotLoseEntries(t *testing.T) {
	setup(t)

	const writers = 20
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			alias := fmt.Sprintf("org-%02d", i)
			site := fmt.Sprintf("site-%02d.datadoghq.com", i)
			if err := config.StoreOrganization(alias, config.Organization{Site: site}); err != nil {
				t.Errorf("StoreOrganization(%s): %v", alias, err)
			}
		}(i)
	}
	wg.Wait()

	config.ClearCache()
	cfg := config.Read()
	if len(cfg.Organizations) != writers {
		t.Fatalf("Organizations has %d entries, want %d — some were lost to a concurrent write", len(cfg.Organizations), writers)
	}
	for i := range writers {
		alias := fmt.Sprintf("org-%02d", i)
		org, ok := cfg.Organizations[alias]
		if !ok {
			t.Errorf("%s was lost from the config", alias)
			continue
		}
		want := fmt.Sprintf("site-%02d.datadoghq.com", i)
		if org.Site != want {
			t.Errorf("%s Site = %q, want %q", alias, org.Site, want)
		}
	}
}
