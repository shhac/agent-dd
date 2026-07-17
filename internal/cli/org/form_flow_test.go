package org_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shhac/lib-agent-cli/dialog"
	"github.com/shhac/lib-agent-cli/dialog/dialogtest"
)

// TestOrgAddForm_DialogFillsMissingSecrets drives the full command tree with
// --form and no key flags: both secrets come from the (faked) OS dialog, get
// stored, and never appear in the stdout receipt.
func TestOrgAddForm_DialogFillsMissingSecrets(t *testing.T) {
	dir := setupHeadless(t)

	const apiCanary = "API-CANARY-9F2C"
	const appCanary = "APP-CANARY-4B7E"
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{
			{ID: "api-key", Value: apiCanary},
			{ID: "app-key", Value: appCanary},
		},
	}
	defer dialog.SetDefault(rec)()

	stdout, stderr := runOrg(t, "org", "add", "prod", "--form")

	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	status := decodeStatus(t, stdout)
	if status["status"] != "added" {
		t.Errorf("status = %v, want \"added\"", status["status"])
	}
	// The typed secrets must never surface in the stdout receipt.
	if strings.Contains(stdout, apiCanary) || strings.Contains(stdout, appCanary) {
		t.Fatalf("secret canary leaked to stdout: %s", stdout)
	}
	if len(rec.Calls) != 1 {
		t.Fatalf("expected 1 dialog call, got %d", len(rec.Calls))
	}

	index := readCredsIndex(t, dir)
	entry, ok := index["prod"]
	if !ok {
		t.Fatal("credentials index missing \"prod\" entry")
	}
	if entry["api_key"] != apiCanary {
		t.Errorf("stored api_key = %v, want the dialog-supplied value", entry["api_key"])
	}
	if entry["app_key"] != appCanary {
		t.Errorf("stored app_key = %v, want the dialog-supplied value", entry["app_key"])
	}
}

// TestOrgAddForm_FlagFillsDialogPromptsRemainder confirms a key passed as a
// flag is not re-prompted; only the missing one comes from the dialog.
func TestOrgAddForm_FlagFillsDialogPromptsRemainder(t *testing.T) {
	dir := setupHeadless(t)

	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{{ID: "app-key", Value: "app-from-dialog"}},
	}
	defer dialog.SetDefault(rec)()

	stdout, stderr := runOrg(t, "org", "add", "prod", "--api-key", "api-from-flag", "--form")
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if s := decodeStatus(t, stdout); s["status"] != "added" {
		t.Errorf("status = %v, want \"added\"", s["status"])
	}
	if len(rec.Calls) != 1 || len(rec.Calls[0].Items) != 1 || rec.Calls[0].Items[0].ID != "app-key" {
		t.Fatalf("expected dialog to prompt only app-key, got calls: %+v", rec.Calls)
	}

	entry := readCredsIndex(t, dir)["prod"]
	if entry["api_key"] != "api-from-flag" {
		t.Errorf("api_key = %v, want \"api-from-flag\"", entry["api_key"])
	}
	if entry["app_key"] != "app-from-dialog" {
		t.Errorf("app_key = %v, want \"app-from-dialog\"", entry["app_key"])
	}
}

// TestOrgAddForm_CancelWritesRetryError verifies a cancelled dialog surfaces a
// structured retry error on stderr and writes no credentials.
func TestOrgAddForm_CancelWritesRetryError(t *testing.T) {
	dir := setupHeadless(t)

	rec := &dialogtest.Recorder{
		PromptErr: dialog.ErrCancelled,
	}
	defer dialog.SetDefault(rec)()

	stdout, stderr := runOrg(t, "org", "add", "prod", "--form")
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty stdout on cancel, got %q", stdout)
	}

	var errObj map[string]any
	if err := json.Unmarshal([]byte(stderr), &errObj); err != nil {
		t.Fatalf("stderr not JSON error (%q): %v", stderr, err)
	}
	if errObj["fixable_by"] != "retry" {
		t.Errorf("fixable_by = %v, want \"retry\"", errObj["fixable_by"])
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials.json")); !os.IsNotExist(err) {
		t.Errorf("cancelled form should not have written credentials.json (stat err = %v)", err)
	}
}
