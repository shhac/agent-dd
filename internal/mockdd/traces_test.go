package mockdd_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/mockdd"
)

// Drives SearchTraces against the real mockdd handler so the v2 envelope
// and `error` object decoding paths are covered end-to-end. Catches drift
// between the fixture shape mockdd emits and the structs api expects to
// decode.
func TestMockddTraceSearchDecodesV2ErrorObject(t *testing.T) {
	srv := httptest.NewServer(mockdd.NewHandler())
	t.Cleanup(srv.Close)

	client := api.NewTestClient(srv.URL+"/api", "test-api-key", "test-app-key")
	resp, err := client.SearchTraces(context.Background(), "*", "", "now-1h", "now", 50, "")
	if err != nil {
		t.Fatalf("SearchTraces: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected mockdd to return spans, got 0")
	}

	var sawError, sawOK bool
	for _, d := range resp.Data {
		attrs := d.Attributes
		if attrs.OperationName == "" {
			t.Errorf("span missing operation_name: %+v", attrs)
		}
		if attrs.StartTimestamp == "" {
			t.Errorf("span missing start_timestamp: %+v", attrs)
		}
		if attrs.EndTimestamp == "" {
			t.Errorf("span missing end_timestamp: %+v", attrs)
		}
		if attrs.Env == "" {
			t.Errorf("span missing env: %+v", attrs)
		}
		if len(attrs.Tags) == 0 {
			t.Errorf("span missing tags: %+v", attrs)
		}

		switch attrs.Status {
		case "error":
			sawError = true
			if attrs.Error == nil {
				t.Errorf("error-status span has nil SpanError: %+v", attrs)
				continue
			}
			if attrs.Error.Message == "" || attrs.Error.Type == "" {
				t.Errorf("error span missing detail fields: %+v", attrs.Error)
			}
		case "ok":
			sawOK = true
			if attrs.Error != nil {
				t.Errorf("ok-status span has non-nil SpanError: %+v", attrs.Error)
			}
		}
	}
	if !sawError {
		t.Error("mockdd produced no error-status spans; coverage gap for v2 error object decoding")
	}
	if !sawOK {
		t.Error("mockdd produced no ok-status spans; coverage gap for absent error field")
	}
}
