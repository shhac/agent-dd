package org

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/lib-agent-cli/dialog"
	"github.com/shhac/lib-agent-cli/dialog/dialogtest"
)

func TestPromptMissingViaDialogReturnsEarlyWhenAllFlagsSupplied(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{{ID: "api-key", Value: "should not be used"}},
	}
	defer dialog.SetDefault(rec)()

	api, app, err := promptMissingViaDialog(context.Background(), "prod", "api-flag", "app-flag")
	if err != nil {
		t.Fatalf("promptMissingViaDialog() error = %v", err)
	}
	if api != "api-flag" || app != "app-flag" {
		t.Fatalf("returned api/app = %q/%q, want api-flag/app-flag", api, app)
	}
	if len(rec.Calls) != 0 {
		t.Errorf("Prompt should not have been called, got %d calls", len(rec.Calls))
	}
}

func TestPromptMissingViaDialogPromptsOnlyMissingAppKey(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{{ID: "app-key", Value: "from-dialog"}},
	}
	defer dialog.SetDefault(rec)()

	api, app, err := promptMissingViaDialog(context.Background(), "prod", "api-flag", "")
	if err != nil {
		t.Fatalf("promptMissingViaDialog() error = %v", err)
	}
	if api != "api-flag" {
		t.Errorf("api-key = %q, want unchanged 'api-flag'", api)
	}
	if app != "from-dialog" {
		t.Errorf("app-key = %q, want 'from-dialog'", app)
	}
	if len(rec.Calls) != 1 {
		t.Fatalf("expected 1 prompt call, got %d", len(rec.Calls))
	}
	spec := rec.Calls[0]
	if len(spec.Items) != 1 {
		t.Fatalf("expected 1 field in spec, got %d", len(spec.Items))
	}
	if spec.Items[0].ID != "app-key" || spec.Items[0].InputType != dialog.Password {
		t.Errorf("spec field = %+v, want app-key/Password", spec.Items[0])
	}
	if !strings.Contains(spec.Title, "prod") {
		t.Errorf("spec title = %q, want it to contain the alias", spec.Title)
	}
}

func TestPromptMissingViaDialogPromptsBothFieldsWhenBothMissing(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptResults: []dialog.Result{
			{ID: "api-key", Value: "api-typed"},
			{ID: "app-key", Value: "app-typed"},
		},
	}
	defer dialog.SetDefault(rec)()

	api, app, err := promptMissingViaDialog(context.Background(), "prod", "", "")
	if err != nil {
		t.Fatalf("promptMissingViaDialog() error = %v", err)
	}
	if api != "api-typed" || app != "app-typed" {
		t.Fatalf("api/app = %q/%q, want api-typed/app-typed", api, app)
	}
	spec := rec.Calls[0]
	if len(spec.Items) != 2 {
		t.Fatalf("expected 2 fields, got %d: %+v", len(spec.Items), spec.Items)
	}
	if spec.Items[0].ID != "api-key" || spec.Items[0].InputType != dialog.Password {
		t.Errorf("first field = %+v, want api-key/Password", spec.Items[0])
	}
	if spec.Items[1].ID != "app-key" || spec.Items[1].InputType != dialog.Password {
		t.Errorf("second field = %+v, want app-key/Password", spec.Items[1])
	}
}

