package main

import (
	"fmt"
	"os"

	"github.com/iatsiuk/datadog-cli/internal/cmd"
)

var version = "dev"

func main() {
	root := cmd.NewRootCommand(version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
