package shared

import (
	"context"
	"errors"
	"fmt"
	"os"
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
		return nil
	}

	if err := fn(ctx, client); err != nil {
		output.WriteError(os.Stderr, err)
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
	f := output.ResolveFormat(format)
	if f == output.FormatNDJSON {
		w := output.NewNDJSONWriter(os.Stdout)
		for _, item := range items {
			w.WriteItem(item)
		}
		if pagination != nil {
			w.WritePagination(pagination)
		}
		return
	}
	result := map[string]any{"data": items}
	if pagination != nil {
		result["pagination"] = pagination
	}
	output.PrintJSON(result, true)
}

type GlobalsFunc = func() *GlobalFlags
