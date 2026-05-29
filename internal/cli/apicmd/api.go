// Package apicmd implements `agent-dd api` — a raw Datadog API escape hatch.
//
// It exists for the endpoints the typed commands don't wrap yet (server-side
// span/log percentile aggregation, facet discovery, anything new) without
// asking callers to hand-roll auth. It reuses the exact credential/site
// resolution and error classification of every other command, so the secret
// keys are never exposed and HTTP failures still arrive as the familiar
// {error, fixable_by, hint} contract. It is deliberately a power tool, not the
// primary interface: mutating requests require an explicit --allow-write.
package apicmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/cli/shared"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	var bodyArg string
	var queryParams []string
	var allowWrite, printRequest bool

	cmd := &cobra.Command{
		Use:   "api [METHOD] PATH",
		Short: "Make a raw Datadog API request (escape hatch for unwrapped endpoints)",
		Long: `Make a raw Datadog API request using the resolved org credentials.

PATH is relative to the site's /api base, e.g. /v2/spans/analytics/aggregate.
With one positional arg the method defaults to GET; pass two to be explicit.

Reads (GET/HEAD, and POST to /search or /aggregate paths) run by default.
Mutating requests (PUT/PATCH/DELETE, or any other POST) require --allow-write.

Examples:
  agent-dd api /v2/apm/services
  agent-dd api POST /v2/spans/analytics/aggregate --body @agg.json
  agent-dd api DELETE /v1/monitor/123 --allow-write
  agent-dd api POST /v2/incidents --body @inc.json --print-request`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := globals()

			method, path := parseMethodPath(args)
			path, err := buildPath(path, queryParams)
			if err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}

			body, err := resolveBody(bodyArg, cmd.InOrStdin())
			if err != nil {
				output.WriteError(os.Stderr, err)
				return nil
			}

			// --print-request never sends, so it may safely preview a mutating
			// request without --allow-write.
			if isWriteRequest(method, path) && !allowWrite && !printRequest {
				output.WriteError(os.Stderr, agenterrors.Newf(agenterrors.FixableByAgent,
					"%s %s looks like a mutating request", method, path).
					WithHint("Re-run with --allow-write if you intend to change Datadog state"))
				return nil
			}

			return shared.WithClient(g.Org, g.Timeout, func(ctx context.Context, client *api.Client) error {
				if printRequest {
					output.Print(client.PreviewRequest(method, path, body), resolveFormat(g.Format), false)
					return nil
				}

				raw, err := client.RawRequest(ctx, method, path, bodyOrNil(body))
				if err != nil {
					return err
				}
				writeRaw(raw, resolveFormat(g.Format))
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&bodyArg, "body", "", "Request body: inline JSON, @file, or @- for stdin")
	cmd.Flags().StringArrayVar(&queryParams, "query", nil, "Query param as key=value (repeatable)")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false, "Permit mutating requests (PUT/PATCH/DELETE or non-search POST)")
	cmd.Flags().BoolVar(&printRequest, "print-request", false, "Print the request that would be sent (credentials redacted) without sending it")
	root.AddCommand(cmd)
}

// parseMethodPath interprets the positional args: one arg is a GET of that
// path; two args are an explicit METHOD PATH. The method is upper-cased.
func parseMethodPath(args []string) (method, path string) {
	if len(args) == 2 {
		return strings.ToUpper(args[0]), args[1]
	}
	return "GET", args[0]
}

// buildPath normalizes the path to be relative to the client's /api base and
// appends any --query params. A leading /api is stripped (the base already
// ends in /api), so both "/v2/x" and "/api/v2/x" resolve identically.
func buildPath(path string, params []string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimPrefix(path, "/api")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if len(params) == 0 {
		return path, nil
	}

	base, existing, _ := strings.Cut(path, "?")
	values, err := url.ParseQuery(existing)
	if err != nil {
		return "", agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return "", agenterrors.Newf(agenterrors.FixableByAgent, "invalid --query %q, expected key=value", p)
		}
		values.Add(k, v)
	}
	return base + "?" + values.Encode(), nil
}

// resolveBody reads the --body argument: literal JSON, @file, or @- for stdin.
// A non-empty body must be valid JSON since it is sent as application/json.
func resolveBody(bodyArg string, stdin io.Reader) (json.RawMessage, error) {
	if bodyArg == "" {
		return nil, nil
	}

	raw := []byte(bodyArg)
	if strings.HasPrefix(bodyArg, "@") {
		src := strings.TrimPrefix(bodyArg, "@")
		var data []byte
		var err error
		if src == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(src)
		}
		if err != nil {
			return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent).
				WithHint("Check the --body file path (use @- to read JSON from stdin)")
		}
		raw = data
	}

	if !json.Valid(raw) {
		return nil, agenterrors.New("--body is not valid JSON", agenterrors.FixableByAgent).
			WithHint("Pass a JSON object/array; the body is sent as application/json")
	}
	return json.RawMessage(raw), nil
}

// isWriteRequest classifies a request as mutating. GET/HEAD are always reads;
// POST is a read only for the search/aggregate endpoints Datadog models as
// POST; everything else (PUT/PATCH/DELETE, other POSTs) is a write.
func isWriteRequest(method, path string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return false
	case "POST":
		lower := strings.ToLower(path)
		return !strings.Contains(lower, "search") && !strings.Contains(lower, "aggregate")
	default:
		return true
	}
}

// bodyOrNil avoids sending an empty (nil) RawMessage as a body.
func bodyOrNil(body json.RawMessage) any {
	if len(body) == 0 {
		return nil
	}
	return body
}

func resolveFormat(flagFormat string) output.Format {
	return output.ResolveFormat(flagFormat, output.FormatJSON)
}

// writeRaw prints the response faithfully — no null-pruning, since this is a
// raw passthrough. Valid JSON is re-rendered in the requested format; anything
// else is written through untouched.
func writeRaw(raw json.RawMessage, format output.Format) {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		fmt.Fprintln(os.Stdout, string(raw))
		return
	}
	output.Print(decoded, format, false)
}
