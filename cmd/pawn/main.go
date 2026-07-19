package main

import (
	"context"
	"os"

	"github.com/pawnkit/pawnkit-cli/pkg/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, version))
}
