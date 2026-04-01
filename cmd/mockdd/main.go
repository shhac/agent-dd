package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/shhac/agent-dd/internal/mockdd"
)

func main() {
	port := "8321"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	fmt.Fprintf(os.Stderr, "Mock Datadog API running on http://localhost:%s\n\n", port)
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  export DD_API_URL=http://localhost:%s/api\n", port)
	fmt.Fprintf(os.Stderr, "  export DD_API_KEY=mock\n")
	fmt.Fprintf(os.Stderr, "  export DD_APP_KEY=mock\n\n")
	fmt.Fprintf(os.Stderr, "  agent-dd monitors list --status alert\n")
	fmt.Fprintf(os.Stderr, "  agent-dd logs search --query \"status:error\" --from now-1h\n")

	if err := http.ListenAndServe(":"+port, mockdd.NewHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
