package org_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/org"
	"github.com/shhac/agent-dd/internal/config"
)

// setupHeadless points config + credentials at a temp dir and forces the
// keychain opt-out, so credential writes deterministically take the 0600 file
// fallback (no `security` GUI prompt on darwin). Mirrors
// TestStore_Headless_FileFallback in internal/credential.
func setupHeadless(t *testing.T) string {
	t.Helper()
	t.Setenv("AGENT_DD_NO_KEYCHAIN", "1")
	dir := t.TempDir()
	config.SetConfigDir(dir)
	config.ClearCache()
	t.Cleanup(func() { config.SetConfigDir(""); config.ClearCache() })
	return dir
}

// runOrg builds a fresh root, registers the org command tree, and runs it with
// the given args while capturing stdout and stderr. Stdin is an empty,
// non-terminal stream so credential commands never block on the real os.Stdin.
func runOrg(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	return runOrgStdin(t, "", args...)
}

// runOrgStdin is runOrg with a caller-supplied stdin payload, exercising the
// piped-secret path (creds.ReadSecretLines). A strings.Reader is not an
// *os.File, so the helper's isInteractive check treats it as a non-terminal
// pipe and reads its lines.
func runOrgStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string) {
	t.Helper()

	root := &cobra.Command{Use: "agent-dd"}
	org.Register(root)
	root.SetArgs(args)
	root.SetIn(strings.NewReader(stdin))

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	runErr := root.Execute()

	_ = outW.Close()
	_ = errW.Close()
	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)
	os.Stdout, os.Stderr = origOut, origErr

	if runErr != nil {
		t.Fatalf("Execute(%v) returned error: %v", args, runErr)
	}
	return string(outBytes), string(errBytes)
}

// readCredsIndex reads the credentials.json index written by the file fallback.
func readCredsIndex(t *testing.T, dir string) map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatalf("reading credentials.json: %v", err)
	}
	var index map[string]map[string]any
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("unmarshalling credentials.json: %v", err)
	}
	return index
}

func decodeStatus(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var status map[string]any
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("status output is not JSON (%q): %v", stdout, err)
	}
	return status
}

func TestOrgAdd_Success(t *testing.T) {
	dir := setupHeadless(t)

	stdout, _ := runOrg(t, "org", "add", "prod",
		"--api-key", "api-prod", "--app-key", "app-prod", "--site", "datadoghq.eu")

	status := decodeStatus(t, stdout)
	if status["status"] != "added" {
		t.Errorf("status = %v, want \"added\"", status["status"])
	}
	if status["alias"] != "prod" {
		t.Errorf("alias = %v, want \"prod\"", status["alias"])
	}
	if status["storage"] != "file" {
		t.Errorf("storage = %v, want \"file\"", status["storage"])
	}
	if status["site"] != "datadoghq.eu" {
		t.Errorf("site = %v, want \"datadoghq.eu\"", status["site"])
	}

	index := readCredsIndex(t, dir)
	entry, ok := index["prod"]
	if !ok {
		t.Fatal("credentials index missing \"prod\" entry")
	}
	if entry["api_key"] != "api-prod" {
		t.Errorf("stored api_key = %v, want \"api-prod\" (file fallback stores keys directly)", entry["api_key"])
	}
	if entry["app_key"] != "app-prod" {
		t.Errorf("stored app_key = %v, want \"app-prod\"", entry["app_key"])
	}
}

func TestOrgAdd_RequiredFlagGuard(t *testing.T) {
	dir := setupHeadless(t)

	tests := []struct {
		name string
		args []string
	}{
		{"missing both", []string{"org", "add", "x"}},
		{"missing app-key", []string{"org", "add", "x", "--api-key", "only-api"}},
		{"missing api-key", []string{"org", "add", "x", "--app-key", "only-app"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr := runOrg(t, tt.args...)
			var errObj map[string]any
			if err := json.Unmarshal([]byte(stderr), &errObj); err != nil {
				t.Fatalf("stderr not JSON error (%q): %v", stderr, err)
			}
			if errObj["error"] == nil || errObj["error"] == "" {
				t.Errorf("expected non-empty error, got %v", errObj["error"])
			}
			if _, err := os.Stat(filepath.Join(dir, "credentials.json")); !os.IsNotExist(err) {
				t.Errorf("guard should not have written credentials.json (stat err = %v)", err)
			}
		})
	}
}

func TestOrgAdd_StdinBothLines(t *testing.T) {
	dir := setupHeadless(t)

	// No flags: both keys arrive on stdin, line 1 → api-key, line 2 → app-key.
	stdout, _ := runOrgStdin(t, "api-piped\napp-piped\n", "org", "add", "prod")

	status := decodeStatus(t, stdout)
	if status["status"] != "added" {
		t.Fatalf("status = %v, want \"added\"", status["status"])
	}

	entry := readCredsIndex(t, dir)["prod"]
	if entry["api_key"] != "api-piped" {
		t.Errorf("api_key = %v, want \"api-piped\" (stdin line 1)", entry["api_key"])
	}
	if entry["app_key"] != "app-piped" {
		t.Errorf("app_key = %v, want \"app-piped\" (stdin line 2)", entry["app_key"])
	}
}

