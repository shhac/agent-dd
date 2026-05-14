// Package mockddtest provides test helpers that drive the mockdd handler
// from any test in the repo. The point is to make mockdd the single source
// of truth for Datadog wire shapes — when a test wants to assert "this
// fixture decodes through api.Client cleanly", it should reach for these
// helpers instead of hand-rolling another httptest.NewServer with bespoke
// response JSON.
//
// Bespoke handlers are still appropriate for tests that:
//   - Assert request shape (path, method, body) per request.
//   - Inject specific error responses (HTTP 5xx, malformed JSON, missing envelopes).
//   - Probe edge cases mockdd intentionally doesn't model.
//
// Anything else — happy-path decode coverage, end-to-end CLI rendering —
// should use this package so future shape drift in mockdd is caught
// uniformly across the codebase.
package mockddtest

import (
	"bytes"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/mockdd"
)

// NewServer boots the canonical mockdd handler behind an httptest.Server
// and registers t.Cleanup to tear it down at end-of-test.
func NewServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mockdd.NewHandler())
	t.Cleanup(srv.Close)
	return srv
}

// NewTestClient returns an api.Client wired against a fresh mockdd server.
// Use this when the test only cares about decoding mockdd's canonical
// responses through the real api types.
func NewTestClient(t *testing.T) *api.Client {
	t.Helper()
	srv := NewServer(t)
	return api.NewTestClient(srv.URL+"/api", "test-api-key", "test-app-key")
}

// InstallClientFactory wires the mockdd server into shared.ClientFactory so
// cobra commands invoked from tests pick up the mocked client. The factory
// is unwired automatically via t.Cleanup. Use this for CLI-rendering tests
// where the command resolves its own client via the global factory.
func InstallClientFactory(t *testing.T) *httptest.Server {
	t.Helper()
	srv := NewServer(t)
	shared.ClientFactory = func() (*api.Client, error) {
		return api.NewTestClient(srv.URL+"/api", "test-api-key", "test-app-key"), nil
	}
	t.Cleanup(func() { shared.ClientFactory = nil })
	return srv
}

// CaptureStdout swaps os.Stdout for the duration of fn and returns whatever
// was written. CLI tests use it to assert NDJSON output emitted by cobra
// commands.
func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}
