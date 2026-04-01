package config_test

import (
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