func TestOrgAdd_FlagsWinOverStdin(t *testing.T) {
	dir := setupHeadless(t)

	// Both flags provided: stdin must be ignored entirely (all-or-nothing
	// contract — any flag present means stdin is never consulted).
	stdout, _ := runOrgStdin(t, "stdin-api\nstdin-app\n",
		"org", "add", "prod", "--api-key", "flag-api", "--app-key", "flag-app")

	status := decodeStatus(t, stdout)
	if status["status"] != "added" {
		t.Fatalf("status = %v, want \"added\"", status["status"])
	}

	entry := readCredsIndex(t, dir)["prod"]
	if entry["api_key"] != "flag-api" {
		t.Errorf("api_key = %v, want \"flag-api\" (flag wins, stdin ignored)", entry["api_key"])
	}
	if entry["app_key"] != "flag-app" {
		t.Errorf("app_key = %v, want \"flag-app\" (flag wins, stdin ignored)", entry["app_key"])
	}
}

func TestOrgAdd_StdinSingleLineHitsRequiredGuard(t *testing.T) {
	dir := setupHeadless(t)

	// One line only: api-key fills, app-key stays empty, so the required-key
	// guard fires and nothing is stored.
	_, stderr := runOrgStdin(t, "only-api\n", "org", "add", "prod")

	var errObj map[string]any
	if err := json.Unmarshal([]byte(stderr), &errObj); err != nil {
		t.Fatalf("stderr not JSON error (%q): %v", stderr, err)
	}
	if errObj["error"] == nil || errObj["error"] == "" {
		t.Errorf("expected non-empty error, got %v", errObj["error"])
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials.json")); !os.IsNotExist(err) {
		t.Errorf("guard should not have written credentials.json (stat err = %v)", err)
	}
}

func TestOrgUpdate_StdinBothLines(t *testing.T) {
	dir := setupHeadless(t)

	runOrg(t, "org", "add", "stage", "--api-key", "api-orig", "--app-key", "app-orig")

	// Both keys rotated via stdin (no flags): partial merge overlays both.
	stdout, _ := runOrgStdin(t, "api-rot\napp-rot\n", "org", "update", "stage")

	status := decodeStatus(t, stdout)
	if status["status"] != "updated" {
		t.Fatalf("status = %v, want \"updated\"", status["status"])
	}

	entry := readCredsIndex(t, dir)["stage"]
	if entry["api_key"] != "api-rot" {
		t.Errorf("api_key = %v, want \"api-rot\" (stdin line 1)", entry["api_key"])
	}
	if entry["app_key"] != "app-rot" {
		t.Errorf("app_key = %v, want \"app-rot\" (stdin line 2)", entry["app_key"])
	}
}

func TestOrgUpdate_StdinSingleLinePreservesAppKey(t *testing.T) {
	dir := setupHeadless(t)

	runOrg(t, "org", "add", "stage", "--api-key", "api-orig", "--app-key", "app-orig")

	// One line: only api-key updates; the empty app-key preserves the stored
	// value via updateCredentials' partial merge.
	runOrgStdin(t, "api-rot\n", "org", "update", "stage")

	entry := readCredsIndex(t, dir)["stage"]
	if entry["api_key"] != "api-rot" {
		t.Errorf("api_key = %v, want \"api-rot\" (stdin line 1)", entry["api_key"])
	}
	if entry["app_key"] != "app-orig" {
		t.Errorf("app_key = %v, want \"app-orig\" (partial merge preserves unprovided key)", entry["app_key"])
	}
}

func TestOrgUpdate_PartialMerge(t *testing.T) {
	dir := setupHeadless(t)

	// Seed an existing org with both keys.
	runOrg(t, "org", "add", "stage", "--api-key", "api-orig", "--app-key", "app-orig")

	// Update only the api-key; app-key must be preserved via the merge in
	// updateCredentials (reads existing, overlays only the provided key).
	stdout, _ := runOrg(t, "org", "update", "stage", "--api-key", "api-new")

	status := decodeStatus(t, stdout)
	if status["status"] != "updated" {
		t.Errorf("status = %v, want \"updated\"", status["status"])
	}

	index := readCredsIndex(t, dir)
	entry := index["stage"]
	if entry["api_key"] != "api-new" {
		t.Errorf("api_key = %v, want \"api-new\" (updated key)", entry["api_key"])
	}
	if entry["app_key"] != "app-orig" {
		t.Errorf("app_key = %v, want \"app-orig\" (partial update must preserve the unprovided key)", entry["app_key"])
	}
}
