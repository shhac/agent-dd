package main

import (
	"github.com/shhac/agent-dd/internal/cli"
)

var version = "dev"

func main() {
	cli.Run(version)
}
