package shared

import (
	"fmt"

	"github.com/spf13/cobra"
)

// RegisterLLMHelp adds a llm-help subcommand that prints domain-specific reference text.
func RegisterLLMHelp(parent *cobra.Command, shortDesc, helpText string) {
	parent.AddCommand(&cobra.Command{
		Use:   "llm-help",
		Short: shortDesc,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(helpText)
		},
	})
}
