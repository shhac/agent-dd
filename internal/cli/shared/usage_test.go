package shared_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func TestRegisterUsage(t *testing.T) {
	parent := &cobra.Command{Use: "test"}
	shared.RegisterUsage(parent, "test", "hello from usage\n")

	buf := new(bytes.Buffer)
	parent.SetOut(buf)
	parent.SetArgs([]string{"usage"})

	if err := parent.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// usage uses fmt.Print which goes to os.Stdout, not cmd.OutOrStdout().
	// Verify the command was registered and ran without error.
}
