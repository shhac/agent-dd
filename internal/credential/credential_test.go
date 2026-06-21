package credential_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/shhac/agent-dd/internal/config"
	"github.com/shhac/agent-dd/internal/credential"
)

func setup(t *testing.T) {
	t.Helper()
	config.SetConfigDir(t.TempDir())
	config.ClearCache()
}

func TestStoreAndGet(t *testing.T) {
	setup(t)

	_, err := credential.Store("prod", credential.Credential{
		APIKey: "api-key-123",
		AppKey: "app-key-456",
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	cred, err := credential.Get("prod")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Keys might be in keychain on macOS; file fallback stores them directly.
	if cred.APIKey == "" {
		t.Error("APIKey should not be empty")
	}
	if cred.AppKey == "" {
		t.Error("AppKey should not be empty")
	}

	if !cred.KeychainManaged {
		if cred.APIKey != "api-key-123" {
			t.Errorf("APIKey = %q, want %q", cred.APIKey, "api-key-123")
		}
		if cred.AppKey != "app-key-456" {
			t.Errorf("AppKey = %q, want %q", cred.AppKey, "app-key-456")
		}
	}
}

// TestStore_Headless_FileFallback exercises the real credential-WRITE path
// non-interactively. Setting the per-CLI keychain opt-out (derived by
// lib-agent-cli from the "app.paulie.agent-dd" service) makes the keychain
// backend report unavailable, so Store deterministically takes the 0600 file
// fallback on every platform — including darwin, where it would otherwise reach
// the `security` CLI and its GUI prompt. The per-CLI env var also proves the
// lib's prefix derivation.
func TestStore_Headless_FileFallback(t *testing.T) {
	t.Setenv("AGENT_DD_NO_KEYCHAIN", "1")
	dir := t.TempDir()
	config.SetConfigDir(dir)
	config.ClearCache()
	t.Cleanup(func() { config.SetConfigDir(""); config.ClearCache() })

	storage, err := credential.Store("headless", credential.Credential{
		APIKey: "api-headless",
		AppKey: "app-headless",
	})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if storage != "file" {
		t.Fatalf("storage = %q, want \"file\" (keychain opt-out should force the file path)", storage)
	}

	credsPath := filepath.Join(dir, "credentials.json")
	info, err := os.Stat(credsPath)
	if err != nil {
		t.Fatalf("credentials file not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("credentials mode = %o, want 0600", mode)
	}

	cred, err := credential.Get("headless")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cred.KeychainManaged {
		t.Error("KeychainManaged = true, want false (keychain should not have been used)")
	}
	if cred.APIKey != "api-headless" {
		t.Errorf("APIKey = %q, want %q (file fallback stores keys directly)", cred.APIKey, "api-headless")
	}
	if cred.AppKey != "app-headless" {
		t.Errorf("AppKey = %q, want %q", cred.AppKey, "app-headless")
	}

	if err := credential.Remove("headless"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err = credential.Get("headless")
	var notFound *credential.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("after Remove, Get should return *NotFoundError, got %T: %v", err, err)
	}
}

func TestGetNotFound(t *testing.T) {
	setup(t)

	_, err := credential.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent credential")
	}

	var notFound *credential.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
	}
	if notFound.Name != "nonexistent" {
		t.Errorf("Name = %q, want %q", notFound.Name, "nonexistent")
	}
}

func TestRemove(t *testing.T) {
	setup(t)

	credential.Store("temp", credential.Credential{
		APIKey: "key1",
		AppKey: "key2",
	})

	if err := credential.Remove("temp"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err := credential.Get("temp")
	var notFound *credential.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("after Remove, Get should return *NotFoundError, got %T: %v", err, err)
	}
}

func TestList(t *testing.T) {
	setup(t)

	credential.Store("org-a", credential.Credential{APIKey: "a1", AppKey: "a2"})
	credential.Store("org-b", credential.Credential{APIKey: "b1", AppKey: "b2"})

	names, err := credential.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	sort.Strings(names)
	if len(names) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(names))
	}
	if names[0] != "org-a" || names[1] != "org-b" {
		t.Errorf("List = %v, want [org-a org-b]", names)
	}
}
