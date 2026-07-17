package org

import (
	"context"
	"fmt"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/lib-agent-cli/dialog"
)

// promptMissingViaDialog asks the user (via a native OS dialog) for any
// secret fields not supplied by --api-key / --app-key. Returns the
// (potentially filled-in) values.
//
// On any dialog failure, returns an *agenterrors.APIError with the
// classification supplied by dialog.ClassifyError. The wrapped sentinel is
// preserved so callers can errors.Is downstream.
func promptMissingViaDialog(ctx context.Context, alias, apiKey, appKey string) (string, string, error) {
	spec, slots := buildCredentialSpec(alias, &apiKey, &appKey)
	if len(spec.Items) == 0 {
		return apiKey, appKey, nil
	}

	if err := dialog.Default.Available(); err != nil {
		return apiKey, appKey, classifyDialogErr(err, alias)
	}

	results, err := dialog.Default.Prompt(ctx, spec)
	if err != nil {
		return apiKey, appKey, classifyDialogErr(err, alias)
	}

	applyResults(results, slots)
	return apiKey, appKey, nil
}

// fieldSlot pairs a dialog.Field with the variable that should receive its
// value. Keeping them adjacent (built once, consumed once) removes the
// string-coupling between spec construction and result folding.
type fieldSlot struct {
	field dialog.Field
	dest  *string
}

// buildCredentialSpec assembles the dialog Spec for any blank credential
// fields. The returned slots have the same length as Spec.Items and share the
// order; applyResults walks them in lockstep. Both keys are secrets, so both
// use dialog.Password (hidden entry). --site is not secret and is never
// prompted here.
func buildCredentialSpec(alias string, apiKey, appKey *string) (dialog.Spec, []fieldSlot) {
	candidates := []fieldSlot{
		{
			field: dialog.Field{ID: "api-key", Label: "Datadog API key", InputType: dialog.Password},
			dest:  apiKey,
		},
		{
			field: dialog.Field{ID: "app-key", Label: "Datadog application key", InputType: dialog.Password},
			dest:  appKey,
		},
	}
	slots := make([]fieldSlot, 0, len(candidates))
	items := make([]dialog.Field, 0, len(candidates))
	for _, c := range candidates {
		if *c.dest != "" {
			continue
		}
		slots = append(slots, c)
		items = append(items, c.field)
	}
	return dialog.Spec{
		Title: fmt.Sprintf("agent-dd org: %s", alias),
		Items: items,
	}, slots
}

// applyResults writes each Result's Value into the slot's destination by
// matching field ID. Order is preserved so an i-by-i walk works, but we match
// by ID for safety against future spec rearrangement.
func applyResults(results []dialog.Result, slots []fieldSlot) {
	byID := make(map[string]*string, len(slots))
	for _, s := range slots {
		byID[s.field.ID] = s.dest
	}
	for _, r := range results {
		if dest, ok := byID[r.ID]; ok {
			*dest = r.Value
		}
	}
}

// classifyDialogErr is the agent-dd adapter from a dialog package error to our
// APIError envelope. The heavy lifting (sentinel→category) lives in
// dialog.ClassifyError so the mapping itself doesn't drift across siblings.
func classifyDialogErr(err error, alias string) error {
	cat, hint := dialog.ClassifyError(err)

	// Augment the generic hint with agent-dd-specific guidance.
	switch cat {
	case dialog.CategoryHuman:
		hint = "agent-dd org add --form requires a graphical desktop session. " +
			"Ask the user to run on their local machine, or fall back to non-interactive: " +
			fmt.Sprintf("agent-dd org add %s --api-key <key> --app-key <key>", alias)
	case dialog.CategoryRetry:
		hint = "User cancelled the dialog. Re-run agent-dd org add --form to retry."
	}

	return agenterrors.Wrap(err, categoryToFixableBy(cat)).WithHint(hint)
}

// categoryToFixableBy bridges dialog's neutral Category to agent-dd's FixableBy
// enum. The two are isomorphic; this is a one-line mapping.
func categoryToFixableBy(c dialog.Category) agenterrors.FixableBy {
	switch c {
	case dialog.CategoryHuman:
		return agenterrors.FixableByHuman
	case dialog.CategoryRetry:
		return agenterrors.FixableByRetry
	default:
		return agenterrors.FixableByAgent
	}
}
