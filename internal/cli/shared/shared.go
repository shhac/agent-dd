package shared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shhac/agent-dd/internal/api"
	"github.com/shhac/agent-dd/internal/config"
	"github.com/shhac/agent-dd/internal/credential"
	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

type GlobalFlags struct {
	Org     string
	Format  string
	Timeout int
}

func MakeContext(timeoutMs int) (context.Context, context.CancelFunc) {
	if timeoutMs > 0 {
		return context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	}
	return context.WithCancel(context.Background())
}

func ResolveOrg(orgAlias string) (string, error) {
	if orgAlias != "" {
		return orgAlias, nil
	}
	if env := os.Getenv("DD_ORG"); env != "" {
		return env, nil
	}
	cfg := config.Read()
	if cfg.DefaultOrg != "" {
		return cfg.DefaultOrg, nil
	}
	available := make([]string, 0)
	for name := range cfg.Organizations {
		available = append(available, name)
	}
	hint := "No organizations configured. Add one with 'agent-dd org add <alias>'"
	if len(available) > 0 {
		hint = fmt.Sprintf("Available organizations: %s. Set a default with 'agent-dd org set-default <alias>'", strings.Join(available, ", "))
	}
	return "", agenterrors.New("no organization specified", agenterrors.FixableByAgent).WithHint(hint)
}

func NewClientFromOrg(orgAlias string) (*api.Client, error) {
	if apiURL := os.Getenv("DD_API_URL"); apiURL != "" {
		return api.NewTestClient(apiURL, os.Getenv("DD_API_KEY"), os.Getenv("DD_APP_KEY")), nil
	}

	if apiKey, appKey := os.Getenv("DD_API_KEY"), os.Getenv("DD_APP_KEY"); orgAlias == "" && apiKey != "" && appKey != "" {
		site := os.Getenv("DD_SITE")
		if site == "" {
			site = "datadoghq.com"
		}
		return api.NewClient(apiKey, appKey, site), nil
	}

	alias, err := ResolveOrg(orgAlias)
	if err != nil {
		return nil, err
	}

	cred, err := credential.Get(alias)
	if err != nil {
		var nf *credential.NotFoundError
		if errors.As(err, &nf) {
			return nil, agenterrors.Newf(agenterrors.FixableByHuman, "credentials for organization %q not found", alias).
				WithHint("Add credentials with 'agent-dd org add " + alias + " --api-key <key> --app-key <key>'")
		}
		return nil, agenterrors.Wrap(err, agenterrors.FixableByHuman)
	}

	if cred.APIKey == "" {
		return nil, agenterrors.Newf(agenterrors.FixableByHuman, "organization %q has no API key", alias).
			WithHint("Update with 'agent-dd org update " + alias + " --api-key <key>'")
	}

	cfg := config.Read()
	site := "datadoghq.com"
	if org, ok := cfg.Organizations[alias]; ok && org.Site != "" {
		site = org.Site
	}

	return api.NewClient(cred.APIKey, cred.AppKey, site), nil
}

var ClientFactory func() (*api.Client, error)

func WithClient(orgAlias string, timeout int, fn func(ctx context.Context, client *api.Client) error) error {
	ctx, cancel := MakeContext(timeout)
	defer cancel()

	var client *api.Client
	var err error
	if ClientFactory != nil {
		client, err = ClientFactory()
	} else {
		client, err = NewClientFromOrg(orgAlias)
	}
	if err != nil {
		output.WriteError(os.Stderr, err)
		return err
	}

	if err := fn(ctx, client); err != nil {
		output.WriteError(os.Stderr, err)
		return err
	}
	return nil
}

func ToAnySlice[T any](s []T) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

func WritePaginatedList(items []any, pagination *output.Pagination, format string) {
	WritePaginatedListWithMeta(items, pagination, nil, format)
}

// WritePaginatedListWithMeta writes a paginated list plus optional meta
// rollups. In NDJSON mode each meta key is emitted as its own line before
// the data rows; the convention is to use an `@`-prefix on the key (e.g.
// "@counts") so consumers can filter the row from the data stream. In JSON
// mode the meta keys are merged into the top-level envelope alongside `data`.
func WritePaginatedListWithMeta(items []any, pagination *output.Pagination, meta map[string]any, format string) {
	f := output.ResolveFormat(format, output.FormatNDJSON)
	if f == output.FormatNDJSON {
		w := output.NewNDJSONWriter(os.Stdout)
		for key, val := range meta {
			_ = w.WriteMetaLine(key, val)
		}
		for _, item := range items {
			_ = w.WriteItem(item)
		}
		if pagination != nil {
			_ = w.WritePagination(pagination)
		}
		return
	}
	result := map[string]any{"data": items}
	for k, v := range meta {
		result[k] = v
	}
	if pagination != nil {
		result["pagination"] = pagination
	}
	output.Print(result, f, true)
}

// WriteItem writes a single item in the resolved format (default: JSON).
func WriteItem(data any, format string) {
	f := output.ResolveFormat(format, output.FormatJSON)
	output.Print(data, f, true)
}

// CursorPagination builds pagination metadata from a cursor string.
// Returns nil if cursor is empty (no more pages).
func CursorPagination(cursor string) *output.Pagination {
	if cursor == "" {
		return nil
	}
	return &output.Pagination{HasMore: true, NextCursor: cursor}
}

// RequireFlag checks that a flag value is non-empty, writing an error to stderr if not.
// Returns true if the value is present, false if missing (error already written).
func RequireFlag(flag, value, hint string) bool {
	if value != "" {
		return true
	}
	err := agenterrors.Newf(agenterrors.FixableByAgent, "--%s is required", flag)
	if hint != "" {
		err = err.WithHint(hint)
	}
	output.WriteError(os.Stderr, err)
	return false
}

// SingleTag converts a single tag string to a slice, returning nil if empty.
func SingleTag(tag string) []string {
	if tag == "" {
		return nil
	}
	return []string{tag}
}

// ParseIntArg parses an integer from a command argument, writing an error to stderr on failure.
// Returns the parsed value and true on success, or 0 and false on failure (error already written).
func ParseIntArg(noun, value string) (int, bool) {
	id, err := strconv.Atoi(value)
	if err != nil {
		output.WriteError(os.Stderr, agenterrors.Newf(agenterrors.FixableByAgent, "invalid %s %q — must be an integer", noun, value))
		return 0, false
	}
	return id, true
}

type GlobalsFunc = func() *GlobalFlags
