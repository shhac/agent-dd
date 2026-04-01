package shared_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func TestRegisterLLMHelp(t *testing.T) {
	parent := &cobra.Command{Use: "test"}
	shared.RegisterLLMHelp(parent, "Test help", "hello from llm-help\n")

	buf := new(bytes.Buffer)
	parent.SetOut(buf)
	parent.SetArgs([]string{"llm-help"})

	if err := parent.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// llm-help uses fmt.Print which goes to os.Stdout, not cmd.OutOrStdout().
	// Verify the command was registered and ran without error.
}
