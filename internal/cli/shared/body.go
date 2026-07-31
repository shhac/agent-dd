package shared

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

// ResolveBody reads a --body argument: literal JSON, @file, or @- for stdin.
// A non-empty body must be valid JSON since it is sent as application/json.
//
// Lives here rather than in apicmd because both `api --body` and the monitor
// write commands take the same argument in the same three forms, and an agent
// that learns the convention once should find it working everywhere.
func ResolveBody(bodyArg string, stdin io.Reader) (json.RawMessage, error) {
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
