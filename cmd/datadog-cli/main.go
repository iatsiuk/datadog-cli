package main

import (
	"os"

	"github.com/iatsiuk/datadog-cli/internal/cmd"
)

var version = "dev"

func main() {
	root := cmd.NewRootCommand(version)
	if err := root.Execute(); err != nil {
		// pre-scan os.Args so --json is honoured even when cobra errors out
		// before it parses the flag (e.g. unknown flag preceding --json)
		asJSON := cmd.ParseJSONFlag(os.Args[1:])
		cmd.WriteError(os.Stderr, err, asJSON)
		os.Exit(1)
	}
}
