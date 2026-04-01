package errors_test

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/shhac/agent-dd/internal/errors"
)

func TestNew(t *testing.T) {
	err := errors.New("something broke", errors.FixableByHuman)
	if err.Message != "something broke" {
		t.Errorf("Message = %q, want %q", err.Message, "something broke")
	}
	if err.FixableBy != errors.FixableByHuman {
		t.Errorf("FixableBy = %q, want %q", err.FixableBy, errors.FixableByHuman)
	}
	if err.Error() != "something broke" {
		t.Errorf("Error() = %q, want %q", err.Error(), "something broke")
	}
}

func TestNewf(t *testing.T) {
	err := errors.Newf(errors.FixableByAgent, "code %d: %s", 404, "not found")
	want := "code 404: not found"
	if err.Message != want {
		t.Errorf("Message = %q, want %q", err.Message, want)
	}
	if err.FixableBy != errors.FixableByAgent {
		t.Errorf("FixableBy = %q, want %q", err.FixableBy, errors.FixableByAgent)
	}
}

func TestWrap(t *testing.T) {
	orig := fmt.Errorf("underlying error")
	wrapped := errors.Wrap(orig, errors.FixableByRetry)

	if wrapped.Message != "underlying error" {
		t.Errorf("Message = %q, want %q", wrapped.Message, "underlying error")
	}
	if wrapped.Cause != orig {
		t.Error("Cause should be the original error")
	}
	if wrapped.FixableBy != errors.FixableByRetry {
		t.Errorf("FixableBy = %q, want %q", wrapped.FixableBy, errors.FixableByRetry)
	}
}

func TestWrapNil(t *testing.T) {
	if got := errors.Wrap(nil, errors.FixableByAgent); got != nil {
		t.Errorf("Wrap(nil) = %v, want nil", got)
	}
}

func TestWithHint(t *testing.T) {
	err := errors.New("bad request", errors.FixableByAgent).WithHint("check the query")
	if err.Hint != "check the query" {
		t.Errorf("Hint = %q, want %q", err.Hint, "check the query")
	}
	if err.Message != "bad request" {
		t.Errorf("Message = %q, want %q", err.Message, "bad request")
	}
}

func TestWithCause(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := errors.New("wrapped", errors.FixableByHuman).WithCause(cause)
	if err.Cause != cause {
		t.Error("Cause should be set by WithCause")
	}
	if err.Message != "wrapped" {
		t.Errorf("Message = %q, want %q", err.Message, "wrapped")
	}
}

func TestAPIErrorAs(t *testing.T) {
	cause := fmt.Errorf("root")
	apiErr := errors.Wrap(cause, errors.FixableByRetry)
	wrapped := fmt.Errorf("outer: %w", apiErr)

	var target *errors.APIError
	if !stderrors.As(wrapped, &target) {
		t.Fatal("errors.As should find *APIError through wrapping")
	}
	if target.Message != "root" {
		t.Errorf("Message = %q, want %q", target.Message, "root")
	}
}

func TestFixableByConstants(t *testing.T) {
	tests := []struct {
		name string
		val  errors.FixableBy
		want string
	}{
		{"FixableByAgent", errors.FixableByAgent, "agent"},
		{"FixableByHuman", errors.FixableByHuman, "human"},
		{"FixableByRetry", errors.FixableByRetry, "retry"},
	}
	for _, tt := range tests {
		if string(tt.val) != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.val, tt.want)
		}
	}
}