func TestPromptMissingViaDialogReturnsHumanErrorWhenNoGUI(t *testing.T) {
	rec := &dialogtest.Recorder{
		AvailableErr: fmt.Errorf("%w: SSH session detected", dialog.ErrNoGUI),
	}
	defer dialog.SetDefault(rec)()

	_, _, err := promptMissingViaDialog(context.Background(), "prod", "api-flag", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if aerr.FixableBy != agenterrors.FixableByHuman {
		t.Errorf("FixableBy = %q, want human", aerr.FixableBy)
	}
	if !strings.Contains(aerr.Hint, "graphical desktop") {
		t.Errorf("hint = %q, want it to mention graphical desktop fallback", aerr.Hint)
	}
	if !strings.Contains(aerr.Hint, "--api-key") {
		t.Errorf("hint = %q, want it to suggest the non-interactive fallback", aerr.Hint)
	}
	// Sentinel chain must be preserved so callers can errors.Is downstream.
	if !errors.Is(err, dialog.ErrNoGUI) {
		t.Errorf("errors.Is(err, ErrNoGUI) = false, want true (sentinel chain broken)")
	}
}

func TestPromptMissingViaDialogReturnsRetryErrorOnCancel(t *testing.T) {
	rec := &dialogtest.Recorder{
		PromptErr: fmt.Errorf("%w (Datadog API key)", dialog.ErrCancelled),
	}
	defer dialog.SetDefault(rec)()

	_, _, err := promptMissingViaDialog(context.Background(), "prod", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var aerr *agenterrors.APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if aerr.FixableBy != agenterrors.FixableByRetry {
		t.Errorf("FixableBy = %q, want retry", aerr.FixableBy)
	}
	if !strings.Contains(aerr.Hint, "cancelled") && !strings.Contains(aerr.Hint, "Re-run") {
		t.Errorf("hint = %q, should mention cancellation and re-run", aerr.Hint)
	}
	// Sentinel chain must be preserved so callers can errors.Is downstream.
	if !errors.Is(err, dialog.ErrCancelled) {
		t.Errorf("errors.Is(err, ErrCancelled) = false, want true (sentinel chain broken)")
	}
}

// TestBuildCredentialSpec verifies that only blank fields are added to the
// spec, and that result slots line up with the items.
func TestBuildCredentialSpec(t *testing.T) {
	cases := []struct {
		name    string
		apiKey  string
		appKey  string
		wantIDs []string
	}{
		{"both supplied — empty spec", "api", "app", nil},
		{"app-key only missing — one item", "api", "", []string{"app-key"}},
		{"api-key only missing — one item", "", "app", []string{"api-key"}},
		{"both missing — two items", "", "", []string{"api-key", "app-key"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api, app := tc.apiKey, tc.appKey
			spec, slots := buildCredentialSpec("prod", &api, &app)

			if len(spec.Items) != len(tc.wantIDs) {
				t.Fatalf("len(spec.Items) = %d, want %d", len(spec.Items), len(tc.wantIDs))
			}
			for i, want := range tc.wantIDs {
				if spec.Items[i].ID != want {
					t.Errorf("spec.Items[%d].ID = %q, want %q", i, spec.Items[i].ID, want)
				}
				if slots[i].field.ID != want {
					t.Errorf("slots[%d].field.ID = %q, want %q", i, slots[i].field.ID, want)
				}
			}
			if !strings.Contains(spec.Title, "prod") {
				t.Errorf("spec.Title = %q, want it to contain the alias", spec.Title)
			}
		})
	}
}

// TestApplyResultsMatchesByID confirms that result-folding is robust to
// reordering: if a future change reorders Spec.Items, the values still land in
// the correct slot via ID lookup.
func TestApplyResultsMatchesByID(t *testing.T) {
	api, app := "", ""
	slots := []fieldSlot{
		{field: dialog.Field{ID: "api-key"}, dest: &api},
		{field: dialog.Field{ID: "app-key"}, dest: &app},
	}
	// Results in REVERSE order — applyResults must still place them correctly.
	applyResults([]dialog.Result{
		{ID: "app-key", Value: "app-val"},
		{ID: "api-key", Value: "api-val"},
	}, slots)
	if api != "api-val" {
		t.Errorf("api = %q, want api-val", api)
	}
	if app != "app-val" {
		t.Errorf("app = %q, want app-val", app)
	}
}

func TestApplyResultsIgnoresUnknownIDs(t *testing.T) {
	api := ""
	slots := []fieldSlot{
		{field: dialog.Field{ID: "api-key"}, dest: &api},
	}
	applyResults([]dialog.Result{
		{ID: "api-key", Value: "api-val"},
		{ID: "extraneous", Value: "should-be-ignored"},
	}, slots)
	if api != "api-val" {
		t.Errorf("api = %q, want api-val", api)
	}
}

func TestCategoryToFixableBy(t *testing.T) {
	cases := map[dialog.Category]agenterrors.FixableBy{
		dialog.CategoryHuman:              agenterrors.FixableByHuman,
		dialog.CategoryRetry:              agenterrors.FixableByRetry,
		dialog.CategoryAgent:              agenterrors.FixableByAgent,
		dialog.Category("unknown-future"): agenterrors.FixableByAgent,
	}
	for in, want := range cases {
		t.Run(string(in), func(t *testing.T) {
			if got := categoryToFixableBy(in); got != want {
				t.Errorf("categoryToFixableBy(%q) = %q, want %q", in, got, want)
			}
		})
	}
}
